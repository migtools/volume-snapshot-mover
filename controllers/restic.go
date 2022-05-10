package controllers

import (
	"context"
	"fmt"
	"strings"

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
	AWSAccessKey     = "AWS_ACCESS_KEY_ID"
	AWSSecretKey     = "AWS_SECRET_ACCESS_KEY"
	AWSDefaultRegion = "AWS_DEFAULT_REGION"

	// TODO: GCP and Azure

	// Restic repo vars
	ResticPassword   = "RESTIC_PASSWORD"
	ResticRepository = "RESTIC_REPOSITORY"

	// Datamover annotation keys
	DatamoverResticRepository = "datamover.io/restic-repository"
	DatamoverSourcePVCName    = "datamover.io/source-pvc-name"
	DatamoverSourcePVCSize    = "datamover.io/source-pvc-size"
)

// Restic secret vars to create new secrets
var (
	AWSAccessValue        []byte
	AWSSecretValue        []byte
	AWSDefaultRegionValue []byte

	// TODO: GCP and Azure

	ResticPasswordValue []byte
	ResticRepoValue     string
)

const (
	resticSecretName = "restic-secret"
)

func (r *DataMoverBackupReconciler) CreateResticSecret(log logr.Logger) (bool, error) {

	// get datamoverbackup from cluster
	dmb := pvcv1alpha1.DataMoverBackup{}
	if err := r.Get(r.Context, r.req.NamespacedName, &dmb); err != nil {
		r.Log.Error(err, "unable to fetch DataMoverBackup CR")
		return false, err
	}

	// get cloned pvc
	pvcName := fmt.Sprintf("%s-pvc", dmb.Spec.VolumeSnapshotContent.Name)
	pvc := corev1.PersistentVolumeClaim{}
	if err := r.Get(r.Context, types.NamespacedName{Name: pvcName, Namespace: r.NamespacedName.Namespace}, &pvc); err != nil {
		r.Log.Error(err, "unable to fetch PVC")
		return false, err
	}

	// define Restic secret to be created
	newResticSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-secret", dmb.Name),
			Namespace: r.NamespacedName.Namespace,
			Labels: map[string]string{
				DMBLabel: dmb.Name,
			},
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

	// set created Restic repo to DMB status
	dmb.Status.ResticRepository = string(newResticSecret.Data[ResticRepository])

	// Update DMB status
	err = r.Status().Update(context.Background(), &dmb)
	if err != nil {
		return false, err
	}

	if op == controllerutil.OperationResultCreated || op == controllerutil.OperationResultUpdated {
		r.EventRecorder.Event(newResticSecret,
			corev1.EventTypeNormal,
			"ResticSecretBackupReconciled",
			fmt.Sprintf("%s restic secret %s", op, newResticSecret.Name),
		)
	}
	return true, nil
}

func (r *DataMoverBackupReconciler) buildResticSecret(secret *corev1.Secret, dmb *pvcv1alpha1.DataMoverBackup, pvc *corev1.PersistentVolumeClaim) error {

	// get restic secret from user
	resticSecret := corev1.Secret{}
	if err := r.Get(r.Context, types.NamespacedName{Namespace: r.NamespacedName.Namespace, Name: resticSecretName}, &resticSecret); err != nil {
		r.Log.Error(err, "unable to fetch Restic Secret")
		return err
	}

	// assign new restic secret values
	for key, val := range resticSecret.Data {
		switch {
		case key == AWSAccessKey:
			AWSAccessValue = val
		case key == AWSSecretKey:
			AWSSecretValue = val
		case key == AWSDefaultRegion:
			AWSDefaultRegionValue = val
		case key == ResticPassword:
			ResticPasswordValue = val
		case key == ResticRepository:

			// if trailing '/' in user-created Restic repo, remove it
			stringVal := string(val)
			stringVal = strings.TrimRight(stringVal, "/")

			ResticRepoValue = stringVal
		}
	}

	// create new repo path for snapshot
	newRepoName := fmt.Sprintf("%s/%s/%s", ResticRepoValue, pvc.Namespace, pvc.Name)

	// build new Restic secret
	resticSecretData := &corev1.Secret{
		Data: map[string][]byte{
			AWSAccessKey:     AWSAccessValue,
			AWSSecretKey:     AWSSecretValue,
			AWSDefaultRegion: AWSDefaultRegionValue,
			ResticPassword:   ResticPasswordValue,
			ResticRepository: []byte(newRepoName),
		},
	}

	secret.Data = resticSecretData.Data
	return nil
}

// TODO: move these 2 functions to a common.go and check for DMB or DMR being used
func (r *DataMoverRestoreReconciler) CreateDMRResticSecret(log logr.Logger) (bool, error) {
	// get datamoverrestore from cluster
	dmr := pvcv1alpha1.DataMoverRestore{}
	if err := r.Get(r.Context, r.req.NamespacedName, &dmr); err != nil {
		r.Log.Error(err, "unable to fetch DataMoverRestore CR")
		return false, err
	}

	// define Restic secret to be created
	newResticSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-secret", dmr.Name),
			Namespace: r.NamespacedName.Namespace,
			Labels: map[string]string{
				DMRLabel: dmr.Name,
			},
		},
		Type: corev1.SecretTypeOpaque,
	}

	// Create Restic secret in OADP namespace
	op, err := controllerutil.CreateOrUpdate(r.Context, r.Client, newResticSecret, func() error {

		return r.buildDMRResticSecret(newResticSecret, &dmr)
	})
	if err != nil {
		return false, err
	}

	if op == controllerutil.OperationResultCreated || op == controllerutil.OperationResultUpdated {
		r.EventRecorder.Event(newResticSecret,
			corev1.EventTypeNormal,
			"ResticSecretRestoreReconciled",
			fmt.Sprintf("%s restic secret %s", op, newResticSecret.Name),
		)
	}
	return true, nil
}

// TODO: move these 2 functions to a common.go and check for DMB or DMR being used
func (r *DataMoverRestoreReconciler) buildDMRResticSecret(secret *corev1.Secret, dmr *pvcv1alpha1.DataMoverRestore) error {

	// get restic secret from user
	resticSecret := corev1.Secret{}
	if err := r.Get(r.Context, types.NamespacedName{Namespace: r.NamespacedName.Namespace, Name: resticSecretName}, &resticSecret); err != nil {
		r.Log.Error(err, "unable to fetch Restic Secret")
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
		}
	}

	// build new Restic secret
	resticSecretData := &corev1.Secret{
		Data: map[string][]byte{
			AWSAccessKey:     AWSAccessValue,
			AWSSecretKey:     AWSSecretValue,
			ResticPassword:   ResticPasswordValue,
			ResticRepository: []byte(dmr.Spec.DataMoverBackupref.ResticRepository),
		},
	}

	secret.Data = resticSecretData.Data
	return nil
}
