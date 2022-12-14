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
users to specify a batch number of `volumeSnapshotBackups` and 
`volumeSnapshotRestores` that can be simultaneously `inProgress` at once.


## Implementation details
A configurable value, `dpa.spec.dataMover.batchNumber`, can be used to specify 
a number of `volumeSnapshotBackup` and `volumeSnapshotRestore` CRs that should 
be operated on at once. For example, given 100 PVs, a user may want to use 
VolumeSnapshotMover to backup 15 at a time for better performance. 

To implement this in the the VSM controller, we can keep track of the current 
number of VSBs/VSRs that are not yet completed. At the beginning of the reconcile, 
a counter can be incremented for each VSB/VSR that starts.  
When this counter is equal to the batch number, then requeue. 
Once a CR status changes to completed, this value will be decremented, and another
`volumeSnapshotBackup` or `volumeSnapshotRestore` can start.

If `dpa.spec.dataMover.batchNumber` is not set, then the controller will run as
it currently does, with each `volumeSnapshotBackup` and `volumeSnapshotRestore` 
until completed.


### DPA config:

```
...
spec:  
  features:  
    dataMover:  
      enable: true  
      credentialName: <dm-restic-secret-name>
      batchNumber: 12
...
```


## Alternate Design Ideas
- Batching for backup/restore in upstream Velero
    - This approach may be too slow for OADP 1.2