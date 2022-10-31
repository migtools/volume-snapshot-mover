package controllers

import (
	"context"
	"errors"
	"fmt"

	k8serrors "k8s.io/apimachinery/pkg/api/errors"

	volsyncv1alpha1 "github.com/backube/volsync/api/v1alpha1"
	"github.com/go-logr/logr"
	volsnapmoverv1alpha1 "github.com/konveyor/volume-snapshot-mover/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	k8serror "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func (r *VolumeSnapshotRestoreReconciler) CreateReplicationDestination(log logr.Logger) (bool, error) {

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

	// define replicationDestination to be created
	repDestination := &volsyncv1alpha1.ReplicationDestination{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-rep-dest", vsr.Name),
			Namespace: r.NamespacedName.Namespace,
			Labels: map[string]string{
				VSRLabel: vsr.Name,
			},
		},
	}

	// Create ReplicationDestination in protected namespace
	op, err := controllerutil.CreateOrUpdate(r.Context, r.Client, repDestination, func() error {

		return r.buildReplicationDestination(repDestination, &vsr)
	})
	if err != nil {
		return false, err
	}

	if op == controllerutil.OperationResultCreated || op == controllerutil.OperationResultUpdated {
		r.EventRecorder.Event(repDestination,
			corev1.EventTypeNormal,
			"ReplicationDestinationReconciled",
			fmt.Sprintf("%s replicationdestination %s", op, repDestination.Name),
		)
	}
	return true, nil
}

func (r *VolumeSnapshotRestoreReconciler) buildReplicationDestination(replicationDestination *volsyncv1alpha1.ReplicationDestination, vsr *volsnapmoverv1alpha1.VolumeSnapshotRestore) error {

	// get restic secret created by controller
	dmresticSecretName := fmt.Sprintf("%s-secret", vsr.Name)
	resticSecret := corev1.Secret{}
	if err := r.Get(r.Context, types.NamespacedName{Namespace: r.NamespacedName.Namespace, Name: dmresticSecretName}, &resticSecret); err != nil {
		r.Log.Error(err, fmt.Sprintf("unable to fetch restic secret %s/%s", r.NamespacedName.Namespace, dmresticSecretName))
		return err
	}

	stringCapacity := vsr.Spec.VolumeSnapshotMoverBackupref.BackedUpPVCData.Size
	capacity := resource.MustParse(stringCapacity)
	// build ReplicationDestination
	replicationDestinationSpec := volsyncv1alpha1.ReplicationDestinationSpec{
		Trigger: &volsyncv1alpha1.ReplicationDestinationTriggerSpec{
			Manual: fmt.Sprintf("%s-trigger", vsr.Name),
		},
		Restic: &volsyncv1alpha1.ReplicationDestinationResticSpec{
			// TODO: create restic secret from secret from VSB CR status
			Repository: resticSecret.Name,
			ReplicationDestinationVolumeOptions: volsyncv1alpha1.ReplicationDestinationVolumeOptions{
				AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
				CopyMethod:  volsyncv1alpha1.CopyMethodSnapshot,
				// let replicationDestination create PVC
				Capacity:                &capacity,
				StorageClassName:        &vsr.Spec.VolumeSnapshotMoverBackupref.BackedUpPVCData.StorageClassName,
				VolumeSnapshotClassName: &vsr.Spec.VolumeSnapshotMoverBackupref.VolumeSnapshotClassName,
			},
		},
	}
	if replicationDestination.CreationTimestamp.IsZero() {
		replicationDestination.Spec = replicationDestinationSpec
	}

	return nil
}

func (r *VolumeSnapshotRestoreReconciler) SetVSRStatus(log logr.Logger) (bool, error) {

	vsr := volsnapmoverv1alpha1.VolumeSnapshotRestore{}
	if err := r.Get(r.Context, r.req.NamespacedName, &vsr); err != nil {
		// ignore is not found error
		if k8serrors.IsNotFound(err) {
			return true, nil
		}
		r.Log.Error(err, fmt.Sprintf("unable to fetch volumesnapshotrestore %s", r.req.NamespacedName))
		return false, err
	}

	//update vsr status from restore
	err := updateVSRFromRestore(&vsr, r.Client, log)
	if err != nil {
		return false, err
	}

	if vsr.Status.Phase == volsnapmoverv1alpha1.SnapMoverRestorePhaseFailed || vsr.Status.Phase == volsnapmoverv1alpha1.SnapMoverRestorePhasePartiallyFailed {
		return false, errors.New("vsr failed to complete")
	}

	repDestName := fmt.Sprintf("%s-rep-dest", vsr.Name)
	repDest := volsyncv1alpha1.ReplicationDestination{}
	if err := r.Get(r.Context, types.NamespacedName{Namespace: vsr.Spec.ProtectedNamespace, Name: repDestName}, &repDest); err != nil {
		if k8serror.IsNotFound(err) {
			return false, nil
		}
		r.Log.Info(fmt.Sprintf("error getting replicationdestination %s/%s", vsr.Spec.ProtectedNamespace, repDestName))
		return false, err
	}

	if repDest.Status == nil {
		r.Log.Info(fmt.Sprintf("replication destination %s/%s is yet to have a status", vsr.Spec.ProtectedNamespace, repDest))
		return false, nil
	}

	if repDest.Status != nil {
		reconConditionProgress := metav1.Condition{}

		// save replicationDestination status conditions
		for i := range repDest.Status.Conditions {
			if repDest.Status.Conditions[i].Reason == volsyncv1alpha1.SynchronizingReasonSync {
				reconConditionProgress = repDest.Status.Conditions[i]
			}
		}

		// for manual trigger, if spec.trigger.manual == status.lastManualSync, sync has completed
		// VSR is completed
		if len(repDest.Status.LastManualSync) > 0 && len(repDest.Spec.Trigger.Manual) > 0 {
			sourceStatus := repDest.Status.LastManualSync
			sourceSpec := repDest.Spec.Trigger.Manual
			if sourceStatus == sourceSpec {

				vsr.Status.Phase = volsnapmoverv1alpha1.SnapMoverRestoreVolSyncPhaseCompleted

				// Update VSR status as completed
				err := r.Status().Update(context.Background(), &vsr)
				if err != nil {
					return false, err
				}
				r.Log.Info(fmt.Sprintf("marking volumesnapshotrestore %s as completed", r.req.NamespacedName))
				return true, nil
			}

			// VSR is in progress
		} else if reconConditionProgress.Status == metav1.ConditionTrue && reconConditionProgress.Reason == volsyncv1alpha1.SynchronizingReasonSync {

			vsr.Status.Phase = volsnapmoverv1alpha1.SnapMoverRestorePhaseInProgress
			err := r.Status().Update(context.Background(), &vsr)
			if err != nil {
				return false, err
			}
			r.Log.Info(fmt.Sprintf("marking volumesnapshotrestore %s as in progress", r.req.NamespacedName))
			return false, nil

			// if not in progress or completed, phase failed
		} else {
			vsr.Status.Phase = volsnapmoverv1alpha1.SnapMoverRestorePhaseFailed

			err := r.Status().Update(context.Background(), &vsr)
			if err != nil {
				return false, err
			}
			r.Log.Info(fmt.Sprintf("marking volumesnapshotrestore %s as failed", r.req.NamespacedName))
			return false, nil
		}
	}

	r.Log.Info(fmt.Sprintf("waiting for replicationdestination %s/%s to complete", vsr.Spec.ProtectedNamespace, repDestName))
	return false, nil
}
