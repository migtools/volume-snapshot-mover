package controllers

import (
	"context"
	"fmt"
	"testing"

	controllerruntime "sigs.k8s.io/controller-runtime"

	volsyncv1alpha1 "github.com/backube/volsync/api/v1alpha1"
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

const (
	aws_access_key_id     = "some_aws_access_key_id"
	aws_secret_access_key = "some_aws_secret_access_key"
	restic_custom_ca      = "sample-vsb-secret"
	restic_password       = "some_restic_password"
	restic_repo           = "some_restic_repo"
	namespace             = "foo"
)

var (
	secretData = map[string][]byte{
		"AWS_ACCESS_KEY_ID":     []byte(aws_access_key_id),
		"AWS_SECRET_ACCESS_KEY": []byte(aws_secret_access_key),
		"RESTIC_PASSWORD":       []byte(restic_password),
		"RESTIC_REPOSITORY":     []byte(restic_repo),
		"RESTIC_CUSTOM_CA":      []byte(restic_custom_ca),
	}
)

func getSchemeForFakeClientRepSrc() (*runtime.Scheme, error) {
	err := volsnapmoverv1alpha1.AddToScheme(scheme.Scheme)
	if err != nil {
		return nil, err
	}
	err = snapv1.AddToScheme(scheme.Scheme)
	if err != nil {
		return nil, err
	}
	err = volsyncv1alpha1.AddToScheme(scheme.Scheme)
	if err != nil {
		return nil, err
	}

	return scheme.Scheme, nil
}

func getFakeClientFromObjectsRepSrc(objs ...client.Object) (client.WithWatch, error) {
	schemeForFakeClient, err := getSchemeForFakeClientRepSrc()
	if err != nil {
		return nil, err
	}

	return fake.NewClientBuilder().WithScheme(schemeForFakeClient).WithObjects(objs...).Build(), nil
}

func TestVolumeSnapshotMoverBackupReconciler_BuildReplicationSource(t *testing.T) {
	tests := []struct {
		name        string
		vsb         *volsnapmoverv1alpha1.VolumeSnapshotBackup
		pvc         *corev1.PersistentVolumeClaim
		repsrc      *volsyncv1alpha1.ReplicationSource
		secret      *corev1.Secret
		configMap   *corev1.ConfigMap
		serviceAcct *corev1.ServiceAccount
		wantErr     bool
		validate    func(*volsyncv1alpha1.ReplicationSource) error
	}{
		// TODO: Add test cases.
		{
			name: "given valid pvc,secret -> create valid rep src",
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
			pvc: &corev1.PersistentVolumeClaim{
				ObjectMeta: v1.ObjectMeta{
					Name:      "sample-pvc",
					Namespace: namespace,
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
					Name:      "sample-vsb-secret",
					Namespace: namespace,
				},
				Data: secretData,
			},
			repsrc: &volsyncv1alpha1.ReplicationSource{
				ObjectMeta: v1.ObjectMeta{
					Name:      "sample-vsb-rep-src",
					Namespace: namespace,
				},
			},
			configMap: &corev1.ConfigMap{
				ObjectMeta: v1.ObjectMeta{
					Name:      "datamover-config",
					Namespace: namespace,
				},
			},
			serviceAcct: &corev1.ServiceAccount{
				ObjectMeta: v1.ObjectMeta{
					Name:      "velero",
					Namespace: namespace,
				},
			},
			wantErr: false,
		},
		{
			name: "given invalid secret -> err",
			vsb: &volsnapmoverv1alpha1.VolumeSnapshotBackup{
				ObjectMeta: v1.ObjectMeta{
					Name:      "sample-vsb",
					Namespace: "bar",
				},
				Spec: volsnapmoverv1alpha1.VolumeSnapshotBackupSpec{
					VolumeSnapshotContent: corev1.ObjectReference{
						Name: "sample-snapshot",
					},
					ProtectedNamespace: namespace,
				},
			},
			pvc: &corev1.PersistentVolumeClaim{
				ObjectMeta: v1.ObjectMeta{
					Name:      "pvc",
					Namespace: namespace,
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
					Name:      "sample-vsb",
					Namespace: namespace,
				},
				Data: secretData,
			},
			repsrc: &volsyncv1alpha1.ReplicationSource{
				ObjectMeta: v1.ObjectMeta{
					Name:      "sample-vsb-rep-src",
					Namespace: namespace,
				},
			},
			configMap: &corev1.ConfigMap{
				ObjectMeta: v1.ObjectMeta{
					Name:      "datamover-config",
					Namespace: namespace,
				},
			},
			serviceAcct: &corev1.ServiceAccount{
				ObjectMeta: v1.ObjectMeta{
					Name:      "velero",
					Namespace: namespace,
				},
			},
			wantErr: true,
		},
		{
			name: "Given nil repsrc CR, should error out",
			vsb: &volsnapmoverv1alpha1.VolumeSnapshotBackup{
				ObjectMeta: v1.ObjectMeta{
					Name:      "sample-vsb",
					Namespace: "bar",
				},
				Spec: volsnapmoverv1alpha1.VolumeSnapshotBackupSpec{
					VolumeSnapshotContent: corev1.ObjectReference{
						Name: "sample-snapshot",
					},
					ProtectedNamespace: namespace,
				},
			},
			pvc: &corev1.PersistentVolumeClaim{
				ObjectMeta: v1.ObjectMeta{
					Name:      "pvc",
					Namespace: namespace,
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
			repsrc: nil,
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
			serviceAcct: &corev1.ServiceAccount{
				ObjectMeta: v1.ObjectMeta{
					Name:      "velero",
					Namespace: namespace,
				},
			},
			wantErr: true,
		},
		{
			name: "Given nil serviceAccount, should error out",
			vsb: &volsnapmoverv1alpha1.VolumeSnapshotBackup{
				ObjectMeta: v1.ObjectMeta{
					Name:      "sample-vsb",
					Namespace: "bar",
				},
				Spec: volsnapmoverv1alpha1.VolumeSnapshotBackupSpec{
					VolumeSnapshotContent: corev1.ObjectReference{
						Name: "sample-snapshot",
					},
					ProtectedNamespace: namespace,
				},
			},
			pvc: &corev1.PersistentVolumeClaim{
				ObjectMeta: v1.ObjectMeta{
					Name:      "pvc",
					Namespace: namespace,
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
			repsrc: &volsyncv1alpha1.ReplicationSource{
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
			serviceAcct: nil,
			wantErr:     true,
		},
		{
			name: "Should pass custom CA field through to restic secret",
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
			pvc: &corev1.PersistentVolumeClaim{
				ObjectMeta: v1.ObjectMeta{
					Name:      "sample-pvc",
					Namespace: namespace,
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
					Name:      restic_custom_ca,
					Namespace: namespace,
				},
				Data: secretData,
			},
			repsrc: &volsyncv1alpha1.ReplicationSource{
				ObjectMeta: v1.ObjectMeta{
					Name:      "sample-vsb-rep-src",
					Namespace: namespace,
				},
			},
			configMap: &corev1.ConfigMap{
				ObjectMeta: v1.ObjectMeta{
					Name:      "datamover-config",
					Namespace: namespace,
				},
			},
			serviceAcct: &corev1.ServiceAccount{
				ObjectMeta: v1.ObjectMeta{
					Name:      "velero",
					Namespace: namespace,
				},
			},
			wantErr: false,
			validate: func(rs *volsyncv1alpha1.ReplicationSource) error {
				t.Logf("Secret data: %s", secretData)
				if rs.Spec.Restic.CustomCA.Key != ResticCustomCA {
					return fmt.Errorf("restic custom CA key name mismatch, got %s, expected %s", rs.Spec.Restic.CustomCA.Key, ResticCustomCA)
				}
				if rs.Spec.Restic.CustomCA.SecretName != restic_custom_ca {
					return fmt.Errorf("restic custom CA secret name mismatch, got %s, expected %s", rs.Spec.Restic.CustomCA.SecretName, restic_custom_ca)
				}
				return nil
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeClient, err := getFakeClientFromObjectsRepSrc(tt.vsb, tt.pvc, tt.secret)
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
			err = r.buildReplicationSource(tt.repsrc, tt.vsb, tt.pvc, tt.configMap, tt.serviceAcct)
			if (err != nil) != tt.wantErr {
				t.Errorf("VolumeSnapshotMoverBackupReconciler.buildReplicationSource() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.validate != nil {
				if err = tt.validate(tt.repsrc); err != nil {
					t.Errorf("validation error: %v", err)
				}
			}
		})
	}
}

func TestVolumeSnapshotBackupReconciler_setStatusFromRepSource(t *testing.T) {

	tests := []struct {
		name           string
		vsb            *volsnapmoverv1alpha1.VolumeSnapshotBackup
		repSource      *volsyncv1alpha1.ReplicationSource
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
		{
			name: "Given nil VSB CR, should error out",
			vsb:  nil,
			repSource: &volsyncv1alpha1.ReplicationSource{
				ObjectMeta: v1.ObjectMeta{
					Name:      "sample-vsb-rep-src",
					Namespace: namespace,
				},
			},
			want:    false,
			wantErr: true,
		},
		{
			name: "Given nil repsrc CR, should error out",
			vsb: &volsnapmoverv1alpha1.VolumeSnapshotBackup{
				ObjectMeta: v1.ObjectMeta{
					Name:      "sample-vsb",
					Namespace: "bar",
				},
				Spec: volsnapmoverv1alpha1.VolumeSnapshotBackupSpec{
					VolumeSnapshotContent: corev1.ObjectReference{
						Name: "sample-snapshot",
					},
					ProtectedNamespace: namespace,
				},
			},
			repSource: nil,
			want:      false,
			wantErr:   true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &VolumeSnapshotBackupReconciler{
				Client:         tt.Client,
				Scheme:         tt.Scheme,
				Log:            tt.Log,
				Context:        tt.Context,
				NamespacedName: tt.NamespacedName,
				EventRecorder:  tt.EventRecorder,
				req:            tt.req,
			}
			got, err := r.setStatusFromRepSource(tt.vsb, tt.repSource)
			if (err != nil) != tt.wantErr {
				t.Errorf("setStatusFromRepSource() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("setStatusFromRepSource() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestVolumeSnapshotBackupReconciler_isRepSourceCompleted(t *testing.T) {
	tests := []struct {
		name           string
		Client         client.Client
		Scheme         *runtime.Scheme
		Log            logr.Logger
		Context        context.Context
		NamespacedName types.NamespacedName
		EventRecorder  record.EventRecorder
		req            controllerruntime.Request
		vsb            *volsnapmoverv1alpha1.VolumeSnapshotBackup
		want           bool
		wantErr        bool
	}{
		// TODO: Add test cases.
		{
			name:    "Given nil VSB CR, should error out",
			vsb:     nil,
			want:    false,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &VolumeSnapshotBackupReconciler{
				Client:         tt.Client,
				Scheme:         tt.Scheme,
				Log:            tt.Log,
				Context:        tt.Context,
				NamespacedName: tt.NamespacedName,
				EventRecorder:  tt.EventRecorder,
				req:            tt.req,
			}
			got, err := r.isRepSourceCompleted(tt.vsb)
			if (err != nil) != tt.wantErr {
				t.Errorf("isRepSourceCompleted() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("isRepSourceCompleted() got = %v, want %v", got, tt.want)
			}
		})
	}
}
