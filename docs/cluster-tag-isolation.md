# Cluster tag isolation

Several clusters in one AWS account can run this driver, and several clusters can even provision volumes inside one
shared FSx for OpenZFS file system. The shared policy supports that, because it allows the controller to change every
resource this driver created. It also means that each cluster's controller has IAM permissions to modify the file
systems, volumes and snapshots of the other clusters. This page describes the policies, and how to tag the resources
each cluster owns so that a tag-based policy can limit every controller to its own cluster.

The examples use EKS Pod Identity, account `111122223333`, region `us-west-2` and cluster `my-cluster`.

- [Choosing a policy](#choosing-a-policy)
- [The shared policy](#the-shared-policy)
- [The tag-based policy](#the-tag-based-policy)
- [The backfill policy](#the-backfill-policy)
- [New clusters](#new-clusters)
- [Existing clusters](#existing-clusters)
- [What each mismatch does](#what-each-mismatch-does)
- [Settings](#settings)

## Choosing a policy

The controller tags every file system, volume and snapshot it creates `fsx.openzfs.csi.aws.com/cluster=true` (the
driver tag). When `--cluster-name` and `--cluster-tag-key` are set, it also tags them `<cluster-tag-key>=<cluster-name>`
(the cluster tag), in every `--cluster-tag-isolation` state.

| Policy | The controller can change | Use it when |
|---|---|---|
| [`example-iam-policy-cluster-tag.json`](./example-iam-policy-cluster-tag.json) | Only resources tagged for its own cluster | Each cluster should only reach its own resources. Recommended for new clusters |
| [`example-iam-policy-shared.json`](./example-iam-policy-shared.json) | Every resource that carries the driver tag, in any cluster in the account | Several clusters create volumes in the same parent volume, or you don't need clusters isolated |

IAM can't use a variable in a condition key, so each policy names its tag key, and `--cluster-tag-key` must match it:

| Policy file | `--cluster-tag-key` |
|---|---|
| `example-iam-policy-cluster-tag.json`, `example-iam-policy-cluster-tag-backfill.json` | `eks:eks-cluster-name` |
| `example-iam-policy-shared.json` | Any, or unset. The policy doesn't read the cluster tag |

A parent volume carries one cluster tag value, so clusters that share one parent volume or file system can't use the
tag-based policy. Use the shared policy for them; it doesn't isolate clusters.

The tag-based policy reads the `eks-cluster-name` and `eks-cluster-arn` session tags that EKS Pod Identity adds. See
[IRSA and other credentials](#irsa-and-other-credentials) for credentials without them.

## The shared policy

[`example-iam-policy-shared.json`](./example-iam-policy-shared.json) limits the controller to resources that carry the
driver tag. It reads no principal or session tags, so it works with EKS Pod Identity, IRSA and any other credentials:

- `CreateFileSystem`, `CreateVolume` and `CreateSnapshot` require the driver tag in the request. `CreateVolume` also
  requires it on the parent volume and the origin snapshot, and `CreateSnapshot` on the source volume.
- `UpdateFileSystem`, `DeleteFileSystem`, `DeleteVolume` and `DeleteSnapshot` require the driver tag on the resource.
  The final backup of a deleted file system is allowed with any tags.
- `TagResource` is allowed only as part of a create request.

Every resource the driver created already has the driver tag, except the root volumes of its file systems: FSx creates
them without tags unless the StorageClass sets `CopyTagsToVolumes: 'true'`. In `disable` (the default), the controller
adds the driver tag to those root volumes; see [What the sweep does](#what-the-sweep-does). The shared policy itself
doesn't allow that, so set `CopyTagsToVolumes: 'true'` in file system StorageClasses once it is attached; otherwise
each sweep counts the new root volumes in `failed`. A parent volume you
created yourself (StorageClass `ParentVolumeId`) needs the driver tag before the controller can create volumes in it:

```sh
aws fsx tag-resource --tags Key=fsx.openzfs.csi.aws.com/cluster,Value=true \
    --resource-arn arn:aws:fsx:us-west-2:111122223333:volume/fs-0123456789abcdef0/fsvol-0123456789abcdef0
```

To move from `AmazonFSxFullAccess` to the shared policy:

1. Upgrade the driver, and keep `AmazonFSxFullAccess` attached.
2. Wait until every controller replica logs `cluster tag backfill complete` with `tagKey="fsx.openzfs.csi.aws.com/cluster"`,
   as in [Confirm backfill is complete](#confirm-backfill-is-complete) with `TAG_KEY=fsx.openzfs.csi.aws.com/cluster`
   and `CLUSTER=true`. Tag the resources reported for an administrator, if the controller should change them.
3. Attach `example-iam-policy-shared.json`, then detach `AmazonFSxFullAccess`.

## The tag-based policy

[`example-iam-policy-cluster-tag.json`](./example-iam-policy-cluster-tag.json) restricts the controller to resources
tagged for its own cluster, as far as the FSx APIs allow:

- `CreateFileSystem`, `CreateVolume` and `CreateSnapshot` require the cluster tag with this cluster's name in the
  request. `CreateVolume` also requires the parent volume and the origin snapshot to carry this cluster's tag, and
  `CreateSnapshot` requires it on the source volume.
- `UpdateFileSystem`, `DeleteFileSystem`, `DeleteVolume` and `DeleteSnapshot` require this cluster's tag on the
  resource. A final backup must get the cluster tag in the delete request.
- `TagResource` is allowed only as part of a create request. The controller can't change any tag after creation, so
  it can't change the cluster tag or the `*OnDeletion` deletion options of an existing resource.

The policy needs no editing for the key `eks:eks-cluster-name`: it compares the tag with the
`${aws:PrincipalTag/eks-cluster-name}` session tag that EKS Pod Identity adds.

**Roles per cluster.** One IAM role and this policy can serve every cluster in the account and still keep them
isolated. The Pod Identity trust policy names no cluster, and EKS sets the session tags for whichever cluster's
controller assumes the role. Repeat only the pod identity association and the Helm settings in each cluster.

These actions can't be scoped by tag:

| Action | What it can still reach |
|---|---|
| `fsx:DescribeFileSystems`, `fsx:DescribeVolumes`, `fsx:DescribeSnapshots` | No resource-level permissions. The metadata of every FSx resource in the cluster's region. |
| `fsx:ListTagsForResource` | The tags of every FSx file system, volume and snapshot in the cluster's region. |
| `iam:CreateServiceLinkedRole` | IAM is global. Limited to the FSx service-linked role, which FSx needs when the first file system in the account is created. |

FSx checks a delete only against its target. `DeleteFileSystem` and `DeleteVolume` with
`DELETE_CHILD_VOLUMES_AND_SNAPSHOTS` also delete every child volume and snapshot of the target, whichever cluster
created them. Don't create volumes of one cluster inside a file system or parent volume of another cluster.

**Region and account.** ARNs use `*` for the partition, region and account. Every FSx statement requires
`aws:PrincipalTag/eks-cluster-arn` to match `arn:*:eks:${aws:RequestedRegion}:*:cluster/*`, so the controller can call
FSx only in its cluster's region. FSx has no resource-based policies, so a role can only reach FSx resources of its own
account.

**Root and parent volumes.** Snapshots of a `ResourceType: filesystem` PersistentVolume, and volumes created inside
that file system, need its root volume to carry the cluster tag. Set `CopyTagsToVolumes: 'true'` in file system
StorageClasses, or let the sweep tag the root volumes (see [Existing clusters](#existing-clusters)). A StorageClass
`ParentVolumeId` that you created yourself needs the cluster tag:

```sh
aws fsx tag-resource --tags Key=eks:eks-cluster-name,Value=my-cluster \
    --resource-arn arn:aws:fsx:us-west-2:111122223333:volume/fs-0123456789abcdef0/fsvol-0123456789abcdef0
```

### IRSA and other credentials

The tag-based policy reads the `eks-cluster-name` and `eks-cluster-arn` session tags of EKS Pod Identity, so it denies
IRSA and other credentials without them. Use the [shared policy](#the-shared-policy) with
those credentials, and leave `--cluster-name` and `--cluster-tag-key` unset, or adapt the tag-based policy to read the
cluster name from something those credentials provide, such as a tag on the IAM role.

## The backfill policy

[`example-iam-policy-cluster-tag-backfill.json`](./example-iam-policy-cluster-tag-backfill.json) is attached together
with the tag-based policy only during `backfill`. It grants only `fsx:TagResource` with exactly the cluster tag, set
to this cluster's name, on file systems, volumes and snapshots that carry the driver tag and no cluster tag yet.
The shared policy allows `TagResource` only at create time, so the backfill policy is needed alongside it. Detach the
backfill policy after `enforce`.

The driver tag says that a driver created the resource, not which cluster. While the backfill policy is attached, the
role may tag any untagged resource with the driver tag in the region, whichever cluster created it. The controller
itself only tags resources that its own cluster's PersistentVolumes and VolumeSnapshotContents refer to (see
[What the sweep does](#what-the-sweep-does)). Keep the backfill window short, or tag the resources yourself.

## Isolation covers the whole account

A cluster's resources are protected only when every role for this driver in the account uses the tag-based policy. A
cluster whose controller uses the shared policy has IAM permissions to modify every resource this driver created.

## New clusters

Every resource is tagged when it is created, so no backfill is needed.

1. Create a role with the [EKS Pod Identity trust policy](https://docs.aws.amazon.com/eks/latest/userguide/pod-id-role.html),
   attach the tag-based policy, and associate the role with the controller's ServiceAccount:

   ```sh
   aws iam put-role-policy --role-name FSxOpenZFSCSIDriverRole --policy-name cluster-tag \
       --policy-document file://docs/example-iam-policy-cluster-tag.json
   aws eks create-pod-identity-association --cluster-name my-cluster --namespace kube-system \
       --service-account fsx-openzfs-csi-controller-sa \
       --role-arn arn:aws:iam::111122223333:role/FSxOpenZFSCSIDriverRole
   ```

2. Install the driver with the cluster name, the key and `enforce`:

   ```sh
   helm upgrade --install aws-fsx-openzfs-csi-driver aws-fsx-openzfs-csi-driver/aws-fsx-openzfs-csi-driver \
       --namespace kube-system \
       --set controller.clusterName=my-cluster \
       --set controller.clusterTagKey=eks:eks-cluster-name \
       --set controller.clusterTagIsolation=enforce
   ```

   Without Helm, pass the same values as controller flags:
   `--cluster-name=my-cluster --cluster-tag-key=eks:eks-cluster-name --cluster-tag-isolation=enforce`.

## Existing clusters

`--cluster-tag-isolation` must match the IAM policy attached to the controller. The controller can't check this.

| State | IAM attached | Controller behavior |
|---|---|---|
| `disable` (default) | The shared policy | No isolation. Adds the cluster tag to new resources when `--cluster-name` and `--cluster-tag-key` are set. Sweeps, adding the driver tag to the root volumes of file systems it created. |
| `backfill` | The shared policy, the tag-based policy and the backfill policy | Requires the cluster name and key. Sweeps, adding the cluster tag to existing resources it created for this cluster. Does not filter. |
| `enforce` | The tag-based policy (after the rollout below) | Requires the cluster name and key. Refuses resources that lack this cluster's tag value. Does not sweep. |

The value is a string and must be exactly `disable`, `backfill` or `enforce`; an empty value means `disable`. The chart
and the controller reject any other value. At startup the controller logs one line, `cluster tag isolation settings`,
with `clusterName`, `tagKey`, `clusterTagIsolation` and, except in `enforce`, `backfillInterval`. In `disable` with
only one of `--cluster-name` and `--cluster-tag-key` set, it also logs
`only one of --cluster-name and --cluster-tag-key is set; no cluster tag is written`.

In `enforce`, the controller reads the tags of the target before it deletes, expands or snapshots a resource, deletes
a snapshot, or creates a volume from a snapshot. If the target lacks this cluster's tag value, the request fails with
`FailedPrecondition` and the controller doesn't act on it. Each refused resource is logged at info level as
`ignoring resource not tagged for this cluster`, with `resource`, `tagKey`, `clusterName` and `tagValue` (the value
found, empty when there is none). If the tags can't be read, the controller logs `failed to read cluster tag,
continuing` and IAM decides.

Roll out in this order. Keep the policy the controller uses today, the shared policy or `AmazonFSxFullAccess`, attached
until `enforce` is set: the tag-based policy allows nothing on resources without this
cluster's tag, and it needs the cluster name set before the controller can create anything. IAM allows an action when
any attached policy allows it, so adding policies first changes nothing.

1. Tag the resources the driver didn't create with this cluster's tag: parent volumes you created (StorageClass
   `ParentVolumeId`), and the file systems and volumes of static PersistentVolumes. The sweep lists every one it finds,
   so you can also do this in step 4.
2. Attach the tag-based policy and the backfill policy to the controller's role, **alongside** the current policy.
3. Set `backfill`. From now on, new resources get the cluster tag too:

   ```sh
   helm upgrade aws-fsx-openzfs-csi-driver aws-fsx-openzfs-csi-driver/aws-fsx-openzfs-csi-driver \
       --namespace kube-system --reuse-values \
       --set controller.clusterName=my-cluster \
       --set controller.clusterTagKey=eks:eks-cluster-name \
       --set controller.clusterTagIsolation=backfill
   ```

4. Wait until every controller replica logs `cluster tag backfill complete`, and tag every resource listed in
   `needsAdministratorResources` on the latest `cluster tag backfill sweep finished` line. See
   [Confirm backfill is complete](#confirm-backfill-is-complete).
5. Set `enforce`:

   ```sh
   helm upgrade aws-fsx-openzfs-csi-driver aws-fsx-openzfs-csi-driver/aws-fsx-openzfs-csi-driver \
       --namespace kube-system --reuse-values \
       --set controller.clusterTagIsolation=enforce
   ```

6. Detach the current policy and the backfill policy. Only the tag-based policy stays, and isolation starts here.

Changing `--cluster-tag-key` later, for example to use your own policy, means backfilling again with the new key.

### What the sweep does

In `disable` and `backfill` the controller sweeps at startup and then every hour; in `enforce` it doesn't. The sweep
adds one tag: the driver tag in `disable`, and the cluster tag in `backfill`. It reads this driver's PersistentVolumes
and VolumeSnapshotContents and the `ParentVolumeId` of its StorageClasses, and visits the FSx resources they refer to,
the root volume of each file system that carries the driver tag, and the parent volume of each volume.

- A resource that already has the tag is left alone.
- A resource the controller created for this cluster gets the tag, in one `TagResource` request that contains only
  that tag. The controller created it when it carries the driver tag and either:
  - its PersistentVolume has the annotation `pv.kubernetes.io/provisioned-by: fsx.openzfs.csi.aws.com` and, for a
    volume, the volume's name is the PersistentVolume's name; or
  - its VolumeSnapshotContent was created dynamically (`spec.source.volumeHandle` is set).

  The root volume of such a file system also counts. In `backfill` it must carry the driver tag as well, because the
  backfill policy requires it, so let the `disable` sweep tag it first or set `CopyTagsToVolumes: 'true'`.
- Any other resource without the tag is not changed, and is reported for an administrator: static PersistentVolumes
  and VolumeSnapshotContents, parent volumes you created, root volumes of file systems the controller didn't create,
  resources tagged for another cluster, and resources the controller can't find (deleted, or not visible to its
  credentials).

A resource tagged for another cluster blocks `cluster tag backfill complete` in this cluster for good. Don't re-tag it:
decide which cluster owns it, or stop referring to it from this cluster. A PersistentVolume whose resource was deleted
also blocks completion until you delete the PersistentVolume.

Each controller replica runs its own sweep, so each line can appear once per replica. Each sweep logs exactly one info
line, `cluster tag backfill sweep finished`, with these fields:

| Field | Meaning |
|---|---|
| `tagged` | `TagResource` calls that succeeded in this sweep |
| `failed` | `TagResource` calls that failed in this sweep |
| `needsAdministrator` | Distinct resources this cluster refers to that an administrator must tag |
| `remaining` | Resources, or Kubernetes listings (1 each), the sweep couldn't read |
| `needsAdministratorResources` | The first 20 ARNs counted in `needsAdministrator` (the ID for a resource that wasn't found), with `needsAdministratorTotal` when there are more. Omitted when empty |
| `failedResources` | The first 20 ARNs counted in `failed`, with `failedTotal` when there are more. Omitted when empty |
| `firstError` | The first error of the sweep. Only present when `failed` or `remaining` is above zero |

`cluster tag backfill complete`, with `tagKey` and `tagValue` (the tag the sweep adds), is logged once per controller
process, after the first sweep that ends with `failed`, `needsAdministrator` and `remaining` all zero. Later sweeps
still log their summary line, and never log `complete` again; a restarted replica logs it again after its own clean
sweep.

At verbosity 4 (`controller.logLevel: 4`) the controller also logs each resource, with `resource` and `tagKey`:
`added cluster tag to existing resource`, `failed to add cluster tag to existing resource` (also with `err`),
`resource needs cluster tag from an administrator` (also with `currentValue`), `cluster tag backfill: resource not
found`, and `cluster tag backfill could not read resource` and `cluster tag backfill could not list Kubernetes objects`
(both with `err`, counted in `remaining`).

### Confirm backfill is complete

Read the completion and summary lines of every replica. `kubectl logs -l` prints only the last 10 lines of each pod
unless you pass `--tail=-1`:

```sh
kubectl -n kube-system logs -l app.kubernetes.io/name=fsx-openzfs-csi-controller -c fsx-openzfs-plugin \
    --prefix --tail=-1 | grep -E 'cluster tag backfill (complete|sweep finished)'
```

These lines come from a test rollout. They were captured with an earlier log format and are shown here in the current
one:

```
I1021 22:08:08.778116       1 clustertag.go:91] "cluster tag isolation settings" clusterName="my-cluster" tagKey="eks:eks-cluster-name" clusterTagIsolation="backfill" backfillInterval="1h0m0s"
I1021 22:08:09.222419       1 backfill.go:368] "cluster tag backfill sweep finished" tagged=1 failed=0 needsAdministrator=0 remaining=0
I1021 22:08:09.222534       1 backfill.go:372] "cluster tag backfill complete" tagKey="eks:eks-cluster-name" tagValue="my-cluster"
```

If the backfill policy is missing, each sweep shows the failure and never completes:

```
I1021 22:13:52.622439       1 backfill.go:368] "cluster tag backfill sweep finished" tagged=0 failed=1 needsAdministrator=0 remaining=0 failedResources=["arn:aws:fsx:us-west-2:111122223333:volume/fs-0ae418ec0e347df6a/fsvol-085fe65a8a9a7f88b"] firstError="operation error FSx: TagResource, https response error StatusCode: 400, api error AccessDeniedException: User: arn:aws:sts::111122223333:assumed-role/FSxOpenZFSCSIDriverRole/eks-my-cluster-fsx-openzf-... is not authorized to perform: fsx:TagResource on resource: arn:aws:fsx:us-west-2:111122223333:volume/fs-0ae418ec0e347df6a/fsvol-085fe65a8a9a7f88b because no identity-based policy allows the fsx:TagResource action"
```

To check without logs, list every resource this cluster refers to that lacks the tag. Set `TAG_KEY` to the tag the
sweep adds (`--cluster-tag-key` in `backfill`, `fsx.openzfs.csi.aws.com/cluster` in `disable`) and `CLUSTER` to its
value (the cluster name, or `true`):

```sh
TAG_KEY=eks:eks-cluster-name
CLUSTER=my-cluster
```

The command prints each resource without `$TAG_KEY=$CLUSTER`, with the value found (`None` when there is none). Empty
output means everything is tagged; `describe failed` means a call failed, for example because the resource was
deleted. It describes one ID at a time, because one missing ID fails a batched describe call. It doesn't check root
volumes: run it again with the `RootVolumeId` of each file system it lists
(`aws fsx describe-file-systems --file-system-ids <ID> --query 'FileSystems[0].OpenZFSConfiguration.RootVolumeId' --output text`).

```sh
{
  kubectl get pv -o jsonpath='{range .items[?(@.spec.csi.driver=="fsx.openzfs.csi.aws.com")]}{.spec.csi.volumeHandle}{"\n"}{end}'
  kubectl get volumesnapshotcontent -o jsonpath='{range .items[?(@.spec.driver=="fsx.openzfs.csi.aws.com")]}{.status.snapshotHandle}{"\n"}{end}'
  kubectl get storageclass -o jsonpath='{range .items[?(@.provisioner=="fsx.openzfs.csi.aws.com")]}{.parameters.ParentVolumeId}{"\n"}{end}' | tr -d '"'
} | sort -u | while read -r id; do
  [ -n "$id" ] || continue
  case "$id" in
    fs-*) arn=$(aws fsx describe-file-systems --file-system-ids "$id" --query 'FileSystems[0].ResourceARN' --output text) ;;
    fsvol-*) arn=$(aws fsx describe-volumes --volume-ids "$id" --query 'Volumes[0].ResourceARN' --output text) ;;
    *) arn=$(aws fsx describe-snapshots --snapshot-ids "$id" --query 'Snapshots[0].ResourceARN' --output text) ;;
  esac &&
    aws fsx list-tags-for-resource --resource-arn "$arn" --output text \
      --query "[@][?!(Tags[?Key=='$TAG_KEY' && Value=='$CLUSTER'])].['$id', Tags[?Key=='$TAG_KEY']|[0].Value]" ||
    printf '%s\tdescribe failed\n' "$id"
done
```

For each resource listed, check that it belongs to this cluster, then tag it. Several resources can be tagged at once:

```sh
aws resourcegroupstaggingapi tag-resources --tags "$TAG_KEY=$CLUSTER" \
    --resource-arn-list arn:aws:fsx:us-west-2:111122223333:volume/fs-0123456789abcdef0/fsvol-0123456789abcdef2
```

## What each mismatch does

| Setting | Policy attached | Result |
|---|---|---|
| `backfill` | Shared policy, without the backfill policy | Every `TagResource` call fails: each summary line counts them in `failed`, lists them in `failedResources`, and shows `AccessDeniedException` in `firstError`. `cluster tag backfill complete` never appears. |
| `enforce` | Backfill policy still attached | No effect on the controller, which doesn't sweep in `enforce`. Detach the policy. |
| `backfill` or `enforce` | Shared policy still attached | Everything works, but IAM doesn't isolate: in `enforce` the controller refuses other clusters' resources, but its role could still change them. Detach the shared policy after `enforce`. |
| `disable` | Tag-based policy alone, before the cluster name is set | Creates fail with `AccessDeniedException`, because the request has no cluster tag. This is what detaching the shared policy too early does; reattach it and follow the steps above. |
| `disable`, cluster name and key set | Shared policy | Works. The cluster tag is added to new resources, which the shared policy allows. |
| Any | Tag-based policy, with IRSA or other credentials without Pod Identity session tags | Every FSx call is denied. |
| `enforce` | Tag-based policy, `--cluster-tag-key` not `eks:eks-cluster-name` | Creates are denied, because the policy requires `eks:eks-cluster-name` in the request. Use the key of the policy. |
| `enforce` | Tag-based policy, `--cluster-name` not the EKS cluster name | Creates are denied, because the tag value must equal the `eks-cluster-name` session tag. |
| `enforce` | Tag-based policy, parent or root volume without this cluster's tag | Volumes in that parent and snapshots of that file system fail with `AccessDeniedException`. Tag the parent or root volume. |

## Settings

The controller is configured with flags only; it reads no environment variables for these settings.

| Flag | Helm value | Default | Description |
|---|---|---|---|
| `--cluster-name` | `controller.clusterName` | empty | Name of the cluster, the value of the cluster tag. |
| `--cluster-tag-key` | `controller.clusterTagKey` | empty | Key of the cluster tag. Empty writes no cluster tag. Use `eks:eks-cluster-name` with the tag-based policy. Must not start with `aws:`, be the driver tag, or end with `OnDeletion`. |
| `--cluster-tag-isolation` | `controller.clusterTagIsolation` | `disable` | `disable`, `backfill` or `enforce`, as described above. `backfill` and `enforce` fail at startup, and at `helm install`, without both `--cluster-name` and `--cluster-tag-key`; whitespace-only values count as empty. |
| `--extra-tags` | `controller.extraTags` (map) | empty | Tags (`k1=v1,k2=v2`) added to every AWS resource the controller creates. Not used to find, filter or isolate resources, and not the cluster tag. StorageClass and VolumeSnapshotClass `Tags` override them for the same key. They can't set the cluster tag key, the driver tag, `aws:` keys or keys ending in `OnDeletion`; such a value fails at startup. |

StorageClass and VolumeSnapshotClass `Tags`, and `FinalBackupTagsOnDeletion`, must not set the driver tag or the
cluster tag key. Such a request fails with `InvalidArgument`.
