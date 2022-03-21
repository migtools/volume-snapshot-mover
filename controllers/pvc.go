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
	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-pvc", dmb.Spec.VolumeSnapshotContent.Name),
			Namespace: r.NamespacedName.Namespace,
		},
	}

	// check if pvc exists
	// create pvc only if it doesnt exists
	_ = r.Get(r.Context, r.NamespacedName, pvc)

	if pvc.Status.Phase == "" {

		op, err := controllerutil.CreateOrUpdate(r.Context, r.Client, pvc, func() error {

			err := controllerutil.SetOwnerReference(&dmb, pvc, r.Scheme)
			if err != nil {
				return err
			}
			return r.buildPVC(pvc, &vs)
		})
		if err != nil {
			r.Log.Info(fmt.Sprintf("error: %v", err))
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

		r.Log.Info(fmt.Sprintf("pod: %v", dp))

		op, err = controllerutil.CreateOrUpdate(r.Context, r.Client, dp, func() error {

			err := controllerutil.SetOwnerReference(&dmb, dp, r.Scheme)
			if err != nil {
				r.Log.Info(fmt.Sprintf("error: %v", err))
				return err
			}
			return r.buildPod(pvc, dp)
		})

		if err != nil {
			r.Log.Info(fmt.Sprintf("error: %v", err))
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
				corev1.ResourceStorage: *vs.Status.RestoreSize,
			},
		},
	}
	pvc.Spec = pvcspec
	return nil
}

func (r *DataMoverBackupReconciler) buildPod(pvc *corev1.PersistentVolumeClaim, p *corev1.Pod) error {

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
