package controllers

import (
	"context"
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

func (r *VolumeSnapshotBackupReconciler) BindPVC(log logr.Logger) (bool, error) {
	// Get volumesnapshotbackup from cluster
	vsb := datamoverv1alpha1.VolumeSnapshotBackup{}
	if err := r.Get(r.Context, r.req.NamespacedName, &vsb); err != nil {
		r.Log.Error(err, "unable to fetch VolumeSnapshotBackup CR")
		return false, err
	}
	// Check if Volumesnapshot is present in the protected namespace
	vs := snapv1.VolumeSnapshot{}
	if err := r.Get(r.Context,
		types.NamespacedName{Name: fmt.Sprintf("%s-volumesnapshot", vsb.Spec.VolumeSnapshotContent.Name), Namespace: vsb.Spec.ProtectedNamespace}, &vs); err != nil {
		r.Log.Info("cloned volumesnapshot not available in the protected namespace")
		return false, nil
	}

	// Create a PVC with the above volumesnapshot as the source
	pvc := corev1.PersistentVolumeClaim{}
	_ = r.Get(r.Context,
		types.NamespacedName{Name: fmt.Sprintf("%s-pvc", vsb.Spec.VolumeSnapshotContent.Name), Namespace: r.NamespacedName.Namespace}, &pvc)
	// Check if the exists or not.
	// If exists, do nothing
	// If not, create a PVC and bind it to static pod
	if pvc.Name == "" {

		pvc := &corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name:      fmt.Sprintf("%s-pvc", vsb.Spec.VolumeSnapshotContent.Name),
				Namespace: r.NamespacedName.Namespace,
				Labels: map[string]string{
					VSBLabel: vsb.Name,
				},
			},
		}

		op, err := controllerutil.CreateOrUpdate(r.Context, r.Client, pvc, func() error {

			return r.buildPVC(pvc, &vs)
		})
		if err != nil {
			r.Log.Info(fmt.Sprintf("err: %v", err))
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
				Name:      fmt.Sprintf("%s-pod", vsb.Name),
				Namespace: r.NamespacedName.Namespace,
				Labels: map[string]string{
					VSBLabel: vsb.Name,
				},
			},
		}

		op, err = controllerutil.CreateOrUpdate(r.Context, r.Client, dp, func() error {

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

func (r *VolumeSnapshotBackupReconciler) buildPVC(pvc *corev1.PersistentVolumeClaim, vs *snapv1.VolumeSnapshot) error {
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

func (r *VolumeSnapshotBackupReconciler) buildDummyPod(pvc *corev1.PersistentVolumeClaim, p *corev1.Pod) error {

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
func (r *VolumeSnapshotBackupReconciler) getSourcePVC() (*corev1.PersistentVolumeClaim, error) {

	// Get volumesnapshotbackup from cluster
	vsb := datamoverv1alpha1.VolumeSnapshotBackup{}
	if err := r.Get(r.Context, r.req.NamespacedName, &vsb); err != nil {
		return nil, err
	}
	vscInCluster := snapv1.VolumeSnapshotContent{}
	if err := r.Get(r.Context, types.NamespacedName{Name: vsb.Spec.VolumeSnapshotContent.Name}, &vscInCluster); err != nil {
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

	// set source PVC name in DMB status
	vsb.Status.SourcePVCData.Name = pvc.Name

	// set source PVC size in DMB status
	size := pvc.Spec.Resources.Requests.Storage()
	sizeString := size.String()
	vsb.Status.SourcePVCData.Size = sizeString

	// Update DMB status
	err := r.Status().Update(context.Background(), &vsb)
	if err != nil {
		return nil, err
	}

	return pvc, nil
}
