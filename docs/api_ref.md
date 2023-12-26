<h1>API References</h1>

Pre-requisites: Install Volume Snapshot Mover to your cluster. before proceeding to the next steps.

Run `oc api-resources | grep -e 'datamover'` to get available APIs

Example output (subject to change, depending on the version of Volume Snapshot Mover installed):
```
volumesnapshotbackups                 vsb                 datamover.oadp.openshift.io/v1alpha1          true         VolumeSnapshotBackup
volumesnapshotrestores                vsr                 datamover.oadp.openshift.io/v1alpha1          true         VolumeSnapshotRestore
```

You can use `oc explain <full-name|kind|short-name>.<fields>` to explore available APIs. For example
```
$ oc explain vsb.spec.protectedNamespace
KIND:     VolumeSnapshotBackup
VERSION:  datamover.oadp.openshift.io/v1alpha1

FIELD:    protectedNamespace <string>

DESCRIPTION:
     Namespace where the Velero deployment is present
```

See also [![Go Reference](https://pkg.go.dev/badge/github.com/konveyor/volume-snapshot-mover.svg)](https://pkg.go.dev/github.com/konveyor/volume-snapshot-mover@master) for a deeper dive.
