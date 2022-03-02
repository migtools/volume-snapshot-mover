package controllers

import (
	"github.com/go-logr/logr"
)

func (r *VolumeSnapshotBackupReconciler) BindPVC(log logr.Logger) (bool, error) {
	return true, nil
}
