# v1.1.0

### Notable changes
* Add support for Multi-AZ ([#49](https://github.com/kubernetes-sigs/aws-fsx-openzfs-csi-driver/pull/49), [@gomesjason](https://github.com/gomesjason/))
* Improved error logging for improperly formatted parameters ([#45](https://github.com/kubernetes-sigs/aws-fsx-openzfs-csi-driver/pull/45), [@hughdanliu](https://github.com/hughdanliu/))

# v1.0.0

### Notable changes
* Modularize the controller and node services; Add support for JSON logging ([#30](https://github.com/kubernetes-sigs/aws-fsx-openzfs-csi-driver/pull/30), [@hughdanliu](https://github.com/hughdanliu/))
* Add support for operating modes; Perform metadata collection via the Kubernetes API ([#34](https://github.com/kubernetes-sigs/aws-fsx-openzfs-csi-driver/pull/34), [@hughdanliu](https://github.com/hughdanliu/))
* Add support for node startup taints ([#35](https://github.com/kubernetes-sigs/aws-fsx-openzfs-csi-driver/pull/35), [@hughdanliu](https://github.com/hughdanliu/))
* Add inflight checks to node mounting operations  ([#36](https://github.com/kubernetes-sigs/aws-fsx-openzfs-csi-driver/pull/36), [@hughdanliu](https://github.com/hughdanliu/))

### Bug Fixes
* Fix snapshot parameters bug ([#32](https://github.com/kubernetes-sigs/aws-fsx-openzfs-csi-driver/pull/32), [@gomesjason](https://github.com/gomesjason/))
* Fix update In progress failure during ResizeFileSystem ([#33](https://github.com/kubernetes-sigs/aws-fsx-openzfs-csi-driver/pull/33), [@gomesjason](https://github.com/gomesjason/))

### Improvements
* Adopt Kubernetes recommended labels ([#29](https://github.com/kubernetes-sigs/aws-fsx-openzfs-csi-driver/pull/29), [@hughdanliu](https://github.com/hughdanliu/))
* Adopt Kubernetes standard logging patterns ([#38](https://github.com/kubernetes-sigs/aws-fsx-openzfs-csi-driver/pull/38), [@hughdanliu](https://github.com/hughdanliu/))

# v0.1.0

### Notable changes
* Add support for static provisioning of file systems and child volumes ([#1](https://github.com/kubernetes-sigs/aws-fsx-openzfs-csi-driver/pull/1), [@hughdanliu](https://github.com/hughdanliu/))
* Add support for dynamically creating volume snapshots ([#3](https://github.com/kubernetes-sigs/aws-fsx-openzfs-csi-driver/pull/3), [@hughdanliu](https://github.com/hughdanliu/))
* Add dynamic provisioning ([#4](https://github.com/kubernetes-sigs/aws-fsx-openzfs-csi-driver/pull/4), [@gomesjason](https://github.com/gomesjason/))
* Add volume expansion ([#5](https://github.com/kubernetes-sigs/aws-fsx-openzfs-csi-driver/pull/5), [@gomesjason](https://github.com/gomesjason/))
* Add helm chart files ([#6](https://github.com/kubernetes-sigs/aws-fsx-openzfs-csi-driver/pull/6), [@hughdanliu](https://github.com/hughdanliu/))
* Use minimal base docker image ([#13](https://github.com/kubernetes-sigs/aws-fsx-openzfs-csi-driver/pull/13), [@hughdanliu](https://github.com/hughdanliu/))
