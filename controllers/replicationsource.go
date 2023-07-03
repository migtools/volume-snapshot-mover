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

	// no need to create rs for the vsb if the datamovement has already completed
	if len(vsb.Status.Phase) > 0 && vsb.Status.Phase == volsnapmoverv1alpha1.SnapMoverBackupPhaseCompleted {
		r.Log.Info(fmt.Sprintf("skipping create rs step for vsb %s/%s as datamovement is complete", vsb.Namespace, vsb.Name))
		return true, nil
	}

	// get cloned pvc
	pvcName := fmt.Sprintf("%s-pvc", vsb.Spec.VolumeSnapshotContent.Name)
	clonedPVC := corev1.PersistentVolumeClaim{}
	if err := r.Get(r.Context, types.NamespacedName{Namespace: vsb.Spec.ProtectedNamespace, Name: pvcName}, &clonedPVC); err != nil {
		r.Log.Error(err, fmt.Sprintf("unable to fetch cloned PVC %s/%s", vsb.Spec.ProtectedNamespace, pvcName))
		return false, err
	}

	if len(vsb.Status.SourcePVCData.Name) == 0 {
		return false, nil
	}

	cm, err := GetDataMoverConfigMap(vsb.Spec.ProtectedNamespace, vsb.Status.SourcePVCData.StorageClassName, r.Log, r.Client)
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

	//fetch the schedule crop expression specified in the secret
	scheduleCron := ""
	if len(resticSecret.Data[SnapshotScheduleCron]) > 0 {
		scheduleCron = string(resticSecret.Data[SnapshotScheduleCron])
	}

	if len(pruneInterval) > 0 {
		pruneIntervalInt, err = strconv.ParseInt(string(pruneInterval), 10, 32)
		if err != nil {
			return err
		}
	}

	// fetch the retain policy from restic secret and then pass it on to replication source CR
	// modify the configureRepSourceResticVolOptions function to
	rpolicy := RetainPolicy{}
	rpolicy.hourly = string(resticSecret.Data[SnapshotRetainPolicyHourly])
	rpolicy.monthly = string(resticSecret.Data[SnapshotRetainPolicyMonthly])
	rpolicy.daily = string(resticSecret.Data[SnapshotRetainPolicyDaily])
	rpolicy.weekly = string(resticSecret.Data[SnapshotRetainPolicyWeekly])
	rpolicy.yearly = string(resticSecret.Data[SnapshotRetainPolicyYearly])
	rpolicy.within = string(resticSecret.Data[SnapshotRetainPolicyWithin])

	resticVolOptions, err := r.configureRepSourceResticVolOptions(vsb, resticSecret.Name, pvc, cm, sa, rpolicy)
	if err != nil {
		return err
	}

	// build ReplicationSource
	replicationSourceSpec := r.getReplicationSourceSpec(vsb.Name, pvc.Name, scheduleCron, resticVolOptions)

	if pruneIntervalInt != 0 {
		replicationSourceSpec.Restic.PruneIntervalDays = pointer.Int32(int32(pruneIntervalInt))
	}
	if replicationSource.CreationTimestamp.IsZero() {
		replicationSource.Spec = replicationSourceSpec
	}

	// pass along a custom CA if specified
	resticCustomCA := resticSecret.Data[ResticCustomCA]
	if len(resticCustomCA) > 0 {
		replicationSource.Spec.Restic.CustomCA.SecretName = resticSecret.Name
		replicationSource.Spec.Restic.CustomCA.Key = ResticCustomCA
	}

	return nil
}

func (r *VolumeSnapshotBackupReconciler) getReplicationSourceSpec(vsbName, pvcName, scheduleCron string, volOpts *volsyncv1alpha1.ReplicationSourceResticSpec) volsyncv1alpha1.ReplicationSourceSpec {
	if len(scheduleCron) > 0 {
		return volsyncv1alpha1.ReplicationSourceSpec{
			SourcePVC: pvcName,
			Trigger: &volsyncv1alpha1.ReplicationSourceTriggerSpec{
				Schedule: &scheduleCron,
			},
			Restic: volOpts,
		}
	}

	return volsyncv1alpha1.ReplicationSourceSpec{
		SourcePVC: pvcName,
		Trigger: &volsyncv1alpha1.ReplicationSourceTriggerSpec{
			Manual: fmt.Sprintf("%s-trigger", vsbName),
		},
		Restic: volOpts,
	}
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

	if (len(repSource.Spec.Trigger.Manual) > 0 && repSourceCompleted && reconConditionCompleted.Type == volsyncv1alpha1.ConditionSynchronizing && vsb.Status.Phase != volsnapmoverv1alpha1.SnapMoverBackupPhaseCompleted) ||
		(repSource.Spec.Trigger.Schedule != nil && repSourceCompleted && vsb.Status.Phase != volsnapmoverv1alpha1.SnapMoverBackupPhaseCompleted) {

		r.Log.Info(fmt.Sprintf("marking volumesnapshotbackup %s batching status as completed", vsb.Name))
		err = r.updateVSBBatchingStatus(volsnapmoverv1alpha1.SnapMoverBackupBatchingCompleted, r.Client)
		if err != nil {
			return false, err
		}

		r.Log.Info(fmt.Sprintf("marking volumesnapshotbackup %s VolSync phase as complete", r.req.NamespacedName))
		err := r.updateVSBStatusPhase(repSource, volsnapmoverv1alpha1.SnapMoverVolSyncPhaseCompleted, r.Client)
		if err != nil {
			return false, err
		}

		processingVSBs--

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
		// for schedule trigger, LastSyncTime will be set at the end of every replication.
		if repSource.Spec.Trigger.Schedule != nil && repSource.Status.NextSyncTime != nil && repSource.Status.LastSyncTime != nil {
			return true, nil
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
			if spec == SourceStorageClassName {
				repSourceStorageClass = cm.Data[SourceStorageClassName]
			}

			// check for config accessMode, otherwise use source PVC accessMode
			if spec == SourceAccessMoce {
				repSourceAccessMode = cm.Data[SourceAccessMoce]
				repSourceAccessModeAM = []corev1.PersistentVolumeAccessMode{corev1.PersistentVolumeAccessMode(repSourceAccessMode)}
			}
		}
	}

	repSrcVolOptions.StorageClassName = &repSourceStorageClass
	repSrcVolOptions.AccessModes = repSourceAccessModeAM

	return &repSrcVolOptions, nil
}

