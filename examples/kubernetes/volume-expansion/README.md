# Volume Expansion

## Context

Users are able to use the driver to change the storage capacity of their FSx for OpenZFS file systems. 
Currently, the capacity can ONLY be increased, and NOT decreased.
Only the storage capacity of a file system can be adjusted; throughput capacity can NOT be modified.
Additionally, the StorageCapacityReservation and StorageCapacityQuota of an FSx for OpenZFS volume cannot be changed via the CSI driver.


When expanding the storage capacity of an FSx for OpenZFS file system, it must be increased by at least 10% of the current capacity, up to a max size of 512 TiB.
For more information see this [document](https://docs.aws.amazon.com/fsx/latest/OpenZFSGuide/managing-storage-capacity.html).

## Prerequisites

1. Kubernetes 1.13+ (CSI 1.0).
2. The [aws-fsx-openzfs-csi-driver](https://github.com/kubernetes-sigs/aws-fsx-openzfs-csi-driver) installed.

## Usage

This example shows you how to expand the capacity of a volume.

Values in the example files may be modified or removed based on preferences.

### Expand an FSx for OpenZFS File System

1. Run the following to apply manifests needed to create an FSx for OpenZFS file system.
See this [example](../dynamic-provisioning/README.md) for more information on dynamic provisioning.
    ```sh
   kubectl apply -f ../dynamic-provisioning/filesystem/
    ```
2. Update the requested storage capacity of respective claim:
    ```sh
   export KUBE_EDITOR="nano" && kubectl edit pvc fsx-pvc
    ```
3. Verify the capacity is updated on both the PersistentVolume and PersistentVolumeClaim:
   ```sh
   kubectl get pv && kubectl get pvc
    ```
4. Run the following to delete the associated resources that were created.
   ```sh
   kubectl delete -f ../dynamic-provisioning/filesystem/
   ```
