# Static Provisioning

## Context

FSx for OpenZFS resources can be mounted to a given pod either statically or dynamically.
Static provisioning requires the user to manually pre-create an FSx for OpenZFS resource.
Dynamic provisioning will automatically create FSx for OpenZFS resources based on user specifications.

This guide will detail the steps needed to statically create and mount an FSx for OpenZFS resource.
For details on dynamically mounting an FSx for OpenZFS resource see this [guide](../dynamic-provisioning/README.md).

This CSI driver supports the use of both FSx for OpenZFS file systems and FSx for OpenZFS volumes as container storage interfaces.
This guide will detail the steps needed to deploy both types of resources.
See this [guide](https://docs.aws.amazon.com/fsx/latest/OpenZFSGuide/administering-file-systems.html) for more details on each.

## Prerequisites

1. Kubernetes 1.13+ (CSI 1.0).
2. The [aws-fsx-openzfs-csi-driver](https://github.com/kubernetes-sigs/aws-fsx-openzfs-csi-driver) installed.

## Usage

This example shows you how to statically provision an FSx for OpenZFS file system and FSx for OpenZFS volume in your cluster.

Values in the example files may be modified or removed based on preferences.

### Create and Mount an FSx for OpenZFS Resource

It is assumed that the necessary FSx for OpenZFS resource has already been created. 
For information on creating an FSx for OpenZFS file system see this [guide](https://docs.aws.amazon.com/fsx/latest/OpenZFSGuide/creating-file-systems.html). 
For information on creating an FSx for OpenZFS volume see this [guide](https://docs.aws.amazon.com/fsx/latest/OpenZFSGuide/managing-volumes.html).

1. Run **one of the following** to apply the manifests necessary to mount the pre-created FSx for OpenZFS file system or FSx for OpenZFS volume.
    1. FSx for OpenZFS file system:
    ```sh
   kubectl apply -f filesystem/,pod/
    ```
    2. FSx for OpenZFS volume:
   ```sh
   kubectl apply -f volume/,pod/
    ```
2. Verify the PersistentVolumeClaim enters a `Bound` state:
    ```sh
    kubectl get pvc fsx-pvc
    ```
3. Verify data can be written from the pod:
   ```sh
   kubectl exec -ti fsx-app -- tail -f /data/out.txt
    ```
4. Run **one of the following** to delete the associated Kubernetes resources that were created.
   1. FSx for OpenZFS file system:
   ```sh
   kubectl delete -f pod/,filesystem/
   ```
   2. FSx for OpenZFS volume:
   ```sh
   kubectl delete -f pod/,volume/
   ```
   