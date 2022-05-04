package controllers

import (
	"fmt"

	volsyncv1alpha1 "github.com/backube/volsync/api/v1alpha1"
	"github.com/go-logr/logr"
	pvcv1alpha1 "github.com/konveyor/volume-snapshot-mover/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func (r *DataMoverRestoreReconciler) CreateReplicationDestination(log logr.Logger) (bool, error) {

	// get datamoverrestore from cluster
	dmr := pvcv1alpha1.DataMoverRestore{}
	if err := r.Get(r.Context, r.req.NamespacedName, &dmr); err != nil {
		r.Log.Error(err, "unable to fetch DataMoverRestore CR")
		return false, err
	}

	// get datamoverbackup from cluster
	// TODO: get DMB from backup
	dmb := pvcv1alpha1.DataMoverBackup{}
	if err := r.Get(r.Context, types.NamespacedName{Name: "datamoverbackup-sample", Namespace: r.NamespacedName.Namespace}, &dmb); err != nil {
		r.Log.Error(err, "unable to fetch DataMoverBackup CR")
		return false, err
	}

	// define replicationDestination to be created
	repDestination := &volsyncv1alpha1.ReplicationDestination{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-rep-dest", dmr.Name),
			Namespace: r.NamespacedName.Namespace,
		},
	}

	// Create ReplicationDestination in OADP namespace
	op, err := controllerutil.CreateOrUpdate(r.Context, r.Client, repDestination, func() error {

		return r.buildReplicationDestination(repDestination, &dmb, &dmr)
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

func (r *DataMoverRestoreReconciler) buildReplicationDestination(replicationDestination *volsyncv1alpha1.ReplicationDestination, dmb *pvcv1alpha1.DataMoverBackup, dmr *pvcv1alpha1.DataMoverRestore) error {

	// get restic secret created by controller
	resticSecretName := fmt.Sprintf("%s-secret", dmr.Name)
	resticSecret := corev1.Secret{}
	if err := r.Get(r.Context, types.NamespacedName{Namespace: r.NamespacedName.Namespace, Name: resticSecretName}, &resticSecret); err != nil {
		r.Log.Error(err, "unable to fetch Restic Secret")
		return err
	}

	// TODO: use DMR for this field once DMB is added to backup
	stringCapacity := dmb.Status.SourcePVCData.Size
	capacity := resource.MustParse(stringCapacity)

	// build ReplicationDestination
	replicationDestinationSpec := volsyncv1alpha1.ReplicationDestinationSpec{
		Trigger: &volsyncv1alpha1.ReplicationDestinationTriggerSpec{
			// TODO: handle better
			Manual: fmt.Sprintf("%s-trigger", dmr.Name),
		},
		Restic: &volsyncv1alpha1.ReplicationDestinationResticSpec{
			// TODO: create restic secret from secret from DMB CR status
			Repository: resticSecret.Name,
			ReplicationDestinationVolumeOptions: volsyncv1alpha1.ReplicationDestinationVolumeOptions{
				AccessModes:    []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
				CopyMethod:     volsyncv1alpha1.CopyMethodSnapshot,
				DestinationPVC: &dmr.Spec.DestinationClaimRef.Name,
				Capacity:       &capacity,
			},
		},
	}
	replicationDestination.Spec = replicationDestinationSpec
	return nil
}
