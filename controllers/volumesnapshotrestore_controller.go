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
	"strconv"
	"time"

	k8serrors "k8s.io/apimachinery/pkg/api/errors"

	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	volsyncv1alpha1 "github.com/backube/volsync/api/v1alpha1"
	"github.com/go-logr/logr"
	volsnapmoverv1alpha1 "github.com/konveyor/volume-snapshot-mover/api/v1alpha1"
	snapv1 "github.com/kubernetes-csi/external-snapshotter/client/v4/apis/volumesnapshot/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

var processingVSRs = 0
var VSRBatchNumber = 0

// VolumeSnapshotRestoreReconciler reconciles a VolumeSnapshotRestore object
type VolumeSnapshotRestoreReconciler struct {
	client.Client
	Scheme         *runtime.Scheme
	Log            logr.Logger
	Context        context.Context
	NamespacedName types.NamespacedName
	EventRecorder  record.EventRecorder
	req            ctrl.Request
}

//+kubebuilder:rbac:groups=datamover.oadp.openshift.io,resources=volumesnapshotrestores,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=datamover.oadp.openshift.io,resources=volumesnapshotrestores/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=datamover.oadp.openshift.io,resources=volumesnapshotrestores/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the VolumeSnapshotRestore object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.11.0/pkg/reconcile

func (r *VolumeSnapshotRestoreReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	// Set reconciler vars
	r.Log = log.FromContext(ctx).WithValues("vsr", req.NamespacedName)
	result := ctrl.Result{}
	r.Context = ctx
	// needed to preserve the application ns whenever we fetch the latest VSR instance
	r.req = req

	// Get VSR CR from cluster
	vsr := volsnapmoverv1alpha1.VolumeSnapshotRestore{}
	if err := r.Get(ctx, req.NamespacedName, &vsr); err != nil {
		// ignore is not found error
		if k8serrors.IsNotFound(err) {
			return result, nil
		}
		r.Log.Error(err, "unable to fetch VolumeSnapshotRestore CR")
		return result, err
	}

	// add protected namespace
	r.NamespacedName = types.NamespacedName{
		Namespace: vsr.Spec.ProtectedNamespace,
		Name:      vsr.Name,
	}

	if VSRBatchNumber == 0 {
		batchValue, err := GetRestoreBatchValue(vsr.Spec.ProtectedNamespace, r.Client)
		if err != nil {
			return ctrl.Result{}, err
		}
		VSRBatchNumber, err = strconv.Atoi(batchValue)
		if err != nil {
			return ctrl.Result{}, err
		}
	}

	// stop reconciling on this resource when completed or failed
	if (vsr.Status.Phase == volsnapmoverv1alpha1.SnapMoverRestorePhaseCompleted ||
		vsr.Status.Phase == volsnapmoverv1alpha1.SnapMoverRestorePhaseFailed ||
		vsr.Status.Phase == volsnapmoverv1alpha1.SnapMoverRestorePhasePartiallyFailed) &&
		vsr.DeletionTimestamp.IsZero() {

		return ctrl.Result{
			Requeue: false,
		}, nil
	}

	// Add Finalizer to VSR
	if !controllerutil.ContainsFinalizer(&vsr, dmFinalizer) {
		controllerutil.AddFinalizer(&vsr, dmFinalizer)
		err := r.Update(ctx, &vsr)
		if err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	}

	// Check and add VSRs to queue until full
	processed, err := r.setVSRQueue(&vsr, r.Log)
	if err != nil {
		return ctrl.Result{}, err
	}

	// no error but VSR queue is full
	if !processed && err == nil {
		r.Log.Info(fmt.Sprintf("requeuing vsr %v as max vsrs are being processed", vsr.Name))
		return ctrl.Result{Requeue: true, RequeueAfter: 5 * time.Second}, nil
	}

	if !vsr.DeletionTimestamp.IsZero() {
		// remove VSR from queue if deleted
		if vsr.Status.BatchingStatus != "" && vsr.Status.BatchingStatus != volsnapmoverv1alpha1.SnapMoverRestoreBatchingCompleted {
			processingVSRs--
		}

		_, err := r.CleanRestoreResources(r.Log)
		if err != nil {
			return ctrl.Result{}, err
		}

		controllerutil.RemoveFinalizer(&vsr, dmFinalizer)
		err = r.Update(ctx, &vsr)
		if err != nil {
			return ctrl.Result{}, err
		}

		return ctrl.Result{}, nil
	}

	// Run through all reconcilers associated with VSR needs
	// Reconciliation logic

	reconFlag, err := ReconcileBatch(r.Log,
		r.ValidateVolumeSnapshotMoverRestore,
		r.CreateVSRResticSecret,
		r.CreateReplicationDestination,
		r.WaitForVolSyncSnapshotContentToBeReady,
		r.CleanRestoreResources,
	)

	// Update the status with any errors, or set completed condition
	if err != nil {
		r.Log.Info(fmt.Sprintf("Error from batch reconcile: %v", err))
		// Set failed status condition
		apimeta.SetStatusCondition(&vsr.Status.Conditions,
			metav1.Condition{
				Type:    ConditionReconciled,
				Status:  metav1.ConditionFalse,
				Reason:  ReconciledReasonError,
				Message: err.Error(),
			})

	} else {
		// Set complete status condition
		apimeta.SetStatusCondition(&vsr.Status.Conditions,
			metav1.Condition{
				Type:    ConditionReconciled,
				Status:  metav1.ConditionTrue,
				Reason:  ReconciledReasonComplete,
				Message: ReconcileCompleteMessage,
			})
	}

	statusErr := r.Client.Status().Update(ctx, &vsr)
	if err == nil { // Don't mask previous error
		err = statusErr
	}

	VSRComplete, err := r.SetVSRStatus(r.Log)
	if !VSRComplete {
		return ctrl.Result{Requeue: true, RequeueAfter: 5 * time.Second}, err
	}

	if !reconFlag {
		return ctrl.Result{Requeue: true, RequeueAfter: 5 * time.Second}, err
	}

	return ctrl.Result{}, err
}

// SetupWithManager sets up the controller with the Manager.
func (r *VolumeSnapshotRestoreReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&volsnapmoverv1alpha1.VolumeSnapshotRestore{}).
		Owns(&v1.PersistentVolumeClaim{}).
		Owns(&snapv1.VolumeSnapshotContent{}).
		Owns(&volsyncv1alpha1.ReplicationDestination{}).
		WithEventFilter(volumeSnapshotRestorePredicate(r.Scheme)).
		Complete(r)
}
