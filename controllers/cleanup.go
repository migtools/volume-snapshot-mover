package controllers

import (
	"context"
	"fmt"

	volsyncv1alpha1 "github.com/backube/volsync/api/v1alpha1"
	"github.com/go-logr/logr"
	volsnapmoverv1alpha1 "github.com/konveyor/volume-snapshot-mover/api/v1alpha1"
	snapv1 "github.com/kubernetes-csi/external-snapshotter/client/v4/apis/volumesnapshot/v1"
	corev1 "k8s.io/api/core/v1"
	k8serror "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var cleanupVSRTypes = []client.Object{
	&corev1.Secret{},
	&volsyncv1alpha1.ReplicationDestination{},
}

func (r *VolumeSnapshotBackupReconciler) CleanBackupResources(log logr.Logger) (bool, error) {
	cleanupVSBTypes := []client.Object{
		&corev1.PersistentVolumeClaim{},
		&corev1.Pod{},
		&snapv1.VolumeSnapshot{},
		&snapv1.VolumeSnapshotContent{},
		&corev1.Secret{},
	}

	// get volumesnapshotbackup from cluster
	vsb := volsnapmoverv1alpha1.VolumeSnapshotBackup{}
	if err := r.Get(r.Context, r.req.NamespacedName, &vsb); err != nil {
		r.Log.Error(err, fmt.Sprintf("unable to fetch volumesnapshotbackup %s", r.req.NamespacedName))
		return false, err
	}

	// make sure VSB is completed or failed before deleting resources AND VSB has not been deleted
	if vsb.Status.Phase != volsnapmoverv1alpha1.SnapMoverVolSyncPhaseCompleted &&
		vsb.Status.Phase != volsnapmoverv1alpha1.SnapMoverBackupPhaseFailed &&
		vsb.Status.Phase != volsnapmoverv1alpha1.SnapMoverBackupPhasePartiallyFailed &&
		vsb.DeletionTimestamp.IsZero() {
		r.Log.Info(fmt.Sprintf("waiting for volsync to complete before deleting volumesnapshotbackup %s resources", r.req.NamespacedName))
		return false, nil
	}

	// no need to perfrom cleanup for the vsb if the datamovement has already completed, completed phase comes after cleanup
	if len(vsb.Status.Phase) > 0 && vsb.Status.Phase == volsnapmoverv1alpha1.SnapMoverBackupPhaseCompleted {
		r.Log.Info(fmt.Sprintf("skipping CleanBackupResources step for vsb %s/%s as datamovement is complete", vsb.Namespace, vsb.Name))
		return true, nil
	}

	// get resources with VSB controller label in protected ns
	deleteOptions := []client.DeleteAllOfOption{
		client.MatchingLabels{VSBLabel: vsb.Name},
		client.InNamespace(vsb.Spec.ProtectedNamespace),
	}

	// Update VSB status as Cleanup
	vsb.Status.Phase = volsnapmoverv1alpha1.SnapMoverBackupPhaseCleanup
	err := r.Status().Update(context.Background(), &vsb)
	if err != nil {
		return false, err
	}

	// Check if retain policy is set by the user for datamover backups
	retainPolicyPresent, err := r.isRetainPolicySet(&vsb)
	if err != nil {
		return false, err
	}

	// Check if schedule cron trigger is set by the user for datamover backups
	scheduleCronTriggerPresent, err := r.scheduleCronTriggerSet(&vsb)
	if err != nil {
		return false, err
	}

	// If retainPolicy or schedule cron trigger is not set then cleanup the ReplicationSource objects as well
	if !retainPolicyPresent && !scheduleCronTriggerPresent {
		cleanupVSBTypes = append(cleanupVSBTypes, &volsyncv1alpha1.ReplicationSource{})
	}

	for _, obj := range cleanupVSBTypes {
		err := r.DeleteAllOf(r.Context, obj, deleteOptions...)
		if err != nil {
			r.Log.Error(err, fmt.Sprintf("unable to delete volumesnapshotbackup %s resources", r.req.NamespacedName))
			return false, err
		}
	}

	// Update VSB status as completed
	if vsb.DeletionTimestamp.IsZero() {
		vsb.Status.Phase = volsnapmoverv1alpha1.SnapMoverBackupPhaseCompleted
		err := r.Status().Update(context.Background(), &vsb)
		if err != nil {
			return false, err
		}
	}

	return true, nil
}

