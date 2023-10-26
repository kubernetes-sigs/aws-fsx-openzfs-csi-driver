# Debugging

## Dynamic Provisioning

### Why isn't the driver dynamically provisioning my resource?
Typically, this is due to syntax or request errors based on the parameters provided in the StorageClass.
Run `kubectl get pods -n kube-system` and `kubectl logs ${driver_leader} -n kube-system` to grab the driver's logs.
These logs should output the cause of failure.

### Why is the driver throwing an error for a parameter that is correct?
Double check you are using the most up-to-date driver version.
If you are, the driver's SDK version may be outdated.
Please cut a ticket for us to bump the SDK version, as it will allow the driver to pull any API changes.

## Mounting

### Why can't I mount my resource?
Typically, this is due to a network or environmental issue.
Verify that the DNS is reachable from your pod.
Double check that filesystem lives in the same VPC as your cluster, the security group policy allow ingress NFS traffic, and the route tables are associated with the cluster's subnets.
See this [document](https://docs.aws.amazon.com/fsx/latest/OpenZFSGuide/access-within-aws.html) for more information.

## Maintenance

### How do I know what resources in my account are maintained by the driver?
Any resource created by the FSx for OpenZFS CSI driver is given tag.
Assuming these tags are not manually changes, you can find all resources under your account using this [hack](../hack/print-resources).
See [tagging](tagging.md) for more details.
 