package controllers

import (
	pvcv1alpha1 "github.com/konveyor/volume-snapshot-mover/api/v1alpha1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

func datamoverBackupPredicate(scheme *runtime.Scheme) predicate.Predicate {
	return predicate.Funcs{
		// Update returns true if the Update event should be processed
		UpdateFunc: func(e event.UpdateEvent) bool {
			if e.ObjectOld.GetGeneration() == e.ObjectNew.GetGeneration() {
				return false
			}
			return isObjectOursBackup(scheme, e.ObjectOld)
		},
		// Create returns true if the Create event should be processed
		CreateFunc: func(e event.CreateEvent) bool {
			return isObjectOursBackup(scheme, e.Object)
		},
		// Delete returns true if the Delete event should be processed
		DeleteFunc: func(e event.DeleteEvent) bool {
			return !e.DeleteStateUnknown && isObjectOursBackup(scheme, e.Object)
		},
	}
}

// isObjectOurs returns true if the object is ours.
// it first checks if the object has our group, version, and kind
// else it will check for non empty VolumeSnapshotMoverController labels
func isObjectOursBackup(scheme *runtime.Scheme, object client.Object) bool {
	objGVKs, _, err := scheme.ObjectKinds(object)
	if err != nil {
		return false
	}
	if len(objGVKs) != 1 {
		return false
	}
	gvk := objGVKs[0]
	if gvk.Group == pvcv1alpha1.GroupVersion.Group && gvk.Version == pvcv1alpha1.GroupVersion.Version && gvk.Kind == pvcv1alpha1.DMBKind {
		return true
	}
	return object.GetLabels()[DMBLabel] != ""
}

func datamoverRestorePredicate(scheme *runtime.Scheme) predicate.Predicate {
	return predicate.Funcs{
		// Update returns true if the Update event should be processed
		UpdateFunc: func(e event.UpdateEvent) bool {
			if e.ObjectOld.GetGeneration() == e.ObjectNew.GetGeneration() {
				return false
			}
			return isObjectOursRestore(scheme, e.ObjectOld)
		},
		// Create returns true if the Create event should be processed
		CreateFunc: func(e event.CreateEvent) bool {
			return isObjectOursRestore(scheme, e.Object)
		},
		// Delete returns true if the Delete event should be processed
		DeleteFunc: func(e event.DeleteEvent) bool {
			return !e.DeleteStateUnknown && isObjectOursRestore(scheme, e.Object)
		},
	}
}

func isObjectOursRestore(scheme *runtime.Scheme, object client.Object) bool {
	objGVKs, _, err := scheme.ObjectKinds(object)
	if err != nil {
		return false
	}
	if len(objGVKs) != 1 {
		return false
	}
	gvk := objGVKs[0]
	if gvk.Group == pvcv1alpha1.GroupVersion.Group && gvk.Version == pvcv1alpha1.GroupVersion.Version && gvk.Kind == pvcv1alpha1.DMBKind {
		return true
	}
	return object.GetLabels()[DMBLabel] != ""
}
