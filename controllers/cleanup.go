package controllers

import (
	"fmt"

	volsyncv1alpha1 "github.com/backube/volsync/api/v1alpha1"
	"github.com/go-logr/logr"
	datamoverv1alpha1 "github.com/konveyor/volume-snapshot-mover/api/v1alpha1"
	snapv1 "github.com/kubernetes-csi/external-snapshotter/client/v4/apis/volumesnapshot/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
)

func (r *VolumeSnapshotBackupReconciler) CleanBackupResources(log logr.Logger) (bool, error) {
	r.Log.Info("In function CleanBackupResources")
	// get datamoverbackup from cluster
	dmb := datamoverv1alpha1.VolumeSnapshotBackup{}
	if err := r.Get(r.Context, r.req.NamespacedName, &dmb); err != nil {
		return false, err
	}

	// wait for replicationSource to complete before deleting DMB resources
	if dmb.Status.Phase != datamoverv1alpha1.DatamoverBackupPhaseCompleted {
		r.Log.Info("waiting for datamoverbackup to complete before deleting resources")
		return false, nil
	}

	err := r.deleteVSandVSC(&dmb)
	if err != nil {
		return false, err
	}

	err = r.deleteResticSecret(&dmb)
	if err != nil {
		return false, err
	}

	err = r.deleteRepSource(&dmb)
	if err != nil {
		return false, err
	}

	err = r.deletePod(&dmb)
	if err != nil {
		return false, err
	}

	err = r.deletePVC(&dmb)
	if err != nil {
		return false, err
	}
	r.Log.Info("deleting datamoverbackup resources")
	return true, nil
}

func (r *VolumeSnapshotBackupReconciler) deleteVSandVSC(vsb *datamoverv1alpha1.VolumeSnapshotBackup) error {

	// get source VSC
	vscInCluster := snapv1.VolumeSnapshotContent{}
	if err := r.Get(r.Context, types.NamespacedName{Name: vsb.Spec.VolumeSnapshotContent.Name}, &vscInCluster); err != nil {
		return err
	}

	// get cloned VSC
	vscName := fmt.Sprintf("%s-clone", vscInCluster.Name)
	vsc := snapv1.VolumeSnapshotContent{}
	if err := r.Get(r.Context, types.NamespacedName{Name: vscName}, &vsc); err != nil {
		return err
	}

	// get cloned VS
	vsName := fmt.Sprintf("%s-volumesnapshot", vsb.Spec.VolumeSnapshotContent.Name)
	vs := snapv1.VolumeSnapshot{}
	if err := r.Get(r.Context, types.NamespacedName{Namespace: r.NamespacedName.Namespace, Name: vsName}, &vs); err != nil {
		return err
	}

	// delete cloned VS
	if err := r.Delete(r.Context, &vs); err != nil {
		return err
	}

	// delete cloned VSC
	if err := r.Delete(r.Context, &vsc); err != nil {
		return err
	}
	return nil
}

func (r *VolumeSnapshotBackupReconciler) deleteResticSecret(vsb *datamoverv1alpha1.VolumeSnapshotBackup) error {

	// get restic secret created by controller
	resticSecretName := fmt.Sprintf("%s-secret", vsb.Name)
	resticSecret := corev1.Secret{}
	if err := r.Get(r.Context, types.NamespacedName{Namespace: r.NamespacedName.Namespace, Name: resticSecretName}, &resticSecret); err != nil {
		return err
	}

	// delete controller created restic secret
	if err := r.Delete(r.Context, &resticSecret); err != nil {
		return err
	}
	return nil
}

func (r *VolumeSnapshotBackupReconciler) deleteRepSource(vsb *datamoverv1alpha1.VolumeSnapshotBackup) error {

	// get replicationsource
	repSourceName := fmt.Sprintf("%s-rep-src", vsb.Name)
	repSource := volsyncv1alpha1.ReplicationSource{}
	if err := r.Get(r.Context, types.NamespacedName{Namespace: r.NamespacedName.Namespace, Name: repSourceName}, &repSource); err != nil {
		return err
	}

	// delete replicationsource
	if err := r.Delete(r.Context, &repSource); err != nil {
		return err
	}
	return nil
}

func (r *VolumeSnapshotBackupReconciler) deletePod(vsb *datamoverv1alpha1.VolumeSnapshotBackup) error {

	// get dummy pod
	podName := fmt.Sprintf("%s-pod", vsb.Name)
	dummyPod := corev1.Pod{}
	if err := r.Get(r.Context, types.NamespacedName{Name: podName, Namespace: r.NamespacedName.Namespace}, &dummyPod); err != nil {
		return err
	}

	// delete dummy pod
	if err := r.Delete(r.Context, &dummyPod); err != nil {
		return err
	}
	return nil
}

func (r *VolumeSnapshotBackupReconciler) deletePVC(vsb *datamoverv1alpha1.VolumeSnapshotBackup) error {

	// get cloned pvc
	pvcName := fmt.Sprintf("%s-pvc", vsb.Spec.VolumeSnapshotContent.Name)
	pvc := corev1.PersistentVolumeClaim{}
	if err := r.Get(r.Context, types.NamespacedName{Namespace: r.NamespacedName.Namespace, Name: pvcName}, &pvc); err != nil {
		return err
	}

	// delete cloned pvc
	if err := r.Delete(r.Context, &pvc); err != nil {
		return err
	}
	return nil
}
