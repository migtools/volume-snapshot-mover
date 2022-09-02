<div align="center">
<h1>Volume Snapshot Mover</h1>

<h2>A Data Mover for CSI snapshots</h2>
</div>

VolumeSnapshotMover relocates snapshots off of the cluster into an object store to be used during a restore process to recover stateful applications 
in instances such as cluster deletion or disaster. 

#### Design Proposal: https://github.com/openshift/oadp-operator/blob/master/docs/design/datamover.md

# Table of Contents

1. [Getting Started](#pre-reqs)
2. Quickstart using Volume Snapshot Mover:
    1. [Backup](#backup)
    2. [Restore](#restore)


<h2>Prerequisites:<a id="pre-reqs"></a></h2>

<hr style="height:1px;border:none;color:#333;">

- Have a stateful application running in a separate namespace. 

- Have an appropriate StorageClass and VolumeShapshotClass. **Make sure there is only one default of each.**
  - Include the label `velero.io/csi-volumesnapshot-class: 'true'` in your `VolumeSnapshotClass` 
  to let Velero know which to use.
  - `deletionPolicy` must be set to `Retain`

```
apiVersion: snapshot.storage.k8s.io/v1
kind: VolumeSnapshotClass
metadata:
  name: example-snapclass
  labels:
    velero.io/csi-volumesnapshot-class: 'true'
  annotations:
    snapshot.storage.kubernetes.io/is-default-class: 'true'
driver: ebs.csi.aws.com
deletionPolicy: Retain
```

```
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: gp2-csi
  annotations:
    storageclass.kubernetes.io/is-default-class: 'true'
provisioner: ebs.csi.aws.com
parameters:
  type: gp2
reclaimPolicy: Delete
volumeBindingMode: WaitForFirstConsumer
allowVolumeExpansion: true
```

- Install the [OADP Operator](https://github.com/openshift/oadp-operator/blob/master/docs/install_olm.md) using OLM.

- Install the [VolSync operator](https://volsync.readthedocs.io/en/stable/installation/index.html) using OLM.

![Volsync_install](/docs/examples/images/volsync_install.png)


- We will be using VolSync's Restic option, hence configure a [restic secret](https://volsync.readthedocs.io/en/stable/usage/restic/index.html#id2)

  
```
cat << EOF > ./restic-secret.yaml
apiVersion: v1
kind: Secret
metadata:
  name: <secret-name>
type: Opaque
stringData:
  # The repository encryption key
  RESTIC_PASSWORD: my-secure-restic-password
EOF
```


- Create a DPA similar to below:
  - Add the restic secret name from the previous step to your DPA CR in `spec.features.dataMover.credentialName`.  
    If this step is not completed then it will default to the secret name `dm-credential`.
  - Note the CSI `defaultPlugin` and `dataMover.enable` flag.


```
apiVersion: oadp.openshift.io/v1alpha1
kind: DataProtectionApplication
metadata:
  name: velero-sample
  namespace: openshift-adp
spec:
  features:
    dataMover: 
      enable: true
      credentialName: <secret-name>
  backupLocations:
    - velero:
        config:
          profile: default
          region: us-east-1
        credential:
          key: cloud
          name: cloud-credentials
        default: true
        objectStorage:
          bucket: <bucket-name>
          prefix: <bucket-prefix>
        provider: aws
  configuration:
    restic:
      enable: false
    velero:
      defaultPlugins:
        - openshift
        - aws
        - csi
      featureFlags:
        - EnableCSI
```


<hr style="height:1px;border:none;color:#333;">

<h4> For backup <a id="backup"></a></h4>

- Create a backup CR:

```
apiVersion: velero.io/v1
kind: Backup
metadata:
  name: <backup-name>
  namespace: <protected-ns>
spec:
  includedNamespaces:
  - <app-ns>
  storageLocation: velero-sample-1
```

- Wait several minutes and check the VolumeSnapshotBackup CR status for completed: 

`oc get backup`

`oc get backup <name> -ojsonpath="{.status.phase}"`

- There should now be a snapshot in the object store that was given in the restic secret.

<h4> For restore <a id="restore"></a></h4>

- Make sure the application namespace is deleted, as well as the volumeSnapshotContent
  that was created by the Velero CSI plugin.


- Create a restore CR:
  - Make sure `restorePVs` is set to `true`.

```
apiVersion: velero.io/v1
kind: Restore
metadata:
  name: <restore-name>
  namespace: <protected-ns>
spec:
  backupName: <previous-backup-name>
  restorePVs: true
```

- Wait several minutes and check the VolumeSnapshotRestore CR status for completed: 

`oc get vsr -n <app-ns>`

`oc get vsr <vsr-name> -n <app-ns> -ojsonpath="{.status.phase}` 

- Check that your application data has been restored:

`oc get route <route-name> -n <app-ns> -ojsonpath="{.spec.host}"`
