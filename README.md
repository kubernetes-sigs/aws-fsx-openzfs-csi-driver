**WARNING**: This driver is in pre ALPHA currently. This means that there may potentially be backwards compatible breaking changes moving forward. Do NOT use this driver in a production environment in its current state.

**DISCLAIMER**: This is not an officially supported Amazon product

## Amazon FSx for OpenZFS CSI Driver
### Overview

The [Amazon FSx for OpenZFS](https://aws.amazon.com/fsx/openzfs/) Container Storage Interface (CSI) Driver provides a [CSI](https://github.com/container-storage-interface/spec/blob/master/spec.md) interface used by container orchestrators to manage the lifecycle of Amazon FSx for OpenZFS file systems and volumes.

This driver is in alpha stage. Basic volume operations that are functional include NodePublishVolume/NodeUnpublishVolume.

### CSI Specification Compatibility Matrix
| AWS FSx for OpenZFS CSI Driver \ CSI Version | v1.x.x |
|----------------------------------------------|--------|
| master branch                                | yes    |

### Kubernetes Version Compatibility Matrix
| AWS FSx for OpenZFS CSI Driver \ Kubernetes Version | v1.17+ |
|-----------------------------------------------------|--------|
| master branch                                       | yes    |

## Features
* Dynamic Provisioning - Automatically create FSx OpenZFS file systems and volumes based on configurations. These can then be mounted to a given pod.
* Static Provisioning - An FSx for OpenZFS file system or volume needs to be created manually first, then it could be mounted inside container as a volume using the driver.
* Mount Options - NFS mount options can be specified in a storage class to define how the volume should be mounted.

## CSI Interfaces
* Controller Service: ControllerGetCapabilities, CreateVolume, DeleteVolume, CreateSnapshot, DeleteSnapshot
* Node Service: NodePublishVolume, NodeUnpublishVolume, NodeGetCapabilities, NodeGetInfo, NodeGetId
* Identity Service: GetPluginInfo, GetPluginCapabilities, Probe

## Development
Please go through [CSI Spec](https://github.com/container-storage-interface/spec/blob/master/spec.md) and [General CSI driver development guideline](https://kubernetes-csi.github.io/docs/Development.html) to get some basic understanding of CSI driver before you start.

### Requirements
* Golang 1.9+

### Testing
To execute all unit tests, run: `make test`

## License
This library is licensed under the Apache 2.0 License. 