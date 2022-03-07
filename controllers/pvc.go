package controllers

import (
	"github.com/go-logr/logr"
)

func (r *DataMoverBackupReconciler) BindPVC(log logr.Logger) (bool, error) {
	return true, nil
}
