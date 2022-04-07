<div align="center">
<h1>Volume Snapshot Mover</h1>
A datamover for CSI snapshots
</div>

Design Proposal (in-progress): https://github.com/openshift/oadp-operator/pull/597/files

###Basic CSI snapshot workflow steps are as follows:
- Install OADP Operator, have a stateful application running and create a backup, please
follow the steps specified [here](https://github.com/openshift/oadp-operator/blob/master/docs/examples/csi_example.md)
- [Install](https://volsync.readthedocs.io/en/stable/installation/index.html) VolSync controller
- We will be using VolSync's restic option, hence configure a [restic secret](https://volsync.readthedocs.io/en/stable/usage/restic/index.html#id2).
The restic secret should be of the name `restic-config`.
- Install the VolumeSnapshotMover CRDs using the following command:
```
oc create -f config/crd/bases/
```
- Now, create a DataMoverBackup CR using the snapshotcontent name of the volumesnapshot that you want to move to object storage:
```
apiVersion: pvc.oadp.openshift.io/v1alpha1
kind: DataMoverBackup
metadata:
  name: datamoverbackup-sample
spec:
  volumeSnapshotContent:
    name: <INSERT-YOUR-VOLUMESNAPSHOTCONTENT-NAME>
```
- Finally, execute `make run`
