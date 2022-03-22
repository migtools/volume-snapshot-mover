package controllers

import (
	"errors"
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
	// Get datamoverbackup from cluster
	// TODO: handle multiple DMBs
	dmb := pvcv1alpha1.DataMoverBackup{}
	if err := r.Get(r.Context, r.NamespacedName, &dmb); err != nil {
		return false, errors.New("dataMoverBackup not found")
	}

	// define replicationSource to be created
	repSource := &volsyncv1alpha1.ReplicationSource{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-repsource",
			Namespace: dmb.Namespace,
		},
	}
	// Create ReplicationSource in OADP namespace
	op, err := controllerutil.CreateOrUpdate(r.Context, r.Client, repSource, func() error {
		return r.buildReplicationSource(repSource, &dmb)
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

func (r *DataMoverBackupReconciler) buildReplicationSource(replicationSource *volsyncv1alpha1.ReplicationSource, dmb *pvcv1alpha1.DataMoverBackup) error {

	// get pvc in OADP namespace
	pvc := corev1.PersistentVolumeClaim{}
	// TODO: get pvc name
	if err := r.Get(r.Context, types.NamespacedName{Namespace: dmb.Namespace, Name: "mssql-pvc"}, &pvc); err != nil {
		return err
	}

	// get restic secret in OADP namespace
	resticSecretName := fmt.Sprintf("%s-resticonfig", pvc.Name)
	resticSecret := corev1.Secret{}
	if err := r.Get(r.Context, types.NamespacedName{Namespace: dmb.Namespace, Name: resticSecretName}, &resticSecret); err != nil {
		return err
	}

	// build ReplicationSource
	replicationSourceSpec := volsyncv1alpha1.ReplicationSourceSpec{
		SourcePVC: pvc.Name,
		Trigger: &volsyncv1alpha1.ReplicationSourceTriggerSpec{
			// TODO: handle better
			Manual: "triggertest",
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
