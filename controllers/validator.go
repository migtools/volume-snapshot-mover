package controllers

import (
	"errors"
	"fmt"

	"github.com/go-logr/logr"
	pvcv1alpha1 "github.com/konveyor/volume-snapshot-mover/api/v1alpha1"
	snapv1 "github.com/kubernetes-csi/external-snapshotter/client/v4/apis/volumesnapshot/v1beta1"
	"k8s.io/apimachinery/pkg/types"
)

func (r *VolumeSnapshotBackupReconciler) ValidateVolumeSnapshotBackup(log logr.Logger) (bool, error) {
	vsb := pvcv1alpha1.VolumeSnapshotBackup{}
	if err := r.Get(r.Context, r.NamespacedName, &vsb); err != nil {
		return false, err
	}
	// Check if VolumeSnapshotContent is nil
	if vsb.Spec.VolumeSnapshotContent.Name == "" {
		return false, errors.New("volumeSnapShotBackup CR snapshot name cannot be nil")
	}

	vscInCluster := snapv1.VolumeSnapshotContent{}
	if err := r.Get(r.Context, types.NamespacedName{Name: vsb.Spec.VolumeSnapshotContent.Name}, &vscInCluster); err != nil {
		return false, errors.New("volumeSnapShotContent not found")
	}
	r.Log.Info(fmt.Sprintf("+++++ vscInCluster : %s", vscInCluster.UID))
	return true, nil
}
