package controllers

import (
	"fmt"

	"github.com/go-logr/logr"
	pvcv1alpha1 "github.com/konveyor/volume-snapshot-mover/api/v1alpha1"
	snapv1 "github.com/kubernetes-csi/external-snapshotter/client/v4/apis/volumesnapshot/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func (r *DataMoverBackupReconciler) MirrorVolumeSnapshot(log logr.Logger) (bool, error) {
	// Get VSB from cluster
	vsb := pvcv1alpha1.DataMoverBackup{}
	if err := r.Get(r.Context, r.NamespacedName, &vsb); err != nil {
		return false, err
	}

	// define VSC to be created as clone of spec VSC
	vsc := &snapv1.VolumeSnapshotContent{
		ObjectMeta: metav1.ObjectMeta{
			Name: fmt.Sprintf("%s-clone", vsb.Spec.VolumeSnapshotContent.Name),
		},
	}

	// Create VSC in cluster
	op, err := controllerutil.CreateOrUpdate(r.Context, r.Client, vsc, func() error {
		err := controllerutil.SetControllerReference(&vsb, vsc, r.Scheme)
		if err != nil {
			return err
		}
		return r.buildVolumeSnapshotContent(vsc, &vsb)
	})

	if err != nil {
		return false, err
	}

	if op == controllerutil.OperationResultCreated || op == controllerutil.OperationResultUpdated {
		r.EventRecorder.Event(vsc,
			corev1.EventTypeNormal,
			"VolumeSnapshotContentReconciled",
			fmt.Sprintf("performed %s on volumesnapshotcontent %s", op, vsc.Name),
		)
	}

	return true, nil
}

func (r *DataMoverBackupReconciler) buildVolumeSnapshotContent(vsc *snapv1.VolumeSnapshotContent, vsb *pvcv1alpha1.DataMoverBackup) error {
	// Get VSC that is defined in spec
	vscInCluster := snapv1.VolumeSnapshotContent{}
	if err := r.Get(r.Context, types.NamespacedName{Name: vsb.Spec.VolumeSnapshotContent.Name}, &vscInCluster); err != nil {
		return err
	}

	// Make a new spec that points to same snapshot handle
	newSpec := snapv1.VolumeSnapshotContentSpec{
		Source: snapv1.VolumeSnapshotContentSource{
			SnapshotHandle: vscInCluster.Status.SnapshotHandle,
		},
	}

	// Make a cloned VSC with new spec
	// TODO: This spec is missing volume snapshot reference
	vsc.Spec = newSpec
	return nil
}
