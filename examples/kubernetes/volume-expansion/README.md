# Volume Expansion

## Context

Users are able to change the capacity of their volumes. 
Currently, the capacity can ONLY be increased, and NOT decreased.
Only storage capacity can be adjusted. Throughput can NOT be modified.
When scaling an FSxZ Volume, both StorageCapacityQuota and StorageCapacityReservation are increased to the same value.

When expanding the storage capacity of an FSxZ filesystem it must be increased by at least 10% of the current capacity, up to a max size of 512 TiB.
For more information see this [document](https://docs.aws.amazon.com/fsx/latest/OpenZFSGuide/managing-storage-capacity.html).

## Prerequisites

1. Kubernetes 1.13+ (CSI 1.0).
2. The [aws-fsx-openzfs-csi-driver](https://github.com/kubernetes-sigs/aws-fsx-openzfs-csi-driver) installed.

## Usage

This example shows you how to expand the capacity of a volume.

Values in the example files may be modified or removed based on preferences.

### Expand an FSxZ Resource Capacity

1. Run **one of the following** to apply manifests needed to create either an FSxZ filesystem or an FSxZ volume.
See this [example](../dynamic-provisioning/README.md) for more information on dynamic provisioning.
   1. FSxZ filesystem:
    ```sh
   kubectl apply -f ../dynamic-provisioning/filesystem/,../dynamic-provisioning/manifests/
    ```
   2. FSxZ volume:
   ```sh
   kubectl apply -f ../dynamic-provisioning/volume/,../dynamic-provisioning/manifests/
    ```
2. Update the storage capacity of respective claim:
    ```sh
   export KUBE_EDITOR="nano" && kubectl edit pvc fsx-pvc
    ```
3. Verify the capacity is updated on both the PersistentVolume and PersistentVolumeClaim:
   ```sh
   kubectl get pv && kubectl get pvc
    ```
4. Run **one of the following** to delete the associated resources that were created.
   1. FSxZ filesystem:
   ```sh
   kubectl delete -f ../dynamic-provisioning/manifests/,../dynamic-provisioning/filesystem/
   ```
   2. FSxZ volume:
   ```sh
   kubectl delete -f ../dynamic-provisioning/manifests/,../dynamic-provisioning/volume/
   ```