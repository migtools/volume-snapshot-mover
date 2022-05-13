package controllers

import (
	"errors"
	"fmt"

	"github.com/go-logr/logr"
	datamoverv1alpha1 "github.com/konveyor/volume-snapshot-mover/api/v1alpha1"
	snapv1 "github.com/kubernetes-csi/external-snapshotter/client/v4/apis/volumesnapshot/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func (r *VolumeSnapshotBackupReconciler) MirrorVolumeSnapshotContent(log logr.Logger) (bool, error) {
	// Get volumesnapshotbackup from cluster
	// TODO: handle multiple VSBs
	vsb := datamoverv1alpha1.VolumeSnapshotBackup{}
	if err := r.Get(r.Context, r.req.NamespacedName, &vsb); err != nil {
		r.Log.Error(err, "unable to fetch DataMoverBackup CR")
		return false, err
	}

	// fetch original vsc
	vscInCluster := snapv1.VolumeSnapshotContent{}
	if err := r.Get(r.Context, types.NamespacedName{Name: vsb.Spec.VolumeSnapshotContent.Name}, &vscInCluster); err != nil {
		r.Log.Error(err, "original volumesnapshotcontent not found")
		return false, err
	}

	// define VSC to be created as clone of spec VSC
	vscClone := &snapv1.VolumeSnapshotContent{
		ObjectMeta: metav1.ObjectMeta{
			Name: fmt.Sprintf("%s-clone", vscInCluster.Name),
			Labels: map[string]string{
				VSBLabel: vsb.Name,
			},
		},
	}

	// Create VSC clone in cluster
	op, err := controllerutil.CreateOrUpdate(r.Context, r.Client, vscClone, func() error {

		return r.buildVolumeSnapshotContentClone(vscClone, &vsb)
	})

	if err != nil {
		return false, err
	}

	if op == controllerutil.OperationResultCreated || op == controllerutil.OperationResultUpdated {

		r.EventRecorder.Event(vscClone,
			corev1.EventTypeNormal,
			"VolumeSnapshotContentReconciled",
			fmt.Sprintf("performed %s on volumesnapshotcontent %s", op, vscClone.Name),
		)
	}

	return true, nil
}

func (r *VolumeSnapshotBackupReconciler) MirrorVolumeSnapshot(log logr.Logger) (bool, error) {
	// Get volumesnapshotbackup from cluster
	// TODO: handle multiple VSBs
	vsb := datamoverv1alpha1.VolumeSnapshotBackup{}
	if err := r.Get(r.Context, r.req.NamespacedName, &vsb); err != nil {
		r.Log.Error(err, "unable to fetch VolumeSnapshotBackup CR")
		return false, err
	}

	// fetch vsc clone
	vscClone := snapv1.VolumeSnapshotContent{}
	if err := r.Get(r.Context, types.NamespacedName{Name: fmt.Sprintf("%s-clone", vsb.Spec.VolumeSnapshotContent.Name)}, &vscClone); err != nil {
		r.Log.Error(err, "volumesnapshotcontent clone not found")
		return false, err
	}

	// check if vsc clone is ready to use before going ahead with vs clone creation
	if vscClone.Status == nil || vscClone.Status.ReadyToUse == nil || *vscClone.Status.ReadyToUse != true {
		return false, errors.New("volumesnapshotcontent clone is not ready to use")
	}

	// keep the snapshot name the same as referred in the vsc clone
	// draft vs clone
	vsClone := &snapv1.VolumeSnapshot{
		ObjectMeta: metav1.ObjectMeta{
			Name:      vscClone.Spec.VolumeSnapshotRef.Name,
			Namespace: vsb.Spec.ProtectedNamespace,
			Labels: map[string]string{
				VSBLabel: vsb.Name,
			},
		},
	}

	// Create VolumeSnapshot clone in the protected namespace
	op, err := controllerutil.CreateOrUpdate(r.Context, r.Client, vsClone, func() error {

		return r.buildVolumeSnapshotClone(vsClone, &vscClone)
	})
	if err != nil {
		return false, err
	}
	if op == controllerutil.OperationResultCreated || op == controllerutil.OperationResultUpdated {

		r.EventRecorder.Event(vsClone,
			corev1.EventTypeNormal,
			"VolumeSnapshotReconciled",
			fmt.Sprintf("performed %s on volumesnapshot %s", op, vsClone.Name),
		)
	}

	return true, nil
}

func (r *VolumeSnapshotBackupReconciler) buildVolumeSnapshotContentClone(vscClone *snapv1.VolumeSnapshotContent, vsb *datamoverv1alpha1.VolumeSnapshotBackup) error {
	// Get VSC that is defined in spec
	vscInCluster := snapv1.VolumeSnapshotContent{}
	if err := r.Get(r.Context, types.NamespacedName{Name: vsb.Spec.VolumeSnapshotContent.Name}, &vscInCluster); err != nil {
		r.Log.Error(err, "unable to fetch original volumesnapshotcontent in cluster")
		return err
	}

	// Make a new spec that points to same snapshot handle
	newSpec := snapv1.VolumeSnapshotContentSpec{
		DeletionPolicy: vscInCluster.Spec.DeletionPolicy,
		Driver:         vscInCluster.Spec.Driver,
		VolumeSnapshotRef: corev1.ObjectReference{
			APIVersion: vscInCluster.Spec.VolumeSnapshotRef.APIVersion,
			Kind:       vscInCluster.Spec.VolumeSnapshotRef.Kind,
			Namespace:  vsb.Spec.ProtectedNamespace,
			Name:       fmt.Sprintf("%s-volumesnapshot", vscClone.Name),
		},
		VolumeSnapshotClassName: vscInCluster.Spec.VolumeSnapshotClassName,
		Source: snapv1.VolumeSnapshotContentSource{
			SnapshotHandle: vscInCluster.Status.SnapshotHandle,
		},
	}

	vscClone.Spec = newSpec
	return nil
}

func (r *VolumeSnapshotBackupReconciler) buildVolumeSnapshotClone(vsClone *snapv1.VolumeSnapshot, vscClone *snapv1.VolumeSnapshotContent) error {
	// Get VS that is defined in spec
	vsSpec := snapv1.VolumeSnapshotSpec{
		Source: snapv1.VolumeSnapshotSource{
			VolumeSnapshotContentName: &vscClone.Name,
		},
	}

	vsClone.Spec = vsSpec
	return nil
}
