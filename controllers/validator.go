package controllers

import (
	"github.com/go-logr/logr"
)

func (r *VolumeSnapshotBackupReconciler) ValidateVolumeSnapshotBackup(log logr.Logger) (bool, error) {
	return true, nil
}
