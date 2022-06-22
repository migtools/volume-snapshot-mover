package controllers

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	datamoverv1alpha1 "github.com/konveyor/volume-snapshot-mover/api/v1alpha1"
	resticcommon "github.com/konveyor/volume-snapshot-mover/pkg"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// Restic secret data keys
const (
	// AWS vars
	AWSAccessKey     = "AWS_ACCESS_KEY_ID"
	AWSSecretKey     = "AWS_SECRET_ACCESS_KEY"
	AWSDefaultRegion = "AWS_DEFAULT_REGION"

	// TODO: GCP and Azure

	// Restic repo vars
	ResticPassword   = "RESTIC_PASSWORD"
	ResticRepository = "RESTIC_REPOSITORY"

	// VolumeSnapshotMover annotation keys
	SnapMoverResticRepository = "datamover.io/restic-repository"
	SnapMoverSourcePVCName    = "datamover.io/source-pvc-name"
	SnapMoverSourcePVCSize    = "datamover.io/source-pvc-size"
)

// Restic secret vars to create new secrets
var (
	AWSAccessValue        []byte
	AWSSecretValue        []byte
	AWSDefaultRegionValue []byte

	// TODO: GCP and Azure

	ResticPasswordValue []byte
	ResticRepoValue     string
)

const (
	resticSecretName = "restic-secret"
)

func (r *VolumeSnapshotBackupReconciler) CreateVSBResticSecret(log logr.Logger) (bool, error) {
	// get volumesnapshotbackup from cluster
	vsb := datamoverv1alpha1.VolumeSnapshotBackup{}
	if err := r.Get(r.Context, r.req.NamespacedName, &vsb); err != nil {
		r.Log.Error(err, "unable to fetch VolumeSnapshotBackup CR")
		return false, err
	}

	// get cloned pvc
	pvcName := fmt.Sprintf("%s-pvc", vsb.Spec.VolumeSnapshotContent.Name)
	pvc := corev1.PersistentVolumeClaim{}
	if err := r.Get(r.Context, types.NamespacedName{Name: pvcName, Namespace: vsb.Spec.ProtectedNamespace}, &pvc); err != nil {
		r.Log.Error(err, "unable to fetch PVC")
		return false, err
	}
	// get restic secret from user
	resticSecret := corev1.Secret{}
	if err := r.Get(r.Context, types.NamespacedName{Namespace: r.NamespacedName.Namespace, Name: resticSecretName}, &resticSecret); err != nil {
		r.Log.Error(err, "unable to fetch Restic Secret")
		return false, err
	}

	rsecret, err := resticcommon.PopulateResticSecret(&vsb, nil)
	if err != nil {
		return false, err
	}

	// Create Restic secret in OADP namespace
	op, err := controllerutil.CreateOrUpdate(r.Context, r.Client, rsecret, func() error {

		return resticcommon.BuildVSBResticSecret(&resticSecret, rsecret, &pvc)
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
	vsr := datamoverv1alpha1.VolumeSnapshotRestore{}
	if err := r.Get(r.Context, r.req.NamespacedName, &vsr); err != nil {
		r.Log.Error(err, "unable to fetch VolumeSnapshotRestore CR")
		return false, err
	}
	// get restic secret from user
	resticSecret := corev1.Secret{}
	if err := r.Get(r.Context, types.NamespacedName{Namespace: r.NamespacedName.Namespace, Name: resticSecretName}, &resticSecret); err != nil {
		r.Log.Error(err, "unable to fetch Restic Secret")
		return false, err
	}

	// define Restic secret to be created
	newResticSecret, err := resticcommon.PopulateResticSecret(nil, &vsr)
	if err != nil {
		return false, err
	}

	// Create Restic secret in OADP namespace
	op, err := controllerutil.CreateOrUpdate(r.Context, r.Client, newResticSecret, func() error {

		return resticcommon.BuildVSRResticSecret(&resticSecret, newResticSecret, &vsr)
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
