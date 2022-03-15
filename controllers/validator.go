package controllers

import (
	"errors"

	"github.com/go-logr/logr"
	pvcv1alpha1 "github.com/konveyor/volume-snapshot-mover/api/v1alpha1"
	snapv1 "github.com/kubernetes-csi/external-snapshotter/client/v4/apis/volumesnapshot/v1"
	"k8s.io/apimachinery/pkg/types"
)

func (r *DataMoverBackupReconciler) ValidateDataMoverBackup(log logr.Logger) (bool, error) {
	dmb := pvcv1alpha1.DataMoverBackup{}
	if err := r.Get(r.Context, r.NamespacedName, &dmb); err != nil {
		return false, err
	}
	// Check if VolumeSnapshotContent is nil
	if dmb.Spec.VolumeSnapshotContent.Name == "" {
		return false, errors.New("dataMoverBackup CR snapshot name cannot be nil")
	}
	vscInCluster := snapv1.VolumeSnapshotContent{}
	if err := r.Get(r.Context, types.NamespacedName{Name: dmb.Spec.VolumeSnapshotContent.Name}, &vscInCluster); err != nil {
		return false, errors.New("volumeSnapShotContent not found")
	}
	return true, nil
}
