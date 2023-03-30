package controllers

import (
	"context"
	"errors"
	"fmt"

	k8serrors "k8s.io/apimachinery/pkg/api/errors"

	volsyncv1alpha1 "github.com/backube/volsync/api/v1alpha1"
	"github.com/go-logr/logr"
	volsnapmoverv1alpha1 "github.com/konveyor/volume-snapshot-mover/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	k8serror "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func (r *VolumeSnapshotRestoreReconciler) CreateReplicationDestination(log logr.Logger) (bool, error) {

	// get volumesnapshotrestore from cluster
	vsr := volsnapmoverv1alpha1.VolumeSnapshotRestore{}
	if err := r.Get(r.Context, r.req.NamespacedName, &vsr); err != nil {
		// ignore is not found error
		if k8serrors.IsNotFound(err) {
			return true, nil
		}
		r.Log.Error(err, fmt.Sprintf("unable to fetch volumesnapshotrestore %s", r.req.NamespacedName))
		return false, err
	}

	// define replicationDestination to be created
	repDestination := &volsyncv1alpha1.ReplicationDestination{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-rep-dest", vsr.Name),
			Namespace: r.NamespacedName.Namespace,
			Labels: map[string]string{
				VSRLabel: vsr.Name,
			},
		},
	}

	// get restic secret created by controller
	dmresticSecretName := fmt.Sprintf("%s-secret", vsr.Name)
	resticSecret := corev1.Secret{}
	if err := r.Get(r.Context, types.NamespacedName{Namespace: r.NamespacedName.Namespace, Name: dmresticSecretName}, &resticSecret); err != nil {
		if k8serrors.IsNotFound(err) {
			return false, nil
		}
		r.Log.Error(err, fmt.Sprintf("unable to fetch restic secret %s/%s", r.NamespacedName.Namespace, dmresticSecretName))
		return false, err
	}

	cm, err := GetDataMoverConfigMap(vsr.Spec.ProtectedNamespace, r.Log, r.Client)
	if err != nil {
		return false, err
	}

	veleroSA, err := GetVeleroServiceAccount(vsr.Spec.ProtectedNamespace, r.Client)
	if err != nil {
		return false, err
	}

	// Create ReplicationDestination in protected namespace
	op, err := controllerutil.CreateOrUpdate(r.Context, r.Client, repDestination, func() error {

		return r.buildReplicationDestination(repDestination, &vsr, &resticSecret, cm, veleroSA)
	})
	if err != nil {
		return false, err
	}

	if op == controllerutil.OperationResultCreated || op == controllerutil.OperationResultUpdated {
		r.EventRecorder.Event(repDestination,
			corev1.EventTypeNormal,
			"ReplicationDestinationReconciled",
			fmt.Sprintf("%s replicationdestination %s", op, repDestination.Name),
		)
	}
	return true, nil
}

func (r *VolumeSnapshotRestoreReconciler) buildReplicationDestination(replicationDestination *volsyncv1alpha1.ReplicationDestination, vsr *volsnapmoverv1alpha1.VolumeSnapshotRestore,
	resticSecret *corev1.Secret, cm *corev1.ConfigMap, sa *corev1.ServiceAccount) error {
	if vsr == nil {
		return errors.New("nil vsr in buildReplicationDestination")
	}

	if replicationDestination == nil {
		return errors.New("nil replicationDestination in buildReplicationDestination")
	}

	if resticSecret == nil {
		return errors.New("nil resticSecret in buildReplicationDestination")
	}

	if sa == nil {
		return errors.New("nil serviceAccount in buildReplicationDestination")
	}

	stringCapacity := vsr.Spec.VolumeSnapshotMoverBackupref.BackedUpPVCData.Size
	capacity := resource.MustParse(stringCapacity)

	resticVolOptions, err := r.configureRepDestResticVolOptions(vsr, resticSecret.Name, cm, &capacity, sa)
	if err != nil {
		return err
	}

	// build ReplicationDestination
	replicationDestinationSpec := volsyncv1alpha1.ReplicationDestinationSpec{
		Trigger: &volsyncv1alpha1.ReplicationDestinationTriggerSpec{
			Manual: fmt.Sprintf("%s-trigger", vsr.Name),
		},
		Restic: resticVolOptions,
	}

	if replicationDestination.CreationTimestamp.IsZero() {
		replicationDestination.Spec = replicationDestinationSpec
	}

	return nil
}

