<div align="center">
<h1>Volume Snapshot Mover</h1>

<h2>A Data Mover for CSI snapshots</h2>
</div>

VolumeSnapshotMover relocates snapshots off of the cluster into an object store so that 
they can be used during a restore process to recover stateful applications 
in instances such as cluster deletion or disaster. 

### Design Proposal (in-progress): https://github.com/openshift/oadp-operator/pull/597/files

### Prerequisites:
To use the data mover controller, you will first need a volumesnapshot. This can be done
by using the Velero CSI plugin during backup of the stateful application.

- Install OADP Operator using [these steps](https://github.com/openshift/oadp-operator/blob/master/docs/install_olm.md).

- Have a stateful application running in a separate namespace, and then create a Velero backup using CSI snapshotting.
  - Follow the steps specified [here](https://github.com/openshift/oadp-operator/blob/master/docs/examples/csi_example.md).

- [Install](https://volsync.readthedocs.io/en/stable/installation/index.html) VolSync controller.

- We will be using VolSync's Restic option, hence configure a [restic secret](https://volsync.readthedocs.io/en/stable/usage/restic/index.html#id2) 
and make sure to name the secret `restic-secret`.


### Run the controller:

- Install the VolumeSnapshotMover CRDs `DataMoverBackup` and `DataMoverRestore` using: `oc create -f config/crd/bases/`

#### For backup:
- Create a `DataMoverBackup` CR using the volumesnapshotcontent name that was created during the Velero backup using CSI.
This is the snapshot that will be moved to object storage:

```
apiVersion: pvc.oadp.openshift.io/v1alpha1
kind: DataMoverBackup
metadata:
  name: datamoverbackup-sample
spec:
  volumeSnapshotContent:
    name: <INSERT-YOUR-VOLUMESNAPSHOTCONTENT-NAME>
```

#### For restore:
- Create a `DataMoverRestore` CR 

```
apiVersion: pvc.oadp.openshift.io/v1alpha1
kind: DataMoverRestore
metadata:
  name: datamoverrestore-sample
spec:
  // TODO...fill out
```

- To run the controller, execute `make run`
