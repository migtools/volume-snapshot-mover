package controllers

import (
	"errors"
	"strconv"

	"github.com/go-logr/logr"
	volsnapmoverv1alpha1 "github.com/konveyor/volume-snapshot-mover/api/v1alpha1"
	snapv1 "github.com/kubernetes-csi/external-snapshotter/client/v4/apis/volumesnapshot/v1"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (r *VolumeSnapshotBackupReconciler) ValidateVolumeSnapshotMoverBackup(log logr.Logger) (bool, error) {
	vsb := volsnapmoverv1alpha1.VolumeSnapshotBackup{}
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

	hasOneDefaultVSClass, err := r.checkForOneDefaultSnapClass(log)
	if !hasOneDefaultVSClass {
		return false, err
	}

	hasOneDefaultStorageClass, err := r.checkForOneDefaultStorageClass(log)
	if !hasOneDefaultStorageClass {
		return false, err
	}

	r.Log.Info("returning true In function ValidateVolumeSnapshotMoverBackup")
	return true, nil
}

func (r *VolumeSnapshotRestoreReconciler) ValidateVolumeSnapshotMoverRestore(log logr.Logger) (bool, error) {
	vsr := volsnapmoverv1alpha1.VolumeSnapshotRestore{}
	if err := r.Get(r.Context, r.req.NamespacedName, &vsr); err != nil {
		r.Log.Error(err, "unable to fetch VolumeSnapshotRestore CR")
		return false, err
	}

	// Check if restic secret ref is empty
	if len(vsr.Spec.ResticSecretRef.Name) == 0 {
		return false, errors.New("VolumeSnapshotRestore CR ResticSecretRef name cannot be empty")
	}

	// Check if VolumeSnapshotMoverbackuRef attributes are empty
	if len(vsr.Spec.VolumeSnapshotMoverBackupref.ResticRepository) == 0 {
		return false, errors.New("VolumeSnapshotRestore CR volumeSnapshotMoverBackupref ResticRepository cannot be empty")
	}

	if len(vsr.Spec.VolumeSnapshotMoverBackupref.BackedUpPVCData.Name) == 0 {
		return false, errors.New("VolumeSnapshotRestore CR volumeSnapshotMoverBackupref BackedUpPVCData name cannot be empty")
	}

	if len(vsr.Spec.VolumeSnapshotMoverBackupref.BackedUpPVCData.Size) == 0 {
		return false, errors.New("VolumeSnapshotRestore CR volumeSnapshotMoverBackupref BackedUpPVCData size cannot be empty")
	}

	if len(vsr.Spec.ProtectedNamespace) == 0 {
		return false, errors.New("VolumeSnapshotRestore CR protected ns cannot be empty")
	}
	r.Log.Info("returning true In function ValidateVolumeSnapshotMoverRestore")
	return true, nil
}

func (r *VolumeSnapshotBackupReconciler) checkForOneDefaultSnapClass(log logr.Logger) (bool, error) {

	vsClassList := snapv1.VolumeSnapshotClassList{}
	vsClassOptions := []client.ListOption{}

	// get all volumeSnapshotClasses in cluster
	if err := r.List(r.Context, &vsClassList, vsClassOptions...); err != nil {
		return false, err
	}

	numDefaultClasses := 0
	for _, vsClass := range vsClassList.Items {

		isDefaultClass, _ := vsClass.Annotations[volumeSnapshotClassDefaultKey]
		boolIsDefault, _ := strconv.ParseBool(isDefaultClass)

		// found a default volumeSnapshotClass
		if boolIsDefault {
			numDefaultClasses++
		}

		if numDefaultClasses > 1 {
			r.Log.Info("cannot have more than one default volumeSnapshotClass")
			return false, errors.New("cannot have more than one default volumeSnapshotClass")
		}
	}

	return true, nil
}

func (r *VolumeSnapshotBackupReconciler) checkForOneDefaultStorageClass(log logr.Logger) (bool, error) {
	storageClassList := storagev1.StorageClassList{}
	storageClassOptions := []client.ListOption{}

	// get all volumeSnapshotClasses in cluster
	if err := r.List(r.Context, &storageClassList, storageClassOptions...); err != nil {
		return false, err
	}

	numDefaultClasses := 0
	for _, storageClass := range storageClassList.Items {

		isDefaultClass, _ := storageClass.Annotations[storageClassDefaultKey]
		boolIsDefault, _ := strconv.ParseBool(isDefaultClass)

		// found a default storageClass
		if boolIsDefault {
			numDefaultClasses++
		}

		if numDefaultClasses > 1 {
			r.Log.Info("cannot have more than one default storageClass")
			return false, errors.New("cannot have more than one default storageClass")
		}
	}

	return true, nil
}
