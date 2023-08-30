# Helm chart
# v1.1.0
* Use driver v1.1.0

# v1.0.0
* Use driver v1.0.0
* Add driver modes for controller and node pods
* Allow for json logging
* Added support for node startup taint (please see install documentation for more information)
* Adopt Kubernetes recommended labels
* Remove hostNetwork: true from the node daemonset
* Allow users to specify the AWS region in the controller deployment

# v0.1.0
* Released the AWS FSx for OpenZFS CSI Driver with helm support
