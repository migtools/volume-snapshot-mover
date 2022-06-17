package controllers

import (
	"github.com/go-logr/logr"
)

const (
	VSBLabel      = "datamover.oadp.openshift.io/vsb"
	VSRLabel      = "datamover.oadp.openshift.io/vsr"
	DummyPodImage = "quay.io/konveyor/rsync-transfer:latest"

	volumeSnapshotClassDefaultKey = "snapshot.storage.kubernetes.io/is-default-class"
	storageClassDefaultKey        = "storageclass.kubernetes.io/is-default-class"
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
