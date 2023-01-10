package controllers

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	volsyncv1alpha1 "github.com/backube/volsync/api/v1alpha1"
	"github.com/go-logr/logr"
	volsnapmoverv1alpha1 "github.com/konveyor/volume-snapshot-mover/api/v1alpha1"
	snapv1 "github.com/kubernetes-csi/external-snapshotter/client/v4/apis/volumesnapshot/v1"
	velero "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	k8serror "k8s.io/apimachinery/pkg/api/errors"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	VSBLabel                      = "datamover.oadp.openshift.io/vsb"
	VSRLabel                      = "datamover.oadp.openshift.io/vsr"
	backupLabel                   = "velero.io/backup-name"
	restoreLabel                  = "velero.io/restore-name"
	DummyPodImage                 = "quay.io/konveyor/rsync-transfer:latest"
	volumeSnapshotClassDefaultKey = "snapshot.storage.kubernetes.io/is-default-class"
	storageClassDefaultKey        = "storageclass.kubernetes.io/is-default-class"
	OADPBSLProviderName           = "openshift.io/oadp-bsl-provider"
)

// Restic secret data keys
const (
	// AWS vars
	AWSAccessKey     = "AWS_ACCESS_KEY_ID"
	AWSSecretKey     = "AWS_SECRET_ACCESS_KEY"
	AWSDefaultRegion = "AWS_DEFAULT_REGION"

	// Azure vars
	AzureAccountName = "AZURE_ACCOUNT_NAME"
	AzureAccountKey  = "AZURE_ACCOUNT_KEY"

	// GCP vars
	GoogleApplicationCredentials = "GOOGLE_APPLICATION_CREDENTIALS"

	// Restic repo vars
	ResticPassword   = "RESTIC_PASSWORD"
	ResticRepository = "RESTIC_REPOSITORY"

	// Datamover annotation keys
	DatamoverResticRepository = "datamover.io/restic-repository"
	DatamoverSourcePVCName    = "datamover.io/source-pvc-name"
	DatamoverSourcePVCSize    = "datamover.io/source-pvc-size"

	// Providers
	AWSProvider   = "aws"
	AzureProvider = "azure"
	GCPProvider   = "gcp"
)

// Restic secret vars to create new secrets
var (
	AWSAccessValue        []byte
	AWSSecretValue        []byte
	AWSDefaultRegionValue []byte

	AzureAccountNameValue []byte
	AzureAccountKeyValue  []byte

	GoogleApplicationCredentialsValue []byte

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

func PopulateResticSecret(name string, namespace string, label string) (*corev1.Secret, error) {

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

func BuildResticSecret(givensecret *corev1.Secret, secret *corev1.Secret, resticrepo string) error {
	if givensecret == nil {
		return errors.New("nil givensecret in BuildResticSecret")
	}
	if secret == nil {
		return errors.New("nil secret in BuildResticSecret")
	}

	provider := givensecret.Labels[OADPBSLProviderName]

	switch provider {
	case AWSProvider:
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
				ResticRepository: []byte(resticrepo),
			},
		}
		secret.Data = resticSecretData.Data
		return nil

	case AzureProvider:
		// assign new restic secret values
		for key, val := range givensecret.Data {
			switch {
			case key == AzureAccountName:
				AzureAccountNameValue = val
			case key == AzureAccountKey:
				AzureAccountKeyValue = val
			case key == ResticPassword:
				ResticPasswordValue = val
			}
		}

		// build new Restic secret
		resticSecretData := &corev1.Secret{
			Data: map[string][]byte{
				AzureAccountName: AzureAccountNameValue,
				AzureAccountKey:  AzureAccountKeyValue,
				ResticPassword:   ResticPasswordValue,
				ResticRepository: []byte(resticrepo),
			},
		}
		secret.Data = resticSecretData.Data
		return nil

	case GCPProvider:
		// assign new restic secret values
		for key, val := range givensecret.Data {
			switch {
			case key == GoogleApplicationCredentials:
				GoogleApplicationCredentialsValue = val
			case key == ResticPassword:
				ResticPasswordValue = val
			}
		}

		// build new Restic secret
		resticSecretData := &corev1.Secret{
			Data: map[string][]byte{
				GoogleApplicationCredentials: GoogleApplicationCredentialsValue,
				ResticPassword:               ResticPasswordValue,
				ResticRepository:             []byte(resticrepo),
			},
		}
		secret.Data = resticSecretData.Data
		return nil
	}

	return nil
}

