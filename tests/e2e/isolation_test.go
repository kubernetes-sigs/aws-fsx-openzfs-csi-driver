/*
Copyright 2026 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/fsx"
	fsxtypes "github.com/aws/aws-sdk-go-v2/service/fsx/types"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/kubernetes/test/e2e/framework"
	e2epv "k8s.io/kubernetes/test/e2e/framework/pv"

	"github.com/kubernetes-sigs/aws-fsx-openzfs-csi-driver/pkg/driver"
)

var controllerRoleName = flag.String("controller-role-name", "", "IAM role the controller uses through EKS Pod Identity; the isolation tests are skipped when empty")

const (
	// The tag key the Pod Identity policies check.
	isolationTagKey = "eks:eks-cluster-name"
	// The cluster name the other cluster's resources are tagged with.
	otherClusterName = "fsx-openzfs-e2e-other-cluster"

	controllerNamespace  = "kube-system"
	controllerDeployment = "fsx-openzfs-csi-controller"
	controllerContainer  = "fsx-openzfs-plugin"
	controllerSA         = "fsx-openzfs-csi-controller-sa"

	fsCreateTimeout  = 30 * time.Minute
	volCreateTimeout = 10 * time.Minute
	isolationPoll    = 15 * time.Second
	// Time for IAM to apply a changed inline policy.
	iamPropagation = 30 * time.Second
	// Name prefix of the volumes the IAM probes try to create; AfterAll deletes any that were created.
	probeVolumeName = "isolationprobe"
)

var (
	volumeSnapshotGVR      = schema.GroupVersionResource{Group: "snapshot.storage.k8s.io", Version: "v1", Resource: "volumesnapshots"}
	volumeSnapshotClassGVR = schema.GroupVersionResource{Group: "snapshot.storage.k8s.io", Version: "v1", Resource: "volumesnapshotclasses"}
	volumeSnapshotContGVR  = schema.GroupVersionResource{Group: "snapshot.storage.k8s.io", Version: "v1", Resource: "volumesnapshotcontents"}
)

// The tests walk one cluster through the rollout in docs/cluster-tag-isolation.md, with
// the docs/example-iam-policy*.json policies swapped on the controller's Pod Identity role:
//
//  1. disable with the broad policy: resources get the driver tag but no cluster tag, and
//     the sweep adds the driver tag to the root volume of the file system;
//  2. disable with the shared policy: volumes and snapshots in driver-tagged parents work,
//     and IAM denies untagged parents, untagged file systems and tagging after create;
//  3. backfill with the shared, tag-based and backfill policies: existing resources get
//     the cluster tag;
//  4. enforce with the tag-based policy only: new volumes, snapshots and restores carry
//     the cluster tag;
//  5. the controller refuses another cluster's volume, and IAM denies the controller's
//     role on another cluster's parent, snapshot and volume;
//  6. this cluster's resources are deleted.
//
// The suite's file system (BeforeSuite) stands in for a file system another cluster's
// administrator created: neither it nor its root volume has the driver tag. A child volume
// and a snapshot in it are tagged for another cluster. The specs depend on each other.
var _ = Describe("[fsx-openzfs-isolation] Cluster tag isolation", Ordered, func() {
	var (
		ctx  = context.Background()
		cs   clientset.Interface
		dyn  dynamic.Interface
		c    *Cloud
		ns   string
		vsc  string
		scs  []string
		pvcs []string

		legacyFS, legacyRoot   string
		legacyVol, legacySnap  string
		legacyVolPVC           string
		legacySnapName         string
		foreignRoot            string
		foreignVol             string
		foreignSnap            string
		foreignSnapARN         string
		volSC, restoreSC       string
		enforcedVol, restorVol string
		enforcedSnap           string
		foreignPV              string
	)

	policy := func(name string) string {
		return filepath.Join(framework.TestContext.RepoRoot, "docs", name)
	}
	setPolicies := func(files ...string) {
		args := append([]string{"set-policies", *controllerRoleName}, files...)
		runScript("hack/e2e/pod-identity.sh", args...)
		time.Sleep(iamPropagation)
	}
	setIsolation := func(isolation string, tagged bool) {
		name, key := "", ""
		if tagged {
			name, key = ClusterName, isolationTagKey
		}
		runScript("hack/e2e/helm-upgrade.sh",
			"--set", "controller.clusterName="+name,
			"--set", "controller.clusterTagKey="+key,
			"--set", "controller.clusterTagIsolation="+isolation)
	}
	e2eTags := fmt.Sprintf(`[{"Key":"%s","Value":"%s"}]`, CSIDriverE2ETagKey, PullRequest)
	createSC := func(name string, params map[string]string) string {
		reclaim := v1.PersistentVolumeReclaimDelete
		sc, err := cs.StorageV1().StorageClasses().Create(ctx, &storagev1.StorageClass{
			ObjectMeta:    metav1.ObjectMeta{Name: ns + "-" + name},
			Provisioner:   driver.DriverName,
			Parameters:    params,
			ReclaimPolicy: &reclaim,
		}, metav1.CreateOptions{})
		framework.ExpectNoError(err)
		scs = append(scs, sc.Name)
		return sc.Name
	}
	createPVC := func(name, sc, size string, source *v1.TypedLocalObjectReference) string {
		pvc, err := cs.CoreV1().PersistentVolumeClaims(ns).Create(ctx, &v1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{Name: name},
			Spec: v1.PersistentVolumeClaimSpec{
				AccessModes:      []v1.PersistentVolumeAccessMode{v1.ReadWriteMany},
				StorageClassName: &sc,
				DataSource:       source,
				Resources:        v1.VolumeResourceRequirements{Requests: v1.ResourceList{v1.ResourceStorage: resource.MustParse(size)}},
			},
		}, metav1.CreateOptions{})
		framework.ExpectNoError(err)
		pvcs = append(pvcs, pvc.Name)
		return pvc.Name
	}
	tags := func(id string) map[string]string {
		t, err := c.Tags(ctx, id)
		framework.ExpectNoError(err)
		return t
	}

	BeforeAll(func() {
		if *controllerRoleName == "" {
			Skip("--controller-role-name is not set")
		}
		var err error
		cs, err = framework.LoadClientset()
		framework.ExpectNoError(err)
		config, err := framework.LoadConfig()
		framework.ExpectNoError(err)
		dyn, err = dynamic.NewForConfig(config)
		framework.ExpectNoError(err)
		c = NewCloud(Region)

		nsObj, err := cs.CoreV1().Namespaces().Create(ctx, &v1.Namespace{
			ObjectMeta: metav1.ObjectMeta{GenerateName: "fsx-openzfs-isolation-"},
		}, metav1.CreateOptions{})
		framework.ExpectNoError(err)
		ns = nsObj.Name

		class := &unstructured.Unstructured{Object: map[string]interface{}{
			"apiVersion":     "snapshot.storage.k8s.io/v1",
			"kind":           "VolumeSnapshotClass",
			"metadata":       map[string]interface{}{"name": ns},
			"driver":         driver.DriverName,
			"deletionPolicy": "Delete",
			"parameters":     map[string]interface{}{"Tags": e2eTags},
		}}
		_, err = dyn.Resource(volumeSnapshotClassGVR).Create(ctx, class, metav1.CreateOptions{})
		framework.ExpectNoError(err)
		vsc = ns

		// Another cluster's volume and snapshot, inside a file system the driver didn't create.
		foreignRoot = *filesystem.OpenZFSConfiguration.RootVolumeId
		otherTags := []fsxtypes.Tag{
			{Key: aws.String(driver.AwsFsxOpenZfsDriverTagKey), Value: aws.String("true")},
			{Key: aws.String(isolationTagKey), Value: aws.String(otherClusterName)},
			{Key: aws.String(CSIDriverE2ETagKey), Value: aws.String(PullRequest)},
		}
		input := getDefaultCreateVolumeInput(foreignRoot)
		input.Name = aws.String(strings.ReplaceAll(ns, "-", "_"))
		input.Tags = otherTags
		vol, err := c.CreateVolume(ctx, input)
		framework.ExpectNoError(err)
		foreignVol = *vol.VolumeId
		framework.ExpectNoError(c.WaitForVolumeAvailable(ctx, foreignVol))
		snap, err := c.FSx.CreateSnapshot(ctx, &fsx.CreateSnapshotInput{Name: aws.String(ns), VolumeId: &foreignVol, Tags: otherTags})
		framework.ExpectNoError(err)
		foreignSnap, foreignSnapARN = *snap.Snapshot.SnapshotId, *snap.Snapshot.ResourceARN
		framework.ExpectNoError(c.WaitForSnapshotAvailable(ctx, foreignSnap))
	})

	AfterAll(func() {
		if cs == nil {
			return
		}
		var errs []error
		// Leave the controller as eksctl.sh installed it.
		runScript("hack/e2e/pod-identity.sh", "set-policies", *controllerRoleName, policy("example-iam-policy.json"))
		runScript("hack/e2e/helm-upgrade.sh", "--set", "controller.clusterName=", "--set", "controller.clusterTagKey=", "--set", "controller.clusterTagIsolation=disable")

		if foreignPV != "" {
			if err := cs.CoreV1().PersistentVolumes().Delete(ctx, foreignPV, metav1.DeleteOptions{}); err != nil && !strings.Contains(err.Error(), "not found") {
				errs = append(errs, err)
			}
		}
		if err := cs.CoreV1().Namespaces().Delete(ctx, ns, metav1.DeleteOptions{}); err != nil {
			errs = append(errs, err)
		}
		for _, sc := range scs {
			if err := cs.StorageV1().StorageClasses().Delete(ctx, sc, metav1.DeleteOptions{}); err != nil {
				errs = append(errs, err)
			}
		}
		if vsc != "" {
			if err := dyn.Resource(volumeSnapshotClassGVR).Delete(ctx, vsc, metav1.DeleteOptions{}); err != nil {
				errs = append(errs, err)
			}
		}
		// Snapshots and volumes a probe created by mistake, then the other cluster's resources.
		if foreignVol != "" {
			snaps, err := c.FSx.DescribeSnapshots(ctx, &fsx.DescribeSnapshotsInput{Filters: []fsxtypes.SnapshotFilter{
				{Name: fsxtypes.SnapshotFilterNameVolumeId, Values: []string{foreignVol}},
			}})
			if err != nil {
				errs = append(errs, err)
			} else {
				for _, s := range snaps.Snapshots {
					if strings.HasPrefix(aws.ToString(s.Name), probeVolumeName) {
						errs = append(errs, fmt.Errorf("an IAM probe created snapshot %s", *s.SnapshotId))
						if _, err := c.FSx.DeleteSnapshot(ctx, &fsx.DeleteSnapshotInput{SnapshotId: s.SnapshotId}); err != nil {
							errs = append(errs, err)
						} else if err := c.WaitForSnapshotDeleted(ctx, *s.SnapshotId); err != nil {
							errs = append(errs, err)
						}
					}
				}
			}
		}
		fsIds := []string{*filesystem.FileSystemId}
		if legacyFS != "" {
			fsIds = append(fsIds, legacyFS)
		}
		probes, err := c.FSx.DescribeVolumes(ctx, &fsx.DescribeVolumesInput{Filters: []fsxtypes.VolumeFilter{
			{Name: fsxtypes.VolumeFilterNameFileSystemId, Values: fsIds},
		}})
		if err != nil {
			errs = append(errs, err)
		} else {
			for _, v := range probes.Volumes {
				if strings.HasPrefix(aws.ToString(v.Name), probeVolumeName) {
					errs = append(errs, fmt.Errorf("an IAM probe created volume %s", *v.VolumeId))
					if err := c.DeleteVolume(ctx, fsx.DeleteVolumeInput{VolumeId: v.VolumeId}); err != nil {
						errs = append(errs, err)
					}
				}
			}
		}
		if foreignSnap != "" {
			if _, err := c.FSx.DeleteSnapshot(ctx, &fsx.DeleteSnapshotInput{SnapshotId: &foreignSnap}); err != nil {
				errs = append(errs, err)
			} else if err := c.WaitForSnapshotDeleted(ctx, foreignSnap); err != nil {
				errs = append(errs, err)
			}
		}
		if foreignVol != "" {
			if err := c.DeleteVolume(ctx, fsx.DeleteVolumeInput{VolumeId: &foreignVol}); err != nil {
				errs = append(errs, err)
			}
		}
		framework.ExpectNoError(errors.Join(errs...))
	})

	It("tags new resources with the driver tag and sweeps root volumes in disable", func() {
		setPolicies(policy("example-iam-policy.json"))
		setIsolation("disable", false)

		subnets, _ := json.Marshal(subnetIds)
		groups, _ := json.Marshal(securityGroupIds)
		fsParams := getDefaultFilesystemParameters(string(subnets), string(groups))
		fsSC := createSC("fs", fsParams)
		fsPVC := createPVC("legacy-fs", fsSC, "64Gi", nil)
		legacyFS = waitForBoundHandle(ctx, cs, ns, fsPVC, fsCreateTimeout)

		fs, err := c.DescribeFileSystem(ctx, legacyFS)
		framework.ExpectNoError(err)
		legacyRoot = *fs.OpenZFSConfiguration.RootVolumeId
		Expect(tags(legacyFS)).To(HaveKeyWithValue(driver.AwsFsxOpenZfsDriverTagKey, "true"))
		Expect(tags(legacyFS)).NotTo(HaveKey(isolationTagKey))
		// FSx creates the root volume without tags; the controller's sweep adds the driver tag.
		Expect(tags(legacyRoot)).NotTo(HaveKey(driver.AwsFsxOpenZfsDriverTagKey))
		restartController(ctx, cs)
		Eventually(func() map[string]string { return tags(legacyRoot) }).
			WithTimeout(10 * time.Minute).WithPolling(isolationPoll).Should(HaveKeyWithValue(driver.AwsFsxOpenZfsDriverTagKey, "true"))

		volSC = createSC("vol", map[string]string{"ResourceType": "volume", "ParentVolumeId": strconv.Quote(legacyRoot), "Tags": e2eTags})
		restoreSC = createSC("restore", map[string]string{
			"ResourceType": "volume", "ParentVolumeId": strconv.Quote(legacyRoot),
			"OriginSnapshot": `{"CopyStrategy": "CLONE"}`, "Tags": e2eTags,
		})
		legacyVolPVC = createPVC("legacy-vol", volSC, "1Gi", nil)
		legacyVol = waitForBoundHandle(ctx, cs, ns, legacyVolPVC, volCreateTimeout)
		Expect(tags(legacyVol)).To(HaveKeyWithValue(driver.AwsFsxOpenZfsDriverTagKey, "true"))
		Expect(tags(legacyVol)).NotTo(HaveKey(isolationTagKey))

		legacySnapName = "legacy-snap"
		legacySnap = createSnapshot(ctx, dyn, ns, legacySnapName, vsc, legacyVolPVC)
		Expect(tags(legacySnap)).To(HaveKeyWithValue(driver.AwsFsxOpenZfsDriverTagKey, "true"))
	})

	It("works with the shared policy on driver resources and denies the rest", func() {
		setPolicies(policy("example-iam-policy-shared.json"))

		// Positive, through the controller: a volume in the driver-tagged root volume, a
		// snapshot of it, and deleting both.
		pvc := createPVC("shared-vol", volSC, "1Gi", nil)
		waitForBoundHandle(ctx, cs, ns, pvc, volCreateTimeout)
		snap := createSnapshot(ctx, dyn, ns, "shared-snap", vsc, pvc)
		deleteSnapshot(ctx, dyn, c, ns, "shared-snap", snap)
		deletePVC(ctx, cs, ns, pvc)

		// Negative, in IAM. A call IAM wrongly allows fails anyway: the file system and the
		// volume have children, and AfterAll deletes any probe volume.
		out := runWithControllerRole(ctx, cs, fmt.Sprintf(`set -u
aws fsx describe-volumes --volume-ids %[1]s --query 'Volumes[0].VolumeId' --output text && echo DESCRIBE_OK
aws fsx create-volume --volume-type OPENZFS --name %[2]s1 --open-zfs-configuration ParentVolumeId=%[3]s \
  --tags Key=%[4]s,Value=true 2>&1 | grep -q AccessDenied && echo UNTAGGED_PARENT_DENIED
aws fsx delete-file-system --file-system-id %[5]s 2>&1 | grep -q AccessDenied && echo UNTAGGED_FS_DELETE_DENIED
aws fsx tag-resource --resource-arn %[6]s --tags Key=probe,Value=x 2>&1 | grep -q AccessDenied && echo TAG_AFTER_CREATE_DENIED
true`, legacyVol, probeVolumeName, foreignRoot, driver.AwsFsxOpenZfsDriverTagKey, *filesystem.FileSystemId, arnOf(ctx, c, legacyVol)))
		Expect(out).To(ContainSubstring("DESCRIBE_OK"))
		Expect(out).To(ContainSubstring("UNTAGGED_PARENT_DENIED"))
		Expect(out).To(ContainSubstring("UNTAGGED_FS_DELETE_DENIED"))
		Expect(out).To(ContainSubstring("TAG_AFTER_CREATE_DENIED"))
	})

	It("backfills the cluster tag on existing resources", func() {
		// As in docs/cluster-tag-isolation.md: add the tag-based policies alongside the
		// broad one, which stays attached until enforce is set.
		setPolicies(policy("example-iam-policy-shared.json"), policy("example-iam-policy-cluster-tag.json"), policy("example-iam-policy-cluster-tag-backfill.json"))
		setIsolation("backfill", true)

		// The controller sweeps at startup and then hourly. Restart it if the first sweep
		// ran before IAM applied the new policies.
		start := time.Now()
		Eventually(func(g Gomega) {
			if time.Since(start) > 5*time.Minute {
				start = time.Now()
				restartController(ctx, cs)
			}
			for _, id := range []string{legacyFS, legacyRoot, legacyVol, legacySnap} {
				g.Expect(tags(id)).To(HaveKeyWithValue(isolationTagKey, ClusterName), id)
			}
		}).WithTimeout(20 * time.Minute).WithPolling(isolationPoll).Should(Succeed())

		Eventually(func() string { return controllerLogs(ctx, cs) }).
			WithTimeout(5 * time.Minute).WithPolling(isolationPoll).Should(ContainSubstring("cluster tag backfill complete"))
	})

	It("tags new volumes, snapshots and restores with the cluster tag in enforce", func() {
		setIsolation("enforce", true)
		// Then detach the shared and backfill policies. The remaining specs run with only
		// the tag-based policy.
		setPolicies(policy("example-iam-policy-cluster-tag.json"))

		pvc := createPVC("enforced-vol", volSC, "1Gi", nil)
		enforcedVol = waitForBoundHandle(ctx, cs, ns, pvc, volCreateTimeout)
		Expect(tags(enforcedVol)).To(HaveKeyWithValue(isolationTagKey, ClusterName))

		enforcedSnap = createSnapshot(ctx, dyn, ns, "enforced-snap", vsc, pvc)
		Expect(tags(enforcedSnap)).To(HaveKeyWithValue(isolationTagKey, ClusterName))

		apiGroup := "snapshot.storage.k8s.io"
		restored := createPVC("restored-vol", restoreSC, "1Gi", &v1.TypedLocalObjectReference{APIGroup: &apiGroup, Kind: "VolumeSnapshot", Name: "enforced-snap"})
		restorVol = waitForBoundHandle(ctx, cs, ns, restored, volCreateTimeout)
		Expect(tags(restorVol)).To(HaveKeyWithValue(isolationTagKey, ClusterName))
	})

	It("refuses to delete another cluster's volume in enforce", func() {
		pv, pvc := createStaticVolume(ctx, cs, c, ns, "foreign", foreignVol)
		foreignPV = pv
		framework.ExpectNoError(cs.CoreV1().PersistentVolumeClaims(ns).Delete(ctx, pvc, metav1.DeleteOptions{}))
		Eventually(func() string { return volumeFailedDeleteMessage(ctx, cs, pv) }).
			WithTimeout(5 * time.Minute).WithPolling(isolationPoll).Should(ContainSubstring("FailedPrecondition"))
		Expect(tags(foreignVol)).To(HaveKeyWithValue(isolationTagKey, otherClusterName))

		// Keep the volume, and let AfterAll delete it.
		_, err := cs.CoreV1().PersistentVolumes().Patch(ctx, pv, types.MergePatchType,
			[]byte(`{"spec":{"persistentVolumeReclaimPolicy":"Retain"}}`), metav1.PatchOptions{})
		framework.ExpectNoError(err)
	})

	It("denies the controller role another cluster's resources in IAM", func() {
		// Calls FSx directly with the controller's role, bypassing the controller's own
		// check. A call IAM wrongly allows fails anyway: the volume has a snapshot, and
		// AfterAll deletes any probe volume.
		out := runWithControllerRole(ctx, cs, fmt.Sprintf(`set -u
own="Key=%[1]s,Value=%[2]s Key=%[3]s,Value=true"
aws fsx describe-volumes --volume-ids %[4]s --query 'Volumes[0].VolumeId' --output text && echo DESCRIBE_OK
aws fsx create-volume --volume-type OPENZFS --name %[5]s2 --open-zfs-configuration ParentVolumeId=%[6]s --tags $own \
  2>&1 | grep -q AccessDenied && echo OTHER_PARENT_DENIED
aws fsx create-volume --volume-type OPENZFS --name %[5]s3 --open-zfs-configuration ParentVolumeId=%[7]s --tags $own \
  2>&1 | grep -q AccessDenied && echo UNTAGGED_PARENT_DENIED
aws fsx create-volume --volume-type OPENZFS --name %[5]s4 \
  --open-zfs-configuration "ParentVolumeId=%[8]s,OriginSnapshot={SnapshotARN=%[9]s,CopyStrategy=CLONE}" --tags $own \
  2>&1 | grep -q AccessDenied && echo OTHER_SNAPSHOT_DENIED
aws fsx create-volume --volume-type OPENZFS --name %[5]s5 --open-zfs-configuration ParentVolumeId=%[8]s \
  --tags Key=%[1]s,Value=%[10]s Key=%[3]s,Value=true 2>&1 | grep -q AccessDenied && echo OTHER_REQUEST_TAG_DENIED
aws fsx create-snapshot --name %[5]s6 --volume-id %[6]s --tags $own 2>&1 | grep -q AccessDenied && echo OTHER_SNAPSHOT_SOURCE_DENIED
aws fsx delete-volume --volume-id %[6]s 2>&1 | grep -q AccessDenied && echo OTHER_VOLUME_DELETE_DENIED
aws fsx tag-resource --resource-arn %[11]s --tags Key=probe,Value=x 2>&1 | grep -q AccessDenied && echo TAG_AFTER_CREATE_DENIED
true`, isolationTagKey, ClusterName, driver.AwsFsxOpenZfsDriverTagKey, enforcedVol, probeVolumeName, foreignVol, foreignRoot,
			legacyRoot, foreignSnapARN, otherClusterName, arnOf(ctx, c, enforcedVol)))
		for _, want := range []string{"DESCRIBE_OK", "OTHER_PARENT_DENIED", "UNTAGGED_PARENT_DENIED", "OTHER_SNAPSHOT_DENIED",
			"OTHER_REQUEST_TAG_DENIED", "OTHER_SNAPSHOT_SOURCE_DENIED", "OTHER_VOLUME_DELETE_DENIED", "TAG_AFTER_CREATE_DENIED"} {
			Expect(out).To(ContainSubstring(want))
		}
		Expect(tags(foreignVol)).To(HaveKeyWithValue(isolationTagKey, otherClusterName))
	})

	It("deletes this cluster's resources in enforce", func() {
		deletePVC(ctx, cs, ns, "restored-vol")
		deleteSnapshot(ctx, dyn, c, ns, "enforced-snap", enforcedSnap)
		deletePVC(ctx, cs, ns, "enforced-vol")
		deleteSnapshot(ctx, dyn, c, ns, legacySnapName, legacySnap)
		deletePVC(ctx, cs, ns, legacyVolPVC)
		deletePVC(ctx, cs, ns, "legacy-fs")
	})
})

// runScript runs a script from the repository root.
func runScript(script string, args ...string) {
	cmd := exec.Command(filepath.Join(framework.TestContext.RepoRoot, script), args...)
	out, err := cmd.CombinedOutput()
	framework.Logf("%s %s:\n%s", script, strings.Join(args, " "), out)
	framework.ExpectNoError(err)
}

func arnOf(ctx context.Context, c *Cloud, volumeId string) string {
	out, err := c.FSx.DescribeVolumes(ctx, &fsx.DescribeVolumesInput{VolumeIds: []string{volumeId}})
	framework.ExpectNoError(err)
	return *out.Volumes[0].ResourceARN
}

// waitForBoundHandle waits for the claim to bind and returns its volume handle.
func waitForBoundHandle(ctx context.Context, cs clientset.Interface, ns, pvc string, timeout time.Duration) string {
	framework.ExpectNoError(e2epv.WaitForPersistentVolumeClaimPhase(ctx, v1.ClaimBound, cs, ns, pvc, isolationPoll, timeout))
	claim, err := cs.CoreV1().PersistentVolumeClaims(ns).Get(ctx, pvc, metav1.GetOptions{})
	framework.ExpectNoError(err)
	pv, err := cs.CoreV1().PersistentVolumes().Get(ctx, claim.Spec.VolumeName, metav1.GetOptions{})
	framework.ExpectNoError(err)
	return pv.Spec.CSI.VolumeHandle
}

func deletePVC(ctx context.Context, cs clientset.Interface, ns, pvc string) {
	claim, err := cs.CoreV1().PersistentVolumeClaims(ns).Get(ctx, pvc, metav1.GetOptions{})
	framework.ExpectNoError(err)
	framework.ExpectNoError(cs.CoreV1().PersistentVolumeClaims(ns).Delete(ctx, pvc, metav1.DeleteOptions{}))
	framework.ExpectNoError(e2epv.WaitForPersistentVolumeDeleted(ctx, cs, claim.Spec.VolumeName, isolationPoll, 15*time.Minute))
}

// createSnapshot creates a VolumeSnapshot of the claim, waits until it is ready, and
// returns the FSx snapshot ID.
func createSnapshot(ctx context.Context, dyn dynamic.Interface, ns, name, class, pvc string) string {
	snap := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "snapshot.storage.k8s.io/v1",
		"kind":       "VolumeSnapshot",
		"metadata":   map[string]interface{}{"name": name, "namespace": ns},
		"spec": map[string]interface{}{
			"volumeSnapshotClassName": class,
			"source":                  map[string]interface{}{"persistentVolumeClaimName": pvc},
		},
	}}
	_, err := dyn.Resource(volumeSnapshotGVR).Namespace(ns).Create(ctx, snap, metav1.CreateOptions{})
	framework.ExpectNoError(err)
	var content string
	Eventually(func(g Gomega) {
		s, err := dyn.Resource(volumeSnapshotGVR).Namespace(ns).Get(ctx, name, metav1.GetOptions{})
		g.Expect(err).NotTo(HaveOccurred())
		ready, _, _ := unstructured.NestedBool(s.Object, "status", "readyToUse")
		g.Expect(ready).To(BeTrue(), "VolumeSnapshot %s status: %v", name, s.Object["status"])
		content, _, _ = unstructured.NestedString(s.Object, "status", "boundVolumeSnapshotContentName")
	}).WithTimeout(volCreateTimeout).WithPolling(isolationPoll).Should(Succeed())
	vsc, err := dyn.Resource(volumeSnapshotContGVR).Get(ctx, content, metav1.GetOptions{})
	framework.ExpectNoError(err)
	handle, _, _ := unstructured.NestedString(vsc.Object, "status", "snapshotHandle")
	Expect(handle).NotTo(BeEmpty())
	return handle
}

// deleteSnapshot deletes the VolumeSnapshot and waits until FSx no longer has the snapshot.
func deleteSnapshot(ctx context.Context, dyn dynamic.Interface, c *Cloud, ns, name, id string) {
	framework.ExpectNoError(dyn.Resource(volumeSnapshotGVR).Namespace(ns).Delete(ctx, name, metav1.DeleteOptions{}))
	framework.ExpectNoError(c.WaitForSnapshotDeleted(ctx, id))
}

// createStaticVolume creates a PersistentVolume with reclaim policy Delete for an existing
// volume, and a claim bound to it.
func createStaticVolume(ctx context.Context, cs clientset.Interface, c *Cloud, ns, name, volumeId string) (string, string) {
	out, err := c.FSx.DescribeVolumes(ctx, &fsx.DescribeVolumesInput{VolumeIds: []string{volumeId}})
	framework.ExpectNoError(err)
	dnsName, err := c.GetDNSName(ctx, *out.Volumes[0].FileSystemId)
	framework.ExpectNoError(err)
	pv, err := cs.CoreV1().PersistentVolumes().Create(ctx, &v1.PersistentVolume{
		ObjectMeta: metav1.ObjectMeta{Name: ns + "-" + name},
		Spec: v1.PersistentVolumeSpec{
			AccessModes:                   []v1.PersistentVolumeAccessMode{v1.ReadWriteMany},
			Capacity:                      v1.ResourceList{v1.ResourceStorage: resource.MustParse("1Gi")},
			PersistentVolumeReclaimPolicy: v1.PersistentVolumeReclaimDelete,
			PersistentVolumeSource: v1.PersistentVolumeSource{
				CSI: &v1.CSIPersistentVolumeSource{
					Driver:       driver.DriverName,
					VolumeHandle: volumeId,
					VolumeAttributes: map[string]string{
						"DNSName":      dnsName,
						"VolumePath":   *out.Volumes[0].OpenZFSConfiguration.VolumePath,
						"ResourceType": RESOURCETYPE_VOLUME,
					},
				},
			},
		},
	}, metav1.CreateOptions{})
	framework.ExpectNoError(err)

	empty := ""
	pvc, err := cs.CoreV1().PersistentVolumeClaims(ns).Create(ctx, &v1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: v1.PersistentVolumeClaimSpec{
			AccessModes:      []v1.PersistentVolumeAccessMode{v1.ReadWriteMany},
			StorageClassName: &empty,
			VolumeName:       pv.Name,
			Resources:        v1.VolumeResourceRequirements{Requests: v1.ResourceList{v1.ResourceStorage: resource.MustParse("1Gi")}},
		},
	}, metav1.CreateOptions{})
	framework.ExpectNoError(err)
	framework.ExpectNoError(e2epv.WaitForPersistentVolumeClaimPhase(ctx, v1.ClaimBound, cs, ns, pvc.Name, isolationPoll, 2*time.Minute))
	return pv.Name, pvc.Name
}

func volumeFailedDeleteMessage(ctx context.Context, cs clientset.Interface, pv string) string {
	events, err := cs.CoreV1().Events("").List(ctx, metav1.ListOptions{
		FieldSelector: "involvedObject.kind=PersistentVolume,involvedObject.name=" + pv + ",reason=VolumeFailedDelete",
	})
	framework.ExpectNoError(err)
	var msgs []string
	for _, e := range events.Items {
		msgs = append(msgs, e.Message)
	}
	return strings.Join(msgs, "\n")
}

// controllerLogs returns the plugin logs of every controller pod.
func controllerLogs(ctx context.Context, cs clientset.Interface) string {
	pods, err := cs.CoreV1().Pods(controllerNamespace).List(ctx, metav1.ListOptions{LabelSelector: "app.kubernetes.io/name=" + controllerDeployment})
	framework.ExpectNoError(err)
	var buf bytes.Buffer
	for _, pod := range pods.Items {
		out, err := cs.CoreV1().Pods(controllerNamespace).GetLogs(pod.Name, &v1.PodLogOptions{Container: controllerContainer}).DoRaw(ctx)
		if err != nil {
			framework.Logf("logs of %s: %v", pod.Name, err)
			continue
		}
		buf.Write(out)
	}
	return buf.String()
}

func restartController(ctx context.Context, cs clientset.Interface) {
	framework.Logf("restarting the controller to run another sweep")
	patch := fmt.Sprintf(`{"spec":{"template":{"metadata":{"annotations":{"fsx-openzfs-e2e/restartedAt":%q}}}}}`, time.Now().Format(time.RFC3339Nano))
	_, err := cs.AppsV1().Deployments(controllerNamespace).Patch(ctx, controllerDeployment, types.StrategicMergePatchType, []byte(patch), metav1.PatchOptions{})
	framework.ExpectNoError(err)
}

// runWithControllerRole runs a shell script in an aws-cli pod that uses the controller's
// service account, so EKS Pod Identity gives it the controller's role, and returns its output.
func runWithControllerRole(ctx context.Context, cs clientset.Interface, script string) string {
	pod, err := cs.CoreV1().Pods(controllerNamespace).Create(ctx, &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{GenerateName: "fsx-openzfs-isolation-iam-"},
		Spec: v1.PodSpec{
			ServiceAccountName: controllerSA,
			RestartPolicy:      v1.RestartPolicyNever,
			Containers: []v1.Container{{
				Name:    "aws",
				Image:   "public.ecr.aws/aws-cli/aws-cli:latest",
				Command: []string{"/bin/sh", "-c", script},
				Env:     []v1.EnvVar{{Name: "AWS_REGION", Value: Region}},
			}},
		},
	}, metav1.CreateOptions{})
	framework.ExpectNoError(err)
	defer func() {
		_ = cs.CoreV1().Pods(controllerNamespace).Delete(ctx, pod.Name, metav1.DeleteOptions{})
	}()

	Eventually(func() (v1.PodPhase, error) {
		p, err := cs.CoreV1().Pods(controllerNamespace).Get(ctx, pod.Name, metav1.GetOptions{})
		if err != nil {
			return "", err
		}
		return p.Status.Phase, nil
	}).WithTimeout(5 * time.Minute).WithPolling(5 * time.Second).Should(Equal(v1.PodSucceeded))

	out, err := cs.CoreV1().Pods(controllerNamespace).GetLogs(pod.Name, &v1.PodLogOptions{}).DoRaw(ctx)
	framework.ExpectNoError(err)
	framework.Logf("aws-cli pod output:\n%s", out)
	return string(out)
}
