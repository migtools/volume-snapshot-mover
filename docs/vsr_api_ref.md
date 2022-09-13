<h1>VolumeSnapshotRestore API References</h1>

### VolumeSnapshotRestoreSpec

| Property             | Type                       | Description                                        |
|----------------------|--------------------------------|-------------------------------------------------------|
| ResticSecretRef      | corev1.LocalObjectReference           | ResticSecretRef  is the name of the Restic repository secret.       |
| VolumeSnapshotBackupRef     | VSBRef                                 | VolumeSnapshotBackupRef  is a reference to resources used by VolumeSnapshotBackup.     |
| ProtectedNamespace        | string               | ProtectedNamespace is the namespace in which the Velero deployment is present, and where VolumeSnapshotRestore resources will be created.   |


### VolumeSnapshotRestoreStatus

| Property             | Type                        | Description                                 |
|----------------------|-------------------------------------------------|------------------------------------------------------|
| Phase     | VolumeSnapshotRestorePhase                                                    | volumesnapshot restore phase status    |
| SnapshotHandle     | string                                             | SnapshotHandle is the snaphandle from the volumeSnapshotContent created by VolSync.      |
| Conditions     | []metav1.Condition                                                 | Include references to the volsync CRs and their state as they are running     |


### VSBRef

| Property             | Type               |        Description                         |
|----------------------|---------------------------------------|---------------------------------------------|
| BackedUpPVCData    | PVCData                                    | BackedUpPVCData  is a reference to the source PVC from backup.   |
| ResticRepository     | string                                     | ResticRepository is the location in which the snapshot will be retrieved.        |
| VolumeSnapshotClassName     | string                                     | name of the VolumeSnapshotClass      |


### PVCData

| Property             | Type               |        Description                         |
|----------------------|---------------------------------------|---------------------------------------------|
| Name    | string                                      | Name is the name of the application's source PVC.   |
| Size     | string                                     | Size is the size of the source PVC.           |
| StorageClassName     | string                                     | Name of the StorageClass                          |


### VolumeSnapshotRestorePhase

| Property           |     Type                     |     Description              |
|--------------------|-----------------------------|----------------------------------|
| SnapMoverRestoreVolSyncPhaseCompleted                          | VolumeSnapshotRestorePhase     |  VolumeSnapshotRestore VolSync ReplicationDestination has completed.   |
| SnapMoverRestorePhaseCompleted                                 | VolumeSnapshotRestorePhase  |  VolumeSnapshotRestore has completed.   |
| SnapMoverRestorePhaseInProgress                             | VolumeSnapshotRestorePhase        |   VolumeSnapshotRestore is still in progress. |
| SnapMoverRestorePhasePartiallyFailed                    | VolumeSnapshotRestorePhase    |    VolumeSnapshotRestore has partially failed.   |
| SnapMoverRestorePhaseFailed                                | VolumeSnapshotRestorePhase    |    VolumeSnapshotRestore has failed.   |