func (r *VolumeSnapshotRestoreReconciler) SetVSRStatus(log logr.Logger) (bool, error) {

	vsr := volsnapmoverv1alpha1.VolumeSnapshotRestore{}
	if err := r.Get(r.Context, r.req.NamespacedName, &vsr); err != nil {
		// ignore is not found error
		if k8serrors.IsNotFound(err) {
			return true, nil
		}
		r.Log.Error(err, fmt.Sprintf("unable to fetch volumesnapshotrestore %s", r.req.NamespacedName))
		return false, err
	}

	//update vsr status from restore
	err := updateVSRFromRestore(&vsr, r.Client, log)
	if err != nil {
		return false, err
	}

	if vsr.Status.Phase == volsnapmoverv1alpha1.SnapMoverRestorePhaseFailed ||
		vsr.Status.Phase == volsnapmoverv1alpha1.SnapMoverRestorePhasePartiallyFailed {
		return false, errors.New(fmt.Sprintf("vsr %s/%s failed to complete", vsr.Namespace, vsr.Name))
	}

	repDestName := fmt.Sprintf("%s-rep-dest", vsr.Name)
	repDest := volsyncv1alpha1.ReplicationDestination{}
	if err := r.Get(r.Context, types.NamespacedName{Namespace: vsr.Spec.ProtectedNamespace, Name: repDestName}, &repDest); err != nil {
		if k8serror.IsNotFound(err) {
			return false, nil
		}
		r.Log.Info(fmt.Sprintf("error getting replicationdestination %s/%s", vsr.Spec.ProtectedNamespace, repDestName))
		return false, err
	}

	if repDest.Status == nil {
		r.Log.Info(fmt.Sprintf("replication destination %s/%s is yet to have a status", vsr.Spec.ProtectedNamespace, repDestName))
		return false, nil
	}

	if repDest.Status != nil {
		reconConditionProgress := metav1.Condition{}

		// add replicationdestination name to VSR status
		if len(vsr.Status.ReplicationDestinationData.Name) == 0 {
			vsr.Status.ReplicationDestinationData.Name = repDest.Name
		}

		// save replicationDestination status conditions
		for i := range repDest.Status.Conditions {
			if repDest.Status.Conditions[i].Reason == volsyncv1alpha1.SynchronizingReasonSync {
				reconConditionProgress = repDest.Status.Conditions[i]
			}
		}

		// for manual trigger, if spec.trigger.manual == status.lastManualSync, sync has completed
		// VSR is completed
		if len(repDest.Status.LastManualSync) > 0 && len(repDest.Spec.Trigger.Manual) > 0 {
			sourceStatus := repDest.Status.LastManualSync
			sourceSpec := repDest.Spec.Trigger.Manual
			if sourceStatus == sourceSpec {

				r.Log.Info(fmt.Sprintf("marking volumesnapshotrestore %s as VolSync phase completed", r.req.NamespacedName))
				vsr.Status.Phase = volsnapmoverv1alpha1.SnapMoverRestoreVolSyncPhaseCompleted

				// recording completion timestamp for VSR as completed is a terminal state
				now := metav1.Now()
				vsr.Status.CompletionTimestamp = &now

				if repDest.Status.LastSyncTime != nil {
					vsr.Status.ReplicationDestinationData.CompletionTimestamp = repDest.Status.LastSyncTime
				}

				r.Log.Info(fmt.Sprintf("marking volumesnapshotrestore %s batching status as completed", vsr.Name))
				vsr.Status.BatchingStatus = volsnapmoverv1alpha1.SnapMoverRestoreBatchingCompleted

				processingVSRs--

				// Update VSR status as completed
				err := r.Status().Update(context.Background(), &vsr)
				if err != nil {
					return false, err
				}

				return true, nil
			}

			// VSR is in progress
		} else if reconConditionProgress.Status == metav1.ConditionTrue && reconConditionProgress.Reason == volsyncv1alpha1.SynchronizingReasonSync {

			vsr.Status.Phase = volsnapmoverv1alpha1.SnapMoverRestorePhaseInProgress
			vsr.Status.ReplicationDestinationData.StartTimestamp = repDest.Status.LastSyncStartTime
			err := r.Status().Update(context.Background(), &vsr)
			if err != nil {
				return false, err
			}
			r.Log.Info(fmt.Sprintf("marking volumesnapshotrestore %s as in progress", r.req.NamespacedName))
			return false, nil

			// if not in progress or completed, phase failed
		} else if reconConditionProgress.Reason == volsyncv1alpha1.SynchronizingReasonError {
			vsr.Status.Phase = volsnapmoverv1alpha1.SnapMoverRestorePhaseFailed
			// recording completion timestamp for VSR as failed is a terminal state
			now := metav1.Now()
			vsr.Status.CompletionTimestamp = &now

			err := r.Status().Update(context.Background(), &vsr)
			if err != nil {
				return false, err
			}
			r.Log.Info(fmt.Sprintf("marking volumesnapshotrestore %s as failed", r.req.NamespacedName))
			return false, nil
		}
	}

	r.Log.Info(fmt.Sprintf("waiting for replicationdestination %s/%s to complete", vsr.Spec.ProtectedNamespace, repDestName))
	return false, nil
}

