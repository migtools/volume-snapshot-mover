package controllers

import (
	"fmt"

	volsyncv1alpha1 "github.com/backube/volsync/api/v1alpha1"
	"github.com/go-logr/logr"
	pvcv1alpha1 "github.com/konveyor/volume-snapshot-mover/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func (r *DataMoverBackupReconciler) CreateReplicationSource(log logr.Logger) (bool, error) {

	// get datamoverbackup from cluster
	dmb := pvcv1alpha1.DataMoverBackup{}
	if err := r.Get(r.Context, r.NamespacedName, &dmb); err != nil {
		r.Log.Error(err, "unable to fetch DataMoverBackup CR")
		return false, err
	}

	// get cloned pvc
	pvcName := fmt.Sprintf("%s-pvc", dmb.Spec.VolumeSnapshotContent.Name)
	pvc := corev1.PersistentVolumeClaim{}
	if err := r.Get(r.Context, types.NamespacedName{Namespace: r.NamespacedName.Namespace, Name: pvcName}, &pvc); err != nil {
		r.Log.Error(err, "unable to fetch PVC")
		return false, err
	}

	// define replicationSource to be created
	repSource := &volsyncv1alpha1.ReplicationSource{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-rep-src", dmb.Name),
			Namespace: r.NamespacedName.Namespace,
		},
	}

	// Create ReplicationSource in OADP namespace
	op, err := controllerutil.CreateOrUpdate(r.Context, r.Client, repSource, func() error {

		err := controllerutil.SetOwnerReference(&dmb, repSource, r.Scheme)
		if err != nil {
			return err
		}
		return r.buildReplicationSource(repSource, &dmb, &pvc)
	})
	if err != nil {
		return false, err
	}

	if op == controllerutil.OperationResultCreated || op == controllerutil.OperationResultUpdated {
		r.EventRecorder.Event(repSource,
			corev1.EventTypeNormal,
			"ReplicationSourceReconciled",
			fmt.Sprintf("%s replicationsource %s", op, repSource.Name),
		)
	}
	return true, nil
}

func (r *DataMoverBackupReconciler) buildReplicationSource(replicationSource *volsyncv1alpha1.ReplicationSource, dmb *pvcv1alpha1.DataMoverBackup, pvc *corev1.PersistentVolumeClaim) error {

	// get restic secret created by controller
	resticSecretName := fmt.Sprintf("%s-secret", dmb.Name)
	resticSecret := corev1.Secret{}
	if err := r.Get(r.Context, types.NamespacedName{Namespace: r.NamespacedName.Namespace, Name: resticSecretName}, &resticSecret); err != nil {
		r.Log.Error(err, "unable to fetch Restic Secret")
		return err
	}

	// build ReplicationSource
	replicationSourceSpec := volsyncv1alpha1.ReplicationSourceSpec{
		SourcePVC: pvc.Name,
		Trigger: &volsyncv1alpha1.ReplicationSourceTriggerSpec{
			// TODO: handle better
			Manual: "trigger-test",
		},
		Restic: &volsyncv1alpha1.ReplicationSourceResticSpec{
			Repository: resticSecret.Name,
			ReplicationSourceVolumeOptions: volsyncv1alpha1.ReplicationSourceVolumeOptions{
				CopyMethod: volsyncv1alpha1.CopyMethodNone,
			},
		},
	}
	replicationSource.Spec = replicationSourceSpec
	return nil
}
