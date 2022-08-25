<h1 align="center">Troubleshooting<a id="troubleshooting"></a></h1>

1. [Debugging Failed VolumeSnapshotMover Backups](#backup)
2. [Debugging Failed VolumeSnapshotMover Restores](#restore)
3. Known Issues 
    1. [Restore can take a long period of time to complete](#restoretime)
    2. [GCP object storage is not supported](#gcp)
    3. [Error with multiple default volumeSnapshotClasses and/or storageClasses](#classes)
    4. [volumeSnapshotBackup/volumeSnapshotRestore CRs do not have a status field](#status)
    5. [Backup partially fails but volumeSnapshotBackup completes](#partiallyfail)

<hr style="height:1px;border:none;color:#333;">

<h1 align="center">Debugging Failed VolumeSnapshotMover Backups<a id="backup"></a></h1>

This section includes steps to debug a failed backup using volumeSnapshotMover. 

1. Check for errors in volumeSnapshotMover controller logs:  

    `oc logs <vsm-pod> -n <OADP-namespace>`  

2. Check for errors in the Velero pod:  

    `oc logs <velero-pod> -n <OADP-namespace>`   

3. Check for errors in the backup:  
    `oc describe backup <backupName>`

    - If you have a local Velero installation, you can also run:  
     `velero describe backup <backupName> -n <OADP-namespace>` and `velero backup logs <backupName> -n <OADP-namespace>`

4. Check for errors in the VolSync pod:  
    `oc get po -n openshift-operators`  
    
    `oc logs <volsync-pod> -n openshift-operators`  

5. Check the details of volumeSnapshotContent, volumeSnapshot, and PVC, from volumeSnapshotMover:  
    `oc get <resource> -n <OADP-namespace>`

    `oc describe <resource> <resource-name> -n <OADP-namespace>`

6. Fix errors if any. 

If the issue still persists, [create a new issue](https://github.com/konveyor/volume-snapshot-mover/issues/new) if [an issue doesnt exist already](https://github.com/konveyor/volume-snapshot-mover/issues)


<hr style="height:1px;border:none;color:#333;">

<h1 align="center">Debugging Failed VolumeSnapshotMover Restores<a id="restore"></a></h1>

This section includes how to debug a failed restore using volumeSnapshotMover.

1. Check for errors in volumeSnapshotMover controller logs:  

    `oc logs <vsm-pod> -n <OADP namespace>`  

2. Check for errors in the Velero pod:  

    `oc logs <velero-pod> -n <OADP namespace>`   

3. Check for errors in the restore:  
    `oc describe restore <restoreName>`

    - If you have a local Velero installation, you can also run:    
     `velero describe restore <restoreName> -n <OADP-namespace>` and `velero restore logs <restoreName> -n <OADP-namespace>`

4. Check for errors in the VolSync pod:  
    `oc get po -n openshift-operators`  
    
    `oc logs <volsync-pod> -n openshift-operators`  

5. Check the details of volumeSnapshotContent, volumeSnapshot, and PVC, from volumeSnapshotMover:  
    `oc get <resource> -n <OADP-namespace>`

    `oc describe <resource> <resource-name> -n <OADP-namespace>`

6. Fix errors if any. 

If the issue still persists, [create a new issue](https://github.com/konveyor/volume-snapshot-mover/issues/new) if [an issue doesnt exist already](https://github.com/konveyor/volume-snapshot-mover/issues)


<hr style="height:1px;border:none;color:#333;">

<h1 align="center">Known Issues<a id="misconfig"></a></h1>

<h3>Restore can take a long period of time to complete<a id="restoretime"></a></h3>

- As of now, the CSI plugin blocks the restore process for each PVC that will
    be restored via volumeSnapshotMover restore. 
    - This can create long restore times, so we are working on possible solutions, and should be resolved in OADP 1.1.1
    

<h3>GCP object storage is not supported<a id="gcp"></a></h3>

- Currently, volumeSnapshotMover only supports AWS and Azure object storage locations. 


<h3>Error with multiple default volumeSnapshotClasses and/or storageClasses<a id="classes"></a></h3>

- If there are multiple volumeSnapshotClasses and/or storageClasses, the volumeSnapshotMover controller
    will not progress. 
- If this is the case, VSB or VSR will be created, but not reconciled on.
- Use `oc get volumesnapshotclass` or `oc get storageclass` to check.


<h3>volumeSnapshotBackup/volumeSnapshotRestore CRs do not have a status field<a id="status"></a></h3>

- If you do not see a status section on the VSB or VSR CR, this is due to an error in the status,  
    and will be resolved in OADP 1.1.1.

    ```
    apiVersion: datamover.oadp.openshift.io/v1alpha1
    kind: VolumeSnapshotBackup
    metadata:
        generateName: vsb-
        labels:
            velero.io/backup-name: <backup-name>
        name: <vsb-name>
        namespace: <app-namespace>
    spec:
        protectedNamespace: <OADP-namespace>
        resticSecretRef:
            name: <secret-name>
        volumeSnapshotContent:
            name: <snapcontent-name>
    ```

<h3>Backup partially fails but volumeSnapshotBackup completes<a id="partiallyfail"></a></h3>

- If a volumeSnapshot is encountered during backup from a prior backup or restore, 
    volumeSnapshotBackup `status.phase` is `completed`, but Backup is `partiallyFailed`. 
    - This issue will be resolved in OADP 1.1.1.

- To check whether or not this is the issue, see the example below:  

```
$ oc get volumesnapshot -A
NAMESPACE      NAME              READYTOUSE   SOURCEPVC     SOURCESNAPSHOTCONTENT   RESTORESIZE   SNAPSHOTCLASS       SNAPSHOTCONTENT              CREATIONTIME      AGE
<app-ns>      <stale-snapshot>   true         <pvc-name>                            1Gi           <snapshot-class>    <snapshotcontent-name>        13m              14m
```

```
$ oc get volumesnapshotcontent -A
NAME                                READYTOUSE   RESTORESIZE   DELETIONPOLICY   DRIVER            VOLUMESNAPSHOTCLASS     VOLUMESNAPSHOT        VOLUMESNAPSHOTNAMESPACE   AGE
<snapshotcontent-name>              true         1073741824    Retain           ebs.csi.aws.com   <snapshot-class>       <stale-snapshot>        <app-ns>                 14m
<snapshotcontent-name>              true         1073741824    Retain           ebs.csi.aws.com   <snapshot-class>       <current-snapshot>      <app-ns>                 11m
```


