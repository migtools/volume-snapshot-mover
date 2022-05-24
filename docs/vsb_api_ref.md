<h1>VolumeSnapshotBackup API References</h1>

### VolumeSnapshotBackupSpec

| Property             | Type                       | Description                                        |
|----------------------|--------------------------------|-------------------------------------------------------|
| VolumeSnapshotContent      | corev1.ObjectReference                   | VolumeSnapshotContent is the name of the VolumeSnapshotContent that will be moved to a remote storage location.          |
| ProtectedNamespace      | string                 | ProtectedNamespace is the namespace in which the Velero deployment is present, and where VolumeSnapshotBackup resources will be created.   |


### VolumeSnapshotBackupStatus

| Property             | Type                        | Description                                 |
|----------------------|-------------------------------------------------|------------------------------------------------------|
| Completed     | bool                                                    | Completed is whether or not VolumeSnapshotBackup has completed reconciling.    |
| SourcePVCData      | PVCData                                                | SourcePVCData is a reference to the source PVC.            |
| ResticRepository      | string                                             | ResticRepository is the location in which the snapshot will be stored.        |
| Phase      | VolumeSnapshotBackupPhase                                       | Phase is the VolumeSnapshotBackup phase status.           |


### PVCData

| Property             | Type               |        Description                         |
|----------------------|---------------------------------------|---------------------------------------------|
| Name    | string                                      | Name is the name of the application's source PVC.   |
| Size     | string                                     | Size is the size of the source PVC.           |


### VolumeSnapshotBackupPhase

| Property           |  Description                                            |
|---------------------------|-----------------------------------------------------------|
| VolumeSnapshotBackupPhaseCompleted                                  | VolumeSnapshotBackup has completed.   |
| VolumeSnapshotBackupPhaseInProgress                              | VolumeSnapshotBackup has not yet completed.         |
| VolumeSnapshotBackupPhaseFailed                                 | VolumeSnapshotBackup has failed.      |      