func ValidateResticSecret(resticsecret *corev1.Secret) error {
	if resticsecret == nil {
		return errors.New("empty restic secret. Please create a restic secret")
	}

	if resticsecret.Data == nil {
		return errors.New("secret data is empty")
	}

	provider := resticsecret.Labels[OADPBSLProviderName]

	switch provider {
	case AWSProvider:
		for key, val := range resticsecret.Data {
			switch key {
			case AWSAccessKey:
				b := checkByteArrayIsEmpty(val)
				if !b {
					return errors.New("awsAccessKey value cannot be empty")
				}
			case AWSSecretKey:
				b := checkByteArrayIsEmpty(val)
				if !b {
					return errors.New("awsSecretKey value cannot be empty")
				}
			case ResticPassword:
				b := checkByteArrayIsEmpty(val)
				if !b {
					return errors.New("resticPassword value cannot be empty")
				}
			case ResticRepository:
				b := checkByteArrayIsEmpty(val)
				if !b {
					return errors.New("resticRepository value cannot be empty")
				}
			}
		}

	case AzureProvider:
		for key, val := range resticsecret.Data {
			switch key {
			case AzureAccountName:
				b := checkByteArrayIsEmpty(val)
				if !b {
					return errors.New("azure accout name value cannot be empty")
				}
			case AzureAccountKey:
				b := checkByteArrayIsEmpty(val)
				if !b {
					return errors.New("azure account key value cannot be empty")
				}
			case ResticPassword:
				b := checkByteArrayIsEmpty(val)
				if !b {
					return errors.New("resticPassword value cannot be empty")
				}
			case ResticRepository:
				b := checkByteArrayIsEmpty(val)
				if !b {
					return errors.New("resticRepository value cannot be empty")
				}
			}
		}

	case GCPProvider:
		for key, val := range resticsecret.Data {
			switch key {
			case GoogleApplicationCredentials:
				b := checkByteArrayIsEmpty(val)
				if !b {
					return errors.New("GoogleApplicationCredentials value cannot be empty")
				}
			case ResticPassword:
				b := checkByteArrayIsEmpty(val)
				if !b {
					return errors.New("resticPassword value cannot be empty")
				}
			case ResticRepository:
				b := checkByteArrayIsEmpty(val)
				if !b {
					return errors.New("resticRepository value cannot be empty")
				}
			}
		}
	}

	return nil
}

func checkByteArrayIsEmpty(val []byte) bool {

	return len(val) != 0
}

func (r *VolumeSnapshotBackupReconciler) setVSBStatus(log logr.Logger) (bool, error) {

	vsb := volsnapmoverv1alpha1.VolumeSnapshotBackup{}
	if err := r.Get(r.Context, r.req.NamespacedName, &vsb); err != nil {
		// ignore is not found error
		if k8serrors.IsNotFound(err) {
			return true, nil
		}
		r.Log.Error(err, fmt.Sprintf("unable to fetch volumesnapshotbackup %s", r.req.NamespacedName))
		return false, err
	}

	//update VSB status with Backup phase
	err := updateVSBFromBackup(&vsb, r.Client, log)
	if err != nil {
		return false, err
	}

	if vsb.Status.Phase == volsnapmoverv1alpha1.SnapMoverBackupPhaseFailed ||
		vsb.Status.Phase == volsnapmoverv1alpha1.SnapMoverBackupPhasePartiallyFailed {
		return false, errors.New("vsb failed to complete")
	}

	repSourceName := fmt.Sprintf("%s-rep-src", vsb.Name)
	repSource := volsyncv1alpha1.ReplicationSource{}
	if err := r.Get(r.Context, types.NamespacedName{Namespace: vsb.Spec.ProtectedNamespace, Name: repSourceName}, &repSource); err != nil {
		if k8serror.IsNotFound(err) {
			return false, nil
		}
		return false, err
	}

	if repSource.Status == nil {
		r.Log.Info(fmt.Sprintf("replication source %s/%s is yet to have a status", vsb.Spec.ProtectedNamespace, repSourceName))
		return false, nil
	}

	if repSource.Status != nil {

		// update VSB status with ReplicationSource phase
		hasStatus, err := r.setStatusFromRepSource(&vsb, &repSource)
		if err != nil {
			return false, err
		}
		if !hasStatus {
			r.Log.Info(fmt.Sprintf("replicationSource %s is inProgress", repSourceName))
			return false, nil
		}
	}
	return false, errors.New("replication source status not ready")
}

