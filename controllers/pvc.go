package controllers

import (
	"errors"
	"fmt"

	"github.com/go-logr/logr"
	pvcv1alpha1 "github.com/konveyor/volume-snapshot-mover/api/v1alpha1"
	snapv1 "github.com/kubernetes-csi/external-snapshotter/client/v4/apis/volumesnapshot/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func (r *DataMoverBackupReconciler) BindPVC(log logr.Logger) (bool, error) {
	// Get datamoverbackup from cluster
	dmb := pvcv1alpha1.DataMoverBackup{}
	if err := r.Get(r.Context, r.NamespacedName, &dmb); err != nil {
		return false, err
	}
	// Check if Volumesnapshot is present in the protected namespace
	vs := snapv1.VolumeSnapshot{}
	if err := r.Get(r.Context,
		types.NamespacedName{Name: fmt.Sprintf("%s-volumesnapshot", dmb.Spec.VolumeSnapshotContent.Name), Namespace: r.NamespacedName.Namespace}, &vs); err != nil {
		return false, errors.New("cloned volumesnapshot not available in the protected namespace")
	}

	// Create a PVC with the above volumesnapshot as the source
	pvc := corev1.PersistentVolumeClaim{}
	_ = r.Get(r.Context,
		types.NamespacedName{Name: fmt.Sprintf("%s-pvc", dmb.Spec.VolumeSnapshotContent.Name), Namespace: r.NamespacedName.Namespace}, &pvc)
	// Check if the exists or not.
	// If exists, do nothing
	// If not, create a PVC and bind it to static pod
	if pvc.Name == "" {

		pvc := &corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name:      fmt.Sprintf("%s-pvc", dmb.Spec.VolumeSnapshotContent.Name),
				Namespace: r.NamespacedName.Namespace,
			},
		}

		op, err := controllerutil.CreateOrUpdate(r.Context, r.Client, pvc, func() error {

			err := controllerutil.SetOwnerReference(&dmb, pvc, r.Scheme)
			if err != nil {
				return err
			}
			return r.buildPVC(pvc, &vs)
		})
		if err != nil {
			return false, err
		}
		if op == controllerutil.OperationResultCreated || op == controllerutil.OperationResultUpdated {

			r.EventRecorder.Event(pvc,
				corev1.EventTypeNormal,
				"PVCReconciled",
				fmt.Sprintf("performed %s on PVC %s", op, pvc.Name),
			)
		}

		// Bind the above PVC to a dummy pod
		dp := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      fmt.Sprintf("%s-pod", dmb.Name),
				Namespace: r.NamespacedName.Namespace,
			},
		}

		op, err = controllerutil.CreateOrUpdate(r.Context, r.Client, dp, func() error {

			err := controllerutil.SetOwnerReference(&dmb, dp, r.Scheme)
			if err != nil {
				return err
			}
			return r.buildDummyPod(pvc, dp)
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
	}

	return true, nil
}

func (r *DataMoverBackupReconciler) buildPVC(pvc *corev1.PersistentVolumeClaim, vs *snapv1.VolumeSnapshot) error {
	p, err := r.getSourcePVC()
	if err != nil {
		return err
	}
	pvcspec := corev1.PersistentVolumeClaimSpec{
		DataSource: &corev1.TypedLocalObjectReference{
			Name:     vs.Name,
			Kind:     vs.Kind,
			APIGroup: &vs.APIVersion,
		},
		AccessModes: []corev1.PersistentVolumeAccessMode{
			"ReadWriteOnce",
		},
		Resources: corev1.ResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceStorage: *p.Spec.Resources.Requests.Storage(),
			},
		},
		StorageClassName: p.Spec.StorageClassName,
	}
	pvc.Spec = pvcspec
	return nil
}

func (r *DataMoverBackupReconciler) buildDummyPod(pvc *corev1.PersistentVolumeClaim, p *corev1.Pod) error {

	podspec := corev1.PodSpec{
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
						ClaimName: pvc.Name,
					},
				},
			},
		},
		RestartPolicy: corev1.RestartPolicyNever,
	}

	p.Spec = podspec
	return nil
}

// Get the source PVC from DMB CR's volumesnapshotcontent
// TODO: Add logic for PVC datasource type in DMB CR
func (r *DataMoverBackupReconciler) getSourcePVC() (*corev1.PersistentVolumeClaim, error) {

	// Get datamoverbackup from cluster
	dmb := pvcv1alpha1.DataMoverBackup{}
	if err := r.Get(r.Context, r.NamespacedName, &dmb); err != nil {
		return nil, err
	}
	vscInCluster := snapv1.VolumeSnapshotContent{}
	if err := r.Get(r.Context, types.NamespacedName{Name: dmb.Spec.VolumeSnapshotContent.Name}, &vscInCluster); err != nil {
		return nil, errors.New("cannot obtain source volumesnapshotcontent")
	}

	vsInCluster := snapv1.VolumeSnapshot{}
	if err := r.Get(r.Context,
		types.NamespacedName{Name: vscInCluster.Spec.VolumeSnapshotRef.Name, Namespace: vscInCluster.Spec.VolumeSnapshotRef.Namespace}, &vsInCluster); err != nil {
		return nil, errors.New("cannot obtain source volumesnapshot")
	}

	pvc := &corev1.PersistentVolumeClaim{}
	if err := r.Get(r.Context,
		types.NamespacedName{Name: *vsInCluster.Spec.Source.PersistentVolumeClaimName, Namespace: vsInCluster.ObjectMeta.Namespace}, pvc); err != nil {
		return nil, errors.New("cannot obtain source PVC")
	}

	return pvc, nil
}
