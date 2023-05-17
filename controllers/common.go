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
	appsv1 "k8s.io/api/apps/v1"
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
	dmFinalizer                   = "oadp.openshift.io/oadp-datamover"

	// VSM deployment vars
	vsmDeploymentName = "volume-snapshot-mover"
	vsmContainerName  = "data-mover-controller-container"
	batchBackupName   = "DATAMOVER_CONCURRENT_BACKUP"
	batchRestoreName  = "DATAMOVER_CONCURRENT_RESTORE"
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
	ResticCustomCA      = "RESTIC_CUSTOM_CA"
	ResticPassword      = "RESTIC_PASSWORD"
	ResticRepository    = "RESTIC_REPOSITORY"
	ResticPruneInterval = "restic-prune-interval"

	// Datamover annotation keys
	DatamoverResticRepository = "datamover.io/restic-repository"
	DatamoverSourcePVCName    = "datamover.io/source-pvc-name"
	DatamoverSourcePVCSize    = "datamover.io/source-pvc-size"

	// Providers
	AWSProvider   = "aws"
	AzureProvider = "azure"
	GCPProvider   = "gcp"
)

// VSM configmap values
const (
	// replicationSource values
	SourceStorageClassName      = "SourceStorageClassName"
	SourceAccessMoce            = "SourceAccessMode"
	SourceCacheStorageClassName = "SourceCacheStorageClassName"
	SourceCacheAccessMoce       = "SourceCacheAccessMode"
	SourceCacheCapacity         = "SourceCacheCapacity"
	SourceCacheAccessMode       = "SourceCacheAccessMode"
	SourceMoverSecurityContext  = "SourceMoverSecurityContext"

	// replicationDestination values
	DestinationStorageClassName      = "DestinationStorageClassName"
	DestinationAccessMoce            = "DestinationAccessMode"
	DestinationCacheStorageClassName = "DestinationCacheStorageClassName"
	DestinationCacheAccessMoce       = "DestinationCacheAccessMode"
	DestinationCacheCapacity         = "DestinationCacheCapacity"
	DestinationCacheAccessMode       = "DestinationCacheAccessMode"
	DestinationMoverSecurityContext  = "DestinationMoverSecurityContext"

	// RetainPolicy parameters
	SnapshotRetainPolicyHourly  = "SnapshotRetainPolicyHourly"
	SnapshotRetainPolicyDaily   = "SnapshotRetainPolicyDaily"
	SnapshotRetainPolicyWeekly  = "SnapshotRetainPolicyWeekly"
	SnapshotRetainPolicyMonthly = "SnapshotRetainPolicyMonthly"
	SnapshotRetainPolicyYearly  = "SnapshotRetainPolicyYearly"
	SnapshotRetainPolicyWithin  = "SnapshotRetainPolicyWithin"
)

