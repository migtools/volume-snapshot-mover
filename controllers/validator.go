package controllers

import (
	"context"
	"errors"
	"fmt"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/go-logr/logr"
	volsnapmoverv1alpha1 "github.com/konveyor/volume-snapshot-mover/api/v1alpha1"
	snapv1 "github.com/kubernetes-csi/external-snapshotter/client/v4/apis/volumesnapshot/v1"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (r *VolumeSnapshotBackupReconciler) ValidateVolumeSnapshotMoverBackup(log logr.Logger) (bool, error) {
	VSBStatusUpdateNeeded := false
	var errString string

	vsb := volsnapmoverv1alpha1.VolumeSnapshotBackup{}
	if err := r.Get(r.Context, r.req.NamespacedName, &vsb); err != nil {
		// ignore is not found error
		if k8serrors.IsNotFound(err) {
			return true, nil
		}
		r.Log.Error(err, fmt.Sprintf("unable to fetch volumesnapshotbackup %s", r.req.NamespacedName))
		return false, err
	}
	// Check if VolumeSnapshotContent is nil
	if vsb.Spec.VolumeSnapshotContent.Name == "" {
		VSBStatusUpdateNeeded = true
		errString = fmt.Sprintf("snapshot name cannot be nil for volumesnapshotbackup %s", r.req.NamespacedName)
	}

	if len(vsb.Spec.ProtectedNamespace) == 0 {
		VSBStatusUpdateNeeded = true
		errString = fmt.Sprintf("protected ns cannot be empty for volumesnapshotbackup %s", r.req.NamespacedName)
	}

	vscInCluster := snapv1.VolumeSnapshotContent{}
	if err := r.Get(r.Context, types.NamespacedName{Name: vsb.Spec.VolumeSnapshotContent.Name}, &vscInCluster); err != nil {
		r.Log.Error(err, fmt.Sprintf("volumesnapshotcontent %s not found", vsb.Spec.VolumeSnapshotContent.Name))
		return false, err
	}

	hasOneDefaultVSClass, err := r.checkForOneDefaultVSBSnapClass(log)
	if !hasOneDefaultVSClass {
		VSBStatusUpdateNeeded = true
		errString = err.Error()
	}

	hasOneDefaultStorageClass, err := r.checkForOneDefaultVSBStorageClass(log)
	if !hasOneDefaultStorageClass {
		VSBStatusUpdateNeeded = true
		errString = err.Error()
	}

	if VSBStatusUpdateNeeded {
		err := updateVSBStatusPhase(&vsb, volsnapmoverv1alpha1.SnapMoverBackupPhaseFailed, r.Client)
		if err != nil {
			return false, err
		}

		r.Log.Info(fmt.Sprintf("marking volumesnapshotbackup %s as failed", r.req.NamespacedName))
		return false, errors.New(errString)
	}

	// recording VSB start timestamp as the CR has passed all the validation checks
	now := metav1.Now()
	vsb.Status.StartTimestamp = &now

	// update VSB status with StartTimestamp
	err = r.Client.Status().Update(context.Background(), &vsb)
	if err != nil {
		return false, err
	}

	return true, nil
}

func (r *VolumeSnapshotRestoreReconciler) ValidateVolumeSnapshotMoverRestore(log logr.Logger) (bool, error) {
	VSRStatusUpdateNeeded := false
	var errString string

	vsr := volsnapmoverv1alpha1.VolumeSnapshotRestore{}
	if err := r.Get(r.Context, r.req.NamespacedName, &vsr); err != nil {
		// ignore is not found error
		if k8serrors.IsNotFound(err) {
			return true, nil
		}
		return false, errors.New(fmt.Sprintf("unable to fetch volumesnapshotrestore %s", r.req.NamespacedName))
	}

	// Check if restic secret ref is empty
	if len(vsr.Spec.ResticSecretRef.Name) == 0 {
		VSRStatusUpdateNeeded = true
		errString = fmt.Sprintf("ResticSecretRef name cannot be empty for volumesnapshotrestore %s", r.req.NamespacedName)
	}

	// Check if VolumeSnapshotMoverbackuRef attributes are empty
	if len(vsr.Spec.VolumeSnapshotMoverBackupref.ResticRepository) == 0 {
		VSRStatusUpdateNeeded = true
		errString = fmt.Sprintf("volumeSnapshotMoverBackupref ResticRepository cannot be empty for volumesnapshotrestore %s", r.req.NamespacedName)
	}

	if len(vsr.Spec.VolumeSnapshotMoverBackupref.BackedUpPVCData.Name) == 0 {
		VSRStatusUpdateNeeded = true
		errString = fmt.Sprintf("volumeSnapshotMoverBackupref BackedUpPVCData name cannot be empty cannot be empty for volumesnapshotrestore %s", r.req.NamespacedName)
	}

	if len(vsr.Spec.VolumeSnapshotMoverBackupref.BackedUpPVCData.Size) == 0 {
		VSRStatusUpdateNeeded = true
		errString = fmt.Sprintf("volumeSnapshotMoverBackupref BackedUpPVCData size cannot be empty for volumesnapshotrestore %s", r.req.NamespacedName)
	}

	if len(vsr.Spec.ProtectedNamespace) == 0 {
		VSRStatusUpdateNeeded = true
		errString = fmt.Sprintf("protected ns cannot be empty for volumesnapshotrestore %s", r.req.NamespacedName)
	}

	hasOneDefaultVSClass, err := r.checkForOneDefaultVSRSnapClass(log)
	if !hasOneDefaultVSClass {
		VSRStatusUpdateNeeded = true
		errString = err.Error()
	}

	hasOneDefaultStorageClass, err := r.checkForOneDefaultVSRStorageClass(log)
	if !hasOneDefaultStorageClass {
		VSRStatusUpdateNeeded = true
		errString = err.Error()
	}

	if VSRStatusUpdateNeeded {
		err := updateVSRStatusPhase(&vsr, volsnapmoverv1alpha1.SnapMoverRestorePhaseFailed, r.Client)
		if err != nil {
			return false, err
		}

		r.Log.Info(fmt.Sprintf("marking volumesnapshotrestore %s as failed", r.req.NamespacedName))
		return false, errors.New(errString)
	}

	// recording VSR start timestamp as the CR has passed all the validation checks
	now := metav1.Now()
	vsr.Status.StartTimestamp = &now

	// update VSR status with StartTimestamp
	err = r.Client.Status().Update(context.Background(), &vsr)
	if err != nil {
		return false, err
	}

	return true, nil
}

func (r *VolumeSnapshotBackupReconciler) checkForOneDefaultVSBSnapClass(log logr.Logger) (bool, error) {

	vsClassList := snapv1.VolumeSnapshotClassList{}
	vsClassOptions := []client.ListOption{}

	// get all volumeSnapshotClasses in cluster
	if err := r.List(r.Context, &vsClassList, vsClassOptions...); err != nil {
		return false, err
	}

	_, err := checkForOneDefaultSnapClass(&vsClassList)
	if err != nil {
		return false, err
	}

	return true, nil
}

func (r *VolumeSnapshotBackupReconciler) checkForOneDefaultVSBStorageClass(log logr.Logger) (bool, error) {
	storageClassList := storagev1.StorageClassList{}
	storageClassOptions := []client.ListOption{}

	// get all volumeSnapshotClasses in cluster
	if err := r.List(r.Context, &storageClassList, storageClassOptions...); err != nil {
		return false, err
	}

	_, err := checkForOneDefaultStorageClass(&storageClassList)
	if err != nil {
		return false, err
	}

	return true, nil
}

func (r *VolumeSnapshotRestoreReconciler) checkForOneDefaultVSRSnapClass(log logr.Logger) (bool, error) {

	vsClassList := snapv1.VolumeSnapshotClassList{}
	vsClassOptions := []client.ListOption{}

	// get all volumeSnapshotClasses in cluster
	if err := r.List(r.Context, &vsClassList, vsClassOptions...); err != nil {
		return false, err
	}

	_, err := checkForOneDefaultSnapClass(&vsClassList)
	if err != nil {
		return false, err
	}

	return true, nil
}

func (r *VolumeSnapshotRestoreReconciler) checkForOneDefaultVSRStorageClass(log logr.Logger) (bool, error) {
	storageClassList := storagev1.StorageClassList{}
	storageClassOptions := []client.ListOption{}

	// get all storageClasses in cluster
	if err := r.List(r.Context, &storageClassList, storageClassOptions...); err != nil {
		return false, err
	}

	_, err := checkForOneDefaultStorageClass(&storageClassList)
	if err != nil {
		return false, err
	}

	return true, nil
}
