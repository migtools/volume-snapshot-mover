# Batching VolumeSnapshotMover Backups and Restores

## Summary
The VolumeSnapshotMover controller currently runs on each `volumeSnapshotBackup` 
and `volumeSnapshotRestore` until completed. The problem with this is that as 
the number of VolSync processes increases, the time for backup/restore to complete
can become slow, as diminishing returns increases.


## Performance Testing Results
[Performance testing](https://docs.google.com/document/d/1kPKo46_McEkj0Fu9hdkTxiVDnTh00forFQnscTgiEgg/edit?usp=sharing) 
found that there is an optimal batch number of 12 for backup. For restore, 
the results appear to be linear. This is due to bandwidth restrictions of data
being egressed from the cluster, rather than controller performance.

It is also important to note that these results are from a small set of factors 
such a CephFS and Ceph RBD, as well as small volume sizes. It is possible for 
this number to change with different scenarios.


## Motivation
Improve the performance of VolumeSnapshotMover backup and restore by allowing 
users to specify a concurrent number of `volumeSnapshotBackups` and 
`volumeSnapshotRestores` that can be simultaneously `inProgress` at once.


## Implementation details
Configurable *int64 values, `dpa.spec.dataMover.maxConcurrentBackupVolumes` and 
`dpa.spec.dataMover.maxConcurrentRestoreVolumes`, can be used to specify 
a number of `volumeSnapshotBackup` and `volumeSnapshotRestore` CRs that should 
be operated on at once. For example, given 100 PVs, a user may want to use 
VolumeSnapshotMover to backup 15 at a time and restore 20 at a time for
improved performance. 

To implement this in the the VSM controller, we can keep track of the current 
number of `volumeSnapshotBackups` or `volumeSnapshotRestores` that are not yet 
completed by checking the CR status via a List call at the beginning of reconcile.
If a `volumeSnapshotBackup` or `volumeSnapshotRestore` is not completed, then
it will be added to a queue of processing CRs until the maxConcurrentVolume 
number is reached.

Once a `volumeSnapshotBackup` or `volumeSnapshotRestore` status changes to 
completed, it will be removed from the queue, and another CR can start.

If `dpa.spec.dataMover.maxConcurrentBackupVolumes` or
`dpa.spec.dataMover.maxConcurrentRestoreVolumes` is nil, then a default value 
will be used. 

To allow for a default value for a concurrent number of `volumeSnapshotBackups` 
or `volumeSnapshotRestores`, constants can be defined such as 
`DefaultConcurrentBackupVolumes` and `DefaultConcurrentRestoreVolumes` that can 
be set to a determined number, such as 0 (unlimited) or 12. 


### DPA config:

```
...
spec:  
  features:  
    dataMover:  
      enable: true  
      credentialName: <dm-restic-secret-name>
      maxConcurrentBackupVolumes: 12
      maxConcurrentRestoreVolumes: 50
...
```
