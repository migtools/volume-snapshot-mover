package controllers

import (
	"github.com/go-logr/logr"
)

func (r *DataMoverBackupReconciler) SetupDataMoverConfig(log logr.Logger) (bool, error) {
	return true, nil
}

func (r *DataMoverBackupReconciler) RunDataMoverBackup(log logr.Logger) (bool, error) {
	return true, nil
}

func (r *DataMoverBackupReconciler) WaitForDataMoverBackupToComplete(log logr.Logger) (bool, error) {
	return true, nil
}
