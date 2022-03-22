package controllers

import (
	"errors"
	"fmt"

	"github.com/go-logr/logr"
	pvcv1alpha1 "github.com/konveyor/volume-snapshot-mover/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func (r *DataMoverBackupReconciler) CreateResticSecret(log logr.Logger) (bool, error) {
	// Get datamoverbackup from cluster
	// TODO: handle multiple DMBs
	dmb := pvcv1alpha1.DataMoverBackup{}
	if err := r.Get(r.Context, r.NamespacedName, &dmb); err != nil {
		return false, errors.New("dataMoverBackup not found")
	}

	// get pvc in OADP namespace
	pvc := corev1.PersistentVolumeClaim{}
	// TODO: get pvc name
	if err := r.Get(r.Context, types.NamespacedName{Namespace: dmb.Namespace, Name: "mssql-pvc"}, &pvc); err != nil {
		return false, err
	}

	// define Restic secret to be created
	newResticSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-resticonfig", pvc.Name),
			Namespace: dmb.Namespace,
		},
		Type: corev1.SecretTypeOpaque,
	}
	// Create Restic secret in OADP namespace
	op, err := controllerutil.CreateOrUpdate(r.Context, r.Client, newResticSecret, func() error {
		return r.buildResticSecret(newResticSecret, &dmb)
	})
	if err != nil {
		fmt.Printf("err: %v\n", err)
		return false, err
	}

	if op == controllerutil.OperationResultCreated || op == controllerutil.OperationResultUpdated {
		r.EventRecorder.Event(newResticSecret,
			corev1.EventTypeNormal,
			"ReplicationSourceReconciled",
			fmt.Sprintf("%s restic secret %s", op, newResticSecret.Name),
		)
	}
	return true, nil
}

func (r *DataMoverBackupReconciler) buildResticSecret(secret *corev1.Secret, dmb *pvcv1alpha1.DataMoverBackup) error {

	// get pvc in OADP namespace
	pvc := corev1.PersistentVolumeClaim{}
	// TODO: get pvc name
	if err := r.Get(r.Context, types.NamespacedName{Namespace: dmb.Namespace, Name: "mssql-pvc"}, &pvc); err != nil {
		return err
	}

	// get restic secret in OADP namespace
	resticSecret := corev1.Secret{}
	// TODO: get name
	if err := r.Get(r.Context, types.NamespacedName{Namespace: dmb.Namespace, Name: "restic-config"}, &resticSecret); err != nil {
		return err
	}

	var aws_access_key []byte
	var aws_secret_key []byte
	var restic_password []byte
	var restic_repo []byte

	for key, val := range resticSecret.Data {
		if key == "AWS_ACCESS_KEY_ID" {
			aws_access_key = val
		}
		if key == "AWS_SECRET_ACCESS_KEY" {
			aws_secret_key = val
		}
		if key == "RESTIC_PASSWORD" {
			restic_password = val
		}
		if key == "RESTIC_REPOSITORY" {
			restic_repo = val
		}
	}

	// decode restic repo set by user
	decodedRepo := string(restic_repo)
	// create new repo path
	newRepoName := fmt.Sprintf("%s/%s", decodedRepo, pvc.Name)

	// build Restic secret data
	resticSecretData := &corev1.Secret{
		Data: map[string][]byte{
			"AWS_ACCESS_KEY_ID":     aws_access_key,
			"AWS_SECRET_ACCESS_KEY": aws_secret_key,
			"RESTIC_PASSWORD":       restic_password,
			"RESTIC_REPOSITORY":     []byte(newRepoName),
		},
	}

	secret.Data = resticSecretData.Data
	return nil
}
