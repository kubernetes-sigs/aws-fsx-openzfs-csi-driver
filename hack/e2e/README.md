# eksctl e2e tests

`make test-e2e-eksctl` (`hack/e2e/eksctl.sh all`) runs the e2e tests on an EKS cluster created by eksctl, where the
controller uses EKS Pod Identity. `make test-e2e` still runs the tests against an existing cluster.

```sh
TEST_ID=1234 \
CLEAN=false \
AWS_REGION=us-west-2 \
./hack/e2e/eksctl.sh all
```

`eksctl.sh` installs eksctl, helm, gomplate and ginkgo into `bin/` when they are missing. `eksctl.sh up`:

1. builds the driver image with `make image` and pushes it to the ECR repository `aws-fsx-openzfs-csi-driver` in the
   caller's account, creating the repository if needed. Set `IMAGE_REPOSITORY` and `IMAGE_TAG` to use an image you
   built yourself instead;
2. creates the IAM role `fsx-openzfs-e2e-$TEST_ID-controller` with the EKS Pod Identity trust policy
   (`pod-identity.sh create-role`), with `docs/example-iam-policy.json` attached;
3. renders `eksctl-cluster.yaml` with gomplate and creates the cluster from it. The config installs the EKS Pod
   Identity Agent and snapshot controller add-ons, associates the role with
   `kube-system/fsx-openzfs-csi-controller-sa`, and blocks pod access to IMDS. EKS adds the `eks-cluster-name` and
   `eks-cluster-arn` session tags that the tag-based policies read;
4. installs the chart with that image.

`eksctl.sh test` runs the `FSx for OpenZFS` suites (`GINKGO_FOCUS`, `GINKGO_SKIP`), then the
`[fsx-openzfs-isolation]` specs in `tests/e2e/isolation_test.go` alone, because they swap the controller's IAM
policies (`pod-identity.sh set-policies`) and settings (`helm-upgrade.sh`). They walk the rollout in
`docs/cluster-tag-isolation.md`:

1. `disable` with `docs/example-iam-policy.json`: new resources carry the driver tag and no cluster tag, and the
   sweep adds the driver tag to the root volume of a file system the driver created.
2. `disable` with `docs/example-iam-policy-shared.json`: a volume and snapshot in a driver-tagged parent are created
   and deleted. In IAM, a volume in an untagged parent, deleting an untagged file system, and tagging after create
   are denied.
3. `backfill` with the shared, tag-based and backfill policies: the file system, its root volume, a volume and a
   snapshot get `eks:eks-cluster-name=<cluster>`, and the controller logs `cluster tag backfill complete`.
4. `enforce`, then only `docs/example-iam-policy-cluster-tag.json`: a new volume, a snapshot and a restore carry
   the cluster tag.
5. Negative, controller: deleting a PersistentVolume of a volume tagged for another cluster fails with
   `FailedPrecondition`.
6. Negative, IAM: an aws-cli pod with the controller's service account, so its role, can describe this cluster's
   volume, but is denied creating volumes in another cluster's or an untagged parent, from another cluster's
   snapshot, or with another cluster's tag; snapshotting or deleting another cluster's volume; and tagging after
   create.
7. Deleting the claims and snapshots deletes this cluster's resources.

Set `ISOLATION_E2E=false` to skip them, or `GINKGO_FOCUS=` to run only them. `eksctl.sh down` (and `all` with
`CLEAN=true`, also when a step fails) deletes the cluster, the role and the file systems tagged `OpenZFSCSIDriverE2E=$PULL_REQUEST`.