// Restic secret vars to create new secrets
var (
	AWSAccessValue        []byte
	AWSSecretValue        []byte
	AWSDefaultRegionValue []byte

	AzureAccountNameValue []byte
	AzureAccountKeyValue  []byte

	GoogleApplicationCredentialsValue []byte

	ResticCustomCAValue      []byte
	ResticPasswordValue      []byte
	ResticRepoValue          string
	ResticPruneIntervalValue []byte
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

func BuildResticSecret(givensecret *corev1.Secret, secret *corev1.Secret, resticrepo, pruneInterval string) error {
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
			case key == ResticCustomCA:
				ResticCustomCAValue = val
			}
		}

		// build new Restic secret
		resticSecretData := &corev1.Secret{
			Data: map[string][]byte{
				AWSAccessKey:        AWSAccessValue,
				AWSSecretKey:        AWSSecretValue,
				AWSDefaultRegion:    AWSDefaultRegionValue,
				ResticCustomCA:      ResticCustomCAValue,
				ResticPassword:      ResticPasswordValue,
				ResticRepository:    []byte(resticrepo),
				ResticPruneInterval: []byte(pruneInterval),
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
			case key == ResticCustomCA:
				ResticCustomCAValue = val
			}
		}

		// build new Restic secret
		resticSecretData := &corev1.Secret{
			Data: map[string][]byte{
				AzureAccountName:    AzureAccountNameValue,
				AzureAccountKey:     AzureAccountKeyValue,
				ResticCustomCA:      ResticCustomCAValue,
				ResticPassword:      ResticPasswordValue,
				ResticRepository:    []byte(resticrepo),
				ResticPruneInterval: []byte(pruneInterval),
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
			case key == ResticCustomCA:
				ResticCustomCAValue = val
			}
		}

		// build new Restic secret
		resticSecretData := &corev1.Secret{
			Data: map[string][]byte{
				GoogleApplicationCredentials: GoogleApplicationCredentialsValue,
				ResticCustomCA:               ResticCustomCAValue,
				ResticPassword:               ResticPasswordValue,
				ResticRepository:             []byte(resticrepo),
				ResticPruneInterval:          []byte(pruneInterval),
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

		if len(vsb.Status.Phase) > 0 {
			// no need to check replicationSource progess if completed
			if vsb.Status.Phase == volsnapmoverv1alpha1.SnapMoverVolSyncPhaseCompleted {
				return true, nil
			}
		}

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
	return false, nil
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
		// recording completion timestamp for VSB as partially failed is a terminal state
		now := metav1.Now()
		vsb.Status.CompletionTimestamp = &now
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
		// recording completion timestamp for VSB as partially failed is a terminal state
		now := metav1.Now()
		vsr.Status.CompletionTimestamp = &now
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

func GetDataMoverConfigMap(namespace string, sc string, log logr.Logger, client client.Client) (*corev1.ConfigMap, error) {

	cm := corev1.ConfigMap{}
	err := client.Get(context.Background(), types.NamespacedName{Namespace: namespace, Name: fmt.Sprintf("%v-config", sc)}, &cm)
	// configmap will not exist if config values were not set
	if k8serrors.IsNotFound(err) {
		return nil, nil

	} else if err != nil {
		return nil, errors.New("failed to get data mover configMap")
	}

	return &cm, nil
}

func GetVeleroServiceAccount(namespace string, client client.Client) (*corev1.ServiceAccount, error) {
	sa := corev1.ServiceAccount{}

	err := client.Get(context.Background(), types.NamespacedName{Namespace: namespace, Name: "velero"}, &sa)
	if err != nil {
		return nil, err
	}

	return &sa, nil
}

func getVSMContainer(namespace string, client client.Client) (*corev1.Container, error) {
	vsmDeployment := appsv1.Deployment{}
	err := client.Get(context.TODO(), types.NamespacedName{Name: vsmDeploymentName, Namespace: namespace}, &vsmDeployment)
	if err != nil {
		return nil, err
	}

	// get VSM container
	var vsmContainer *corev1.Container
	for i, container := range vsmDeployment.Spec.Template.Spec.Containers {
		if container.Name == vsmContainerName {
			vsmContainer = &vsmDeployment.Spec.Template.Spec.Containers[i]
			break
		}
	}

	if vsmContainer == nil {
		return nil, errors.New(fmt.Sprintf("cannot obtain vsm container %s", vsmContainerName))
	}
	return vsmContainer, nil
}

func GetBackupBatchValue(namespace string, client client.Client) (string, error) {

	vsmContainer, err := getVSMContainer(namespace, client)
	if err != nil {
		return "", err
	}
	// get batching values from deployment env
	var backupBatchValue string

	for i, env := range vsmContainer.Env {
		if env.Name == batchBackupName {
			backupBatchValue = vsmContainer.Env[i].Value
			break
		}
	}

	if backupBatchValue == "" {
		return "", errors.New(fmt.Sprint("cannot obtain vsb batch value"))
	}

	return backupBatchValue, nil
}

func GetRestoreBatchValue(namespace string, client client.Client) (string, error) {

	vsmContainer, err := getVSMContainer(namespace, client)
	if err != nil {
		return "", err
	}
	// get batching values from deployment env
	var restoreBatchValue string

	for i, env := range vsmContainer.Env {
		if env.Name == batchRestoreName {
			restoreBatchValue = vsmContainer.Env[i].Value
			break
		}
	}

	if restoreBatchValue == "" {
		return "", errors.New(fmt.Sprint("cannot obtain vsb batch value"))
	}

	return restoreBatchValue, nil
}

func (r *VolumeSnapshotBackupReconciler) setVSBQueue(vsb *volsnapmoverv1alpha1.VolumeSnapshotBackup, log logr.Logger) (bool, error) {

	// update non-processed VSB as queued
	if processingVSBs >= VSBBatchNumber && len(vsb.Status.BatchingStatus) == 0 {
		log.Info(fmt.Sprintf("marking vsb %v batching status as queued", vsb.Name))

		vsb.Status.BatchingStatus = volsnapmoverv1alpha1.SnapMoverBackupBatchingQueued
		err := r.Status().Update(context.Background(), vsb)
		if err != nil {
			return false, err
		}

		// requeue VSB is max batch number is still being processed
	} else if processingVSBs >= VSBBatchNumber && vsb.Status.BatchingStatus == volsnapmoverv1alpha1.SnapMoverBackupBatchingQueued {
		return false, nil

		// add a queued VSB to processing batch
	} else if processingVSBs < VSBBatchNumber && (vsb.Status.BatchingStatus == "" ||
		vsb.Status.BatchingStatus == volsnapmoverv1alpha1.SnapMoverBackupBatchingQueued) {

		processingVSBs++
		log.Info(fmt.Sprintf("marking vsb %v batching status as processing", vsb.Name))

		vsb.Status.BatchingStatus = volsnapmoverv1alpha1.SnapMoverBackupBatchingProcessing
		err := r.Status().Update(context.Background(), vsb)
		if err != nil {
			return false, err
		}
	}

	return true, nil
}

func (r *VolumeSnapshotRestoreReconciler) setVSRQueue(vsr *volsnapmoverv1alpha1.VolumeSnapshotRestore, log logr.Logger) (bool, error) {

	// update non-processed VSR as queued
	if processingVSRs >= VSRBatchNumber && len(vsr.Status.BatchingStatus) == 0 {
		log.Info(fmt.Sprintf("marking vsr %v batching status as queued", vsr.Name))

		vsr.Status.BatchingStatus = volsnapmoverv1alpha1.SnapMoverRestoreBatchingQueued
		err := r.Status().Update(context.Background(), vsr)
		if err != nil {
			return false, err
		}

		// requeue VSR is max batch number is still being processed
	} else if processingVSRs >= VSRBatchNumber && vsr.Status.BatchingStatus == volsnapmoverv1alpha1.SnapMoverRestoreBatchingQueued {
		return false, nil

		// add a queued VSR to processing batch
	} else if processingVSRs < VSRBatchNumber && (vsr.Status.BatchingStatus == "" ||
		vsr.Status.BatchingStatus == volsnapmoverv1alpha1.SnapMoverRestoreBatchingQueued) {

		processingVSRs++
		log.Info(fmt.Sprintf("marking vsr %v batching status as processing", vsr.Name))

		vsr.Status.BatchingStatus = volsnapmoverv1alpha1.SnapMoverRestoreBatchingProcessing
		err := r.Status().Update(context.Background(), vsr)
		if err != nil {
			return false, err
		}
	}

	return true, nil
}

func GetPodSecurityContext(namespace string, sourcePVCName string, c client.Client) (*corev1.PodSecurityContext, error) {

	podList := corev1.PodList{}
	if err := c.List(context.Background(), &podList, &client.ListOptions{Namespace: namespace}); err != nil {
		return nil, err
	}

	for _, pod := range podList.Items {
		if pod.Spec.Volumes != nil {
			po := podHasPVCName(&pod, sourcePVCName)

			// pod containing volume
			if po != nil {
				sc := getContainerSecurityContext(po)

				// pod with container securityContext
				if sc != nil {
					podSC := buildPodSecurityContext(*sc, *pod.Spec.SecurityContext)
					return podSC, nil
				}

				// pod with podSecurityContext without container securityContext
				if sc == nil && po.Spec.SecurityContext != nil {
					return po.Spec.SecurityContext, nil
				}

				break
			}
		}
	}

	return nil, nil
}

func podHasPVCName(pod *corev1.Pod, pvcName string) *corev1.Pod {

	for _, vol := range pod.Spec.Volumes {
		if vol.PersistentVolumeClaim != nil &&
			vol.PersistentVolumeClaim.ClaimName == pvcName {
			return pod
		}
	}
	return nil
}

func getContainerSecurityContext(pod *corev1.Pod) *corev1.SecurityContext {

	containerSC := corev1.SecurityContext{}
	for _, container := range pod.Spec.Containers {
		if container.SecurityContext != nil {
			containerSC = *container.SecurityContext
			return &containerSC
		}
	}
	return nil
}

func buildPodSecurityContext(sc corev1.SecurityContext, podSC corev1.PodSecurityContext) *corev1.PodSecurityContext {

	// updated podSecurityContext fields that can also be found in
	// container securityContext
	if sc.SELinuxOptions != nil {
		podSC.SELinuxOptions = sc.SELinuxOptions
	}
	if sc.WindowsOptions != nil {
		podSC.WindowsOptions = sc.WindowsOptions
	}
	if sc.RunAsUser != nil {
		podSC.RunAsUser = sc.RunAsUser
	}
	if sc.RunAsGroup != nil {
		podSC.RunAsGroup = sc.RunAsGroup
	}
	if sc.RunAsNonRoot != nil {
		podSC.RunAsNonRoot = sc.RunAsNonRoot
	}

	return &podSC
}
