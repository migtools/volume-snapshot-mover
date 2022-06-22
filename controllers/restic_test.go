package controllers

import (
	"testing"

	"github.com/go-logr/logr"
	datamoverv1alpha1 "github.com/konveyor/volume-snapshot-mover/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func TestDataMoverBackupReconciler_CreateResticSecret(t *testing.T) {
	tests := []struct {
		name             string
		vsb              *datamoverv1alpha1.VolumeSnapshotBackup
		secret, rpsecret *corev1.Secret
		pvc              *corev1.PersistentVolumeClaim
		want             bool
		wantErr          bool
	}{
		// TODO: Add test cases.
		{
			name: "Given invalid pvc -> error in restic secret creation",
			vsb: &datamoverv1alpha1.VolumeSnapshotBackup{
				ObjectMeta: v1.ObjectMeta{
					Name:      "sample-vsb",
					Namespace: "bar",
				},
				Spec: datamoverv1alpha1.VolumeSnapshotBackupSpec{
					VolumeSnapshotContent: corev1.ObjectReference{
						Name: "sample-snapshot",
					},
					ProtectedNamespace: "foo",
				},
			},
			pvc: &corev1.PersistentVolumeClaim{
				ObjectMeta: v1.ObjectMeta{
					Name:      "sample-snapshot",
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
			secret: &corev1.Secret{
				ObjectMeta: v1.ObjectMeta{
					Name:      resticSecret,
					Namespace: namespace,
				},
				Data: secretData,
			},
			rpsecret: &corev1.Secret{
				ObjectMeta: v1.ObjectMeta{
					Name:      "sample-vsb-secret",
					Namespace: namespace,
				},
			},
			want:    false,
			wantErr: true,
		},
		{
			name: "Given valid vsb,restic secret -> successful creation of pvc specific restic secret",
			vsb: &datamoverv1alpha1.VolumeSnapshotBackup{
				ObjectMeta: v1.ObjectMeta{
					Name:      "sample-vsb",
					Namespace: "bar",
				},
				Spec: datamoverv1alpha1.VolumeSnapshotBackupSpec{
					VolumeSnapshotContent: corev1.ObjectReference{
						Name: "sample-snapshot",
					},
					ProtectedNamespace: "foo",
				},
			},
			pvc: &corev1.PersistentVolumeClaim{
				ObjectMeta: v1.ObjectMeta{
					Name:      "sample-snapshot-pvc",
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
			secret: &corev1.Secret{
				ObjectMeta: v1.ObjectMeta{
					Name:      resticSecret,
					Namespace: namespace,
				},
				Data: secretData,
			},
			rpsecret: &corev1.Secret{
				ObjectMeta: v1.ObjectMeta{
					Name:      "sample-vsb-secret",
					Namespace: namespace,
				},
			},
			want:    true,
			wantErr: false,
		},
		{
			name: "Given invalid vsb -> error in restic secret creation",
			vsb: &datamoverv1alpha1.VolumeSnapshotBackup{
				ObjectMeta: v1.ObjectMeta{
					Name:      "sample-vsb",
					Namespace: "bar",
				},
				Spec: datamoverv1alpha1.VolumeSnapshotBackupSpec{
					VolumeSnapshotContent: corev1.ObjectReference{
						Name: "sample-snapshot",
					},
					ProtectedNamespace: "bar",
				},
			},
			pvc: &corev1.PersistentVolumeClaim{
				ObjectMeta: v1.ObjectMeta{
					Name:      "sample-snapshot",
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
			secret: &corev1.Secret{
				ObjectMeta: v1.ObjectMeta{
					Name:      resticSecret,
					Namespace: namespace,
				},
				Data: secretData,
			},
			rpsecret: &corev1.Secret{
				ObjectMeta: v1.ObjectMeta{
					Name:      "sample-vsb-secret",
					Namespace: namespace,
				},
			},
			want:    false,
			wantErr: true,
		},
		{
			name: "Given invalid base secret -> error in restic secret creation",
			vsb: &datamoverv1alpha1.VolumeSnapshotBackup{
				ObjectMeta: v1.ObjectMeta{
					Name:      "sample-vsb",
					Namespace: "bar",
				},
				Spec: datamoverv1alpha1.VolumeSnapshotBackupSpec{
					VolumeSnapshotContent: corev1.ObjectReference{
						Name: "sample-snapshot",
					},
					ProtectedNamespace: "foo",
				},
			},
			pvc: &corev1.PersistentVolumeClaim{
				ObjectMeta: v1.ObjectMeta{
					Name:      "sample-snapshot-pvc",
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
			secret: &corev1.Secret{
				ObjectMeta: v1.ObjectMeta{
					Name:      "restic",
					Namespace: namespace,
				},
				Data: make(map[string][]byte),
			},
			rpsecret: &corev1.Secret{
				ObjectMeta: v1.ObjectMeta{
					Name:      "sample-vsb-secret",
					Namespace: namespace,
				},
			},
			want:    false,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeClient, err := getFakeClientFromObjects(tt.vsb, tt.secret, tt.pvc, tt.rpsecret)
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
			got, err := r.CreateVSBResticSecret(r.Log)
			if (err != nil) != tt.wantErr {
				t.Errorf("VolumeSnapshotMoverBackupReconciler.CreateResticSecret() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("VolumeSnapshotMoverBackupReconciler.CreateResticSecret() = %v, want %v", got, tt.want)
			}
		})
	}
}
