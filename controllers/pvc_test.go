package controllers

import (
	"context"
	"reflect"
	"testing"

	"github.com/go-logr/logr"
	pvcv1alpha1 "github.com/konveyor/volume-snapshot-mover/api/v1alpha1"
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
var vscName string = "dummy-snapshot-clone"

func getSchemeForFakeClient() (*runtime.Scheme, error) {
	err := pvcv1alpha1.AddToScheme(scheme.Scheme)
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

func TestDataMoverBackupReconciler_getSourcePVC(t *testing.T) {
	tests := []struct {
		name    string
		dmb     *pvcv1alpha1.DataMoverBackup
		vsc     *snapv1.VolumeSnapshotContent
		vs      *snapv1.VolumeSnapshot
		pvc     *corev1.PersistentVolumeClaim
		want    *corev1.PersistentVolumeClaim
		wantErr bool
	}{
		// TODO: Add test cases.
		{
			name: "Given DMB CR, there should be a valid source PVC returned",
			dmb: &pvcv1alpha1.DataMoverBackup{
				ObjectMeta: v1.ObjectMeta{
					Name:      "sample-dmb",
					Namespace: "bar",
				},
				Spec: pvcv1alpha1.DataMoverBackupSpec{
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
						Namespace: "foo",
					},
				},
			},

			vs: &snapv1.VolumeSnapshot{
				ObjectMeta: v1.ObjectMeta{
					Name:      "sample-vs",
					Namespace: "foo",
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
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeClient, err := getFakeClientFromObjects(tt.dmb, tt.vs, tt.vsc, tt.pvc)
			if err != nil {
				t.Errorf("error creating fake client, likely programmer error")
			}
			r := &DataMoverBackupReconciler{
				Client:  fakeClient,
				Scheme:  fakeClient.Scheme(),
				Log:     logr.Discard(),
				Context: newContextForTest(tt.name),
				NamespacedName: types.NamespacedName{
					Namespace: tt.dmb.Namespace,
					Name:      tt.dmb.Name,
				},
				EventRecorder: record.NewFakeRecorder(10),
				req: reconcile.Request{
					NamespacedName: types.NamespacedName{
						Namespace: tt.dmb.Namespace,
						Name:      tt.dmb.Name,
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
				t.Errorf("DataMoverBackupReconciler.getSourcePVC() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got.Name, Wantpvc.Name) {
				t.Errorf("Name does not match DataMoverBackupReconciler.getSourcePVC() = %v, want %v", got, Wantpvc)

			}
			if !reflect.DeepEqual(got.Spec, Wantpvc.Spec) {
				t.Errorf("Spec does not match DataMoverBackupReconciler.getSourcePVC() = %v, want %v", got, Wantpvc)
			}
		})
	}
}
