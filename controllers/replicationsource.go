package controllers

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	volsyncv1alpha1 "github.com/backube/volsync/api/v1alpha1"
	"github.com/go-logr/logr"
	volsnapmoverv1alpha1 "github.com/konveyor/volume-snapshot-mover/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func (r *VolumeSnapshotBackupReconciler) CreateReplicationSource(log logr.Logger) (bool, error) {
	// get volumesnapshotbackup from cluster
	vsb := volsnapmoverv1alpha1.VolumeSnapshotBackup{}
	if err := r.Get(r.Context, r.req.NamespacedName, &vsb); err != nil {
		// ignore is not found error
		if k8serrors.IsNotFound(err) {
			return true, nil
		}
		r.Log.Error(err, fmt.Sprintf("unable to fetch volumesnapshotbackup %s", r.req.NamespacedName))
		return false, err
	}

	// get cloned pvc
	pvcName := fmt.Sprintf("%s-pvc", vsb.Spec.VolumeSnapshotContent.Name)
	clonedPVC := corev1.PersistentVolumeClaim{}
	if err := r.Get(r.Context, types.NamespacedName{Namespace: vsb.Spec.ProtectedNamespace, Name: pvcName}, &clonedPVC); err != nil {
		r.Log.Error(err, fmt.Sprintf("unable to fetch cloned PVC %s/%s", vsb.Spec.ProtectedNamespace, pvcName))
		return false, err
	}

	cm, err := GetDataMoverConfigMap(vsb.Spec.ProtectedNamespace, r.Log, r.Client)
	if err != nil {
		return false, err
	}

	veleroSA, err := GetVeleroServiceAccount(vsb.Spec.ProtectedNamespace, r.Client)
	if err != nil {
		return false, err
	}

	// define replicationSource to be created
	repSource := &volsyncv1alpha1.ReplicationSource{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-rep-src", vsb.Name),
			Namespace: vsb.Spec.ProtectedNamespace,
			Labels: map[string]string{
				VSBLabel: vsb.Name,
			},
		},
	}

	// Create ReplicationSource in OADP namespace
	op, err := controllerutil.CreateOrUpdate(r.Context, r.Client, repSource, func() error {

		return r.buildReplicationSource(repSource, &vsb, &clonedPVC, cm, veleroSA)
	})
	if err != nil {
		return false, err
	}

	if op == controllerutil.OperationResultCreated || op == controllerutil.OperationResultUpdated {
		r.EventRecorder.Event(repSource,
			corev1.EventTypeNormal,
			"ReplicationSourceReconciled",
			fmt.Sprintf("%s replicationsource %s", op, repSource.Name),
		)
	}
	return true, nil
}

func (r *VolumeSnapshotBackupReconciler) buildReplicationSource(replicationSource *volsyncv1alpha1.ReplicationSource, vsb *volsnapmoverv1alpha1.VolumeSnapshotBackup,
	pvc *corev1.PersistentVolumeClaim, cm *corev1.ConfigMap, sa *corev1.ServiceAccount) error {

	if vsb == nil {
		return errors.New("nil vsb in buildReplicationSource")
	}

	if replicationSource == nil {
		return errors.New("nil replicationSource in buildReplicationSource")
	}

	if pvc == nil {
		return errors.New("nil pvc in buildReplicationSource")
	}

	if sa == nil {
		return errors.New("nil serviceAccount in buildReplicationSource")
	}

	// get restic secret created by controller
	resticSecretName := fmt.Sprintf("%s-secret", vsb.Name)
	resticSecret := corev1.Secret{}
	if err := r.Get(r.Context, types.NamespacedName{Namespace: vsb.Spec.ProtectedNamespace, Name: resticSecretName}, &resticSecret); err != nil {
		r.Log.Error(err, fmt.Sprintf("unable to fetch restic secret %s/%s", vsb.Spec.ProtectedNamespace, resticSecretName))
		return err
	}

	// fetch the prune interval if specified in the secret
	var pruneIntervalInt = int64(0)
	var err error
	pruneInterval := resticSecret.Data["restic-prune-interval"]

	if len(pruneInterval) > 0 {
		pruneIntervalInt, err = strconv.ParseInt(string(pruneInterval), 10, 32)
		if err != nil {
			return err
		}
	}

	resticVolOptions, err := r.configureRepSourceResticVolOptions(vsb, resticSecret.Name, pvc, cm, sa)
	if err != nil {
		return err
	}

	// build ReplicationSource
	replicationSourceSpec := volsyncv1alpha1.ReplicationSourceSpec{
		SourcePVC: pvc.Name,
		Trigger: &volsyncv1alpha1.ReplicationSourceTriggerSpec{
			Manual: fmt.Sprintf("%s-trigger", vsb.Name),
		},
		Restic: resticVolOptions,
	}

	if pruneIntervalInt != 0 {
		replicationSourceSpec.Restic.PruneIntervalDays = pointer.Int32(int32(pruneIntervalInt))
	}
	if replicationSource.CreationTimestamp.IsZero() {
		replicationSource.Spec = replicationSourceSpec
	}

	return nil
}

