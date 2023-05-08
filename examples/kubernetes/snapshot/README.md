# Snapshots

## Context

Volume snapshots allow users to quickly store the state of a volume at a given time.
Snapshots can be taken either manually or by utilizing the CSI driver.
Any snapshot taken can be utilized to create new volumes.

Currently only FSxZ volume level snapshots are supported.
Snapshots taken of an FSxZ filesystem will create a snapshot of its root volume.
FSxZ filesystem level backups are NOT supported.
Snapshots can only be restored to volumes that exist under the same filesystem.

## Prerequisites

1. Kubernetes 1.13+ (CSI 1.0).
2. The [aws-fsx-openzfs-csi-driver](https://github.com/kubernetes-sigs/aws-fsx-openzfs-csi-driver) installed.
3. The [external snapshotter](https://github.com/kubernetes-csi/external-snapshotter) installed.
4. The `VolumeSnapshotDataSource` is set in `--feature-gates=` in the `kube-apiserver` specification. This feature is enabled by default from Kubernetes v1.17+.

## Usage

This example shows you how to create snapshots and restore them.

Values in the example files may be modified or removed based on preferences.

### Create VolumeSnapshot From Existing FSxZ Volume Snapshot

It is assumed that the necessary FSxZ snapshot has already been created.
For information on creating an FSxZ snapshot see this [guide](https://docs.aws.amazon.com/fsx/latest/OpenZFSGuide/snapshots-openzfs.html).

1. Apply necessary manifests:
    ```sh
    kubectl apply -f existing-snapshot/
    ```
2. Verify the VolumeSnapshot enters a `ReadyToUse` state:
    ```sh
    kubectl describe volumesnapshot.snapshot.storage.k8s.io fsx-vs
    ```
3. Delete the created Kubernetes resources:
    ```sh
    kubectl delete -f existing-snapshot/
    ```

### Create VolumeSnapshot From PersistentVolume
1. Run **one of the following** to apply the manifests needed to create either an FSxZ filesystem or an FSxZ volume.
   See this [example](../dynamic-provisioning/README.md) for more information on dynamic provisioning.
   1. FSxZ filesystem:
    ```sh
   kubectl apply -f ../dynamic-provisioning/filesystem/,../dynamic-provisioning/manifests/
    ```
   2. FSxZ volume:
   ```sh
   kubectl apply -f ../dynamic-provisioning/volume/,../dynamic-provisioning/manifests/
    ```
2. Verify the PersistentVolumeClaim enters a `Bound` state.
    ```sh
   kubectl get pvc fsx-pvc
    ```
3. Apply necessary manifests to create a snapshot
    ```sh
    kubectl apply -f snapshot/
    ```
4. Verify the VolumeSnapshot enters a `ReadyToUse` state:
    ```sh
    kubectl describe volumesnapshot.snapshot.storage.k8s.io fsx-vs
    ```
5. Delete all the created resources:
   1. FSxZ filesystem:
   ```sh
    kubectl delete -f snapshot/,../dynamic-provisioning/manifests/,../dynamic-provisioning/filesystem/
   ```
   2. FSxZ volume:
   ```sh
   kubectl delete -f snapshot/,../dynamic-provisioning/manifests/,../dynamic-provisioning/volume/
   ```

### Create PersistentVolume From VolumeSnapshot

1. Follow **one of the previous** sections to create a snapshot
2. Apply necessary manifests
    ```sh
    kubectl apply -f restore-snapshot/
    ```
3. Verify the PVC enters a `Bound` state:
   ```sh
   kubectl get pvc fsx-restored-pvc
   ```
4. Verify data can be written:
   ```sh
    kubectl exec -ti fsx-restored-app -- tail -f /data/out.txt
    ```
5. Delete all the created resources:
   1. FSxZ filesystem:
   ```sh
    kubectl delete -f restore-snapshot/,snapshot/,../dynamic-provisioning/manifests/,../dynamic-provisioning/filesystem/
   ```
   2. FSxZ volume:
   ```sh
   kubectl delete -f restore-snapshot/,snapshot/,../dynamic-provisioning/manifests/,../dynamic-provisioning/volume/
   ```