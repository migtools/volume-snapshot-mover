package controllers

import (
	"context"
	"errors"
	"fmt"
	"strings"

	k8serrors "k8s.io/apimachinery/pkg/api/errors"

	"github.com/go-logr/logr"
	volsnapmoverv1alpha1 "github.com/konveyor/volume-snapshot-mover/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// Restic secret data keys
const (

	// VolumeSnapshotMover annotation keys
	SnapMoverResticRepository = "datamover.io/restic-repository"
	SnapMoverSourcePVCName    = "datamover.io/source-pvc-name"
	SnapMoverSourcePVCSize    = "datamover.io/source-pvc-size"
)

type RetainPolicy struct {
	daily   string
	weekly  string
	hourly  string
	monthly string
	yearly  string
	within  string
}

func (r *VolumeSnapshotBackupReconciler) CreateVSBResticSecret(log logr.Logger) (bool, error) {
	// get volumesnapshotbackup from cluster
	vsb := volsnapmoverv1alpha1.VolumeSnapshotBackup{}
	if err := r.Get(r.Context, r.req.NamespacedName, &vsb); err != nil {
		// ignore is not found error
		if k8serrors.IsNotFound(err) {
			return true, nil
		}
		r.Log.Error(err, fmt.Sprintf("unable to fetch volumesnapshotbackup %s", r.req.NamespacedName))
		return false, err
	}

	// no need to perform CreateVSBResticSecret step for the vsb if the datamovement has already completed
	if len(vsb.Status.Phase) > 0 && vsb.Status.Phase == volsnapmoverv1alpha1.SnapMoverBackupPhaseCompleted {
		r.Log.Info(fmt.Sprintf("skipping CreateVSBResticSecret step for vsb %s/%s as datamovement is complete", vsb.Namespace, vsb.Name))
		return true, nil
	}

	// get cloned pvc
	pvcName := fmt.Sprintf("%s-pvc", vsb.Spec.VolumeSnapshotContent.Name)
	pvc := corev1.PersistentVolumeClaim{}
	if err := r.Get(r.Context, types.NamespacedName{Name: pvcName, Namespace: vsb.Spec.ProtectedNamespace}, &pvc); err != nil {
		r.Log.Error(err, fmt.Sprintf("unable to fetch PVC %s/%s", vsb.Spec.ProtectedNamespace, pvcName))
		return false, err
	}

	// get restic secret name
	credName := vsb.Spec.ResticSecretRef.Name
	if credName == "" {
		return false, errors.New(fmt.Sprintf("restic secret name cannot be empty for vsb %s", r.req.NamespacedName))
	}

	// get restic secret from user
	resticSecret := corev1.Secret{}
	if err := r.Get(r.Context, types.NamespacedName{Namespace: r.NamespacedName.Namespace, Name: credName}, &resticSecret); err != nil {
		r.Log.Error(err, fmt.Sprintf("unable to fetch restic  secret %s/%s", r.NamespacedName.Namespace, credName))
		return false, err
	}

	err := ValidateResticSecret(&resticSecret)
	if err != nil {
		r.Log.Error(err, fmt.Sprintf("restic secret %s/%s is malformed", r.NamespacedName.Namespace, credName))
		return false, err
	}

	var pruneInterval = ""
	var scheduleCronExpr = ""
	rpolicy := RetainPolicy{}
	for key, val := range resticSecret.Data {
		stringVal := string(val)
		if key == ResticRepository {
			// if trailing '/' in user-created Restic repo, remove it
			ResticRepoValue = strings.TrimRight(stringVal, "/")
		}
		if key == ResticPruneInterval {
			pruneInterval = stringVal
		}
		if key == SnapshotScheduleCron {
			scheduleCronExpr = stringVal
		}
		if key == SnapshotRetainPolicyMonthly {
			rpolicy.monthly = stringVal
		}
		if key == SnapshotRetainPolicyDaily {
			rpolicy.daily = stringVal
		}
		if key == SnapshotRetainPolicyHourly {
			rpolicy.hourly = stringVal
		}
		if key == SnapshotRetainPolicyWeekly {
			rpolicy.weekly = stringVal
		}
		if key == SnapshotRetainPolicyYearly {
			rpolicy.yearly = stringVal
		}
		if key == SnapshotRetainPolicyWithin {
			rpolicy.within = stringVal
		}
	}

	// TODO should check if label is present first? or it is always present?
	resticrepo := fmt.Sprintf("%s/%s/%s/%s", ResticRepoValue, vsb.Labels["velero.io/backup-name"], pvc.Namespace, pvc.Name)

	rsecret, err := PopulateResticSecret(vsb.Name, vsb.Spec.ProtectedNamespace, VSBLabel)
	if err != nil {
		return false, err
	}

	// Create Restic secret in OADP namespace
	op, err := controllerutil.CreateOrUpdate(r.Context, r.Client, rsecret, func() error {

		return BuildResticSecret(&resticSecret, rsecret, resticrepo, pruneInterval, &rpolicy, scheduleCronExpr)
	})
	if err != nil {
		return false, err
	}

	// set created Restic repo to VSB status
	vsb.Status.ResticRepository = string(rsecret.Data[ResticRepository])

	// Update VSB status
	err = r.Status().Update(context.Background(), &vsb)
	if err != nil {
		return false, err
	}

	if op == controllerutil.OperationResultCreated || op == controllerutil.OperationResultUpdated {
		r.EventRecorder.Event(rsecret,
			corev1.EventTypeNormal,
			"ResticSecretBackupReconciled",
			fmt.Sprintf("%s restic secret %s", op, rsecret.Name),
		)
	}
	return true, nil
}

func (r *VolumeSnapshotRestoreReconciler) CreateVSRResticSecret(log logr.Logger) (bool, error) {
	// get volumesnapshotrestore from cluster
	vsr := volsnapmoverv1alpha1.VolumeSnapshotRestore{}
	if err := r.Get(r.Context, r.req.NamespacedName, &vsr); err != nil {
		// ignore is not found error
		if k8serrors.IsNotFound(err) {
			return true, nil
		}
		r.Log.Error(err, fmt.Sprintf("unable to fetch volumesnapshotrestore %s", r.req.NamespacedName))
		return false, err
	}

	// get restic secret name
	credName := vsr.Spec.ResticSecretRef.Name
	if credName == "" {
		err := r.updateVSRStatusPhase(nil, volsnapmoverv1alpha1.SnapMoverRestorePhaseFailed, r.Client)
		if err != nil {
			return false, err
		}

		r.Log.Info(fmt.Sprintf("marking volumesnapshotrestore %s as failed", r.req.NamespacedName))
		return false, errors.New("restic secret name cannot be empty")
	}
	// get restic secret from user
	resticSecret := corev1.Secret{}
	if err := r.Get(r.Context, types.NamespacedName{Namespace: r.NamespacedName.Namespace, Name: credName}, &resticSecret); err != nil {
		r.Log.Error(err, fmt.Sprintf("unable to fetch restic  secret %s/%s", r.NamespacedName.Namespace, credName))
		return false, err
	}

	err := ValidateResticSecret(&resticSecret)
	if err != nil {
		r.Log.Error(err, fmt.Sprintf("restic secret %s/%s is malformed", r.NamespacedName.Namespace, credName))
		return false, err
	}
	// define Restic secret to be created
	newResticSecret, err := PopulateResticSecret(vsr.Name, vsr.Spec.ProtectedNamespace, VSRLabel)
	if err != nil {
		return false, err
	}
	resticrepo := vsr.Spec.VolumeSnapshotMoverBackupref.ResticRepository

	var rpolicy = RetainPolicy{}
	// Create Restic secret in OADP namespace
	op, err := controllerutil.CreateOrUpdate(r.Context, r.Client, newResticSecret, func() error {

		return BuildResticSecret(&resticSecret, newResticSecret, resticrepo, "", &rpolicy, "")
	})
	if err != nil {
		return false, err
	}

	if op == controllerutil.OperationResultCreated || op == controllerutil.OperationResultUpdated {
		r.EventRecorder.Event(newResticSecret,
			corev1.EventTypeNormal,
			"ResticSecretRestoreReconciled",
			fmt.Sprintf("%s restic secret %s", op, newResticSecret.Name),
		)
	}
	return true, nil
}
