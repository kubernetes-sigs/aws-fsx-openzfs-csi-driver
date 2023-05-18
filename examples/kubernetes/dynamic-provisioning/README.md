# Dynamic Provisioning

## Context

FSx for OpenZFS resources can be mounted to a given pod either statically or dynamically.
Static provisioning requires the user to manually pre-create an FSx for OpenZFS resource.
Dynamic provisioning will automatically create FSx for OpenZFS resources based on user specifications.

This guide will detail the steps needed to dynamically create and mount an FSx for OpenZFS resource.
For details on statically mounting an FSx for OpenZFS resource see this [guide](../static-provisioning/README.md).

This CSI driver supports the use of both FSx for OpenZFS file systems and volumes as container storage interfaces.
This guide will detail the steps needed to deploy both types of resources.
See this [guide](https://docs.aws.amazon.com/fsx/latest/OpenZFSGuide/administering-file-systems.html) for more details on each.

## Prerequisites

1. Kubernetes 1.13+ (CSI 1.0).
2. The [aws-fsx-openzfs-csi-driver](https://github.com/kubernetes-sigs/aws-fsx-openzfs-csi-driver) installed.

## Usage

This example shows you how to dynamically provision an FSx for OpenZFS file system and FSx for OpenZFS volume in your cluster.

Values in the example files may be modified or removed based on preferences.

### Create and Mount an FSx for OpenZFS Resource

When creating an FSx for OpenZFS volume, it is assumed that an FSx for OpenZFS file system or parent volume has already been created.

1. Run **one of the following** to apply the manifests necessary to create either an FSx for OpenZFS file system or an FSx for OpenZFS volume.
    1. FSx for OpenZFS file system:
    ```sh
   kubectl apply -f filesystem/,pod/
    ```
    2. FSx for OpenZFS volume:
   ```sh
   kubectl apply -f volume/,pod/
    ```
2. Verify the PersistentVolumeClaim enters a `Bound` state.
    ```sh
   kubectl get pvc fsx-pvc
    ```
3. Verify data can be written from the pod.
   ```sh
   kubectl exec -ti fsx-app -- tail -f /data/out.txt
    ```
4. Run **one of the following** to delete the associated resources that were created.
   1. FSx for OpenZFS file system:
   ```sh
   kubectl delete -f pod/,filesystem/
   ```
   2. FSx for OpenZFS volume:
   ```sh
   kubectl delete -f pod/,volume/
   ```
   