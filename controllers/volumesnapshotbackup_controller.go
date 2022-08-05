/*
Copyright 2022.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"context"
	"fmt"
	"time"

	k8serrors "k8s.io/apimachinery/pkg/api/errors"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	volsyncv1alpha1 "github.com/backube/volsync/api/v1alpha1"
	volsnapmoverv1alpha1 "github.com/konveyor/volume-snapshot-mover/api/v1alpha1"
	snapv1 "github.com/kubernetes-csi/external-snapshotter/client/v4/apis/volumesnapshot/v1"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
)

const ConditionReconciled = "Reconciled"
const ReconciledReasonError = "Error"
const ReconciledReasonComplete = "Complete"
const ReconcileCompleteMessage = "Reconcile complete"

// VolumeSnapshotBackupReconciler reconciles a VolumeSnapshotBackup object
type VolumeSnapshotBackupReconciler struct {
	client.Client
	Scheme         *runtime.Scheme
	Log            logr.Logger
	Context        context.Context
	NamespacedName types.NamespacedName
	EventRecorder  record.EventRecorder
	req            ctrl.Request
}

const (
	dmFinalizer = "oadp.openshift.io/oadp-datamover"
)

//+kubebuilder:rbac:groups=datamover.oadp.openshift.io,resources=volumesnapshotbackups,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=datamover.oadp.openshift.io,resources=volumesnapshotbackups/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=datamover.oadp.openshift.io,resources=volumesnapshotbackups/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the VolumeSnapshotBackup object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.11.0/pkg/reconcile
func (r *VolumeSnapshotBackupReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	// Set reconciler vars
	r.Log = log.FromContext(ctx).WithValues("vsb", req.NamespacedName)
	result := ctrl.Result{}
	r.Context = ctx
	// needed to preserve the application ns whenever we fetch the latest VSB instance
	r.req = req

	// Get VSB CR from cluster
	vsb := volsnapmoverv1alpha1.VolumeSnapshotBackup{}
	if err := r.Get(ctx, req.NamespacedName, &vsb); err != nil {
		// ignore is not found error
		if k8serrors.IsNotFound(err) {
			return result, nil
		}
		r.Log.Error(err, "unable to fetch VolumeSnapshotBackup CR")
		return result, err
	}

	// add protected namespace
	r.NamespacedName = types.NamespacedName{
		Namespace: vsb.Spec.ProtectedNamespace,
		Name:      vsb.Name,
	}

	// stop reconciling on this resource when completed or failed
	if vsb.Status.Phase == volsnapmoverv1alpha1.SnapMoverBackupPhaseCompleted ||
		vsb.Status.Phase == volsnapmoverv1alpha1.SnapMoverBackupPhaseFailed ||
		vsb.Status.Phase == volsnapmoverv1alpha1.SnapMoverBackupPhasePartiallyFailed {
		return ctrl.Result{
			Requeue: false,
		}, nil
	}

	// Run through all reconcilers associated with VSB needs
	// Reconciliation logic

	reconFlag, err := ReconcileBatch(r.Log,
		r.ValidateVolumeSnapshotMoverBackup,
		r.MirrorVolumeSnapshotContent,
		r.WaitForClonedVolumeSnapshotContentToBeReady,
		r.MirrorVolumeSnapshot,
		r.WaitForClonedVolumeSnapshotToBeReady,
		r.MirrorPVC,
		r.BindPVCToDummyPod,
		r.CreateVSBResticSecret,
		r.IsPVCBound,
		r.CreateReplicationSource,
		//r.CleanBackupResources,
	)

	// Update the status with any errors, or set completed condition
	if err != nil {
		r.Log.Info(fmt.Sprintf("Error from batch reconcile: %v", err))
		// Set failed status condition
		apimeta.SetStatusCondition(&vsb.Status.Conditions,
			metav1.Condition{
				Type:    ConditionReconciled,
				Status:  metav1.ConditionFalse,
				Reason:  ReconciledReasonError,
				Message: err.Error(),
			})

	} else {
		// Set complete status condition
		apimeta.SetStatusCondition(&vsb.Status.Conditions,
			metav1.Condition{
				Type:    ConditionReconciled,
				Status:  metav1.ConditionTrue,
				Reason:  ReconciledReasonComplete,
				Message: ReconcileCompleteMessage,
			})
	}

	statusErr := r.Client.Status().Update(ctx, &vsb)
	if err == nil { // Don't mask previous error
		err = statusErr
	}

	// Add Finalizer to VSB
	if !controllerutil.ContainsFinalizer(&vsb, dmFinalizer) {
		controllerutil.AddFinalizer(&vsb, dmFinalizer)
		err := r.Update(ctx, &vsb)
		if err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	}

	if vsb.DeletionTimestamp != nil {
		_, err := r.CleanBackupResources(r.Log)
		if err != nil {
			return ctrl.Result{}, err
		}

		controllerutil.RemoveFinalizer(&vsb, dmFinalizer)
		err = r.Update(ctx, &vsb)
		if err != nil {
			return ctrl.Result{}, err
		}
		r.Log.Info("Clean up successful")
		return ctrl.Result{}, nil
	}

	VSBComplete, err := r.setVSBStatus(r.Log)
	if !VSBComplete {
		return ctrl.Result{Requeue: true}, err
	}

	if !reconFlag {
		return ctrl.Result{Requeue: true, RequeueAfter: 5 * time.Second}, err
	}

	return ctrl.Result{}, err
}

// SetupWithManager sets up the controller with the Manager.
func (r *VolumeSnapshotBackupReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&volsnapmoverv1alpha1.VolumeSnapshotBackup{}).
		Owns(&snapv1.VolumeSnapshotContent{}).
		Owns(&snapv1.VolumeSnapshot{}).
		Owns(&v1.PersistentVolumeClaim{}).
		Owns(&volsyncv1alpha1.ReplicationSource{}).
		Owns(&v1.Pod{}).
		WithEventFilter(volumeSnapshotBackupPredicate(r.Scheme)).
		Complete(r)
}
