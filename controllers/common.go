package controllers

import (
	"github.com/go-logr/logr"
)

const (
	VSBLabel      = "datamover.oadp.openshift.io/dmb"
	VSRLabel      = "datamover.oadp.openshift.io/dmr"
	DummyPodImage = "quay.io/konveyor/rsync-transfer:latest"
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
