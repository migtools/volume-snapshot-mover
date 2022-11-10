package controllers

import (
	"context"
	"errors"
	"fmt"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"

	"github.com/go-logr/logr"
	volsnapmoverv1alpha1 "github.com/konveyor/volume-snapshot-mover/api/v1alpha1"
	snapv1 "github.com/kubernetes-csi/external-snapshotter/client/v4/apis/volumesnapshot/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func (r *VolumeSnapshotBackupReconciler) MirrorPVC(log logr.Logger) (bool, error) {
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
		r.Log.Info(fmt.Sprintf("cloned volumesnapshot %s/%s not available in the protected namespace", vsb.Spec.ProtectedNamespace, fmt.Sprintf(vscClone.Spec.VolumeSnapshotRef.Name)))
		return false, nil
	}

	// check if vsClone is ready to use
	if vsClone.Status == nil || vsClone.Status.ReadyToUse == nil || *vsClone.Status.ReadyToUse != true {
		r.Log.Info(fmt.Sprintf("cloned volumesnapshot %s/%s is not ready to use in the protected namespace", vsb.Spec.ProtectedNamespace, fmt.Sprintf(vscClone.Spec.VolumeSnapshotRef.Name)))
		return false, nil
	}

	// Create a PVC with the above volumesnapshot clone as the source

	pvcClone := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-pvc", vsb.Spec.VolumeSnapshotContent.Name),
			Namespace: vsb.Spec.ProtectedNamespace,
			Labels: map[string]string{
				VSBLabel: vsb.Name,
			},
		},
	}

	op, err := controllerutil.CreateOrUpdate(r.Context, r.Client, pvcClone, func() error {

		return r.buildPVCClone(pvcClone, &vsClone)
	})
	if err != nil {
		r.Log.Info(fmt.Sprintf("err building pvc clone: %v", err))
		return false, err
	}
	if op == controllerutil.OperationResultCreated || op == controllerutil.OperationResultUpdated {

		r.EventRecorder.Event(pvcClone,
			corev1.EventTypeNormal,
			"PVCReconciled",
			fmt.Sprintf("performed %s on PVC %s", op, pvcClone.Name),
		)
	}

	return true, nil
}

func (r *VolumeSnapshotBackupReconciler) buildPVCClone(pvcClone *corev1.PersistentVolumeClaim, vsClone *snapv1.VolumeSnapshot) error {
	sourcePVC, err := r.getSourcePVC()
	if err != nil {
		return err
	}

	if pvcClone.CreationTimestamp.IsZero() {
		apiGroup := "snapshot.storage.k8s.io"
		pvcClone.Spec.DataSource = &corev1.TypedLocalObjectReference{
			Name:     vsClone.Name,
			Kind:     vsClone.Kind,
			APIGroup: &apiGroup,
		}

		pvcClone.Spec.AccessModes = sourcePVC.Spec.AccessModes

		pvcClone.Spec.Resources = sourcePVC.Spec.Resources

		pvcClone.Spec.StorageClassName = sourcePVC.Spec.StorageClassName
	}

	return nil
}

func (r *VolumeSnapshotBackupReconciler) BindPVCToDummyPod(log logr.Logger) (bool, error) {
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

	// fetch the cloned PVC
	clonedPVC := corev1.PersistentVolumeClaim{}
	err := r.Get(r.Context,
		types.NamespacedName{Name: fmt.Sprintf("%s-pvc", vsb.Spec.VolumeSnapshotContent.Name), Namespace: vsb.Spec.ProtectedNamespace}, &clonedPVC)
	if err != nil {
		r.Log.Error(err, fmt.Sprintf("unable to fetch cloned PVC %s/%s", vsb.Spec.ProtectedNamespace, fmt.Sprintf("%s-pvc", vsb.Spec.VolumeSnapshotContent.Name)))
		return false, err
	}

	// Bind the above cloned PVC to a dummy pod
	dp := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-pod", vsb.Name),
			Namespace: vsb.Spec.ProtectedNamespace,
			Labels: map[string]string{
				VSBLabel: vsb.Name,
			},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  "busybox",
					Image: "quay.io/ocpmigrate/mssql-sample-app:microsoft",
					Command: []string{
						"/bin/sh", "-c", "tail -f /dev/null",
					},

					VolumeMounts: []corev1.VolumeMount{
						{
							Name:      "vol1",
							MountPath: "/mnt/volume1",
						},
					},
				},
			},
			Volumes: []corev1.Volume{
				{
					Name: "vol1",
					VolumeSource: corev1.VolumeSource{
						PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
							ClaimName: clonedPVC.Name,
						},
					},
				},
			},
			RestartPolicy: corev1.RestartPolicyNever,
		},
	}

	op, err := controllerutil.CreateOrUpdate(r.Context, r.Client, dp, func() error {
		return err
	})

	if err != nil {
		return false, err
	}
	if op == controllerutil.OperationResultCreated || op == controllerutil.OperationResultUpdated {

		r.EventRecorder.Event(dp,
			corev1.EventTypeNormal,
			"PodReconciled",
			fmt.Sprintf("performed %s on pod %s", op, dp.Name),
		)
	}

	return true, nil
}

