# Batching VolumeSnapshotMover Backups and Restores

## Summary
The VolumeSnapshotMover controller currently runs on each `volumeSnapshotBackup` 
and `volumeSnapshotRestore` until completed. The problem with this is that as 
the number of VolSync processes increases, the time for backup/restore to complete
can become slow, as diminishing returns increases.


## Performance Testing Results
[Performance testing](https://docs.google.com/document/d/1kPKo46_McEkj0Fu9hdkTxiVDnTh00forFQnscTgiEgg/edit?usp=sharing) 
found that there is an optimal batch number of 12 for backup. For restore, 
the results appear to be linear. 


## Motivation
Improve the performance of VolumeSnapshotMover backup and restore by allowing 
users to specify a concurrent number of `volumeSnapshotBackups` and 
`volumeSnapshotRestores` that can be simultaneously `inProgress` at once.


## Implementation details
Configurable values, `dpa.spec.dataMover.concurrentBackupVolumes` and 
`dpa.spec.dataMover.concurrentRestoreVolumes`, can be used to specify 
a number of `volumeSnapshotBackup` and `volumeSnapshotRestore` CRs that should 
be operated on at once. For example, given 100 PVs, a user may want to use 
VolumeSnapshotMover to backup 15 at a time and restore 20 at a time for
improved performance. 

To implement this in the the VSM controller, we can keep track of the current 
number of `volumeSnapshotBackups` or `volumeSnapshotRestores` that are not yet 
completed. At the beginning of the reconcile, a counter can be incremented for 
each VSB/VSR that starts.  
When this counter is equal to the concurrentVolumes number, then requeue without 
starting another VSB/VSR process.   
Once a CR status changes to completed, this value will be decremented, and another
`volumeSnapshotBackup` or `volumeSnapshotRestore` can start.

If `dpa.spec.dataMover.concurrentBackupVolumes` or
`dpa.spec.dataMover.concurrentRestoreVolumes` is not set or it is set to zero, 
then a default value will be used. This value is TBD.

One option for a default value is to allow for unlimted `volumeSnapshotBackups` or 
`volumeSnapshotRestores` if it is not set. Another option is to defing constants 
such as `DefaultConcurrentBackupVolumes` and `DefaultConcurrentRestoreVolumes` 
that can be set to a determined number, such as 0 or 12. More discussion is needed
around this topic.


### DPA config:

```
...
spec:  
  features:  
    dataMover:  
      enable: true  
      credentialName: <dm-restic-secret-name>
      concurrentBackupVolumes: 12
      concurrentRestoreVolumes: 50
...
```
