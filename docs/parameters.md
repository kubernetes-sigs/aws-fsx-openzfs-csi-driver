# Storage Class Parameters
The AWS FSx for OpenZFS CSI Driver supports the dynamic provisioning of both FSx for OpenZFS file systems AND volumes.

To do this, the driver accepts **TWO** forms of storage classes: one for provisioning new file systems, and one for provisioning new volumes.
A universal `ResourceType` parameter denotes which type of resource is being provisioned.

The parameters that can be inputted to the storage class depends on the respective create and delete APIs.
Parameters under key `OpenZFSConfiguration` are flattened as top layer parameters.
Parameter values should be in JSON format, excluding the CSI specific `ResourceType` parameter which is a plain string.
Mistakes in parameters such as incorrect keys or values will produce errors.
Delete parameters must include the suffix `OnDeletion` on the parameter name.
Some fields are populated by the CSI driver, and therefore should not be specified.

## File System Parameters

###### [FSx CreateFileSystem API](https://docs.aws.amazon.com/fsx/latest/APIReference/API_CreateFileSystem.html)
###### [FSx DeleteFileSystem API](https://docs.aws.amazon.com/fsx/latest/APIReference/API_DeleteFileSystem.html)

#### Parameters the driver populates
- ClientRequestToken
- FileSystemType
- StorageCapacity

Parameters a user can specify when provisioning an FSx OpenZFS file system:

| Parameter                       | Required? | Example Value                                                                                                                                                                                                                                                                            | Notes                                                                                                                                                |
|---------------------------------|-----------|------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|------------------------------------------------------------------------------------------------------------------------------------------------------|
| `ResourceType`                  | `Yes`     | `"filesystem"`                                                                                                                                                                                                                                                                           | Specifies whether the storage class will be used to create an FSx for OpenZFS file system or volume.                                                 |
| `DeploymentType`                | `Yes`     | `'"SINGLE_AZ_1"'`                                                                                                                                                                                                                                                                        |                                                                                                                                                      |
| `ThroughputCapacity`            | `Yes`     | `'64'`                                                                                                                                                                                                                                                                                   |                                                                                                                                                      |
| `SubnetIds`                     | `Yes`     | `'["subnet-016affca9638a1e61"]' `                                                                                                                                                                                                                                                        |                                                                                                                                                      |
| `SkipFinalBackupOnDeletion`     | `Yes`     | `'true'`                                                                                                                                                                                                                                                                                 | Configures whether or not FSx will create a final backup of the file system. Due to CSI limitations, users may only configure this at creation time. |
| `OptionsOnDeletion`             | `No`      | `'["DELETE_CHILD_VOLUMES_AND_SNAPSHOTS"]'`                                                                                                                                                                                                                                               | Configures deletion options for the volume. Due to CSI limitations, users may only configure this at creation time.                                  |
| `KmsKey`                        | `No`      | `'"12345678-90ab-cdef-ghij-klmnopqrstuv"'`                                                                                                                                                                                                                                               |                                                                                                                                                      |
| `AutomaticBackupRetentionDays`  | `No`      | `'1'`                                                                                                                                                                                                                                                                                    |                                                                                                                                                      |
| `CopyTagsToBackups`             | `No`      | `'false'`                                                                                                                                                                                                                                                                                |                                                                                                                                                      |
| `CopyTagsToVolumes`             | `No`      | `'false'`                                                                                                                                                                                                                                                                                |                                                                                                                                                      |
| `DailyAutomaticBackupStartTime` | `No`      | `'"00:00"`                                                                                                                                                                                                                                                                               |                                                                                                                                                      |
| `DiskIopsConfiguration`         | `No`      | `'{"Iops": 300, "Mode": "USER_PROVISIONED"}'`                                                                                                                                                                                                                                            |                                                                                                                                                      |
| `RootVolumeConfiguration`       | `No`      | `'{"CopyTagsToSnapshots": false, "DataCompressionType": "NONE", "NfsExports": [{"ClientConfigurations": [{"Clients": "*", "Options": ["rw","crossmnt"]}]}], "ReadOnly": false, "RecordSizeKiB": 128, "UserAndGroupQuotas": [{"Type": "USER", "Id": 1, "StorageCapacityQuotaGiB": 10}]}'` |                                                                                                                                                      |
| `WeeklyMaintenanceStartTime`    | `No`      | `'"7:09:00"'`                                                                                                                                                                                                                                                                            |                                                                                                                                                      |
| `SecurityGroupIds`              | `No`      | `'["sg-004e025204e2a0a25"]'`                                                                                                                                                                                                                                                             |                                                                                                                                                      |
| `Tags`                          | `No`      | `'[{"Key": "OPENZFS", "Value": "OPENZFS"}]'`                                                                                                                                                                                                                                             |                                                                                                                                                      |