func (r *VolumeSnapshotBackupReconciler) buildDummyPod(clonedPVC *corev1.PersistentVolumeClaim, p *corev1.Pod) error {

	p.Spec.Containers = []corev1.Container{
		{
			Name:  "busybox",
			Image: DummyPodImage,
			Command: []string{
				"/bin/sh", "-c", "tail -f /dev/null",
			},

			VolumeMounts: []corev1.VolumeMount{
				{
					Name:      "vol1",
					MountPath: "/mnt/volume1",
				},
			},
		},
	}

	p.Spec.Volumes = []corev1.Volume{
		{
			Name: "vol1",
			VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
					ClaimName: clonedPVC.Name,
				},
			},
		},
	}

	p.Spec.RestartPolicy = corev1.RestartPolicyNever

	return nil
}

// Get the source PVC from VSB CR's volumesnapshotcontent
// TODO: Add logic for PVC datasource type in VSB CR
func (r *VolumeSnapshotBackupReconciler) getSourcePVC() (*corev1.PersistentVolumeClaim, error) {

	// Get volumesnapshotbackup from cluster
	vsb := volsnapmoverv1alpha1.VolumeSnapshotBackup{}
	if err := r.Get(r.Context, r.req.NamespacedName, &vsb); err != nil {
		// ignore is not found error
		if k8serrors.IsNotFound(err) {
			return nil, nil
		}
		r.Log.Error(err, fmt.Sprintf("unable to fetch volumesnapshotbackup %s", r.req.NamespacedName))
		return nil, err
	}
	vscInCluster := snapv1.VolumeSnapshotContent{}
	if err := r.Get(r.Context, types.NamespacedName{Name: vsb.Spec.VolumeSnapshotContent.Name}, &vscInCluster); err != nil {
		return nil, errors.New(fmt.Sprintf("cannot obtain source volumesnapshotcontent %s", vsb.Spec.VolumeSnapshotContent.Name))
	}

	vsInCluster := snapv1.VolumeSnapshot{}
	if err := r.Get(r.Context,
		types.NamespacedName{Name: vscInCluster.Spec.VolumeSnapshotRef.Name, Namespace: vsb.Namespace}, &vsInCluster); err != nil {
		return nil, errors.New(fmt.Sprintf("cannot obtain source volumesnapshot %s/%s", vsb.Namespace, vscInCluster.Spec.VolumeSnapshotRef.Name))
	}

	if vsInCluster.Spec.Source.PersistentVolumeClaimName == nil {
		return nil, errors.New(fmt.Sprintf("PVC name not set on volume snapshot %s/%s, cannot run VSB", vsb.Namespace, vscInCluster.Spec.VolumeSnapshotRef.Name))
	}

	pvc := &corev1.PersistentVolumeClaim{}
	if err := r.Get(r.Context,
		types.NamespacedName{Name: *vsInCluster.Spec.Source.PersistentVolumeClaimName, Namespace: vsb.Namespace}, pvc); err != nil {
		return nil, errors.New(fmt.Sprintf("cannot obtain source PVC %s/%s", vsb.Namespace, *vsInCluster.Spec.Source.PersistentVolumeClaimName))
	}

	// set source PVC name in VSB status
	vsb.Status.SourcePVCData.Name = pvc.Name

	// set source PVC size in VSB status
	size := pvc.Spec.Resources.Requests.Storage()
	sizeString := size.String()
	vsb.Status.SourcePVCData.Size = sizeString

	// set source PVC storageclass in VSB status
	if pvc.Spec.StorageClassName != nil {
		storageClass := pvc.Spec.StorageClassName
		vsb.Status.SourcePVCData.StorageClassName = *storageClass
	}

	// Update VSB status
	err := r.Status().Update(context.Background(), &vsb)
	if err != nil {
		return nil, err
	}

	return pvc, nil
}

func (r *VolumeSnapshotBackupReconciler) IsPVCBound(log logr.Logger) (bool, error) {
	// get volumesnapshotbackup from cluster
	vsb := volsnapmoverv1alpha1.VolumeSnapshotBackup{}
	if err := r.Get(r.Context, r.req.NamespacedName, &vsb); err != nil {
		// ignore is not found error
		if k8serrors.IsNotFound(err) {
			return true, nil
		}
		r.Log.Error(err, fmt.Sprintf("unable to fetch volumesnapshotbackup %s", r.req.NamespacedName))
		return false, err
	}

	// get cloned pvc
	pvcName := fmt.Sprintf("%s-pvc", vsb.Spec.VolumeSnapshotContent.Name)
	clonedPVC := corev1.PersistentVolumeClaim{}
	if err := r.Get(r.Context, types.NamespacedName{Namespace: vsb.Spec.ProtectedNamespace, Name: pvcName}, &clonedPVC); err != nil {
		r.Log.Error(err, fmt.Sprintf("unable to fetch cloned PVC %s/%s", vsb.Spec.ProtectedNamespace, pvcName))
		return false, err
	}

	// move forward to create replication source only when the PVC is bound
	if clonedPVC.Status.Phase != corev1.ClaimBound {
		r.Log.Info(fmt.Sprintf("cloned PVC %s/%s is not in bound state", vsb.Spec.ProtectedNamespace, pvcName))
		return false, nil
	}

	return true, nil

}
