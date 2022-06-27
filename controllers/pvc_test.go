package controllers

import (
	"context"
	"reflect"
	"testing"

	"github.com/go-logr/logr"
	volsnapmoverv1alpha1 "github.com/konveyor/volume-snapshot-mover/api/v1alpha1"
	snapv1 "github.com/kubernetes-csi/external-snapshotter/client/v4/apis/volumesnapshot/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var pvcName string = "sample-pvc"

func getSchemeForFakeClient() (*runtime.Scheme, error) {
	err := volsnapmoverv1alpha1.AddToScheme(scheme.Scheme)
	if err != nil {
		return nil, err
	}
	err = snapv1.AddToScheme(scheme.Scheme)
	if err != nil {
		return nil, err
	}
	return scheme.Scheme, nil
}

func getFakeClientFromObjects(objs ...client.Object) (client.WithWatch, error) {
	schemeForFakeClient, err := getSchemeForFakeClient()
	if err != nil {
		return nil, err
	}

	return fake.NewClientBuilder().WithScheme(schemeForFakeClient).WithObjects(objs...).Build(), nil
}
func newContextForTest(name string) context.Context {
	return context.TODO()
}

func TestVolumeSnapshotMoverBackupReconciler_getSourcePVC(t *testing.T) {
	tests := []struct {
		name    string
		vsb     *volsnapmoverv1alpha1.VolumeSnapshotBackup
		vsc     *snapv1.VolumeSnapshotContent
		vs      *snapv1.VolumeSnapshot
		pvc     *corev1.PersistentVolumeClaim
		want    *corev1.PersistentVolumeClaim
		wantErr bool
	}{
		// TODO: Add test cases.
		{
			name: "Given VSB CR, there should be a valid source PVC returned",
			vsb: &volsnapmoverv1alpha1.VolumeSnapshotBackup{
				ObjectMeta: v1.ObjectMeta{
					Name:      "sample-vsb",
					Namespace: "bar",
				},
				Spec: volsnapmoverv1alpha1.VolumeSnapshotBackupSpec{
					VolumeSnapshotContent: corev1.ObjectReference{
						Name: "sample-snapshot",
					},
					ProtectedNamespace: "foo",
				},
			},
			vsc: &snapv1.VolumeSnapshotContent{
				ObjectMeta: v1.ObjectMeta{
					Name: "sample-snapshot",
				},
				Spec: snapv1.VolumeSnapshotContentSpec{
					VolumeSnapshotRef: corev1.ObjectReference{
						Name:      "sample-vs",
						Namespace: "bar",
					},
				},
			},

			vs: &snapv1.VolumeSnapshot{
				ObjectMeta: v1.ObjectMeta{
					Name:      "sample-vs",
					Namespace: "bar",
				},
				Spec: snapv1.VolumeSnapshotSpec{
					Source: snapv1.VolumeSnapshotSource{
						PersistentVolumeClaimName: &pvcName,
					},
				},
			},
			pvc: &corev1.PersistentVolumeClaim{
				ObjectMeta: v1.ObjectMeta{
					Name:      "sample-pvc",
					Namespace: "bar",
				},
				Spec: corev1.PersistentVolumeClaimSpec{
					AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceName(corev1.ResourceStorage): resource.MustParse("10Gi"),
						},
					},
				},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeClient, err := getFakeClientFromObjects(tt.vsb, tt.vs, tt.vsc, tt.pvc)
			if err != nil {
				t.Errorf("error creating fake client, likely programmer error")
			}
			r := &VolumeSnapshotBackupReconciler{
				Client:  fakeClient,
				Scheme:  fakeClient.Scheme(),
				Log:     logr.Discard(),
				Context: newContextForTest(tt.name),
				NamespacedName: types.NamespacedName{
					Namespace: tt.vsb.Spec.ProtectedNamespace,
					Name:      tt.vsb.Name,
				},
				EventRecorder: record.NewFakeRecorder(10),
				req: reconcile.Request{
					NamespacedName: types.NamespacedName{
						Namespace: tt.vsb.Namespace,
						Name:      tt.vsb.Name,
					},
				},
			}
			Wantpvc := &corev1.PersistentVolumeClaim{
				ObjectMeta: v1.ObjectMeta{
					Name:      "sample-pvc",
					Namespace: "foo",
				},
				Spec: corev1.PersistentVolumeClaimSpec{
					AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceName(corev1.ResourceStorage): resource.MustParse("10Gi"),
						},
					},
				},
			}
			got, err := r.getSourcePVC()
			if (err != nil) != tt.wantErr {
				t.Errorf("VolumeSnapshotMoverBackupReconciler.getSourcePVC() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got.Name, Wantpvc.Name) {
				t.Errorf("Name does not match VolumeSnapshotMoverBackupReconciler.getSourcePVC() = %v, want %v", got, Wantpvc)

			}
			if !reflect.DeepEqual(got.Spec, Wantpvc.Spec) {
				t.Errorf("Spec does not match VolumeSnapshotMoverBackupReconciler.getSourcePVC() = %v, want %v", got, Wantpvc)
			}
		})
	}
}
