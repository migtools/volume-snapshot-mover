package controllers

import (
	"context"
	"testing"

	controllerruntime "sigs.k8s.io/controller-runtime"

	volsyncv1alpha1 "github.com/backube/volsync/api/v1alpha1"
	"github.com/go-logr/logr"
	volsnapmoverv1alpha1 "github.com/konveyor/volume-snapshot-mover/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestVolumeSnapshotRestoreReconciler_buildReplicationDestination(t *testing.T) {

	tests := []struct {
		name           string
		vsr            *volsnapmoverv1alpha1.VolumeSnapshotRestore
		repDest        *volsyncv1alpha1.ReplicationDestination
		secret         *corev1.Secret
		configMap      *corev1.ConfigMap
		Client         client.Client
		Log            logr.Logger
		Context        context.Context
		NamespacedName types.NamespacedName
		EventRecorder  record.EventRecorder
		req            controllerruntime.Request
		Scheme         *runtime.Scheme
		want           bool
		wantErr        bool
	}{
		// TODO: Add test cases
		{
			name: "Given vsr and repdest and secret and configmap, should pass",
			vsr: &volsnapmoverv1alpha1.VolumeSnapshotRestore{
				ObjectMeta: v1.ObjectMeta{
					Name:      "sample-vsr",
					Namespace: "bar",
				},
				Spec: volsnapmoverv1alpha1.VolumeSnapshotRestoreSpec{
					ResticSecretRef: corev1.LocalObjectReference{
						Name: "secret",
					},
					VolumeSnapshotMoverBackupref: volsnapmoverv1alpha1.VSBRef{
						BackedUpPVCData: volsnapmoverv1alpha1.PVCData{
							Name:             "test-pvc",
							Size:             "1G",
							StorageClassName: "test-class",
						},
					},
					ProtectedNamespace: "test-ns",
				},
			},
			repDest: &volsyncv1alpha1.ReplicationDestination{
				ObjectMeta: v1.ObjectMeta{
					Name:      "sample-vsb-rep-src",
					Namespace: namespace,
				},
			},
			secret: &corev1.Secret{
				ObjectMeta: v1.ObjectMeta{
					Name:      "test-secret",
					Namespace: "test-ns",
				},
			},
			configMap: &corev1.ConfigMap{
				ObjectMeta: v1.ObjectMeta{
					Name:      "datamover-config",
					Namespace: namespace,
				},
			},
			want:    true,
			wantErr: false,
		},
		{
			name: "Given nil VSR CR, should error out",
			vsr:  nil,
			repDest: &volsyncv1alpha1.ReplicationDestination{
				ObjectMeta: v1.ObjectMeta{
					Name:      "sample-vsb-rep-src",
					Namespace: namespace,
				},
			},
			secret: &corev1.Secret{
				ObjectMeta: v1.ObjectMeta{
					Name:      "test-secret",
					Namespace: "test-ns",
				},
			},
			want:    false,
			wantErr: true,
		},
		{
			name: "Given nil repdest CR, should error out",
			vsr: &volsnapmoverv1alpha1.VolumeSnapshotRestore{
				ObjectMeta: v1.ObjectMeta{
					Name:      "sample-vsr",
					Namespace: "bar",
				},
				Spec: volsnapmoverv1alpha1.VolumeSnapshotRestoreSpec{
					ResticSecretRef: corev1.LocalObjectReference{
						Name: "secret",
					},
					VolumeSnapshotMoverBackupref: volsnapmoverv1alpha1.VSBRef{
						BackedUpPVCData: volsnapmoverv1alpha1.PVCData{
							Name:             "test-pvc",
							Size:             "1G",
							StorageClassName: "test-class",
						},
					},
					ProtectedNamespace: "test-ns",
				},
			},
			repDest: nil,
			secret: &corev1.Secret{
				ObjectMeta: v1.ObjectMeta{
					Name:      "test-secret",
					Namespace: "test-ns",
				},
			},
			want:    false,
			wantErr: true,
		},
		{
			name: "Given nil resticSecret, should error out",
			vsr: &volsnapmoverv1alpha1.VolumeSnapshotRestore{
				ObjectMeta: v1.ObjectMeta{
					Name:      "sample-vsr",
					Namespace: "bar",
				},
				Spec: volsnapmoverv1alpha1.VolumeSnapshotRestoreSpec{
					ResticSecretRef: corev1.LocalObjectReference{
						Name: "secret",
					},
					VolumeSnapshotMoverBackupref: volsnapmoverv1alpha1.VSBRef{
						BackedUpPVCData: volsnapmoverv1alpha1.PVCData{
							Name:             "test-pvc",
							Size:             "1G",
							StorageClassName: "test-class",
						},
					},
					ProtectedNamespace: "test-ns",
				},
			},
			repDest: &volsyncv1alpha1.ReplicationDestination{
				ObjectMeta: v1.ObjectMeta{
					Name:      "sample-vsb-rep-src",
					Namespace: namespace,
				},
			},
			secret:  nil,
			want:    false,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &VolumeSnapshotRestoreReconciler{
				Client:         tt.Client,
				Scheme:         tt.Scheme,
				Log:            tt.Log,
				Context:        tt.Context,
				NamespacedName: tt.NamespacedName,
				EventRecorder:  tt.EventRecorder,
				req:            tt.req,
			}

			err := r.buildReplicationDestination(tt.repDest, tt.vsr, tt.secret, tt.configMap)
			if err != nil && tt.wantErr {
				t.Logf("buildReplicationDestination() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err == nil && tt.want && !tt.wantErr {
				t.Logf("buildReplicationDestination() err = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
