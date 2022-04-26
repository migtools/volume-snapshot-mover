package controllers

import (
	"context"
	"errors"
	"fmt"

	volsyncv1alpha1 "github.com/backube/volsync/api/v1alpha1"
	"github.com/go-logr/logr"
	pvcv1alpha1 "github.com/konveyor/volume-snapshot-mover/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func (r *DataMoverBackupReconciler) CreateReplicationSource(log logr.Logger) (bool, error) {

	// get datamoverbackup from cluster
	dmb := pvcv1alpha1.DataMoverBackup{}
	if err := r.Get(r.Context, r.req.NamespacedName, &dmb); err != nil {
		r.Log.Error(err, "unable to fetch DataMoverBackup CR")
		return false, err
	}

	// get cloned pvc
	pvcName := fmt.Sprintf("%s-pvc", dmb.Spec.VolumeSnapshotContent.Name)
	pvc := corev1.PersistentVolumeClaim{}
	if err := r.Get(r.Context, types.NamespacedName{Namespace: r.NamespacedName.Namespace, Name: pvcName}, &pvc); err != nil {
		r.Log.Error(err, "unable to fetch PVC")
		return false, err
	}

	// define replicationSource to be created
	repSource := &volsyncv1alpha1.ReplicationSource{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-rep-src", dmb.Name),
			Namespace: r.NamespacedName.Namespace,
			Labels: map[string]string{
				DMBLabel: dmb.Name,
			},
		},
	}

	// Create ReplicationSource in OADP namespace
	op, err := controllerutil.CreateOrUpdate(r.Context, r.Client, repSource, func() error {

		return r.buildReplicationSource(repSource, &dmb, &pvc)
	})
	if err != nil {
		return false, err
	}

	// get replicationsource
	// to be sure it is created before checking status
	repSourceName := fmt.Sprintf("%s-rep-src", dmb.Name)
	replicationSource := volsyncv1alpha1.ReplicationSource{}
	if err = r.Get(r.Context, types.NamespacedName{Namespace: r.NamespacedName.Namespace, Name: repSourceName}, &replicationSource); err != nil {
		return false, err
	}

	// Update DMB CR with status from ReplicationSource
	err = r.setDMBRepSourceStatus(&replicationSource, &dmb)
	if err != nil {
		return false, err
	}

	if op == controllerutil.OperationResultCreated || op == controllerutil.OperationResultUpdated {
		r.EventRecorder.Event(repSource,
			corev1.EventTypeNormal,
			"ReplicationSourceReconciled",
			fmt.Sprintf("%s replicationsource %s", op, repSource.Name),
		)
	}
	return true, nil
}

func (r *DataMoverBackupReconciler) buildReplicationSource(replicationSource *volsyncv1alpha1.ReplicationSource, dmb *pvcv1alpha1.DataMoverBackup, pvc *corev1.PersistentVolumeClaim) error {

	// get restic secret created by controller
	resticSecretName := fmt.Sprintf("%s-secret", dmb.Name)
	resticSecret := corev1.Secret{}
	if err := r.Get(r.Context, types.NamespacedName{Namespace: dmb.Spec.ProtectedNamespace, Name: resticSecretName}, &resticSecret); err != nil {
		r.Log.Error(err, "unable to fetch Restic Secret")
		return err
	}

	// build ReplicationSource
	replicationSourceSpec := volsyncv1alpha1.ReplicationSourceSpec{
		SourcePVC: pvc.Name,
		Trigger: &volsyncv1alpha1.ReplicationSourceTriggerSpec{
			// TODO: handle better
			Manual: "trigger-test",
		},
		Restic: &volsyncv1alpha1.ReplicationSourceResticSpec{
			Repository: resticSecret.Name,
			ReplicationSourceVolumeOptions: volsyncv1alpha1.ReplicationSourceVolumeOptions{
				CopyMethod: volsyncv1alpha1.CopyMethodNone,
			},
		},
	}
	replicationSource.Spec = replicationSourceSpec
	return nil
}

func (r *DataMoverBackupReconciler) setDMBRepSourceStatus(repSource *volsyncv1alpha1.ReplicationSource, dmb *pvcv1alpha1.DataMoverBackup) error {

	if repSource.Status != nil {

		// check for ReplicationSource phase
		repSourceCompleted, err := r.isRepSourceCompleted(dmb)
		if err != nil {
			return err
		}
		conditions := repSource.Status.Conditions
		reconCondition := metav1.Condition{}
		for i, _ := range conditions {
			if conditions[i].Type == "Reconciled" {
				reconCondition = conditions[i]
			}
		}

		if repSourceCompleted && len(reconCondition.Type) > 0 && reconCondition.Status != metav1.ConditionTrue {
			// Update DMB status as completed
			dmb.Status.Phase = pvcv1alpha1.DatamoverBackupPhaseCompleted
			err := r.Status().Update(context.Background(), dmb)
			if err != nil {
				return err
			}

			// ReplicationSource phase is still in progress
		} else if !repSourceCompleted && len(reconCondition.Type) > 0 && reconCondition.Status != metav1.ConditionFalse {
			dmb.Status.Phase = pvcv1alpha1.DatamoverBackupPhaseInProgress

			// Update DMB status as in progress
			err := r.Status().Update(context.Background(), dmb)
			if err != nil {
				return err
			}

			// if not in progress or completed, phase failed
		} else {
			dmb.Status.Phase = pvcv1alpha1.DatamoverBackupPhaseFailed

			// Update DMB status
			err := r.Status().Update(context.Background(), dmb)
			if err != nil {
				return err
			}
		}
		return nil
	}
	return errors.New("replication source status not ready")
}

func (r *DataMoverBackupReconciler) isRepSourceCompleted(dmb *pvcv1alpha1.DataMoverBackup) (bool, error) {

	// get replicationsource
	repSourceName := fmt.Sprintf("%s-rep-src", dmb.Name)
	repSource := volsyncv1alpha1.ReplicationSource{}
	if err := r.Get(r.Context, types.NamespacedName{Namespace: r.NamespacedName.Namespace, Name: repSourceName}, &repSource); err != nil {
		return false, err
	}

	// used for nil pointer race condition
	if repSource.Status.LastSyncTime == nil {
		return false, nil
	}

	// for manual trigger, if spec.trigger.manual == status.lastManualSync, sync has completed
	sourceStatus := repSource.Status.LastManualSync
	sourceSpec := repSource.Spec.Trigger.Manual
	if sourceStatus == sourceSpec {
		return true, nil
	}

	// ReplicationSource has not yet completed but is not failed
	return false, nil
}
