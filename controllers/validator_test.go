package controllers

import (
	"testing"

	"github.com/go-logr/logr"
	datamoverv1alpha1 "github.com/konveyor/volume-snapshot-mover/api/v1alpha1"
	snapv1 "github.com/kubernetes-csi/external-snapshotter/client/v4/apis/volumesnapshot/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func TestDataMoverBackupReconciler_ValidateDataMoverBackup(t *testing.T) {
	tests := []struct {
		name    string
		dmb     *datamoverv1alpha1.VolumeSnapshotBackup
		vsc     *snapv1.VolumeSnapshotContent
		want    bool
		wantErr bool
	}{
		// TODO: Add test cases.
		{
			name: "Given valid DMB CR -> no validation errors",
			dmb: &datamoverv1alpha1.VolumeSnapshotBackup{
				ObjectMeta: v1.ObjectMeta{
					Name:      "sample-dmb",
					Namespace: "bar",
				},
				Spec: datamoverv1alpha1.VolumeSnapshotBackupSpec{
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
						Name: "sample-vs",
					},
				},
			},
			want:    true,
			wantErr: false,
		},
		{
			name: "Given an invalid DMB CR ->  validation errors",
			dmb: &datamoverv1alpha1.VolumeSnapshotBackup{
				ObjectMeta: v1.ObjectMeta{
					Name:      "sample-dmb",
					Namespace: "bar",
				},
				Spec: datamoverv1alpha1.VolumeSnapshotBackupSpec{
					VolumeSnapshotContent: corev1.ObjectReference{},
					ProtectedNamespace:    "foo",
				},
			},
			vsc: &snapv1.VolumeSnapshotContent{
				ObjectMeta: v1.ObjectMeta{
					Name: "sample-snapshot",
				},
				Spec: snapv1.VolumeSnapshotContentSpec{
					VolumeSnapshotRef: corev1.ObjectReference{
						Name: "sample-vs",
					},
				},
			},
			want:    false,
			wantErr: true,
		},
		{
			name: "Given an invalid VSC ->  validation errors",
			dmb: &datamoverv1alpha1.VolumeSnapshotBackup{
				ObjectMeta: v1.ObjectMeta{
					Name:      "sample-dmb",
					Namespace: "bar",
				},
				Spec: datamoverv1alpha1.VolumeSnapshotBackupSpec{
					VolumeSnapshotContent: corev1.ObjectReference{
						Name: "sample-snapshot-vsc",
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
						Name: "sample-vs",
					},
				},
			},
			want:    false,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeClient, err := getFakeClientFromObjects(tt.dmb, tt.vsc)
			if err != nil {
				t.Errorf("error creating fake client, likely programmer error")
			}
			r := &VolumeSnapshotBackupReconciler{
				Client:  fakeClient,
				Scheme:  fakeClient.Scheme(),
				Log:     logr.Discard(),
				Context: newContextForTest(tt.name),
				NamespacedName: types.NamespacedName{
					Namespace: tt.dmb.Spec.ProtectedNamespace,
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
			got, err := r.ValidateDataMoverBackup(r.Log)
			if (err != nil) != tt.wantErr {
				t.Errorf("DataMoverBackupReconciler.ValidateDataMoverBackup() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("DataMoverBackupReconciler.ValidateDataMoverBackup() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDataMoverRestoreReconciler_ValidateDataMoverRestore(t *testing.T) {
	tests := []struct {
		name    string
		dmr     *datamoverv1alpha1.VolumeSnapshotRestore
		wantErr bool
		want    bool
	}{
		// TODO: Add test cases.
		{
			name: "valid DMR -> no validation errors",
			dmr: &datamoverv1alpha1.VolumeSnapshotRestore{
				ObjectMeta: v1.ObjectMeta{
					Name:      "sample-dmr",
					Namespace: "bar",
				},
				Spec: datamoverv1alpha1.VolumeSnapshotRestoreSpec{
					ResticSecretRef: corev1.LocalObjectReference{
						Name: resticSecret,
					},
					ProtectedNamespace: "foo",
					DataMoverBackupref: datamoverv1alpha1.DMBRef{
						ResticRepository: "s3://sample-path/snapshots",
						BackedUpPVCData: datamoverv1alpha1.PVCData{
							Name: "sample-pvc",
							Size: "10Gi",
						},
					},
				},
			},
			want:    true,
			wantErr: false,
		},
		{
			name: "empty protected ns -> no validation errors",
			dmr: &datamoverv1alpha1.VolumeSnapshotRestore{
				ObjectMeta: v1.ObjectMeta{
					Name:      "sample-dmr",
					Namespace: "bar",
				},
				Spec: datamoverv1alpha1.VolumeSnapshotRestoreSpec{
					ResticSecretRef: corev1.LocalObjectReference{
						Name: resticSecret,
					},
					DataMoverBackupref: datamoverv1alpha1.DMBRef{
						ResticRepository: "s3://sample-path/snapshots",
						BackedUpPVCData: datamoverv1alpha1.PVCData{
							Name: "sample-pvc",
							Size: "10Gi",
						},
					},
				},
			},
			want:    false,
			wantErr: true,
		},
		{
			name: "empty restic repository -> validation errors",
			dmr: &datamoverv1alpha1.VolumeSnapshotRestore{
				ObjectMeta: v1.ObjectMeta{
					Name:      "sample-dmr",
					Namespace: "bar",
				},
				Spec: datamoverv1alpha1.VolumeSnapshotRestoreSpec{
					ResticSecretRef: corev1.LocalObjectReference{
						Name: resticSecret,
					},
					ProtectedNamespace: "foo",
					DataMoverBackupref: datamoverv1alpha1.DMBRef{
						ResticRepository: "",
						BackedUpPVCData: datamoverv1alpha1.PVCData{
							Name: "sample-pvc",
							Size: "10Gi",
						},
					},
				},
			},
			want:    false,
			wantErr: true,
		},
		{
			name: "empty pvc name -> validation errors",
			dmr: &datamoverv1alpha1.VolumeSnapshotRestore{
				ObjectMeta: v1.ObjectMeta{
					Name:      "sample-dmr",
					Namespace: "bar",
				},
				Spec: datamoverv1alpha1.VolumeSnapshotRestoreSpec{
					ResticSecretRef: corev1.LocalObjectReference{
						Name: resticSecret,
					},
					ProtectedNamespace: "foo",
					DataMoverBackupref: datamoverv1alpha1.DMBRef{
						ResticRepository: "s3://sample-path/snapshots",
						BackedUpPVCData: datamoverv1alpha1.PVCData{
							Name: "",
							Size: "10Gi",
						},
					},
				},
			},
			want:    false,
			wantErr: true,
		},
		{
			name: "empty pvc size -> validation errors",
			dmr: &datamoverv1alpha1.VolumeSnapshotRestore{
				ObjectMeta: v1.ObjectMeta{
					Name:      "sample-dmr",
					Namespace: "bar",
				},
				Spec: datamoverv1alpha1.VolumeSnapshotRestoreSpec{
					ResticSecretRef: corev1.LocalObjectReference{
						Name: resticSecret,
					},
					ProtectedNamespace: "foo",
					DataMoverBackupref: datamoverv1alpha1.DMBRef{
						ResticRepository: "s3://sample-path/snapshots",
						BackedUpPVCData: datamoverv1alpha1.PVCData{
							Name: "sample-pvc",
							Size: "",
						},
					},
				},
			},
			want:    false,
			wantErr: true,
		},
		{
			name: "empty secret ->  validation errors",
			dmr: &datamoverv1alpha1.VolumeSnapshotRestore{
				ObjectMeta: v1.ObjectMeta{
					Name:      "sample-dmr",
					Namespace: "bar",
				},
				Spec: datamoverv1alpha1.VolumeSnapshotRestoreSpec{
					ResticSecretRef:    corev1.LocalObjectReference{},
					ProtectedNamespace: "foo",
					DataMoverBackupref: datamoverv1alpha1.DMBRef{
						ResticRepository: "s3://sample-path/snapshots",
						BackedUpPVCData: datamoverv1alpha1.PVCData{
							Name: "sample-pvc",
							Size: "10Gi",
						},
					},
				},
			},
			want:    false,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeClient, err := getFakeClientFromObjects(tt.dmr)
			if err != nil {
				t.Errorf("error creating fake client, likely programmer error")
			}
			r := &VolumeSnapshotRestoreReconciler{
				Client:  fakeClient,
				Scheme:  fakeClient.Scheme(),
				Log:     logr.Discard(),
				Context: newContextForTest(tt.name),

				EventRecorder: record.NewFakeRecorder(10),
				req: reconcile.Request{
					NamespacedName: types.NamespacedName{
						Namespace: tt.dmr.Namespace,
						Name:      tt.dmr.Name,
					},
				},
			}
			got, err := r.ValidateDataMoverRestore(r.Log)
			if (err != nil) != tt.wantErr {
				t.Errorf("DataMoverRestoreReconciler.ValidateDataMoverRestore() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("DataMoverRestoreReconciler.ValidateDataMoverRestore() = %v, want %v", got, tt.want)
			}
		})
	}
}
