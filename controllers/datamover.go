package controllers

import (
	"github.com/go-logr/logr"
)

func (r *VolumeSnapshotBackupReconciler) SetupDataMoverConfig(log logr.Logger) (bool, error) {
	return true, nil
}

func (r *VolumeSnapshotBackupReconciler) RunDataMoverBackup(log logr.Logger) (bool, error) {
	return true, nil
}

func (r *VolumeSnapshotBackupReconciler) WaitForDataMoverBackupToComplete(log logr.Logger) (bool, error) {
	return true, nil
}
