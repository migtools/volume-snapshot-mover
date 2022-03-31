package controllers

import (
	"errors"
	"fmt"
	"time"

	volsyncv1alpha1 "github.com/backube/volsync/api/v1alpha1"
	"github.com/go-logr/logr"
	pvcv1alpha1 "github.com/konveyor/volume-snapshot-mover/api/v1alpha1"
	snapv1 "github.com/kubernetes-csi/external-snapshotter/client/v4/apis/volumesnapshot/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
)

func (r *DataMoverBackupReconciler) CleanBackupResources(log logr.Logger) (bool, error) {

	// get datamoverbackup
	dmb := pvcv1alpha1.DataMoverBackup{}
	if err := r.Get(r.Context, r.NamespacedName, &dmb); err != nil {
		fmt.Printf("err: %v\n", err)
		return false, err
	}

	// get cloned pvc
	pvcName := fmt.Sprintf("%s-pvc", dmb.Spec.VolumeSnapshotContent.Name)
	pvc := corev1.PersistentVolumeClaim{}
	if err := r.Get(r.Context, types.NamespacedName{Namespace: r.NamespacedName.Namespace, Name: pvcName}, &pvc); err != nil {
		return false, err
	}

	// get replicationsource
	repSourceName := fmt.Sprintf("%s-backup", pvc.Name)
	repSource := volsyncv1alpha1.ReplicationSource{}
	if err := r.Get(r.Context, types.NamespacedName{Namespace: r.NamespacedName.Namespace, Name: repSourceName}, &repSource); err != nil {
		return false, err
	}

	err := r.deleteVSandVSC(&dmb)
	if err != nil {
		return false, err
	}

	err = r.deleteResticSecret(&pvc)
	if err != nil {
		return false, err
	}

	err = r.deleteRepSource(&repSource, &pvc)
	if err != nil {
		return false, err
	}

	err = r.deletePod(&dmb)
	if err != nil {
		return false, err
	}

	//  ********** TODO ********** Fix below for wait func

	// wait for all other resources to be deleted before deleting cloned pvc
	fmt.Println("waiting for resources to be deleted")
	err = wait.PollImmediate(5*time.Second, 2*time.Minute, r.areResourcesDeleted(&dmb, &pvc))
	if err != nil {
		fmt.Printf("err: %v\n", err)
		return false, errors.New("failed to delete all resources")
	}

	err = r.deletePVC(&pvc)
	if err != nil {
		return false, err
	}

	// wait for cloned pvc to be deleted before deleting DMB

	err = r.deleteDataMoverBackup(&dmb)
	if err != nil {
		return false, err
	}

	return true, nil
}

func (r *DataMoverBackupReconciler) deleteVSandVSC(dmb *pvcv1alpha1.DataMoverBackup) error {

	// get source vsc
	vscInCluster := snapv1.VolumeSnapshotContent{}
	if err := r.Get(r.Context, types.NamespacedName{Name: dmb.Spec.VolumeSnapshotContent.Name}, &vscInCluster); err != nil {
		return err
	}

	// get cloned vsc
	vscName := fmt.Sprintf("%s-clone", vscInCluster.Name)
	vsc := snapv1.VolumeSnapshotContent{}
	if err := r.Get(r.Context, types.NamespacedName{Name: vscName}, &vsc); err != nil {
		return err
	}

	// get cloned volumesnapshot
	vsName := fmt.Sprintf("%s-volumesnapshot", dmb.Spec.VolumeSnapshotContent.Name)
	vs := snapv1.VolumeSnapshot{}
	if err := r.Get(r.Context, types.NamespacedName{Namespace: r.NamespacedName.Namespace, Name: vsName}, &vs); err != nil {
		return err
	}

	// delete cloned VS
	if err := r.Delete(r.Context, &vs); err != nil {
		return err
	}

	// TODO: delete cloned VSC

	return nil
}

func (r *DataMoverBackupReconciler) deleteResticSecret(pvc *corev1.PersistentVolumeClaim) error {

	// get restic secret created by controller
	resticSecretName := fmt.Sprintf("%s-secret", pvc.Name)
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

func (r *DataMoverBackupReconciler) deleteRepSource(replicationSource *volsyncv1alpha1.ReplicationSource, pvc *corev1.PersistentVolumeClaim) error {

	// delete replicationsource
	if err := r.Delete(r.Context, replicationSource); err != nil {
		return err
	}
	return nil
}

func (r *DataMoverBackupReconciler) deletePod(dmb *pvcv1alpha1.DataMoverBackup) error {

	// get dummy pod
	podName := fmt.Sprintf("%s-pod", dmb.Name)
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

func (r *DataMoverBackupReconciler) deletePVC(pvc *corev1.PersistentVolumeClaim) error {

	// delete cloned pvc
	if err := r.Delete(r.Context, pvc); err != nil {
		return err
	}
	return nil
}

func (r *DataMoverBackupReconciler) deleteDataMoverBackup(dmb *pvcv1alpha1.DataMoverBackup) error {

	// TODO: delete dmb

	return nil
}

func (r *DataMoverBackupReconciler) areResourcesDeleted(dmb *pvcv1alpha1.DataMoverBackup, pvc *corev1.PersistentVolumeClaim) wait.ConditionFunc {
	return func() (bool, error) {

		// get dummy pod
		podName := fmt.Sprintf("%s-pod", dmb.Name)
		dummyPod := corev1.Pod{}
		err := r.Get(r.Context, types.NamespacedName{Name: podName, Namespace: r.NamespacedName.Namespace}, &dummyPod)
		if apierrors.IsNotFound(err) {
			return true, nil
		}

		// get replicationsource
		repSourceName := fmt.Sprintf("%s-backup", pvc.Name)
		repSource := volsyncv1alpha1.ReplicationSource{}
		err = r.Get(r.Context, types.NamespacedName{Namespace: r.NamespacedName.Namespace, Name: repSourceName}, &repSource)
		if apierrors.IsNotFound(err) {
			return true, nil
		}

		// get restic secret created by controller
		resticSecretName := fmt.Sprintf("%s-secret", pvc.Name)
		resticSecret := corev1.Secret{}
		err = r.Get(r.Context, types.NamespacedName{Namespace: r.NamespacedName.Namespace, Name: resticSecretName}, &resticSecret)
		if apierrors.IsNotFound(err) {
			return true, nil
		}

		// get cloned VS
		vsName := fmt.Sprintf("%s-volumesnapshot", dmb.Spec.VolumeSnapshotContent.Name)
		vs := snapv1.VolumeSnapshot{}
		err = r.Get(r.Context, types.NamespacedName{Namespace: r.NamespacedName.Namespace, Name: vsName}, &vs)
		if apierrors.IsNotFound(err) {
			return true, nil
		}

		// TODO: get cloned VSC

		return false, errors.New("failed to delete all DMB resources")

	}
}
