# Installation

## Prerequisites

* Kubernetes Version >= 1.20

* If you are using a self managed cluster, ensure the flag `--allow-privileged=true` for `kube-apiserver`.

* Important: If you intend to use the Volume Snapshot feature, the [Kubernetes Volume Snapshot CRDs](https://github.com/kubernetes-csi/external-snapshotter/tree/master/client/config/crd) must be installed **before** the FSx for OpenZFS CSI driver. For installation instructions, see [CSI Snapshotter Usage](https://github.com/kubernetes-csi/external-snapshotter#usage).

## Installation
### Set up driver permissions
The driver requires IAM permissions to interact with the Amazon FSx for OpenZFS service to create/delete file systems, volumes, and snapshots on user's behalf. 
There are several methods to grant the driver IAM permissions:
* Using [IAM roles for ServiceAccounts](https://docs.aws.amazon.com/eks/latest/userguide/iam-roles-for-service-accounts.html) (**Recommended**) - Create a Kubernetes service account for the driver and attach the AmazonFSxFullAccess AWS-managed policy to it with the following command. If your cluster is in the AWS GovCloud Regions, then replace arn:aws: with arn:aws-us-gov. Likewise, if your cluster is in the AWS China Regions, replace arn:aws: with arn:aws-cn:
```sh
eksctl create iamserviceaccount \
    --name fsx-openzfs-csi-controller-sa \
    --namespace kube-system \
    --cluster $cluster_name \
    --attach-policy-arn arn:aws:iam::aws:policy/AmazonFSxFullAccess \
    --approve \
    --role-name AmazonEKSFSxOpenZFSCSIDriverFullAccess \
    --region $region_code
```

* Using IAM [instance profile](https://docs.aws.amazon.com/IAM/latest/UserGuide/id_roles_use_switch-role-ec2_instance-profiles.html) - Create the following IAM policy and attach the policy to the instance profile IAM role of your cluster's worker nodes. 
See [here](https://docs.aws.amazon.com/eks/latest/userguide/create-node-role.html) for guidelines on how to access your EKS node IAM role.
```sh
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": [
        "iam:CreateServiceLinkedRole",
        "iam:AttachRolePolicy",
        "iam:PutRolePolicy"
      ],
      "Resource": "arn:aws:iam::*:role/aws-service-role/fsx.amazonaws.com/*"
    },
    {
      "Action":"iam:CreateServiceLinkedRole",
      "Effect":"Allow",
      "Resource":"*",
      "Condition":{
        "StringLike":{
          "iam:AWSServiceName":[
            "fsx.amazonaws.com"
          ]
        }
      }
    },
    {
      "Effect": "Allow",
      "Action": [
        "fsx:CreateFileSystem",
        "fsx:DeleteFileSystem",
        "fsx:DescribeFileSystems",
        "fsx:CreateVolume",
        "fsx:DeleteVolume",
        "fsx:DescribeVolumes",
        "fsx:CreateSnapshot",
        "fsx:DeleteSnapshot",
        "fsx:DescribeSnapshots",
        "fsx:TagResource",
        "fsx:ListTagsForResource"
      ],
      "Resource": ["*"]
    }
  ]
}
```

### Configure driver toleration settings
By default, the driver controller tolerates taint `CriticalAddonsOnly` and has `tolerationSeconds` configured as `300`; and the driver node tolerates all taints. 
If you don't want to deploy the driver node on all nodes, please set Helm `Value.node.tolerateAllTaints` to false before deployment. 
Add policies to `Value.node.tolerations` to configure customized toleration for nodes.

### Deploy driver
You may deploy the FSx for OpenZFS CSI driver via Kustomize or Helm

#### Kustomize
```sh
kubectl apply -k "github.com/kubernetes-sigs/aws-fsx-openzfs-csi-driver/deploy/kubernetes/overlays/stable/?ref=release-0.1.0"
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
kubectl get pods -n kube-system -l app.kubernetes.io/name=aws-fsx-openzfs-csi-driver
```
