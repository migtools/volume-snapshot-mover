package controllers

import (
	"testing"

	"github.com/go-logr/logr"
	volsnapmoverv1alpha1 "github.com/konveyor/volume-snapshot-mover/api/v1alpha1"
	velerov1 "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func Test_updateVSBFromBackup(t *testing.T) {
	tests := []struct {
		name    string
		vsb     *volsnapmoverv1alpha1.VolumeSnapshotBackup
		backup  *velerov1.Backup
		client  client.Client
		log     logr.Logger
		wantErr bool
	}{
		{
			name: "Given nil VSB CR, should error out",
			vsb:  nil,
			backup: &velerov1.Backup{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "sample-backup",
					Namespace: "bar",
				},
				Spec: velerov1.BackupSpec{},
				Status: velerov1.BackupStatus{
					Phase: "Failed",
				},
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := updateVSBFromBackup(tt.vsb, tt.client, tt.log); (err != nil) != tt.wantErr {
				t.Errorf("updateVSBFromBackup() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_updateVSRFromRestore(t *testing.T) {
	tests := []struct {
		name    string
		vsr     *volsnapmoverv1alpha1.VolumeSnapshotRestore
		restore *velerov1.Restore
		client  client.Client
		log     logr.Logger
		wantErr bool
	}{
		// TODO: Add test cases
		{
			name: "Given nil VSR CR, should error out",
			vsr:  nil,
			restore: &velerov1.Restore{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "sample-restore",
					Namespace: "bar",
				},
				Spec: velerov1.RestoreSpec{},
				Status: velerov1.RestoreStatus{
					Phase: "Failed",
				},
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := updateVSRFromRestore(tt.vsr, tt.client, tt.log); (err != nil) != tt.wantErr {
				t.Errorf("updateVSRFromRestore() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_BuildResticSecret(t *testing.T) {
	tests := []struct {
		name        string
		givensecret *corev1.Secret
		secret      *corev1.Secret
		client      client.Client
		log         logr.Logger
		want        bool
		wantErr     bool
	}{
		// TODO: Add test cases
		{
			name: "Given secret and givenSecret, should pass",
			givensecret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "givensecret",
					Namespace: "test-ns",
				},
			},
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "secret",
					Namespace: "test-ns",
				},
			},
			want:    true,
			wantErr: false,
		},
		{
			name:        "Given nil givenSecret, should error out",
			givensecret: nil,
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "secret",
					Namespace: "test-ns",
				},
			},
			want:    false,
			wantErr: true,
		},
		{
			name: "Given nil secret, should error out",
			givensecret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "givensecret",
					Namespace: "test-ns",
				},
			},
			secret:  nil,
			want:    false,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := BuildResticSecret(tt.givensecret, tt.secret, "repo", "14", &RetainPolicy{}, "")
			if err != nil && tt.wantErr {
				t.Logf("BuildResticSecret() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err == nil && tt.want && !tt.wantErr {
				t.Logf("BuildResticSecret err = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
