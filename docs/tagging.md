# Tagging
To help manage FSx resources in your AWS accounts, the FSx for OpenZFS CSI driver will automatically add tags to the resources it manages.

| TagKey                          | TagValue                    | Example                                                                  | Description                                                                                                                                                                                                                                                                                                   |
|---------------------------------|-----------------------------|--------------------------------------------------------------------------|---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| CSIVolumeName                   | <pvcName>                   | CSIVolumeName = pvc-a3ab0567-3a48-4608-8cb6-4e3b1485c808                 | Added to all FSx for OpenZFS file systems and volumes created via the CSI Driver.                                                                                                                                                                                                                             |
| CSIVolumeSnapshotName           | <volumeSnapshotContentName> | CSIVolumeSnapshotName = snapcontent-69477690-803b-4d3e-a61a-03c7b2592a76 | Added to all FSx for OpenZFS snapshots created via the CSI Driver.                                                                                                                                                                                                                                            |
| CSISkipFinalBackupOnDeletion    | boolean                     | CSISkipFinalBackupOnDeletion = True                                      | Added to all FSx for OpenZFS file systems created via the CSI Driver.                                                                                                                                                                                                                                         |
| CSIOptionsOnDeletion            | string                      | CSIOptionsOnDeletion = DELETE_CHILD_VOLUMES_AND_SNAPSHOTS                | Added to FSx for OpenZFS file systems and volumes created via the CSI Driver with the OptionsOnDeletion parameter configured.                                                                                                                                                                                 |
| fsx.openzfs.csi.aws.com/cluster | true                        | fsx.openzfs.csi.aws.com/cluster = true                                   | Added to all FSx for OpenZFS file systems, volumes, snapshots, and final backups created via the CSI Driver. This allows users to track which resources were created via the FSx for OpenZFS CSI driver. Users may also use an IAM policy to limit the driver's permissions to just the resources it manages. |                                                                                      |

## Storage Class Tagging

The AWS FSx for OpenZFS CSI Driver supports tagging through `StorageClass.parameters`

**Example**
```
kind: StorageClass
apiVersion: storage.k8s.io/v1
metadata:
  name: fsx-openzfs-sc
provisioner: fsx.openzfs.csi.aws.com
parameters:
  ...
  ...
  tags: "key1=value1,key2=hello world,key3="
```

Provisioning a volume using the above StorageClass will apply the following tags:

```
key1=value1
key2=hello world
key3=<empty string>
```

## Snapshot Tagging
The AWS FSx for OpenZFS CSI Driver supports tagging snapshots through `VolumeSnapshotClass.parameters`, similar to StorageClass tagging.

**Example**
```
apiVersion: snapshot.storage.k8s.io/v1
kind: VolumeSnapshotClass
metadata:
  name: csi-aws-vsc
driver: fsx.openzfs.csi.aws.com
deletionPolicy: Delete
parameters:
  tags: "key1=value1,key2=hello world,key3="
```

Provisioning a snapshot using the above VolumeSnapshotClass will apply the following tags to the snapshot:

```
key1=value1
key2=hello world
key3=<empty string>
```
____

## Failure Modes

There can be multiple failure modes:
* The key/interpolated value do not meet the [AWS Tag Requirements](https://docs.aws.amazon.com/general/latest/gr/aws_tagging.html)
* The tags parameter is incorrectly formatted and not of the form "key=value".
* The key is not allowed (such as keys used internally by the CSI driver e.g., 'CSIVolumeName').
In these cases, the FSx for OpenZFS CSI driver will not provision a volume, but instead return an error.