func checkForOneDefaultSnapClass(vsClassList *snapv1.VolumeSnapshotClassList) (bool, error) {
	if vsClassList == nil {
		return false, errors.New("nil vsClassList in checkForOneDefaultSnapClass")
	}

	foundDefaultClass := false
	for _, vsClass := range vsClassList.Items {

		isDefaultClass, _ := vsClass.Annotations[volumeSnapshotClassDefaultKey]
		boolIsDefault, _ := strconv.ParseBool(isDefaultClass)

		// found a default volumeSnapshotClass
		if boolIsDefault {

			if foundDefaultClass {
				return false, errors.New("cannot have more than one default volumeSnapshotClass")
			}

			foundDefaultClass = true
		}
	}

	return true, nil
}

func checkForOneDefaultStorageClass(storageClassList *storagev1.StorageClassList) (bool, error) {
	if storageClassList == nil {
		return false, errors.New("nil storageClassList in checkForOneDefaultStorageClass")
	}

	foundDefaultClass := false
	for _, storageClass := range storageClassList.Items {

		isDefaultClass, _ := storageClass.Annotations[storageClassDefaultKey]
		boolIsDefault, _ := strconv.ParseBool(isDefaultClass)

		// found a default storageClass
		if boolIsDefault {

			if foundDefaultClass {
				return false, errors.New("cannot have more than one default storageClass")
			}

			foundDefaultClass = true
		}
	}

	return true, nil
}

func updateVSBFromBackup(vsb *volsnapmoverv1alpha1.VolumeSnapshotBackup, client client.Client, log logr.Logger) error {

	if vsb == nil {
		return errors.New("nil vsb in updateVSBFromBackup")
	}

	backupName := vsb.Labels[backupLabel]
	backup := velero.Backup{}
	if err := client.Get(context.TODO(), types.NamespacedName{Namespace: vsb.Spec.ProtectedNamespace, Name: backupName}, &backup); err != nil {
		return err
	}

	if backup.Status.Phase == velero.BackupPhaseFailed || backup.Status.Phase == velero.BackupPhasePartiallyFailed {
		vsb.Status.Phase = volsnapmoverv1alpha1.SnapMoverBackupPhasePartiallyFailed
		err := client.Status().Update(context.Background(), vsb)
		if err != nil {
			return err
		}
		return errors.New("backup failed. Marking volumeSnapshotBackup as partiallyFailed")
	}
	return nil
}

func (r *VolumeSnapshotRestoreReconciler) checkRestoreStatus(log logr.Logger) (bool, error) {

	vsr := volsnapmoverv1alpha1.VolumeSnapshotRestore{}
	if err := r.Get(r.Context, r.req.NamespacedName, &vsr); err != nil {
		// ignore is not found error
		if k8serrors.IsNotFound(err) {
			return true, nil
		}
		r.Log.Error(err, fmt.Sprintf("unable to fetch volumesnapshotrestore %s", r.req.NamespacedName))
		return false, err
	}

	err := updateVSRFromRestore(&vsr, r.Client, log)
	if err != nil {
		return false, err
	}

	return true, nil
}

func updateVSRFromRestore(vsr *volsnapmoverv1alpha1.VolumeSnapshotRestore, client client.Client, log logr.Logger) error {
	if vsr == nil {
		return errors.New("nil vsr in updateVSRFromRestore")
	}

	restoreName := vsr.Labels[restoreLabel]
	restore := velero.Restore{}
	if err := client.Get(context.TODO(), types.NamespacedName{Namespace: vsr.Spec.ProtectedNamespace, Name: restoreName}, &restore); err != nil {
		return err
	}

	if restore.Status.Phase == velero.RestorePhaseFailed || restore.Status.Phase == velero.RestorePhasePartiallyFailed {
		vsr.Status.Phase = volsnapmoverv1alpha1.SnapMoverRestorePhasePartiallyFailed
		err := client.Status().Update(context.Background(), vsr)
		if err != nil {
			return err
		}
		return errors.New("restore failed. Marking volumeSnapshotRestore as partiallyFailed")
	}
	return nil
}

func updateVSRStatusPhase(vsr *volsnapmoverv1alpha1.VolumeSnapshotRestore, phase volsnapmoverv1alpha1.VolumeSnapshotRestorePhase, client client.Client) error {
	if vsr == nil {
		return errors.New("nil vsr in updateVSRStatusPhase")
	}

	vsr.Status.Phase = phase

	err := client.Status().Update(context.Background(), vsr)
	if err != nil {
		return err
	}

	return nil
}

func updateVSBStatusPhase(vsb *volsnapmoverv1alpha1.VolumeSnapshotBackup, phase volsnapmoverv1alpha1.VolumeSnapshotBackupPhase, client client.Client) error {
	vsb.Status.Phase = phase

	err := client.Status().Update(context.Background(), vsb)
	if err != nil {
		return err
	}

	return nil
}
