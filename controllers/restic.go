package controllers

import (
	"fmt"

	"github.com/go-logr/logr"
	pvcv1alpha1 "github.com/konveyor/volume-snapshot-mover/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// Restic secret data keys
const (
	// AWS vars
	AWSAccessKey = "AWS_ACCESS_KEY_ID"
	AWSSecretKey = "AWS_SECRET_ACCESS_KEY"

	// TODO: GCP and Azure

	// Restic repo vars
	ResticPassword   = "RESTIC_PASSWORD"
	ResticRepository = "RESTIC_REPOSITORY"
)

// Restic secret vars to create new secrets
var (
	AWSAccessValue []byte
	AWSSecretValue []byte

	// TODO: GCP and Azure

	ResticPasswordValue []byte
	ResticRepoValue     []byte
)

const (
	resticSecretName = "restic-secret"
)

func (r *DataMoverBackupReconciler) CreateResticSecret(log logr.Logger) (bool, error) {

	// get datamoverbackup from cluster
	dmb := pvcv1alpha1.DataMoverBackup{}
	if err := r.Get(r.Context, r.NamespacedName, &dmb); err != nil {
		return false, err
	}

	// get pvc created by controller
	pvcName := fmt.Sprintf("%s-pvc", dmb.Spec.VolumeSnapshotContent.Name)
	pvc := corev1.PersistentVolumeClaim{}
	if err := r.Get(r.Context, types.NamespacedName{Namespace: dmb.Namespace, Name: pvcName}, &pvc); err != nil {
		return false, err
	}

	// define Restic secret to be created
	newResticSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-secret", pvc.Name),
			Namespace: dmb.Namespace,
		},
		Type: corev1.SecretTypeOpaque,
	}

	// Create Restic secret in OADP namespace
	op, err := controllerutil.CreateOrUpdate(r.Context, r.Client, newResticSecret, func() error {
		return r.buildResticSecret(newResticSecret, &dmb, &pvc)
	})
	if err != nil {
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

func (r *DataMoverBackupReconciler) buildResticSecret(secret *corev1.Secret, dmb *pvcv1alpha1.DataMoverBackup, pvc *corev1.PersistentVolumeClaim) error {

	// get restic secret from user
	resticSecret := corev1.Secret{}
	if err := r.Get(r.Context, types.NamespacedName{Namespace: dmb.Namespace, Name: resticSecretName}, &resticSecret); err != nil {
		return err
	}

	// assign new restic secret values
	for key, val := range resticSecret.Data {
		switch {
		case key == AWSAccessKey:
			AWSAccessValue = val
		case key == AWSSecretKey:
			AWSSecretValue = val
		case key == ResticPassword:
			ResticPasswordValue = val
		case key == ResticRepository:
			ResticRepoValue = val
		}
	}

	// create new repo path for snapshot
	decodedRepoName := string(ResticRepoValue)
	newRepoName := fmt.Sprintf("%s/%s/%s", decodedRepoName, pvc.Namespace, pvc.Name)

	// build new Restic secret
	resticSecretData := &corev1.Secret{
		Data: map[string][]byte{
			AWSAccessKey:     AWSAccessValue,
			AWSSecretKey:     AWSSecretValue,
			ResticPassword:   ResticPasswordValue,
			ResticRepository: []byte(newRepoName),
		},
	}

	secret.Data = resticSecretData.Data
	return nil
}
