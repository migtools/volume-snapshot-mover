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
3. [Advanced Options](#Advanced-Options)



<h2>Prerequisites:<a id="pre-reqs"></a></h2>

<hr style="height:1px;border:none;color:#333;">

- Have a stateful application running in a separate namespace. 

- Have an appropriate StorageClass and VolumeShapshotClass. **Make sure there is only one default of each.**
  - Include the label `velero.io/csi-volumesnapshot-class: 'true'` in your `VolumeSnapshotClass` 
  to let Velero know which to use.
  - `deletionPolicy` must be set to `Retain` for the `VolumeSnapshotClass`.

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
      enable: false #[true, false]
    velero:
      defaultPlugins:
        - openshift
        - aws
        - csi
        - vsm
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

VolumeSnapshotBackup status:

`oc get vsb -n <app-ns>`

`oc get vsb <vsb-name> -n <app-ns> -ojsonpath="{.status.phase}"` 

Alternatively one can use Velero / OADP status:

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

<hr style="height:1px;border:none;color:#333;">

<h4> Advanced Options <a id="Advanced-Options"></a></h4>

The OADP VolumeSnapshotMover feature supports volume snapshots via the CSI driver. 
Both CephRBD and CephFS are [supported via CSI](https://github.com/ceph/ceph-csi).
The OADP VolumeSnapshotMover feature leverages some of the more recently added 
features of Ceph and CSI to be performant in large scale environments.

One of these newly added features for backups with CephFS to be more performant
is the [shallow copy](https://github.com/ceph/ceph-csi/blob/devel/docs/design/proposals/cephfs-snapshot-shallow-ro-vol.md) 
method, which is available > OCP 4.12.

In large scale backups OADP highly recommends OCP 4.12 and above. In OCP 4.12
extra parameters on the DPA are required for this CephFS shallow copy.
These parameters should not be required in OCP >= 4.13. 

For backups on CSI backed by CephFS using shallow copy, OADP requires the 
following volume options specified in the DPA.

```
volumeOptions:
  sourceVolumeOptions:
    accessMode: ReadOnlyMany
    cacheAccessMode: ReadWriteMany
    cacheStorageClassName: ocs-storagecluster-cephfs
    moverSecurityContext: true
    storageClassName: ocs-storagecluster-cephfs-shallow
```

Since the DPA is a cluster wide configuration, if you plan to backup any other
storage type we recommend creating two instances of the DPA with the appropriate 
DPA settings. Note the name and settings of the two following DPA configurations.
The two instances of the DPA can be configured on the same cluster in separate
namespaces.

**Note**: these volumeOptions are configured per storageClass.

For example a CephFS DPA config with:
```
apiVersion: oadp.openshift.io/v1alpha1
kind: DataProtectionApplication
metadata:
  name: cephfs-dpa
  namespace: openshift-adp
spec:
  features:
    dataMover: 
      enable: true
      credentialName: <secret-name>
      volumeOptionsForStorageClasses:
        ocs-storagecluster-cephfs:
          sourceVolumeOptions:
            accessMode: ReadOnlyMany
            cacheAccessMode: ReadWriteMany
            cacheStorageClassName: ocs-storagecluster-cephfs
            moverSecurityContext: true
            storageClassName: ocs-storagecluster-cephfs-shallow
          destinationVolumeOptions:
            cacheAccessMode: ReadWriteOnce
            moverSecurityContext: true
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

For example a non-CephFS DPA config:
```
apiVersion: oadp.openshift.io/v1alpha1
kind: DataProtectionApplication
metadata:
  name: all-other-storage-dpa
  namespace: dpa-openshift
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
