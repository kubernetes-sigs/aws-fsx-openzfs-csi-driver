# Installation

## Prerequisites

* Kubernetes Version >= 1.20
* Cluster running in EKS - the FSx for OpenZFS CSI driver currently does not support usage with self-managed clusters.

* Important: If you intend to use the Volume Snapshot feature, the [Kubernetes Volume Snapshot CRDs](https://github.com/kubernetes-csi/external-snapshotter/tree/master/client/config/crd) must be installed **before** the FSx for OpenZFS CSI driver. For installation instructions, see [CSI Snapshotter Usage](https://github.com/kubernetes-csi/external-snapshotter#usage).

## Installation
### Set up driver permissions
The driver requires IAM permissions to interact with the Amazon FSx for OpenZFS service to create/delete file systems, volumes, and snapshots on the user's behalf.
The node pods need only the EC2 instance ID. They read it from the Node's `spec.providerID` through the Kubernetes API.
The instance metadata service is a deprecated fallback. Only the controller calls AWS APIs.

> [!IMPORTANT]
> These steps are for new clusters. A cluster that already has volumes created by the driver must backfill the cluster
> tag first; see [Existing clusters](./cluster-tag-isolation.md#existing-clusters).

#### Amazon EKS: EKS Pod Identity (Recommended)
The tag-based policy [example-iam-policy-cluster-tag.json](./example-iam-policy-cluster-tag.json) limits each cluster's
controller to the resources tagged for its own cluster. See [cluster tag isolation](./cluster-tag-isolation.md) for what
it allows.

1. Install the [EKS Pod Identity Agent](https://docs.aws.amazon.com/eks/latest/userguide/pod-id-agent-setup.html). It must
   be installed as an EKS add-on; the driver itself can be installed in any of the ways below.
```sh
aws eks create-addon --cluster-name $cluster_name --addon-name eks-pod-identity-agent
```
2. Create an IAM role with the [EKS Pod Identity trust policy](https://docs.aws.amazon.com/eks/latest/userguide/pod-id-role.html),
   and attach the tag-based policy. One role can serve every cluster in the account.
```sh
aws iam put-role-policy --role-name FSxOpenZFSCSIDriverRole --policy-name cluster-tag \
    --policy-document file://docs/example-iam-policy-cluster-tag.json
```
3. Associate the role with the controller's ServiceAccount, in each cluster:
```sh
aws eks create-pod-identity-association --cluster-name $cluster_name --namespace kube-system \
    --service-account fsx-openzfs-csi-controller-sa \
    --role-arn arn:aws:iam::$account_id:role/FSxOpenZFSCSIDriverRole
```
4. When you [deploy the driver](#deploy-driver), set the cluster name, the cluster tag key and `enforce`. With Helm:
```sh
    --set controller.clusterName=$cluster_name \
    --set controller.clusterTagKey=eks:eks-cluster-name \
    --set controller.clusterTagIsolation=enforce
```
   The controller flags are `--cluster-name=$cluster_name --cluster-tag-key=eks:eks-cluster-name --cluster-tag-isolation=enforce`.

#### Other credentials
With IAM roles for service accounts or other credentials without the EKS Pod Identity session tags, use the
[shared policy](./example-iam-policy-shared.json), which limits the controller to resources this driver created. Leave
`controller.clusterName` and `controller.clusterTagKey` unset. The shared policy doesn't isolate clusters; see [IRSA and other credentials](./cluster-tag-isolation.md#irsa-and-other-credentials).

### Configure driver toleration settings
By default, the driver controller pod tolerates taint `CriticalAddonsOnly` and has `tolerationSeconds` configured as `300`. 
Additionally, the driver node pod tolerates all taints. 
If you do not wish to deploy the driver node pod on all nodes, please set Helm `Value.node.tolerateAllTaints` to false before deployment. 
You may then add policies to `Value.node.tolerations` to configure customized tolerations for nodes.

### Configure node startup taint
There are potential race conditions on node startup (especially when a node is first joining the cluster) 
where pods/processes that rely on the FSx for OpenZFS CSI Driver can act on a node before the FSx for OpenZFS CSI Driver is able to start up and become fully ready. 
To combat this, the FSx for OpenZFS CSI Driver contains a feature to automatically remove a taint from the node on startup. 
Users can taint their nodes when they join the cluster and/or on startup.  
This will prevent other pods from running and/or being scheduled on the node prior to the FSx for OpenZFS CSI Driver becoming ready.

This feature is activated by default. Cluster administrators should apply the taint `fsx.openzfs.csi.aws.com/agent-not-ready:NoExecute` to their nodes:
```shell
kubectl taint nodes $NODE_NAME fsx.openzfs.csi.aws.com/agent-not-ready:NoExecute
```
Note that any effect will work, but `NoExecute` is recommended. 

EKS Managed Node Groups support automatically tainting nodes, see [here](https://docs.aws.amazon.com/eks/latest/userguide/node-taints-managed-node-groups.html) for more details.

### Deploy driver
You may deploy the FSx for OpenZFS CSI driver via Kustomize or Helm

*Note: When using custom [CNI Plugins](https://kubernetes.io/docs/concepts/extend-kubernetes/compute-storage-net/network-plugins/) (e.g. Cilium) you might have to enable host networking for mounting the filesystem successfully.*

#### Kustomize
```sh
kubectl apply -k "github.com/kubernetes-sigs/aws-fsx-openzfs-csi-driver/deploy/kubernetes/overlays/stable/?ref=release-1.3"
```

*Note: Using the master branch to deploy the driver is not supported as the master branch may contain upcoming features incompatible with the currently released stable version of the driver.*

#### Helm
- Add the `aws-fsx-openzfs-csi-driver` Helm repository.
```sh
helm repo add aws-fsx-openzfs-csi-driver https://kubernetes-sigs.github.io/aws-fsx-openzfs-csi-driver
helm repo update
```

- Install the latest release of the driver.
```sh
helm upgrade --install aws-fsx-openzfs-csi-driver \
    --namespace kube-system \
    aws-fsx-openzfs-csi-driver/aws-fsx-openzfs-csi-driver
```

Review the [configuration values](https://github.com/kubernetes-sigs/aws-fsx-openzfs-csi-driver/blob/master/charts/aws-fsx-openzfs-csi-driver/values.yaml) for the Helm chart.

#### Once the driver has been deployed, verify the pods are running:
```sh
kubectl get pods -n kube-system -l app.kubernetes.io/part-of=aws-fsx-openzfs-csi-driver
```