func (r *VolumeSnapshotBackupReconciler) setStatusFromRepSource(vsb *volsnapmoverv1alpha1.VolumeSnapshotBackup, repSource *volsyncv1alpha1.ReplicationSource) (bool, error) {

	if vsb == nil {
		return false, errors.New("nil vsb in setStatusFromRepSource")
	}

	if repSource == nil {
		return false, errors.New("nil repSource in setStatusFromRepSource")
	}

	// add replicationsource name to VSB status
	if len(vsb.Status.ReplicationSourceData.Name) == 0 {
		vsb.Status.ReplicationSourceData.Name = repSource.Name
	}

	// check for ReplicationSource phase
	repSourceCompleted, err := r.isRepSourceCompleted(vsb)
	if err != nil {
		return false, err
	}

	reconConditionCompleted := metav1.Condition{}
	reconConditionProgress := metav1.Condition{}

	for i := range repSource.Status.Conditions {
		if repSource.Status.Conditions[i].Status == metav1.ConditionFalse {
			reconConditionCompleted = repSource.Status.Conditions[i]
		}
		if repSource.Status.Conditions[i].Reason == volsyncv1alpha1.SynchronizingReasonSync {
			reconConditionProgress = repSource.Status.Conditions[i]
		}
	}

	if repSourceCompleted && reconConditionCompleted.Type == volsyncv1alpha1.ConditionSynchronizing {

		// Update VSB status as volsync completed
		vsb.Status.Phase = volsnapmoverv1alpha1.SnapMoverVolSyncPhaseCompleted
		r.Log.Info(fmt.Sprintf("marking volumesnapshotbackup %s VolSync phase as complete", r.req.NamespacedName))

		// recording completion timestamp for VSB as completed is a terminal state
		now := metav1.Now()
		vsb.Status.CompletionTimestamp = &now

		//recording replication source completion timestamp on VSB's status
		if repSource.Status.LastSyncTime != nil {
			vsb.Status.ReplicationSourceData.CompletionTimestamp = repSource.Status.LastSyncTime
		}

		vsb.Status.BatchingStatus = volsnapmoverv1alpha1.SnapMoverBackupBatchingCompleted
		r.Log.Info(fmt.Sprintf("marking volumesnapshotbackup %s batching status as completed", vsb.Name))

		processingVSBs--

		err := r.Status().Update(context.Background(), vsb)
		if err != nil {
			return false, err
		}

		return true, nil

		// ReplicationSource phase is still in progress
	} else if !repSourceCompleted && reconConditionProgress.Status == metav1.ConditionTrue {
		vsb.Status.Phase = volsnapmoverv1alpha1.SnapMoverBackupPhaseInProgress
		vsb.Status.ReplicationSourceData.StartTimestamp = repSource.Status.LastSyncStartTime

		// Update VSB status as in progress
		err := r.Status().Update(context.Background(), vsb)
		if err != nil {
			return false, err
		}
		r.Log.Info(fmt.Sprintf("marking volumesnapshotbackup %s as in progress", r.req.NamespacedName))
		return false, nil

		//if not in progress or completed, phase failed
	} else if reconConditionProgress.Reason == volsyncv1alpha1.SynchronizingReasonError {
		vsb.Status.Phase = volsnapmoverv1alpha1.SnapMoverBackupPhaseFailed
		// recording completion timestamp for VSB as failed is a terminal state
		now := metav1.Now()
		vsb.Status.CompletionTimestamp = &now
		// Update VSB status
		err := r.Status().Update(context.Background(), vsb)
		if err != nil {
			return false, err
		}
		r.Log.Info(fmt.Sprintf("marking volumesnapshotbackup %s as failed", r.req.NamespacedName))
		return false, nil
	}
	r.Log.Info(fmt.Sprintf("waiting for replicationsource %s/%s to complete", vsb.Spec.ProtectedNamespace, repSource.Name))
	return false, nil
}

func (r *VolumeSnapshotBackupReconciler) isRepSourceCompleted(vsb *volsnapmoverv1alpha1.VolumeSnapshotBackup) (bool, error) {

	if vsb == nil {
		return false, errors.New("nil vsb in isRepSourceCompleted")
	}

	// get replicationsource
	repSourceName := fmt.Sprintf("%s-rep-src", vsb.Name)
	repSource := volsyncv1alpha1.ReplicationSource{}
	if err := r.Get(r.Context, types.NamespacedName{Namespace: vsb.Spec.ProtectedNamespace, Name: repSourceName}, &repSource); err != nil {
		r.Log.Info(fmt.Sprintf("unable to fetch replicationsource %s/%s", vsb.Spec.ProtectedNamespace, repSourceName))
		return false, err
	}

	if repSource.Status != nil {
		// for manual trigger, if spec.trigger.manual == status.lastManualSync, sync has completed
		if len(repSource.Status.LastManualSync) > 0 && len(repSource.Spec.Trigger.Manual) > 0 {
			sourceStatus := repSource.Status.LastManualSync
			sourceSpec := repSource.Spec.Trigger.Manual
			if sourceStatus == sourceSpec {
				return true, nil
			}
		}
	}

	// ReplicationSource has not yet completed but is not failed
	return false, nil
}

