# OADP and CephFS Storage

To more effectively and efficiently backup CephFS backed PV's in OpenShift, 
OADP is required to use [CephFS snapshot-backed volumes](https://github.com/ceph/ceph-csi/blob/devel/docs/cephfs-snapshot-backed-volumes.md).

To enable CephFS snapshot-backed volumes the OpenShift Administrator will need
to set the `backingSnapshot` parameter to true. 

```
backingSnapshot: “true”
```

More details about the CephFS CSI configuration options can be found [here](https://github.com/ceph/ceph-csi/blob/devel/docs/deploy-cephfs.md)

**Note**

The CephFS CSI configuration is not to be confused the Volume Options one can
provide for CephFS and other storage backends.  Details with regards to OADP and 
Volume Options can be found (here)[https://github.com/migtools/volume-snapshot-mover/pull/216]

** todo update link