func (r *VolumeSnapshotBackupReconciler) configureRepSourceSnapshotRetainPolicy(rpolicy RetainPolicy) (*volsyncv1alpha1.ResticRetainPolicy, error) {
	// check for retain policy params in restic secret data
	var daily, hourly, weekly, monthly, yearly = int64(0), int64(0), int64(0), int64(0), int64(0)
	var err error
	retainPolicy := volsyncv1alpha1.ResticRetainPolicy{}
	// if datamover retainpolicy has data, use these values
	if len(rpolicy.daily) > 0 {
		daily, err = strconv.ParseInt(rpolicy.daily, 10, 32)
		if err != nil {
			return nil, err
		}
	}

	if len(rpolicy.hourly) > 0 {
		hourly, err = strconv.ParseInt(rpolicy.hourly, 10, 32)

		if err != nil {
			return nil, err
		}
	}

	if len(rpolicy.weekly) > 0 {
		weekly, err = strconv.ParseInt(rpolicy.weekly, 10, 32)

		if err != nil {
			return nil, err
		}
	}

	if len(rpolicy.monthly) > 0 {
		monthly, err = strconv.ParseInt(rpolicy.monthly, 10, 32)

		if err != nil {
			return nil, err
		}
	}

	if len(rpolicy.yearly) > 0 {
		yearly, err = strconv.ParseInt(rpolicy.yearly, 10, 32)

		if err != nil {
			return nil, err
		}
	}

	if daily != 0 {
		retainPolicy.Daily = pointer.Int32(int32(daily))
	}

	if hourly != 0 {
		retainPolicy.Hourly = pointer.Int32(int32(hourly))
	}

	if weekly != 0 {
		retainPolicy.Weekly = pointer.Int32(int32(weekly))
	}

	if monthly != 0 {
		retainPolicy.Monthly = pointer.Int32(int32(monthly))
	}

	if yearly != 0 {
		retainPolicy.Yearly = pointer.Int32(int32(yearly))
	}

	if len(rpolicy.within) > 0 {
		retainPolicy.Within = &rpolicy.within
	}

	return &retainPolicy, nil
}

func (r *VolumeSnapshotBackupReconciler) configureRepSourceResticVolOptions(vsb *volsnapmoverv1alpha1.VolumeSnapshotBackup, resticSecretName string,
	pvc *corev1.PersistentVolumeClaim, cm *corev1.ConfigMap, sa *corev1.ServiceAccount, rpolicy RetainPolicy) (*volsyncv1alpha1.ReplicationSourceResticSpec, error) {

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
			if spec == SourceCacheStorageClassName {
				repSourceCacheStorageClass = cm.Data[SourceCacheStorageClassName]
				repSourceCaceheStorageClassPt = &repSourceCacheStorageClass

				repSrcResticVolOptions.CacheStorageClassName = repSourceCaceheStorageClassPt
			}

			// check for config cacheAccessMode, otherwise use source PVC accessMode
			if spec == SourceCacheAccessMoce {
				repSourceCacheAccessMode = cm.Data[SourceCacheAccessMoce]
				repSrcResticVolOptions.CacheAccessModes = []corev1.PersistentVolumeAccessMode{corev1.PersistentVolumeAccessMode(repSourceCacheAccessMode)}
			}

			// check for config cacheCapacity, otherwise use source PVC capacity
			if spec == SourceCacheCapacity {
				repSourceCacheCapacity = cm.Data[SourceCacheCapacity]
				repSourceCacheCapacityCp = resource.MustParse(repSourceCacheCapacity)

				repSrcResticVolOptions.CacheCapacity = &repSourceCacheCapacityCp
			}

			if spec == SourceMoverSecurityContext {
				if cm.Data[SourceMoverSecurityContext] == "true" {
					podSC, err := GetPodSecurityContext(vsb.Namespace, vsb.Status.SourcePVCData.Name, r.Client)
					if err != nil {
						return nil, err
					}
					repSrcResticVolOptions.MoverSecurityContext = podSC
				}
			}

		}
	}

	optionsSpec, err := r.configureRepSourceVolOptions(vsb, pvc, cm)
	if err != nil {
		return nil, err
	}

	retainPolicySpec, err := r.configureRepSourceSnapshotRetainPolicy(rpolicy)
	if err != nil {
		return nil, err
	}

	if optionsSpec != nil {
		repSrcResticVolOptions.ReplicationSourceVolumeOptions = *optionsSpec
	}

	if retainPolicySpec != nil {
		repSrcResticVolOptions.Retain = retainPolicySpec
	}

	return &repSrcResticVolOptions, nil
}