func (r *VolumeSnapshotRestoreReconciler) configureRepDestVolOptions(vsr *volsnapmoverv1alpha1.VolumeSnapshotRestore, capacity *resource.Quantity, cm *corev1.ConfigMap) (*volsyncv1alpha1.ReplicationDestinationVolumeOptions, error) {

	if vsr == nil {
		return nil, errors.New("nil vsb in configureRepDestVolOptions")
	}

	if capacity == nil {
		return nil, errors.New("nil pvc in configureRepDestVolOptions")
	}

	// we do not want users to change these
	repDestVolOptions := volsyncv1alpha1.ReplicationDestinationVolumeOptions{
		CopyMethod:              volsyncv1alpha1.CopyMethodDirect,
		VolumeSnapshotClassName: &vsr.Spec.VolumeSnapshotMoverBackupref.VolumeSnapshotClassName,
		Capacity:                capacity,
	}

	var repDestAccessMode string
	// use source PVC storageClass as default
	repDestStorageClass := vsr.Spec.VolumeSnapshotMoverBackupref.BackedUpPVCData.StorageClassName
	// use source PVC accessMode as default
	repDestAccessModeAM := []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce}

	// if datamover configmap has data, use these values
	if cm != nil && cm.Data != nil {
		for spec := range cm.Data {

			// check for config storageClassName, otherwise use source PVC storageClass
			if spec == DestinationStorageClassName {
				repDestStorageClass = cm.Data[DestinationStorageClassName]
			}

			// check for config accessMode, otherwise use source PVC accessMode
			if spec == DestinationAccessMoce {
				repDestAccessMode = cm.Data[DestinationAccessMoce]
				repDestAccessModeAM = []corev1.PersistentVolumeAccessMode{corev1.PersistentVolumeAccessMode(repDestAccessMode)}
			}
		}
	}

	repDestVolOptions.StorageClassName = &repDestStorageClass
	repDestVolOptions.AccessModes = repDestAccessModeAM

	return &repDestVolOptions, nil
}

func (r *VolumeSnapshotRestoreReconciler) configureRepDestResticVolOptions(vsr *volsnapmoverv1alpha1.VolumeSnapshotRestore, resticSecretName string,
	cm *corev1.ConfigMap, capacity *resource.Quantity, sa *corev1.ServiceAccount) (*volsyncv1alpha1.ReplicationDestinationResticSpec, error) {

	if vsr == nil {
		return nil, errors.New("nil vsr in configureRepDestResticVolOptions")
	}

	if capacity == nil {
		return nil, errors.New("nil capacity in configureRepDestResticVolOptions")
	}

	repDestResticVolOptions := volsyncv1alpha1.ReplicationDestinationResticSpec{}
	repDestResticVolOptions.Repository = resticSecretName

	repDestResticVolOptions.MoverServiceAccount = &sa.Name

	var repDestCacheStorageClass string
	var repDestCaceheStorageClassPt *string

	var repDestCacheAccessMode string

	var repDestCacheCapacity string
	var repDestCacheCapacityCp resource.Quantity

	if cm != nil && cm.Data != nil {
		for spec := range cm.Data {

			// check for config cacheStorageClassName, otherwise use source PVC storageClass
			if spec == DestinationCacheStorageClassName {
				repDestCacheStorageClass = cm.Data[DestinationCacheStorageClassName]
				repDestCaceheStorageClassPt = &repDestCacheStorageClass

				repDestResticVolOptions.CacheStorageClassName = repDestCaceheStorageClassPt
			}

			// check for config cacheAccessMode, otherwise use source PVC accessMode
			if spec == DestinationCacheAccessMoce {
				repDestCacheAccessMode = cm.Data[DestinationCacheAccessMoce]
				repDestResticVolOptions.CacheAccessModes = []corev1.PersistentVolumeAccessMode{corev1.PersistentVolumeAccessMode(repDestCacheAccessMode)}
			}

			// check for config cacheCapacity, otherwise use source PVC capacity
			if spec == DestinationCacheCapacity {
				repDestCacheCapacity = cm.Data[DestinationCacheCapacity]
				repDestCacheCapacityCp = resource.MustParse(repDestCacheCapacity)

				repDestResticVolOptions.CacheCapacity = &repDestCacheCapacityCp
			}

			if spec == DestinationMoverSecurityContext {
				if cm.Data[DestinationMoverSecurityContext] == "true" {
					podSC, err := GetPodSecurityContext(vsr.Namespace, vsr.Spec.VolumeSnapshotMoverBackupref.BackedUpPVCData.Name, r.Client)
					if err != nil {
						return nil, err
					}
					repDestResticVolOptions.MoverSecurityContext = podSC
				}
			}
		}
	}

	optionsSpec, err := r.configureRepDestVolOptions(vsr, capacity, cm)
	if err != nil {
		return nil, err
	}

	if optionsSpec != nil {
		repDestResticVolOptions.ReplicationDestinationVolumeOptions = *optionsSpec
	}

	return &repDestResticVolOptions, nil
}
