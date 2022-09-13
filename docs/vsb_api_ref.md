<h1>VolumeSnapshotBackup API References</h1>

### VolumeSnapshotBackupSpec

| Property              | Type                       | Description                                        |
|-----------------------|--------------------------------|-------------------------------------------------------|
| VolumeSnapshotContent | corev1.ObjectReference                   | VolumeSnapshotContent is the name of the VolumeSnapshotContent that will be moved to a remote storage location.          |
| ProtectedNamespace    | string                 | ProtectedNamespace is the namespace in which the Velero deployment is present, and where VolumeSnapshotBackup resources will be created.   |
| ResticSecretRef       | corev1.LocalObjectReference                 | Restic Secret reference for given BSL  |


### VolumeSnapshotBackupStatus

| Property             | Type                      | Description                                                                 |
|----------------------|---------------------------|-----------------------------------------------------------------------------|
| Completed     | bool                      | Completed is whether or not VolumeSnapshotBackup has completed reconciling. |
| SourcePVCData      | PVCData                   | SourcePVCData is a reference to the source PVC.                             |
| ResticRepository      | string                    | ResticRepository is the location in which the snapshot will be stored.      |
| Phase      | VolumeSnapshotBackupPhase | Phase is the VolumeSnapshotBackup phase status.                             |
| Conditions      | []metav1.Condition        | Include references to the volsync CRs and their state as they are running   |
| VolumeSnapshotClassName      | string                    | name of the VolumeSnapshotClass                           |

### PVCData

| Property             | Type               | Description                                       |
|----------------------|---------------------------------------|---------------------------------------------------|
| Name    | string                                      | Name is the name of the application's source PVC. |
| Size     | string                                     | Size is the size of the source PVC.               |
| StorageClassName     | string                                     | Name of the StorageClass                          |

### VolumeSnapshotBackupPhase

| Property           |     Type                     |     Description              |
|--------------------|-----------------------------|----------------------------------|
| SnapMoverVolSyncPhaseCompleted                          | VolumeSnapshotBackupPhase     |  VolumeSnapshotBackup VolSync ReplicationSource has completed.   |
| SnapMoverBackupPhaseCompleted                                 | VolumeSnapshotBackupPhase  |  VolumeSnapshotBackup has completed.   |
| SnapMoverBackupPhaseInProgress                             | VolumeSnapshotBackupPhase        |   VolumeSnapshotBackup is still in progress. |
| SnapMoverBackupPhasePartiallyFailed                         | VolumeSnapshotBackupPhase    |    VolumeSnapshotBackup has partially failed.   |
| SnapMoverBackupPhaseFailed                                | VolumeSnapshotBackupPhase    |    VolumeSnapshotBackup has failed.   |