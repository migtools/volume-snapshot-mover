package controllers

import (
	volsyncv1alpha1 "github.com/backube/volsync/api/v1alpha1"
	"github.com/go-logr/logr"
	datamoverv1alpha1 "github.com/konveyor/volume-snapshot-mover/api/v1alpha1"
	snapv1 "github.com/kubernetes-csi/external-snapshotter/client/v4/apis/volumesnapshot/v1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var cleanupVSBTypes = []client.Object{
	&corev1.PersistentVolumeClaim{},
	&corev1.Pod{},
	&corev1.Secret{},
	&snapv1.VolumeSnapshot{},
	&snapv1.VolumeSnapshotContent{},
	&volsyncv1alpha1.ReplicationSource{},
}

func (r *VolumeSnapshotBackupReconciler) CleanVSBBackupResources(log logr.Logger) (bool, error) {
	r.Log.Info("In function CleanBackupResources")
	// get volumesnapshotbackup from cluster
	vsb := datamoverv1alpha1.VolumeSnapshotBackup{}
	if err := r.Get(r.Context, r.req.NamespacedName, &vsb); err != nil {
		return false, err
	}

	// make sure VSB is completed before deleting resources
	if vsb.Status.Phase != datamoverv1alpha1.DatamoverBackupPhaseCompleted {
		r.Log.Info("waiting for datamoverbackup to complete before deleting resources")
		return false, nil
	}

	// get resources with VSB controller label in protected ns
	deleteOptions := []client.DeleteAllOfOption{
		client.MatchingLabels{VSBLabel: vsb.Name},
		client.InNamespace(vsb.Spec.ProtectedNamespace),
	}

	for _, obj := range cleanupVSBTypes {
		err := r.DeleteAllOf(r.Context, obj, deleteOptions...)
		if err != nil {
			r.Log.Error(err, "unable to delete VSB resource")
			return false, err
		}
	}

	return true, nil
}
