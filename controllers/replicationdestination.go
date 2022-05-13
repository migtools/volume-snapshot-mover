package controllers

import (
	"fmt"

	volsyncv1alpha1 "github.com/backube/volsync/api/v1alpha1"
	"github.com/go-logr/logr"
	datamoverv1alpha1 "github.com/konveyor/volume-snapshot-mover/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func (r *VolumeSnapshotRestoreReconciler) CreateReplicationDestination(log logr.Logger) (bool, error) {

	// get volumesnapshotrestore from cluster
	vsr := datamoverv1alpha1.VolumeSnapshotRestore{}
	if err := r.Get(r.Context, r.req.NamespacedName, &vsr); err != nil {
		r.Log.Error(err, "unable to fetch VolumeSnapshotRestore CR")
		return false, err
	}

	// define replicationDestination to be created
	repDestination := &volsyncv1alpha1.ReplicationDestination{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-rep-dest", vsr.Name),
			Namespace: r.NamespacedName.Namespace,
		},
	}

	// Create ReplicationDestination in OADP namespace
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

func (r *VolumeSnapshotRestoreReconciler) buildReplicationDestination(replicationDestination *volsyncv1alpha1.ReplicationDestination, vsr *datamoverv1alpha1.VolumeSnapshotRestore) error {

	// get restic secret created by controller
	resticSecretName := fmt.Sprintf("%s-secret", vsr.Name)
	resticSecret := corev1.Secret{}
	if err := r.Get(r.Context, types.NamespacedName{Namespace: r.NamespacedName.Namespace, Name: resticSecretName}, &resticSecret); err != nil {
		r.Log.Error(err, "unable to fetch Restic Secret")
		return err
	}

	stringCapacity := vsr.Spec.DataMoverBackupref.BackedUpPVCData.Size
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
				Capacity: &capacity,
			},
		},
	}
	replicationDestination.Spec = replicationDestinationSpec
	return nil
}
