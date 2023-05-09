# Specifying VolSync volumeOptions per defined storageClass

## Summary
The volumeSnapshotMover allows configurable advanced volumeOptions on the 
(volsync CRs:)[https://volsync.readthedocs.io/en/stable/usage/restic/index.html#backup-options].   
The volumeSnapshotClass can also be specified as an override option: https://github.com/backube/volsync/blob/main/api/v1alpha1/replicationsource_types.go#L92

As of now, these options cannot be defined per storageClass. This will cause 
potential conflict in some backup/restores that have mixed volumes supported by 
different provisioners, such as cephFS and cephRBD. 

A solution to this problem is to allow users to configure these volumeOptions 
per storageClass in `dpa.spec.features.dataMover.storageClass` 
as map[storageClass]volumeOptions:


```
apiVersion: oadp.openshift.io/v1alpha1
kind: DataProtectionApplication
metadata:
  name: dpa-example
  namespace: openshift-adp
spec:
  ....
  features:
    dataMover:
        credentialName: restic-secret
        enable: true
        storageClass:
            cephFS:
                sourceVolumeOptions:
                    storageClassName: cephfs-shallow
                    accessMode: ReadOnlyMany
                    moverSecurityContext: true
                destinationVolumeOptions:
                    cacheAccessMode: ReadWriteOnce
                    cachecapacity: 10Gi
                    moverSecurityContext: true
            cephRBD:
                sourceVolumeOptions:
                    accessMode: ReadOnlyMany
                    moverSecurityContext: true
                destinationVolumeOptions:
                    cacheAccessMode: ReadWriteOnce
                    moverSecurityContext: true
```

## Implementation:
- In the OADP operator, a configMap will be created *per* storageClass defined in  
`...datamover.storageClass`. This storageClass will be verified as an existing   
storageClass in the cluster. If a volumeSnapshotClass is also specified,  
this will also be verified, as well as verifying its correlated storageClass.  


```
kind: ConfigMap
apiVersion: v1
metadata:
    name: cephfs-config
    namespace: openshift-adp
data:
    cephFS:
        sourceVolumeOptions:
            storageClassName: cephfs-shallow
            accessMode: ReadOnlyMany
            moverSecurityContext: true
        destinationVolumeOptions:
            cacheAccessMode: ReadWriteOnce
            cachecapacity: 10Gi
            moverSecurityContext: true
```

```
kind: ConfigMap
apiVersion: v1
metadata:
    name: cephrbd-config
    namespace: openshift-adp
data:
    cephRBD:
        sourceVolumeOptions:
            accessMode: ReadOnlyMany
            moverSecurityContext: true
        destinationVolumeOptions:
            cacheAccessMode: ReadWriteOnce
            moverSecurityContext: true
```

Once a VSM CR is created and the VSM controller process begins, these configMaps  
will be read by the controller and used to create the volsync CR volumeOptions (linked above).  


The source PVC info is saved on VSB/VSR, which can be used to check that the  
correct storageClass volumeOptions are used for each volume by fetching the  
configMap with this PVC's storageClass name.

