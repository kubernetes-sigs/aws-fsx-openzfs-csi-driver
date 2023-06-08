# Frequently Asked Questions
The purpose of this document is to answer questions surrounding the unique design decisions and limitations of the FSx for OpenZFS CSI Driver.

### What does it mean to provision an FSx for OpenZFS file system as a persistent volume in my cluster? What is actually mounted?
In general, Persistent Volumes created with the "filesystem" `volumeType` point at the file system's root volume.
Dynamically provisioning a new FSx for OpenZFS file system as a Persistent Volume (PV) will create the FSx for OpenZFS file system resource,
but attempting to mount the PV on a pod in your cluster will mount the file system's root volume. Likewise, attempting to take a Volume Snapshot
of the PV will create an FSx for OpenZFS snapshot of the file system's root volume. The only exception occurs when attempting to expand the PV;
increasing the requested storage request on a PVC bound to a PV that points at an FSx for OpenZFS file system will increase the storage capacity
of the overarching file system.

### How do I request storage when dynamically provisioning a new FSx for OpenZFS volume as a persistent volume in my cluster?
When dynamically provisioning a new FSx for OpenZFS volume, the FSx for OpenZFS CSI driver ignores the storage requested in the Persistent Volume Claim (PVC).
Instead, the driver configures storage requirements via the storageCapacityReservation and storageCapacityQuota parameters, which are defined in the storage class.
To minimize operator confusion, **the driver will fail to create a volume if the user specifies a value other than the default 1Gi**.

### What happens when I expand the size of my Persistent Volumes?
You may expand the size of a PV bound to an FSx for OpenZFS file system by increasing the requested storage in the PVC.
This will increase the size of the file system's storage capacity to the value requested in the PVC.
Currently, there is no way to adjust the storage requirements of a PV bound to an FSx for OpenZFS volume via the CSI driver.
To do this, you will need to update the volume via the [FSx API](https://docs.aws.amazon.com/cli/latest/reference/fsx/index.html#cli-aws-fsx) outside the context of your cluster.
See [here](../examples/kubernetes/volume-expansion/README.md) for more details.

### What happens if I do not provide a particular parameter in the Storage Class? What if I provide an invalid parameter?
If you fail to provide a required parameter on the storage class or provide an invalid parameter, any attempted creates using that storage class will fail.
The FSx for OpenZFS CSI driver will provide relevant logging detailing the failure mode/reason on the PVC or in the plugin logging.
If you fail to provide an optional parameter in the storage class, the driver will use the default value specified in the [FSx API](https://docs.aws.amazon.com/cli/latest/reference/fsx/index.html#cli-aws-fsx).

### How strict is the parameter validation for the parameters provided in the Storage Class?
The parameter validation conducted by the FSx for OpenZFS CSI driver is as strict as the [FSx API](https://docs.aws.amazon.com/cli/latest/reference/fsx/index.html#cli-aws-fsx). 
This means that incorrectly spelled parameters and incorrectly formatted parameter values will cause the creation to fail. 
Additionally, parameter values must exactly match the enums specified in the FSx API.

### How do I configure deletion parameters on my Persistent Volumes?
Due to limitations in the CSI specification, there is no way to configure deletion parameters for PVs at deletion time. 
As such, you will only be able to configure deletion parameters at creation time. This may be done by setting 
`skipFinalBackupOnDeletion` and `optionsOnDeletion` on the storage class manifests; see [parameters.md](parameters.md) for more information.
These deletion parameters are configured by applying tags on the underlying FSx resource. 
If you wish to revise the deletion behavior of the volume after creation, you will need to edit these tags on the FSx resource. 
This will need to be done outside the context of the CSI driver.
If the these tags are missing from the underlying FSx for OpenZFS resource or the tag does not have a valid value, 
the driver will use the default behavior specified in the [FSx API](https://docs.aws.amazon.com/cli/latest/reference/fsx/index.html#cli-aws-fsx).
Deletion parameters with JSON special characters `"[]{},` are encoded due to [AWS tagging limitations](https://docs.aws.amazon.com/tag-editor/latest/userguide/tagging.html#tag-conventions).
See [tagging](tagging.md) for more details.

### Is there a way to tag FSx resources created via the CSI driver?
The AWS FSx for OpenZFS CSI Driver supports tagging via the `StorageClass.parameters` and `VolumeSnapshotClass.parameters`.
Any resource created by the FSx for OpenZFS CSI driver is provided default tags to allow you to track which 
FSx resources were created via the CSI driver. See [tagging](tagging.md) for more details.