func (r *VolumeSnapshotBackupReconciler) configureRepSourceVolOptions(vsb *volsnapmoverv1alpha1.VolumeSnapshotBackup, pvc *corev1.PersistentVolumeClaim, cm *corev1.ConfigMap) (*volsyncv1alpha1.ReplicationSourceVolumeOptions, error) {

	if vsb == nil {
		return nil, errors.New("nil vsb in configureRepSourceVolOptions")
	}

	if pvc == nil {
		return nil, errors.New("nil pvc in configureRepSourceVolOptions")
	}

	// we do not want users to change these
	repSrcVolOptions := volsyncv1alpha1.ReplicationSourceVolumeOptions{
		CopyMethod:              volsyncv1alpha1.CopyMethodDirect,
		VolumeSnapshotClassName: &vsb.Status.VolumeSnapshotClassName,
	}

	// use source PVC storageClass as default
	repSourceStorageClass := vsb.Status.SourcePVCData.StorageClassName
	// use source PVC accessMode as default
	repSourceAccessModeAM := pvc.Spec.AccessModes

	var repSourceAccessMode string

	// if datamover configmap has data, use these values
	if cm != nil && cm.Data != nil {
		for spec := range cm.Data {

			// check for config storageClassName, otherwise use source PVC storageClass
			if spec == "SourceStorageClassName" {
				repSourceStorageClass = cm.Data["SourceStorageClassName"]
			}

			// check for config accessMode, otherwise use source PVC accessMode
			if spec == "SourceAccessMode" {
				repSourceAccessMode = cm.Data["SourceAccessMode"]
				repSourceAccessModeAM = []corev1.PersistentVolumeAccessMode{corev1.PersistentVolumeAccessMode(repSourceAccessMode)}
			}
		}
	}

	repSrcVolOptions.StorageClassName = &repSourceStorageClass
	repSrcVolOptions.AccessModes = repSourceAccessModeAM

	return &repSrcVolOptions, nil
}

func (r *VolumeSnapshotBackupReconciler) configureRepSourceResticVolOptions(vsb *volsnapmoverv1alpha1.VolumeSnapshotBackup, resticSecretName string,
	pvc *corev1.PersistentVolumeClaim, cm *corev1.ConfigMap, sa *corev1.ServiceAccount) (*volsyncv1alpha1.ReplicationSourceResticSpec, error) {

	if vsb == nil {
		return nil, errors.New("nil vsr in configureRepSourceResticVolOptions")
	}

	if pvc == nil {
		return nil, errors.New("nil pvc in configureRepSourceResticVolOptions")
	}

	repSrcResticVolOptions := volsyncv1alpha1.ReplicationSourceResticSpec{}
	repSrcResticVolOptions.Repository = resticSecretName

	repSrcResticVolOptions.MoverServiceAccount = &sa.Name

	var repSourceCacheStorageClass string
	var repSourceCaceheStorageClassPt *string

	var repSourceCacheAccessMode string

	var repSourceCacheCapacity string
	var repSourceCacheCapacityCp resource.Quantity

	if cm != nil && cm.Data != nil {
		for spec := range cm.Data {

			// check for config cacheStorageClassName, otherwise use source PVC storageClass
			if spec == "SourceCacheStorageClassName" {
				repSourceCacheStorageClass = cm.Data["SourceCacheStorageClassName"]
				repSourceCaceheStorageClassPt = &repSourceCacheStorageClass

				repSrcResticVolOptions.CacheStorageClassName = repSourceCaceheStorageClassPt
			}

			// check for config cacheAccessMode, otherwise use source PVC accessMode
			if spec == "SourceCacheAccessMode" {
				repSourceCacheAccessMode = cm.Data["SourceCacheAccessMode"]
				repSrcResticVolOptions.CacheAccessModes = []corev1.PersistentVolumeAccessMode{corev1.PersistentVolumeAccessMode(repSourceCacheAccessMode)}
			}

			// check for config cacheCapacity, otherwise use source PVC capacity
			if spec == "SourceCacheCapacity" {
				repSourceCacheCapacity = cm.Data["SourceCacheCapacity"]
				repSourceCacheCapacityCp = resource.MustParse(repSourceCacheCapacity)

				repSrcResticVolOptions.CacheCapacity = &repSourceCacheCapacityCp
			}
		}
	}

	optionsSpec, err := r.configureRepSourceVolOptions(vsb, pvc, cm)
	if err != nil {
		return nil, err
	}

	if optionsSpec != nil {
		repSrcResticVolOptions.ReplicationSourceVolumeOptions = *optionsSpec
	}

	return &repSrcResticVolOptions, nil
}
