# Dynamic Provisioning

## Context

FSx for OpenZFS resources can be mounted to a given pod either statically or dynamically.
Static provisioning requires the user to initially create an FSxZ resource manually.
Dynamic provisioning will automatically create FSxZ resources based on user specifications.

This guide will detail the steps needed to dynamically create and mount an FSxZ resource.
For details on statically mounting an FSxZ resource see this [guide](../static-provisioning/README.md).

This CSI driver supports the use of both FSxZ filesystems and FSxZ volumes as container storage interfaces.
This guide will detail the steps needed to deploy both types of resources.
See this [guide](https://docs.aws.amazon.com/fsx/latest/OpenZFSGuide/administering-file-systems.html) for more details on each.

## Prerequisites

1. Kubernetes 1.13+ (CSI 1.0).
2. The [aws-fsx-openzfs-csi-driver](https://github.com/kubernetes-sigs/aws-fsx-openzfs-csi-driver) installed.

## Usage

This example shows you how to dynamically provision an FSxZ file system and FSxZ volume in your cluster.

Values in the example files may be modified or removed based on preferences.

### Create and Mount an FSxZ Resource

When creating an FSxZ volume, it is assumed that an FSxZ filesystem or parent volume has already been created.

1. Run **one of the following** to apply the manifests necessary to create either an FSxZ filesystem or an FSxZ volume.
    1. FSxZ filesystem:
    ```sh
   kubectl apply -f filesystem/,manifests/
    ```
    2. FSxZ volume:
   ```sh
   kubectl apply -f volume/,manifests/
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
   1. FSxZ filesystem:
   ```sh
   kubectl delete -f manifests/,filesystem/
   ```
   2. FSxZ volume:
   ```sh
   kubectl delete -f manifests/,volume/
   ```