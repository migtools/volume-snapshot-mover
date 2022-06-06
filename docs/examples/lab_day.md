# OADP Data Mover Lab

Data mover make use of a custom Velero [CSI plugin](https://github.com/openshift/velero-plugin-for-csi/tree/data-mover) 
and [VolSync](https://volsync.readthedocs.io/en/stable/) to take a snapshot
of a stateful application and relocate this snapshots into an object store.   
The snapshot can then be used during a restore process to recover stateful 
application data in instances such as cluster deletion or disaster. 

## Steps for Data Mover with Rocket Chat

* For this example, you will deploy [rocket chat](https://github.com/konveyor/mig-demo-apps/tree/master/apps/rocket-chat).
* Add data to the rocket chat application.
* Using the custom CSI plugin, take a backup of the application.
* Unfortuantely, you had an "uh oh spaghettios" moment and deleted your application namespace.
* Using the custom CSI plugin, create a restore of the application. Disaster adverted!

## Prerequisites
* Install OADP Operator using [these steps](https://github.com/openshift/oadp-operator/blob/master/docs/install_olm.md).
* [Install](https://volsync.readthedocs.io/en/stable/installation/index.html) the VolSync controller:

    `$ helm repo add backube https://backube.github.io/helm-charts/`  
    `$ helm install -n openshift-adp volsync backube/volsync`

* Install the VolumeSnapshotMover CRDs `VolumeSnapshotBackup` and `VolumeSnapshotRestore`:

    `$ oc create -f config/crd/bases/`

* Have a DPA CR such as below. Note the `` field to enable dataMover.


```
apiVersion: oadp.openshift.io/v1alpha1
kind: DataProtectionApplication
metadata:
  name: velero-sample
  namespace: openshift-adp
spec:
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
          bucket: bucket-name
          prefix: bucket-prefix
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
  snapshotLocations:
    - velero:
        config:
          profile: default
          region: us-west-2
        provider: aws
  unsupportedOverrides:
    csiPluginImageFqin: 'quay.io/spampatt/velero-plugin-for-csi:latest'
    veleroImageFqin: 'quay.io/emcmulla/velero:test2'
```

* Create a [restic secret](https://volsync.readthedocs.io/en/stable/usage/restic/index.html#id2) named `restic-secret` in the adp namespace:

    `$ oc create -n openshift-adp -f ./restic-secret.yaml`

## Deploy Rocket Chat

* Follow the steps in the Rocket Chat [README](https://github.com/konveyor/mig-demo-apps/blob/master/apps/rocket-chat/README.md).

## Add data to Rocket Chat

* You can add data to Rocket Chat by first finding the application route.

    `$ oc get route rocket-chat -n rocket-chat -ojsonpath="{.spec.host}"`

* Once at the application UI, create an account with your information. 

![Rocket_chat_home](/docs/examples/images/rocket_chat_home.png)

* Next, add a message in one of the channels.

![Rocket_chat_message](/docs/examples/images/message.png)
        

## Create backup using custom CSI plugin

* The Velero CSI plugin has been extended to support data mover.  
To use these additional features, you must make use of the `unsupportedOverrides` option 
in your OADP DPA config, as shown above in the prerequisites.