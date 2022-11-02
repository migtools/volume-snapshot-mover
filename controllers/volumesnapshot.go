package controllers

import (
	"context"
	"fmt"
	"time"

	k8serrors "k8s.io/apimachinery/pkg/api/errors"

	volsyncv1alpha1 "github.com/backube/volsync/api/v1alpha1"
	"github.com/go-logr/logr"
	volsnapmoverv1alpha1 "github.com/konveyor/volume-snapshot-mover/api/v1alpha1"
	snapv1 "github.com/kubernetes-csi/external-snapshotter/client/v4/apis/volumesnapshot/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func (r *VolumeSnapshotBackupReconciler) MirrorVolumeSnapshotContent(log logr.Logger) (bool, error) {
	// Get volumesnapshotbackup from cluster
	// TODO: handle multiple VSBs
	vsb := volsnapmoverv1alpha1.VolumeSnapshotBackup{}
	if err := r.Get(r.Context, r.req.NamespacedName, &vsb); err != nil {
		// ignore is not found error
		if k8serrors.IsNotFound(err) {
			return true, nil
		}
		r.Log.Error(err, fmt.Sprintf("unable to fetch volumesnapshotbackup %s", r.req.NamespacedName))
		return false, err
	}

	// fetch original vsc
	time.Sleep(time.Second * 10)
	vscInCluster := snapv1.VolumeSnapshotContent{}
	if err := r.Get(r.Context, types.NamespacedName{Name: vsb.Spec.VolumeSnapshotContent.Name}, &vscInCluster); err != nil {
		r.Log.Error(err, fmt.Sprintf("original volumesnapshotcontent %s not found", vsb.Spec.VolumeSnapshotContent.Name))
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
	vsb := volsnapmoverv1alpha1.VolumeSnapshotBackup{}
	if err := r.Get(r.Context, r.req.NamespacedName, &vsb); err != nil {
		// ignore is not found error
		if k8serrors.IsNotFound(err) {
			return true, nil
		}
		r.Log.Error(err, fmt.Sprintf("unable to fetch volumesnapshotbackup %s", r.req.NamespacedName))
		return false, err
	}

	// fetch vsc clone
	vscClone := snapv1.VolumeSnapshotContent{}
	vscCloneName := fmt.Sprintf("%s-clone", vsb.Spec.VolumeSnapshotContent.Name)
	if err := r.Get(r.Context, types.NamespacedName{Name: vscCloneName}, &vscClone); err != nil {
		r.Log.Error(err, fmt.Sprintf("volumesnapshotcontent clone %s not found", vscCloneName))
		return false, err
	}

	// check if vsc clone is ready to use before going ahead with vs clone creation
	if vscClone.Status == nil || vscClone.Status.ReadyToUse == nil || *vscClone.Status.ReadyToUse != true {
		r.Log.Info(fmt.Sprintf("volumesnapshotcontent clone %s is not ready to use", vscCloneName))
		return false, nil
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

func (r *VolumeSnapshotBackupReconciler) buildVolumeSnapshotContentClone(vscClone *snapv1.VolumeSnapshotContent, vsb *volsnapmoverv1alpha1.VolumeSnapshotBackup) error {
	// Get VSC that is defined in spec
	vscInCluster := snapv1.VolumeSnapshotContent{}
	if err := r.Get(r.Context, types.NamespacedName{Name: vsb.Spec.VolumeSnapshotContent.Name}, &vscInCluster); err != nil {
		r.Log.Error(err, fmt.Sprintf("unable to fetch original volumesnapshotcontent %s in cluster", vsb.Spec.VolumeSnapshotContent.Name))
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

	if vscClone.CreationTimestamp.IsZero() {
		vscClone.Spec = newSpec
	}

	if vsb.Status.VolumeSnapshotClassName == "" {
		// update VSB status to add volumesnapshotclassname
		// set source PVC name in VSB status
		vsb.Status.VolumeSnapshotClassName = *vscClone.Spec.VolumeSnapshotClassName

		// Update VSB status
		err := r.Status().Update(context.Background(), vsb)
		if err != nil {
			return err
		}
	}

	return nil
}

func (r *VolumeSnapshotBackupReconciler) buildVolumeSnapshotClone(vsClone *snapv1.VolumeSnapshot, vscClone *snapv1.VolumeSnapshotContent) error {
	// Get VS that is defined in spec
	vsSpec := snapv1.VolumeSnapshotSpec{
		Source: snapv1.VolumeSnapshotSource{
			VolumeSnapshotContentName: &vscClone.Name,
		},
	}

	if vsClone.CreationTimestamp.IsZero() {
		vsClone.Spec = vsSpec
	}

	return nil
}

func (r *VolumeSnapshotBackupReconciler) WaitForClonedVolumeSnapshotToBeReady(log logr.Logger) (bool, error) {
	// Get volumesnapshotbackup from cluster
	vsb := volsnapmoverv1alpha1.VolumeSnapshotBackup{}
	if err := r.Get(r.Context, r.req.NamespacedName, &vsb); err != nil {
		// ignore is not found error
		if k8serrors.IsNotFound(err) {
			return true, nil
		}
		r.Log.Error(err, fmt.Sprintf("unable to fetch volumesnapshotbackup %s", r.req.NamespacedName))
		return false, err
	}

	// Get the clone VSC
	vscClone := snapv1.VolumeSnapshotContent{}
	vscCloneName := fmt.Sprintf("%s-clone", vsb.Spec.VolumeSnapshotContent.Name)
	if err := r.Get(r.Context, types.NamespacedName{Name: vscCloneName}, &vscClone); err != nil {
		r.Log.Error(err, fmt.Sprintf("cloned volumesnapshotcontent %s not found", vscCloneName))
		return false, err
	}

	// Check if Volumesnapshot clone is present in the protected namespace
	vsClone := snapv1.VolumeSnapshot{}
	if err := r.Get(r.Context,
		types.NamespacedName{Name: fmt.Sprintf(vscClone.Spec.VolumeSnapshotRef.Name), Namespace: vsb.Spec.ProtectedNamespace}, &vsClone); err != nil {
		r.Log.Info(fmt.Sprintf("cloned volumesnapshot %s not available in the protected namespace", vscClone.Spec.VolumeSnapshotRef.Name))
		return false, nil
	}

	//skip waiting if vs is ready
	if vsClone.Status != nil && *vsClone.Status.ReadyToUse == true && *vsClone.Status.BoundVolumeSnapshotContentName == vscClone.Name {
		time.Sleep(time.Second * 10)
		return true, nil
	}

	time.Sleep(time.Second * 30)
	return true, nil
}

func (r *VolumeSnapshotBackupReconciler) WaitForClonedVolumeSnapshotContentToBeReady(log logr.Logger) (bool, error) {
	// fetch clone vsc and skip waiting if its ready
	vsb := volsnapmoverv1alpha1.VolumeSnapshotBackup{}
	if err := r.Get(r.Context, r.req.NamespacedName, &vsb); err != nil {
		// ignore is not found error
		if k8serrors.IsNotFound(err) {
			return true, nil
		}
		r.Log.Error(err, fmt.Sprintf("unable to fetch volumesnapshotbackup %s", r.req.NamespacedName))
		return false, err
	}

	// fetch vsc clone
	vscClone := snapv1.VolumeSnapshotContent{}
	vscCloneName := fmt.Sprintf("%s-clone", vsb.Spec.VolumeSnapshotContent.Name)
	if err := r.Get(r.Context, types.NamespacedName{Name: vscCloneName}, &vscClone); err != nil {
		r.Log.Error(err, fmt.Sprintf("volumesnapshotcontent clone %s not found", vscCloneName))
		return false, err
	}

	//skip waiting if vsc is ready
	if vscClone.Status != nil && *vscClone.Status.ReadyToUse == true {
		// TODO: handle better
		// this prevents the cloned VS being created too quickly after cloned VSC is created
		// which causes long pending time for the cloned PVC
		time.Sleep(time.Second * 10)
		return true, nil
	}

	time.Sleep(time.Second * 30)
	return true, nil
}

func (r *VolumeSnapshotRestoreReconciler) WaitForVolSyncSnapshotContentToBeReady(log logr.Logger) (bool, error) {

	vsr := volsnapmoverv1alpha1.VolumeSnapshotRestore{}
	if err := r.Get(r.Context, r.req.NamespacedName, &vsr); err != nil {
		// ignore is not found error
		if k8serrors.IsNotFound(err) {
			return true, nil
		}
		r.Log.Error(err, fmt.Sprintf("unable to fetch volumesnapshotrestore %s", r.req.NamespacedName))
		return false, err
	}

	vsc, err := r.getVolSyncSnapshotContent(&vsr)
	if err != nil {
		return false, err
	}

	if vsc == nil || vsc.Status == nil {
		r.Log.Info(fmt.Sprintf("volumesnapshotcontent for vsr %s is not yet ready", vsr.Name))
		return false, nil
	}

	if *vsc.Status.ReadyToUse == true && vsc.Status.SnapshotHandle != nil {
		vsr.Status.SnapshotHandle = *vsc.Status.SnapshotHandle

		// Update VSR status
		err := r.Status().Update(context.Background(), &vsr)
		if err != nil {
			return false, err
		}
		return true, nil
	}
	return false, nil
}

func (r *VolumeSnapshotRestoreReconciler) getVolSyncSnapshotContent(vsr *volsnapmoverv1alpha1.VolumeSnapshotRestore) (*snapv1.VolumeSnapshotContent, error) {
	vsc := snapv1.VolumeSnapshotContent{}
	vs := snapv1.VolumeSnapshot{}

	repDestName := fmt.Sprintf("%s-rep-dest", vsr.Name)
	repDest := volsyncv1alpha1.ReplicationDestination{}
	if err := r.Get(r.Context, types.NamespacedName{Namespace: vsr.Spec.ProtectedNamespace, Name: repDestName}, &repDest); err != nil {
		if k8serrors.IsNotFound(err) {
			return nil, nil
		}
		r.Log.Error(err, fmt.Sprintf("unable to fetch replicationdestination %s/%s", vsr.Spec.ProtectedNamespace, repDestName))
		return nil, err
	}

	if repDest.Status != nil && repDest.Status.LastSyncTime != nil {

		volSyncSnapName := repDest.Status.LatestImage.Name

		// fetch vs from replicationDestination
		if err := r.Get(r.Context, types.NamespacedName{Name: volSyncSnapName, Namespace: vsr.Spec.ProtectedNamespace}, &vs); err != nil {
			r.Log.Error(err, fmt.Sprintf("volumesnapshot %s/%s from VolSync not found", vsr.Spec.ProtectedNamespace, volSyncSnapName))
			return nil, err
		}

		volSyncSnapContentName := *vs.Status.BoundVolumeSnapshotContentName

		// fetch vsc from replicationDestination
		if err := r.Get(r.Context, types.NamespacedName{Name: volSyncSnapContentName}, &vsc); err != nil {
			r.Log.Error(err, fmt.Sprintf("volumesnapshotcontent clone %s not found", volSyncSnapContentName))
			return nil, err
		}

	}

	return &vsc, nil
}
