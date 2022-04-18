package controllers

import (
	"context"
	"errors"
	"fmt"
	"time"

	volsyncv1alpha1 "github.com/backube/volsync/api/v1alpha1"
	"github.com/go-logr/logr"
	pvcv1alpha1 "github.com/konveyor/volume-snapshot-mover/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
)

func (r *DataMoverBackupReconciler) SetupDataMoverConfig(log logr.Logger) (bool, error) {
	return true, nil
}

func (r *DataMoverBackupReconciler) RunDataMoverBackup(log logr.Logger) (bool, error) {
	return true, nil
}

func (r *DataMoverBackupReconciler) SetDMBPVCSourceStatus(log logr.Logger) (bool, error) {
	// Get datamoverbackup from cluster
	dmb := pvcv1alpha1.DataMoverBackup{}
	if err := r.Get(r.Context, r.NamespacedName, &dmb); err != nil {
		r.Log.Error(err, "unable to fetch DataMoverBackup CR")
		return false, err
	}

	// TODO: separate func
	// get restic secret created by controller
	resticSecretName := fmt.Sprintf("%s-secret", dmb.Name)
	resticSecret := corev1.Secret{}
	if err := r.Get(r.Context, types.NamespacedName{Namespace: r.NamespacedName.Namespace, Name: resticSecretName}, &resticSecret); err != nil {
		r.Log.Error(err, "unable to fetch Restic Secret")
		return false, err
	}

	p, err := r.getSourcePVC()
	if err != nil {
		r.Log.Error(err, "unable to fetch source PVC")
		return false, err
	}
	// set source PVC name in status
	dmb.Status.SourcePVCData.Name = p.Name

	// set source PVC size in status
	size := p.Spec.Resources.Requests.Storage()
	sizeString := size.String()
	dmb.Status.SourcePVCData.Size = sizeString

	// set created Restic repo
	dmb.Status.ResticRepository = string(resticSecret.Data[ResticRepository])

	err = r.Status().Update(context.Background(), &dmb)
	if err != nil {
		return false, err
	}

	return true, nil
}

func (r *DataMoverBackupReconciler) WaitForDataMoverBackupToComplete(log logr.Logger) (bool, error) {

	// get datamoverbackup from cluster
	dmb := pvcv1alpha1.DataMoverBackup{}
	if err := r.Get(r.Context, r.NamespacedName, &dmb); err != nil {
		r.Log.Error(err, "unable to fetch DataMoverBackup CR")
		return false, err
	}

	// wait for ReplicationSource to complete before deleting resources
	fmt.Println("waiting for ReplicationSource to complete")
	err := r.waitForRepSourceCompletion(&dmb)
	if err != nil {
		return false, err
	}

	return true, nil
}

func (r *DataMoverBackupReconciler) isRepSourceCompleted(dmb *pvcv1alpha1.DataMoverBackup) wait.ConditionFunc {
	return func() (bool, error) {

		// get replicationsource
		repSourceName := fmt.Sprintf("%s-rep-src", dmb.Name)
		repSource := volsyncv1alpha1.ReplicationSource{}

		if err := r.Get(r.Context, types.NamespacedName{Namespace: r.NamespacedName.Namespace, Name: repSourceName}, &repSource); err != nil {
			return false, err
		}

		// TODO: handle better
		// used for nil pointer race condition
		if repSource.Status == nil {
			return false, nil
		}

		// TODO: handle better
		// used for nil pointer race condition
		if repSource.Status.LastManualSync == "" {
			return false, nil
		}

		// for manual trigger, if spec.trigger.manual == status.lastManualSync, sync has completed
		sourceStatus := repSource.Status.LastManualSync
		sourceSpec := repSource.Spec.Trigger.Manual

		if sourceStatus != sourceSpec {
			return false, errors.New("replicationsource failed to complete")
		}

		return true, nil
	}
}

// TODO: requeue if fails
func (r *DataMoverBackupReconciler) waitForRepSourceCompletion(dmb *pvcv1alpha1.DataMoverBackup) error {
	return wait.PollImmediate(5*time.Second, 2*time.Minute, r.isRepSourceCompleted(dmb))
}
