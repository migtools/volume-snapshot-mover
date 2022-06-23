package controllers

import (
	"errors"
	"fmt"
	"strings"

	"github.com/go-logr/logr"
	datamoverv1alpha1 "github.com/konveyor/volume-snapshot-mover/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	VSBLabel                      = "datamover.oadp.openshift.io/vsb"
	VSRLabel                      = "datamover.oadp.openshift.io/vsr"
	DummyPodImage                 = "quay.io/konveyor/rsync-transfer:latest"
	resticSecretName              = "restic-secret"
	volumeSnapshotClassDefaultKey = "snapshot.storage.kubernetes.io/is-default-class"
	storageClassDefaultKey        = "storageclass.kubernetes.io/is-default-class"
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

type ReconcileFunc func(logr.Logger) (bool, error)

// reconcileBatch steps through a list of reconcile functions until one returns
// false or an error.
func ReconcileBatch(l logr.Logger, reconcileFuncs ...ReconcileFunc) (bool, error) {
	for _, f := range reconcileFuncs {
		if cont, err := f(l); !cont || err != nil {
			return cont, err
		}
	}
	return true, nil
}

func PopulateResticSecret(vsb *datamoverv1alpha1.VolumeSnapshotBackup, vsr *datamoverv1alpha1.VolumeSnapshotRestore) (*corev1.Secret, error) {

	var label, name, namespace string
	if vsb != nil {

		label = VSBLabel
		name = vsb.Name
		namespace = vsb.Spec.ProtectedNamespace

	} else if vsr != nil {
		label = VSRLabel
		name = vsr.Name
		namespace = vsr.Spec.ProtectedNamespace
	} else {
		return nil, errors.New("both vsr & vsb reference cannot be empty")
	}
	// define Restic secret to be created
	newResticSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-secret", name),
			Namespace: namespace,
			Labels: map[string]string{
				label: name,
			},
		},
		Type: corev1.SecretTypeOpaque,
	}

	return newResticSecret, nil
}

func BuildVSBResticSecret(givensecret *corev1.Secret, resticsecret *corev1.Secret, pvc *corev1.PersistentVolumeClaim) error {

	// assign new restic secret values
	for key, val := range givensecret.Data {
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

	resticsecret.Data = resticSecretData.Data
	return nil
}

func BuildVSRResticSecret(givensecret *corev1.Secret, secret *corev1.Secret, vsr *datamoverv1alpha1.VolumeSnapshotRestore) error {

	// assign new restic secret values
	for key, val := range givensecret.Data {
		switch {
		case key == AWSAccessKey:
			AWSAccessValue = val
		case key == AWSSecretKey:
			AWSSecretValue = val
		case key == AWSDefaultRegion:
			AWSDefaultRegionValue = val
		case key == ResticPassword:
			ResticPasswordValue = val
		}
	}

	// build new Restic secret
	resticSecretData := &corev1.Secret{
		Data: map[string][]byte{
			AWSAccessKey:     AWSAccessValue,
			AWSSecretKey:     AWSSecretValue,
			AWSDefaultRegion: AWSDefaultRegionValue,
			ResticPassword:   ResticPasswordValue,
			ResticRepository: []byte(vsr.Spec.VolumeSnapshotMoverBackupref.ResticRepository),
		},
	}

	secret.Data = resticSecretData.Data
	return nil
}