func (r *VolumeSnapshotBackupReconciler) isRetainPolicySet(vsb *volsnapmoverv1alpha1.VolumeSnapshotBackup) (bool, error) {
	// get restic secret created by controller
	resticSecretName := fmt.Sprintf("%s-secret", vsb.Name)
	resticSecret := corev1.Secret{}
	if err := r.Get(r.Context, types.NamespacedName{Namespace: vsb.Spec.ProtectedNamespace, Name: resticSecretName}, &resticSecret); err != nil {
		r.Log.Error(err, fmt.Sprintf("unable to fetch restic secret %s/%s", vsb.Spec.ProtectedNamespace, resticSecretName))
		return false, err
	}
	retainPolicy := false
	if resticSecret.Data != nil {
		if (len(resticSecret.Data[SnapshotRetainPolicyDaily]) > 0 ||
			len(resticSecret.Data[SnapshotRetainPolicyHourly]) > 0 ||
			len(resticSecret.Data[SnapshotRetainPolicyWeekly]) > 0 ||
			len(resticSecret.Data[SnapshotRetainPolicyMonthly]) > 0 ||
			len(resticSecret.Data[SnapshotRetainPolicyYearly]) > 0) &&
			len(resticSecret.Data[SnapshotRetainPolicyWithin]) > 0 {
			retainPolicy = true
		}
	}
	return retainPolicy, nil
}

func (r *VolumeSnapshotBackupReconciler) scheduleCronTriggerSet(vsb *volsnapmoverv1alpha1.VolumeSnapshotBackup) (bool, error) {
	// get restic secret created by controller
	resticSecretName := fmt.Sprintf("%s-secret", vsb.Name)
	resticSecret := corev1.Secret{}
	if err := r.Get(r.Context, types.NamespacedName{Namespace: vsb.Spec.ProtectedNamespace, Name: resticSecretName}, &resticSecret); err != nil {
		r.Log.Error(err, fmt.Sprintf("unable to fetch restic secret %s/%s", vsb.Spec.ProtectedNamespace, resticSecretName))
		return false, err
	}
	scheduleTrigger := false
	if resticSecret.Data != nil {
		if len(resticSecret.Data[SnapshotScheduleCron]) > 0 {
			scheduleTrigger = true
		}
	}
	return scheduleTrigger, nil
}

func (r *VolumeSnapshotBackupReconciler) areVSBResourcesDeleted(log logr.Logger, vsb *volsnapmoverv1alpha1.VolumeSnapshotBackup) (bool, error) {

	// check the cloned PVC has been deleted
	clonedPVC := corev1.PersistentVolumeClaim{}
	if err := r.Get(r.Context, types.NamespacedName{Name: fmt.Sprintf("%s-pvc", vsb.Spec.VolumeSnapshotContent.Name), Namespace: vsb.Spec.ProtectedNamespace}, &clonedPVC); err != nil {

		// we expect resource to not be found
		if k8serror.IsNotFound(err) {
			r.Log.Info(fmt.Sprintf("cloned volumesnapshot %s/%s has been deleted", vsb.Spec.ProtectedNamespace, fmt.Sprintf("%s-pvc", vsb.Spec.VolumeSnapshotContent.Name)))
		}
		// other error
		return false, err
	}

	// check dummy pod is deleted
	dummyPod := corev1.Pod{}
	if err := r.Get(r.Context, types.NamespacedName{Name: fmt.Sprintf("%s-pod", vsb.Name), Namespace: vsb.Spec.ProtectedNamespace}, &dummyPod); err != nil {

		// we expect resource to not be found
		if k8serror.IsNotFound(err) {
			r.Log.Info(fmt.Sprintf("dummy pod %s/%s has been deleted", fmt.Sprintf("%s-pod", vsb.Name), vsb.Spec.ProtectedNamespace))
		}
		// other error
		return false, err
	}

	// check the cloned VSC has been deleted
	vscClone := snapv1.VolumeSnapshotContent{}
	if err := r.Get(r.Context, types.NamespacedName{Name: fmt.Sprintf("%s-clone", vsb.Spec.VolumeSnapshotContent.Name)}, &vscClone); err != nil {

		// we expect resource to not be found
		if k8serror.IsNotFound(err) {
			r.Log.Info(fmt.Sprintf("cloned volumesnapshotcontent %s has been deleted", fmt.Sprintf("%s-clone", vsb.Spec.VolumeSnapshotContent.Name)))
		}
		// other error
		return false, err
	}

	// check the cloned VS has been deleted
	vsClone := snapv1.VolumeSnapshotContent{}
	if err := r.Get(r.Context, types.NamespacedName{Name: fmt.Sprintf(vscClone.Spec.VolumeSnapshotRef.Name), Namespace: vsb.Spec.ProtectedNamespace}, &vsClone); err != nil {

		// we expect resource to not be found
		if k8serror.IsNotFound(err) {
			r.Log.Info(fmt.Sprintf("cloned volumesnapshot %s/%s has been deleted", vsb.Spec.ProtectedNamespace, fmt.Sprintf(vscClone.Spec.VolumeSnapshotRef.Name)))
		}
		// other error
		return false, err
	}

	// check secret has been deleted
	secret := corev1.Secret{}
	if err := r.Get(r.Context, types.NamespacedName{Name: fmt.Sprintf("%s-secret", vsb.Name), Namespace: vsb.Spec.ProtectedNamespace}, &secret); err != nil {

		// we expect resource to not be found
		if k8serror.IsNotFound(err) {
			r.Log.Info(fmt.Sprintf("restic secret %s/%s has been deleted", vsb.Spec.ProtectedNamespace, fmt.Sprintf("%s-secret", vsb.Name)))
		}
		// other error
		return false, err
	}

	// check replicationSource has been deleted
	repSource := volsyncv1alpha1.ReplicationSource{}
	if err := r.Get(r.Context, types.NamespacedName{Name: fmt.Sprintf("%s-rep-src", vsb.Name), Namespace: vsb.Spec.ProtectedNamespace}, &repSource); err != nil {

		// we expect resource to not be found
		if k8serror.IsNotFound(err) {
			r.Log.Info(fmt.Sprintf("replicationsource %s/%s has been deleted", vsb.Spec.ProtectedNamespace, fmt.Sprintf("%s-rep-src", vsb.Name)))
		}
		// other error
		return false, err
	}

	//all resources have been deleted
	r.Log.Info("all volumesnapshotbackup resources have been deleted")
	return true, nil
}

