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

	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/go-logr/logr"
	pvcv1alpha1 "github.com/konveyor/volume-snapshot-mover/api/v1alpha1"
	snapv1 "github.com/kubernetes-csi/external-snapshotter/client/v4/apis/volumesnapshot/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// DataMoverRestoreReconciler reconciles a DataMoverRestore object
type DataMoverRestoreReconciler struct {
	client.Client
	Scheme         *runtime.Scheme
	Log            logr.Logger
	Context        context.Context
	NamespacedName types.NamespacedName
	EventRecorder  record.EventRecorder
	req            ctrl.Request
}

//+kubebuilder:rbac:groups=pvc.oadp.openshift.io,resources=datamoverrestores,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=pvc.oadp.openshift.io,resources=datamoverrestores/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=pvc.oadp.openshift.io,resources=datamoverrestores/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the DataMoverRestore object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.11.0/pkg/reconcile
func (r *DataMoverRestoreReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	// Set reconciler vars
	r.Log = log.FromContext(ctx).WithValues("dmr", req.NamespacedName)
	result := ctrl.Result{}
	r.Context = ctx
	// needed to preserve the application ns whenever we fetch the latest DMR instance
	r.req = req

	// Get DMR CR from cluster
	dmr := pvcv1alpha1.DataMoverRestore{}
	if err := r.Get(ctx, req.NamespacedName, &dmr); err != nil {
		r.Log.Error(err, "unable to fetch DataMoverRestore CR")
		return result, err
	}
	if dmr.Status.Completed {
		// stop reconciling on this resource
		return ctrl.Result{
			Requeue: false,
		}, nil
	}

	// Run through all reconcilers associated with DMR needs
	// Reconciliation logic

	_, err := ReconcileBatch(r.Log,
		r.ValidateDataMoverRestore,
		r.CreateDMRResticSecret,
		r.CreateReplicationDestination,
		//r.MirrorVolumeSnapshot,
		//r.CleanupRestoreResources,
	)

	// Update the status with any errors, or set completed condition
	if err != nil {
		r.Log.Info(fmt.Sprintf("Error from batch reconcile: %v", err))
		// Set failed status condition
		apimeta.SetStatusCondition(&dmr.Status.Conditions,
			metav1.Condition{
				Type:    ConditionReconciled,
				Status:  metav1.ConditionFalse,
				Reason:  ReconciledReasonError,
				Message: err.Error(),
			})
	} else {
		// Set complete status condition
		apimeta.SetStatusCondition(&dmr.Status.Conditions,
			metav1.Condition{
				Type:    ConditionReconciled,
				Status:  metav1.ConditionTrue,
				Reason:  ReconciledReasonComplete,
				Message: ReconcileCompleteMessage,
			})
	}

	statusErr := r.Client.Status().Update(ctx, &dmr)
	if err == nil { // Don't mask previous error
		err = statusErr
	}

	return ctrl.Result{}, err
}

// SetupWithManager sets up the controller with the Manager.
func (r *DataMoverRestoreReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&pvcv1alpha1.DataMoverRestore{}).
		Owns(&v1.PersistentVolumeClaim{}).
		Owns(&snapv1.VolumeSnapshotContent{}).
		WithEventFilter(datamoverRestorePredicate(r.Scheme)).
		Complete(r)
}