## Volume Parameters

###### [FSx CreateVolume API](https://docs.aws.amazon.com/fsx/latest/APIReference/API_CreateVolume.html)
###### [FSx DeleteVolume API](https://docs.aws.amazon.com/fsx/latest/APIReference/API_DeleteVolume.html)

#### Parameters the driver populates
- ClientRequestToken
- Name
- VolumeType

Parameters a user can specify when provisioning an FSx OpenZFS volume:

| Parameter                       | Required? | Example Value                                                                    | Notes                                                                                                               |
|---------------------------------|-----------|----------------------------------------------------------------------------------|---------------------------------------------------------------------------------------------------------------------|
| `ResourceType`                  | Yes       | `"volume"`                                                                       | Specifies whether the storage class will be used to create an FSx for OpenZFS file system or volume.                |
| `ParentVolumeId`                | Yes       | `'"fsvol-0739c1035d272c266"'`                                                    |                                                                                                                     |
| `CopyTagsToSnapshots`           | No        | `"false"`                                                                        |                                                                                                                     |
| `DataCompressionType`           | No        | `'"NONE"'`                                                                       |                                                                                                                     |
| `NfsExports`                    | No        | `'[{"ClientConfigurations": [{"Clients": "*", "Options": ["rw","crossmnt"]}]}]'` |                                                                                                                     |
| `ReadOnly`                      | No        | `false`                                                                          |                                                                                                                     |
| `RecordSizeKiB`                 | No        | `128`                                                                            |                                                                                                                     |
| `StorageCapacityReservationGiB` | No        | `-1`                                                                             |                                                                                                                     |
| `StorageCapacityQuotaGiB`       | No        | `-1`                                                                             |                                                                                                                     |
| `UserAndGroupQuotas`            | No        | `[{"Type": "USER", "Id": 1, "StorageCapacityQuotaGiB": 10}]`                     |                                                                                                                     |
| `Tags`                          | No        | `'[{"Key": "OPENZFS", "Value": "OPENZFS"}]'`                                     |                                                                                                                     |
| `OptionsOnDeletion`             | No        | `'["DELETE_CHILD_VOLUMES_AND_SNAPSHOTS"]'`                                       | Configures deletion options for the volume. Due to CSI limitations, users may only configure this at creation time. |

## Snapshots

###### [FSx CreateSnapshot API](https://docs.aws.amazon.com/fsx/latest/APIReference/API_CreateSnapshot.html)

#### Parameters the driver populates
- ClientRequestToken
- Name
- VolumeId

Parameters a user can specify when provisioning an FSx OpenZFS snapshot:

| Parameter                       | Required? | Example Value                                                                    | Notes                                                                                                               |
|---------------------------------|-----------|----------------------------------------------------------------------------------|---------------------------------------------------------------------------------------------------------------------|
| `Tags`                          | No        | `'[{"Key": "OPENZFS", "Value": "OPENZFS"}]'`                                     |                                                                                                                     |

# Persistent Volume Attributes
The AWS FSx for OpenZFS CSI Driver stores the following volumeAttributes on the persistent volume to aid in mounting the underlying FSx for OpenZFS volume.

When dynamically provisioning new FSx for OpenZFS file systems or volumes as persistent volumes in your cluster, these values are set automatically by the driver itself.

When statically provisioning new FSx for OpenZFS file systems or volumes as persistent volumes in your cluster, the user must provide these values themselves.

| Parameter      | Required?                            | Example Value                                     | Description                                                                                                                                                                             |
|----------------|--------------------------------------|---------------------------------------------------|-----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `ResourceType` | Yes                                  | "volume" or "filesystem"                          | Specifies whether the persistent volume contains metadata about an underlying FSx for OpenZFS file system or volume. Mounting a "file system" will mount the file system's root volume. |
| `DNSName`      | Yes                                  | "fs-078106df77a01f1f7.fsx.us-east-1.aws.internal" | DNS name of the FSx for OpenZFS file system associated with the Persistent Volume.                                                                                                      |
| `VolumePath`   | Required for FSx for OpenZFS volumes | "fsx" or "fsx/exampleVolume"                      | Specifies the mount path for the FSx for OpenZFS volume you wish to mount. If `volumeType` is set to "filesystem," this defaults to "fsx".                                              |
