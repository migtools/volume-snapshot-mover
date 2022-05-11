package controllers

import (
	"errors"

	"github.com/go-logr/logr"
	datamoverv1alpha1 "github.com/konveyor/volume-snapshot-mover/api/v1alpha1"
	snapv1 "github.com/kubernetes-csi/external-snapshotter/client/v4/apis/volumesnapshot/v1"
	"k8s.io/apimachinery/pkg/types"
)

func (r *VolumeSnapshotBackupReconciler) ValidateDataMoverBackup(log logr.Logger) (bool, error) {
	vsb := datamoverv1alpha1.VolumeSnapshotBackup{}
	if err := r.Get(r.Context, r.req.NamespacedName, &vsb); err != nil {
		r.Log.Error(err, "unable to fetch VolumeSnapshotBackup CR")
		return false, err
	}
	// Check if VolumeSnapshotContent is nil
	if vsb.Spec.VolumeSnapshotContent.Name == "" {
		return false, errors.New("VolumeSnapshotBackup CR snapshot name cannot be nil")
	}

	if len(vsb.Spec.ProtectedNamespace) == 0 {
		return false, errors.New("VolumeSnapshotBackup CR protected ns cannot be empty")
	}

	vscInCluster := snapv1.VolumeSnapshotContent{}
	if err := r.Get(r.Context, types.NamespacedName{Name: vsb.Spec.VolumeSnapshotContent.Name}, &vscInCluster); err != nil {
		r.Log.Error(err, "volumesnapshotcontent not found")
		return false, err
	}
	return true, nil
}

func (r *VolumeSnapshotRestoreReconciler) ValidateDataMoverRestore(log logr.Logger) (bool, error) {
	vsr := datamoverv1alpha1.VolumeSnapshotRestore{}
	if err := r.Get(r.Context, r.req.NamespacedName, &vsr); err != nil {
		r.Log.Error(err, "unable to fetch VolumeSnapshotRestore CR")
		return false, err
	}

	// Check if restic secret ref is empty
	if len(vsr.Spec.ResticSecretRef.Name) == 0 {
		return false, errors.New("VolumeSnapshotRestore CR ResticSecretRef name cannot be empty")
	}

	// Check if DatamoverbackuRef attributes are empty
	if len(vsr.Spec.DataMoverBackupref.ResticRepository) == 0 {
		return false, errors.New("VolumeSnapshotRestore CR DataMoverBackupref ResticRepository cannot be empty")
	}

	if len(vsr.Spec.DataMoverBackupref.BackedUpPVCData.Name) == 0 {
		return false, errors.New("VolumeSnapshotRestore CR DataMoverBackupref BackedUpPVCData name cannot be empty")
	}

	if len(vsr.Spec.DataMoverBackupref.BackedUpPVCData.Size) == 0 {
		return false, errors.New("VolumeSnapshotRestore CR DataMoverBackupref BackedUpPVCData size cannot be empty")
	}

	if len(vsr.Spec.ProtectedNamespace) == 0 {
		return false, errors.New("VolumeSnapshotRestore CR protected ns cannot be empty")
	}

	return true, nil
}
