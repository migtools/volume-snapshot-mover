package controllers

import (
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

func (r *DataMoverBackupReconciler) WaitForDataMoverBackupToComplete(log logr.Logger) (bool, error) {

	// get datamoverbackup from cluster
	dmb := pvcv1alpha1.DataMoverBackup{}
	if err := r.Get(r.Context, r.NamespacedName, &dmb); err != nil {
		return false, err
	}

	// get cloned pvc
	pvcName := fmt.Sprintf("%s-pvc", dmb.Spec.VolumeSnapshotContent.Name)
	pvc := corev1.PersistentVolumeClaim{}
	if err := r.Get(r.Context, types.NamespacedName{Namespace: r.NamespacedName.Namespace, Name: pvcName}, &pvc); err != nil {
		return false, err
	}

	// wait for ReplicationSource to exist
	fmt.Println("waiting for ReplicationSource to exist")
	err := r.waitForRepSourceCreation(&pvc)
	if err != nil {
		return false, err
	}

	// get replicationsource
	repSourceName := fmt.Sprintf("%s-backup", pvc.Name)
	repSource := volsyncv1alpha1.ReplicationSource{}
	if err := r.Get(r.Context, types.NamespacedName{Namespace: r.NamespacedName.Namespace, Name: repSourceName}, &repSource); err != nil {
		return false, err
	}

	// wait for ReplicationSource to complete before deleting resources
	fmt.Println("waiting for ReplicationSource to complete")
	err = r.waitForRepSourceCompletion(&repSource, &pvc)
	if err != nil {
		return false, err
	}
	return true, nil
}

func (r *DataMoverBackupReconciler) isRepSourceCreated(pvc *corev1.PersistentVolumeClaim) wait.ConditionFunc {
	return func() (bool, error) {

		// get replicationsource
		repSourceName := fmt.Sprintf("%s-backup", pvc.Name)
		repSource := volsyncv1alpha1.ReplicationSource{}

		if err := r.Get(r.Context, types.NamespacedName{Namespace: r.NamespacedName.Namespace, Name: repSourceName}, &repSource); err != nil {
			return false, errors.New("unable to get replicationsource")
		}
		return true, nil
	}
}

func (r *DataMoverBackupReconciler) waitForRepSourceCreation(pvc *corev1.PersistentVolumeClaim) error {
	return wait.PollImmediate(5*time.Second, 2*time.Minute, r.isRepSourceCreated(pvc))
}

func (r *DataMoverBackupReconciler) isRepSourceCompleted(replicationSource *volsyncv1alpha1.ReplicationSource,
	pvc *corev1.PersistentVolumeClaim) wait.ConditionFunc {
	return func() (bool, error) {

		sourceStatus := replicationSource.Status.LastManualSync
		sourceSpec := replicationSource.Spec.Trigger.Manual

		if sourceStatus == sourceSpec {
			return true, nil
		}
		return false, errors.New("replicationsource failed to complete")

	}
}

func (r *DataMoverBackupReconciler) waitForRepSourceCompletion(replicationSource *volsyncv1alpha1.ReplicationSource,
	pvc *corev1.PersistentVolumeClaim) error {
	return wait.PollImmediate(5*time.Second, 2*time.Minute, r.isRepSourceCompleted(replicationSource, pvc))
}