func (r *VolumeSnapshotRestoreReconciler) CleanRestoreResources(log logr.Logger) (bool, error) {

	// get volumesnapshotrestore from cluster
	vsr := volsnapmoverv1alpha1.VolumeSnapshotRestore{}
	if err := r.Get(r.Context, r.req.NamespacedName, &vsr); err != nil {
		r.Log.Error(err, fmt.Sprintf("unable to fetch volumesnapshotrestore %s", r.req.NamespacedName))
		return false, err
	}

	// make sure VSR is completed before deleting resources
	if vsr.Status.Phase != volsnapmoverv1alpha1.SnapMoverRestoreVolSyncPhaseCompleted &&
		vsr.Status.Phase != volsnapmoverv1alpha1.SnapMoverRestorePhaseFailed &&
		vsr.Status.Phase != volsnapmoverv1alpha1.SnapMoverRestorePhasePartiallyFailed &&
		vsr.DeletionTimestamp.IsZero() {
		r.Log.Info(fmt.Sprintf("waiting for volsync to complete before deleting volumesnapshotrestore %s resources", r.req.NamespacedName))
		return false, nil
	}

	// get resources with VSR controller label in protected ns
	deleteOptions := []client.DeleteAllOfOption{
		client.MatchingLabels{VSRLabel: vsr.Name},
		client.InNamespace(vsr.Spec.ProtectedNamespace),
	}

	// Update VSR status as cleanup
	vsr.Status.Phase = volsnapmoverv1alpha1.SnapMoverRestorePhaseCleanup
	err := r.Status().Update(context.Background(), &vsr)
	if err != nil {
		return false, err
	}

	for _, obj := range cleanupVSRTypes {
		err := r.DeleteAllOf(r.Context, obj, deleteOptions...)
		if err != nil {
			r.Log.Error(err, fmt.Sprintf("unable to delete volumesnapshotrestore %s resources", r.req.NamespacedName))
			return false, err
		}
	}

	// get VSR again here due to resourceVersion changes prior to delete
	vsr = volsnapmoverv1alpha1.VolumeSnapshotRestore{}
	if err := r.Get(r.Context, r.req.NamespacedName, &vsr); err != nil {
		r.Log.Error(err, fmt.Sprintf("unable to fetch volumesnapshotrestore %s", r.req.NamespacedName))
		return false, err
	}

	// Update VSR status as completed
	if vsr.DeletionTimestamp.IsZero() {

		vsr.Status.Phase = volsnapmoverv1alpha1.SnapMoverRestorePhaseCompleted
		err := r.Status().Update(context.Background(), &vsr)
		if err != nil {
			return false, err
		}
	}

	return true, nil
}
