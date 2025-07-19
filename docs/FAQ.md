# Frequently Asked Questions
The purpose of this document is to answer questions surrounding the unique design decisions and limitations of the FSx for OpenZFS CSI Driver.

## General

### What does it mean to provision an FSx for OpenZFS file system as a persistent volume in my cluster? What is actually mounted?
In general, Persistent Volumes created with the "filesystem" `ResourceType` point at the file system's root volume.
Dynamically provisioning a new FSx for OpenZFS file system as a Persistent Volume (PV) will create the FSx for OpenZFS file system resource,
but attempting to mount the PV on a pod in your cluster will mount the file system's root volume. Likewise, attempting to take a Volume Snapshot
of the PV will create an FSx for OpenZFS snapshot of the file system's root volume. The only exception occurs when attempting to expand the PV;
increasing the requested storage request on a PVC bound to a PV that points at an FSx for OpenZFS file system will increase the storage capacity
of the overarching file system.

## Parameters

### Why is SkipFinalBackupOnDeletion required by the CSI Driver, and not the API?
The default behavior of the API is to set this value as false, which thus creates a resource after deletion.
To prevent users from creating dangling resources that the driver doesn't maintain, we force users to input this value.

### What happens if I do not provide a particular parameter in the Storage Class? What if I provide an invalid parameter?
If you fail to provide a required parameter on the storage class or provide an invalid parameter, any attempted creates using that storage class will fail.
The FSx for OpenZFS CSI driver will provide relevant logging detailing the failure mode/reason on the PVC or in the plugin logging.
If you fail to provide an optional parameter in the storage class, the driver will use the default value specified in the [FSx API](https://docs.aws.amazon.com/cli/latest/reference/fsx/index.html#cli-aws-fsx).

### How strict is the parameter validation for the parameters provided in the Storage Class?
The parameter validation conducted by the FSx for OpenZFS CSI driver is as strict as the [FSx API](https://docs.aws.amazon.com/cli/latest/reference/fsx/index.html#cli-aws-fsx).
This means that incorrectly spelled parameters and incorrectly formatted parameter values will cause the creation to fail.
Additionally, parameter values must exactly match the enums specified in the FSx API.

### Can I modify parameters defined in the StorageClass after a resource is created?
Unfortunately there is no way to modify these parameters due to the nature of the CSI driver.
If a user wishes to modify a particular parameter, they may do so manually, using our API.
This includes parameters such as ThroughputCapacity, StorageCapacityReservation, and StorageCapacityQuota.

### Why are the parameters formatted the way they are?
FSx for OpenZFS shares its API with other types in the FSx family.
Because of this, most configuration options are nested under an `OpenZFSConfiguration` object.
Since this driver is not shared, we decided it would be best to flatten this object at the top level to enhance readability.
Due to the complex nature of our configurations, we also decided it would be best to align with our API and by requiring parameter values be in JSON format.

## Storage Capacity

### Why must I set storage on a PersistentVolumeClaim to 1 for volumes?
FSx for OpenZFS allows users to set StorageCapacityReservation and StorageCapacityQuota which can impact the storage requirements of a given volume.
There is truly no one size fits all, as there are different requirements based on use case.
Because of this, we decided to ignore the storage value is when creating the volume, and instead have users utilize the StorageClass parameters.
For INTELLIGENT_TIERING storage type the value should be "1Gi",  as INTELLIGENT_TIERING filesystems manage capacity dynamically without requiring upfront capacity specification.
To minimize operator confusion, and to keep the door open for possible changes, **the driver will fail to create or modify a volume if the user specifies a value other than 1Gi**.

## Volume Expansion

### Can I expand the storage capacity of my filesystem?
You may expand the size of a PV bound to an FSx for OpenZFS file system by increasing the requested storage in the PVC.
This will increase the size of the file system's storage capacity to the value requested in the PVC.
Currently, there is no way to adjust the storage requirements of a PV bound to an FSx for OpenZFS volume via the CSI driver.

### Why can't I decrease the storage capacity of my filesystem? Why must I increase the capacity by at least 10%?
These requirements set by the API.
See this [document](https://docs.aws.amazon.com/fsx/latest/OpenZFSGuide/managing-storage-capacity.html) for more information.

## Snapshots

### When I take a snapshot, what FSx resource is created?
When you take a snapshot, the CSI driver will take an FSx Snapshot.
Snapshots occur at the volume level.
If you take a snapshot of a filesystem, it will occur on the root volume of that filesystem.
Therefore, when restoring snapshots, they must be restored as a volume type.

### Can I take an FSx backup using the CSI Driver?
Currently, there is no way to take an FSx backup of a filesystem using the driver.
Any backups must be manually created and deleted using the API.
There is also no way to dynamically create a filesystem from a backup.

## Deletion Parameters

### How do I configure deletion parameters on my Persistent Volumes?
Due to limitations in the CSI specification, there is no way to configure deletion parameters for PVs at deletion time.
Therefore, we allow users to input these parameters in the StorageClass during initialization.
See [parameters.md](parameters.md) for information on setting these parameters.

### How do I edit a deletion parameter after my resource is already created?
Deletion parameters are saved by applying a tag for each parameter on the underlying FSx resource.
If you wish to revise deletion behavior after creation, you will need to manually edit these tags.
This will need to be done outside the context of the CSI driver.
If the these tags are missing from the underlying FSx for OpenZFS resource or the tag does not have a valid value, 
the driver will use the default behavior specified in the [FSx API](https://docs.aws.amazon.com/cli/latest/reference/fsx/index.html#cli-aws-fsx).
Deletion parameters with JSON special characters `"[]{},` are encoded due to [AWS tagging limitations](https://docs.aws.amazon.com/tag-editor/latest/userguide/tagging.html#tag-conventions).
See [tagging](tagging.md) for more details.
