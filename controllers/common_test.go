package controllers

import (
	"github.com/go-logr/logr"
	volsnapmoverv1alpha1 "github.com/konveyor/volume-snapshot-mover/api/v1alpha1"
	velerov1 "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"testing"
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
