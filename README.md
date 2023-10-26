## Amazon FSx for OpenZFS CSI Driver
[![Go Report Card](https://goreportcard.com/badge/github.com/kubernetes-sigs/aws-fsx-openzfs-csi-driver)](https://goreportcard.com/report/github.com/kubernetes-sigs/aws-fsx-openzfs-csi-driver)

### Overview
The [Amazon FSx for OpenZFS](https://aws.amazon.com/fsx/openzfs/) Container Storage Interface (CSI) Driver provides a [CSI](https://github.com/container-storage-interface/spec/blob/master/spec.md) interface used by container orchestrators to manage the lifecycle of Amazon FSx for OpenZFS file systems and volumes.

### Features
* **Static Provisioning** - Associate an externally-created FSx for OpenZFS file system or volume with a [PersistentVolume](https://kubernetes.io/docs/concepts/storage/persistent-volumes/) (PV) for consumption within Kubernetes.
* **Dynamic Provisioning** - Automatically create FSx for OpenZFS file systems or volumes and associated [PersistentVolumes](https://kubernetes.io/docs/concepts/storage/persistent-volumes/) (PV) from [PersistentVolumeClaims](https://kubernetes.io/docs/concepts/storage/persistent-volumes/#dynamic) (PVC). Parameters can be passed via a [StorageClass](https://kubernetes.io/docs/concepts/storage/storage-classes/#the-storageclass-resource) for fine-grained control over volume creation.
* **Mount Options** - NFS Mount options can be specified in the [PersistentVolume](https://kubernetes.io/docs/concepts/storage/persistent-volumes/) (PV) resource to define how the volume should be mounted.
* **Volume Snapshots** - Create and restore [snapshots](https://kubernetes.io/docs/concepts/storage/volume-snapshots/) taken from a volume in Kubernetes. These snapshots can then be used to create new FSx for OpenZFS volumes.
* **Volume Resizing** - Expand an FSx for OpenZFS file system by specifying a new size in the [PersistentVolumeClaim](https://kubernetes.io/docs/concepts/storage/persistent-volumes/#expanding-persistent-volumes-claims) (PVC).

### CSI Interfaces
* Controller Service: ControllerGetCapabilities, CreateVolume, DeleteVolume, CreateSnapshot, DeleteSnapshot, ControllerExpandVolume
* Node Service: NodePublishVolume, NodeUnpublishVolume, NodeGetCapabilities, NodeGetInfo, NodeGetId
* Identity Service: GetPluginInfo, GetPluginCapabilities, Probe

## Container Images

| Driver Version | [ECR Public](https://gallery.ecr.aws/fsx-csi-driver/aws-fsx-openzfs-csi-driver) Image |
|----------------|---------------------------------------------------------------------------------------|
| v1.1.0         | public.ecr.aws/fsx-csi-driver/aws-fsx-openzfs-csi-driver:v1.1.0                       |   

<details>
<summary>Previous Images</summary>

| Driver Version | [ECR Public](https://gallery.ecr.aws/fsx-csi-driver/aws-fsx-openzfs-csi-driver) Image |
|----------------|---------------------------------------------------------------------------------------|
| v1.0.0         | public.ecr.aws/fsx-csi-driver/aws-fsx-openzfs-csi-driver:v1.0.0                       |
| v0.1.0 (Beta)  | public.ecr.aws/fsx-csi-driver/aws-fsx-openzfs-csi-driver:v0.1.0                       | 
</details>

## Releases

The FSx for OpenZFS CSI Driver follows [semantic versioning](https://semver.org/). The version will be bumped following the rules below:

* Significant breaking changes will be released as a `MAJOR` update.
* New features will be released as a `MINOR` update.
* Bug or vulnerability fixes will be released as a `PATCH` update.

### Support

Support will be provided for the latest version and one prior version. Bugs or vulnerabilities found in the latest version will be backported to the previous release in a new minor version.

This policy is non-binding and subject to change.

### Compatibility

The FSx for OpenZFS CSI Driver is compatible with Kubernetes versions v1.17+ and implements the CSI Specification v1.1.0.

### Documentation
Please read the documentation below before using the FSx for OpenZFS CSI Driver.
* [FAQ](docs/FAQ.md)
* [Driver Installation](docs/install.md)
* [Driver Launch Options](docs/options.md)
* [StorageClass Parameters](docs/parameters.md)
* [Volume Tagging](docs/tagging.md)
* [Guides and Examples](/examples/kubernetes)
* [Development and Contributing](CONTRIBUTING.md)

### License
This library is licensed under the [Apache 2.0 License](LICENSE).
