/*
Copyright 2023 The Kubernetes Authors.
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

package driver

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/kubernetes-sigs/aws-fsx-openzfs-csi-driver/pkg/cloud"
	"github.com/kubernetes-sigs/aws-fsx-openzfs-csi-driver/pkg/driver/mocks"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/klog/v2"
)

type fakeLister struct {
	refs *clusterReferences
	err  error
}

func (f *fakeLister) List(context.Context) (*clusterReferences, error) { return f.refs, f.err }

// captureLogs redirects klog to a buffer for the duration of the test.
func captureLogs(t *testing.T) *bytes.Buffer {
	t.Helper()
	fs := flag.NewFlagSet("klog", flag.ContinueOnError)
	klog.InitFlags(fs)
	_ = fs.Set("logtostderr", "false")
	_ = fs.Set("alsologtostderr", "false")
	buf := &bytes.Buffer{}
	klog.SetOutput(buf)
	t.Cleanup(func() {
		klog.SetOutput(nil)
		_ = fs.Set("logtostderr", "true")
	})
	return buf
}

func volumeResource(id string, tags map[string]string, parent string) *cloud.Resource {
	return &cloud.Resource{ID: id, ARN: "arn:aws:fsx:us-east-1:111122223333:volume/fs-1/" + id, Name: "pv-" + id, Tags: tags, ParentVolumeID: parent}
}

// provisioned is a handle of a PersistentVolume this driver provisioned, named like volumeResource.
func provisioned(id string) handle { return handle{ID: id, Provisioned: true, Name: "pv-" + id} }

func TestBackfillSweep(t *testing.T) {
	legacy := map[string]string{AwsFsxOpenZfsDriverTagKey: "true"}
	tagged := map[string]string{AwsFsxOpenZfsDriverTagKey: "true", testTagKey: testClusterName}
	other := map[string]string{AwsFsxOpenZfsDriverTagKey: "true", testTagKey: "other"}
	parentOK := map[string]string{testTagKey: testClusterName}

	t.Run("tags owned legacy resources with only the cluster tag and completes", func(t *testing.T) {
		logs := captureLogs(t)
		mockCtl := gomock.NewController(t)
		mockCloud := mocks.NewMockCloud(mockCtl)
		b := &backfiller{cloud: mockCloud, options: testClusterTagOptions(ClusterTagIsolationBackfill, map[string]string{"team": "a"}), lister: &fakeLister{refs: &clusterReferences{
			Handles: []handle{provisioned("fsvol-1"), provisioned("fsvol-2"), {ID: "fsvolsnap-1", Provisioned: true}, provisioned("fs-1")},
		}}}
		mockCloud.EXPECT().DescribeResource(gomock.Any(), "fsvol-1").Return(volumeResource("fsvol-1", copyTags(legacy), "fsvol-p"), nil)
		mockCloud.EXPECT().DescribeResource(gomock.Any(), "fsvol-2").Return(volumeResource("fsvol-2", copyTags(tagged), "fsvol-p"), nil)
		mockCloud.EXPECT().DescribeResource(gomock.Any(), "fsvolsnap-1").Return(&cloud.Resource{ID: "fsvolsnap-1", ARN: "arn:snap", Tags: copyTags(legacy)}, nil)
		mockCloud.EXPECT().DescribeResource(gomock.Any(), "fsvol-p").Return(volumeResource("fsvol-p", copyTags(parentOK), ""), nil)
		// file system with CopyTagsToVolumes: its root volume carries the driver tag and is tagged too
		mockCloud.EXPECT().DescribeResource(gomock.Any(), "fs-1").Return(&cloud.Resource{ID: "fs-1", ARN: "arn:fs", Tags: copyTags(legacy), RootVolumeID: "fsvol-root"}, nil)
		mockCloud.EXPECT().DescribeResource(gomock.Any(), "fsvol-root").Return(volumeResource("fsvol-root", copyTags(legacy), ""), nil)
		mockCloud.EXPECT().TagResource(gomock.Any(), "arn:aws:fsx:us-east-1:111122223333:volume/fs-1/fsvol-1", map[string]string{testTagKey: testClusterName}).Return(nil)
		mockCloud.EXPECT().TagResource(gomock.Any(), "arn:snap", map[string]string{testTagKey: testClusterName}).Return(nil)
		mockCloud.EXPECT().TagResource(gomock.Any(), "arn:fs", map[string]string{testTagKey: testClusterName}).Return(nil)
		mockCloud.EXPECT().TagResource(gomock.Any(), "arn:aws:fsx:us-east-1:111122223333:volume/fs-1/fsvol-root", map[string]string{testTagKey: testClusterName}).Return(nil)

		counts := b.sweep(context.Background())
		if counts.n() != [4]int{4, 0, 0, 0} {
			t.Fatalf("counts = %+v", counts)
		}
		klog.Flush()
		want := []string{
			`"cluster tag backfill sweep finished" tagged=4 failed=0 needsAdministrator=0 remaining=0`,
			`"cluster tag backfill complete" tagKey="example.com/cluster" tagValue="my-cluster"`,
		}
		// Per-resource lines are verbosity 4 only, so the default output has exactly these two lines.
		if got := logMessages(logs.String()); !reflect.DeepEqual(got, want) {
			t.Fatalf("log lines:\n got %q\nwant %q", got, want)
		}

		// A second clean sweep does not repeat the completion line.
		mockCloud.EXPECT().DescribeResource(gomock.Any(), gomock.Any()).Return(volumeResource("fsvol-1", copyTags(tagged), ""), nil).Times(4)
		logs.Reset()
		b.sweep(context.Background())
		klog.Flush()
		if strings.Contains(logs.String(), `"cluster tag backfill complete"`) {
			t.Fatalf("completion logged twice:\n%s", logs.String())
		}
		mockCtl.Finish()
	})

	t.Run("failed tag call is counted and blocks completion", func(t *testing.T) {
		logs := captureLogs(t)
		mockCtl := gomock.NewController(t)
		mockCloud := mocks.NewMockCloud(mockCtl)
		b := &backfiller{cloud: mockCloud, options: testClusterTagOptions(ClusterTagIsolationBackfill, nil), lister: &fakeLister{refs: &clusterReferences{Handles: []handle{provisioned("fsvol-1")}}}}
		mockCloud.EXPECT().DescribeResource(gomock.Any(), "fsvol-1").Return(volumeResource("fsvol-1", copyTags(legacy), ""), nil)
		mockCloud.EXPECT().TagResource(gomock.Any(), gomock.Any(), gomock.Any()).Return(errors.New("AccessDeniedException"))
		counts := b.sweep(context.Background())
		// A failed tag call counts in failed only.
		if counts.n() != [4]int{0, 1, 0, 0} {
			t.Fatalf("counts = %+v", counts)
		}
		klog.Flush()
		want := []string{`"cluster tag backfill sweep finished" tagged=0 failed=1 needsAdministrator=0 remaining=0 failedResources=["arn:aws:fsx:us-east-1:111122223333:volume/fs-1/fsvol-1"] firstError="AccessDeniedException"`}
		if got := logMessages(logs.String()); !reflect.DeepEqual(got, want) {
			t.Fatalf("log lines:\n got %q\nwant %q", got, want)
		}
		mockCtl.Finish()
	})

	t.Run("resources the controller does not own are reported, not tagged", func(t *testing.T) {
		logs := captureLogs(t)
		mockCtl := gomock.NewController(t)
		mockCloud := mocks.NewMockCloud(mockCtl)
		b := &backfiller{cloud: mockCloud, options: testClusterTagOptions(ClusterTagIsolationBackfill, nil), lister: &fakeLister{refs: &clusterReferences{
			Handles: []handle{
				provisioned("fsvol-static"), provisioned("fsvol-other"), provisioned("fs-1"),
				{ID: "fsvol-unannotated", Name: "pv-fsvol-unannotated"},
				{ID: "fsvol-renamed", Provisioned: true, Name: "pv-elsewhere"},
				{ID: "fsvolsnap-static"},
				provisioned("fsvol-gone"),
			},
			ParentVolumes: []string{"fsvol-admin"},
		}}}
		// static PV of an administrator's volume: no driver tag
		mockCloud.EXPECT().DescribeResource(gomock.Any(), "fsvol-static").Return(volumeResource("fsvol-static", map[string]string{}, "fsvol-admin"), nil)
		// tagged for another cluster
		mockCloud.EXPECT().DescribeResource(gomock.Any(), "fsvol-other").Return(volumeResource("fsvol-other", copyTags(other), ""), nil)
		// file system the driver created; its root volume did not get the driver tag
		mockCloud.EXPECT().DescribeResource(gomock.Any(), "fs-1").Return(&cloud.Resource{ID: "fs-1", ARN: "arn:fs", Tags: copyTags(tagged), RootVolumeID: "fsvol-root"}, nil)
		mockCloud.EXPECT().DescribeResource(gomock.Any(), "fsvol-root").Return(volumeResource("fsvol-root", map[string]string{}, ""), nil)
		// a driver volume behind a PV without the provisioned-by annotation (static PV, maybe of another cluster)
		mockCloud.EXPECT().DescribeResource(gomock.Any(), "fsvol-unannotated").Return(volumeResource("fsvol-unannotated", copyTags(legacy), ""), nil)
		// a driver volume whose name is not the PV name
		mockCloud.EXPECT().DescribeResource(gomock.Any(), "fsvol-renamed").Return(volumeResource("fsvol-renamed", copyTags(legacy), ""), nil)
		// a pre-provisioned VolumeSnapshotContent of a driver snapshot
		mockCloud.EXPECT().DescribeResource(gomock.Any(), "fsvolsnap-static").Return(&cloud.Resource{ID: "fsvolsnap-static", ARN: "arn:snap", Tags: copyTags(legacy)}, nil)
		// not found: deleted, or not visible with these credentials
		mockCloud.EXPECT().DescribeResource(gomock.Any(), "fsvol-gone").Return(nil, cloud.ErrNotFound)
		// StorageClass parent volume, also the parent of the static volume: visited once
		mockCloud.EXPECT().DescribeResource(gomock.Any(), "fsvol-admin").Return(volumeResource("fsvol-admin", copyTags(legacy), ""), nil)

		counts := b.sweep(context.Background())
		if counts.n() != [4]int{0, 0, 8, 0} {
			t.Fatalf("counts = %+v", counts)
		}
		klog.Flush()
		want := []string{`"cluster tag backfill sweep finished" tagged=0 failed=0 needsAdministrator=8 remaining=0 needsAdministratorResources=["arn:aws:fsx:us-east-1:111122223333:volume/fs-1/fsvol-root","fsvol-gone","arn:aws:fsx:us-east-1:111122223333:volume/fs-1/fsvol-other","arn:aws:fsx:us-east-1:111122223333:volume/fs-1/fsvol-renamed","arn:aws:fsx:us-east-1:111122223333:volume/fs-1/fsvol-static","arn:aws:fsx:us-east-1:111122223333:volume/fs-1/fsvol-unannotated","arn:snap","arn:aws:fsx:us-east-1:111122223333:volume/fs-1/fsvol-admin"]`}
		if got := logMessages(logs.String()); !reflect.DeepEqual(got, want) {
			t.Fatalf("log lines:\n got %q\nwant %q", got, want)
		}
		mockCtl.Finish()
	})

	t.Run("disable adds the driver tag to root volumes of owned file systems", func(t *testing.T) {
		logs := captureLogs(t)
		mockCtl := gomock.NewController(t)
		mockCloud := mocks.NewMockCloud(mockCtl)
		b := &backfiller{cloud: mockCloud, options: nil, lister: &fakeLister{refs: &clusterReferences{
			Handles:       []handle{provisioned("fs-1"), {ID: "fs-2"}, provisioned("fsvol-1")},
			ParentVolumes: []string{"fsvol-admin"},
		}}}
		mockCloud.EXPECT().DescribeResource(gomock.Any(), "fs-1").Return(&cloud.Resource{ID: "fs-1", ARN: "arn:fs-1", Tags: copyTags(legacy), RootVolumeID: "fsvol-root1"}, nil)
		mockCloud.EXPECT().DescribeResource(gomock.Any(), "fsvol-root1").Return(volumeResource("fsvol-root1", map[string]string{}, ""), nil)
		// a static PV of a driver file system: its root volume is not owned
		mockCloud.EXPECT().DescribeResource(gomock.Any(), "fs-2").Return(&cloud.Resource{ID: "fs-2", ARN: "arn:fs-2", Tags: copyTags(legacy), RootVolumeID: "fsvol-root2"}, nil)
		mockCloud.EXPECT().DescribeResource(gomock.Any(), "fsvol-root2").Return(volumeResource("fsvol-root2", map[string]string{}, ""), nil)
		mockCloud.EXPECT().DescribeResource(gomock.Any(), "fsvol-1").Return(volumeResource("fsvol-1", copyTags(legacy), "fsvol-admin"), nil)
		mockCloud.EXPECT().DescribeResource(gomock.Any(), "fsvol-admin").Return(volumeResource("fsvol-admin", map[string]string{}, ""), nil)
		mockCloud.EXPECT().TagResource(gomock.Any(), "arn:aws:fsx:us-east-1:111122223333:volume/fs-1/fsvol-root1", map[string]string{AwsFsxOpenZfsDriverTagKey: "true"}).Return(nil)

		counts := b.sweep(context.Background())
		if counts.n() != [4]int{1, 0, 2, 0} {
			t.Fatalf("counts = %+v", counts)
		}
		klog.Flush()
		want := []string{`"cluster tag backfill sweep finished" tagged=1 failed=0 needsAdministrator=2 remaining=0 needsAdministratorResources=["arn:aws:fsx:us-east-1:111122223333:volume/fs-1/fsvol-root2","arn:aws:fsx:us-east-1:111122223333:volume/fs-1/fsvol-admin"]`}
		if got := logMessages(logs.String()); !reflect.DeepEqual(got, want) {
			t.Fatalf("log lines:\n got %q\nwant %q", got, want)
		}
		mockCtl.Finish()
	})

	t.Run("read and list errors count as remaining", func(t *testing.T) {
		captureLogs(t)
		mockCtl := gomock.NewController(t)
		mockCloud := mocks.NewMockCloud(mockCtl)
		b := &backfiller{cloud: mockCloud, options: testClusterTagOptions(ClusterTagIsolationBackfill, nil), lister: &fakeLister{refs: &clusterReferences{Handles: []handle{provisioned("fsvol-1")}}}}
		mockCloud.EXPECT().DescribeResource(gomock.Any(), "fsvol-1").Return(nil, errors.New("throttled"))
		if counts := b.sweep(context.Background()); counts.n() != [4]int{0, 0, 0, 1} || counts.firstError.Error() != "throttled" {
			t.Fatalf("counts = %+v", counts)
		}
		b.lister = &fakeLister{err: errors.New("forbidden")}
		if counts := b.sweep(context.Background()); counts.n() != [4]int{0, 0, 0, 1} || counts.firstError.Error() != "forbidden" {
			t.Fatalf("counts = %+v", counts)
		}
		mockCtl.Finish()
	})

	t.Run("complete is logged once per process and never re-armed", func(t *testing.T) {
		logs := captureLogs(t)
		b := &backfiller{options: testClusterTagOptions(ClusterTagIsolationBackfill, nil), lister: &fakeLister{refs: &clusterReferences{}}}
		b.sweep(context.Background())
		b.lister = &fakeLister{err: errors.New("forbidden")}
		b.sweep(context.Background())
		b.lister = &fakeLister{refs: &clusterReferences{}}
		b.sweep(context.Background())
		klog.Flush()
		want := []string{
			`"cluster tag backfill sweep finished" tagged=0 failed=0 needsAdministrator=0 remaining=0`,
			`"cluster tag backfill complete" tagKey="example.com/cluster" tagValue="my-cluster"`,
			`"cluster tag backfill sweep finished" tagged=0 failed=0 needsAdministrator=0 remaining=1 firstError="forbidden"`,
			`"cluster tag backfill sweep finished" tagged=0 failed=0 needsAdministrator=0 remaining=0`,
		}
		if got := logMessages(logs.String()); !reflect.DeepEqual(got, want) {
			t.Fatalf("log lines:\n got %q\nwant %q", got, want)
		}
	})

	t.Run("summary lists are capped at 20", func(t *testing.T) {
		logs := captureLogs(t)
		mockCloud := mocks.NewMockCloud(gomock.NewController(t))
		var handles []handle
		var admin []string
		for i := 0; i < 22; i++ {
			id := fmt.Sprintf("fsvol-%02d", i)
			handles = append(handles, provisioned(id))
			if i < 20 {
				admin = append(admin, "arn:aws:fsx:us-east-1:111122223333:volume/fs-1/"+id)
			}
			mockCloud.EXPECT().DescribeResource(gomock.Any(), id).Return(volumeResource(id, map[string]string{}, ""), nil)
		}
		b := &backfiller{cloud: mockCloud, options: testClusterTagOptions(ClusterTagIsolationBackfill, nil), lister: &fakeLister{refs: &clusterReferences{Handles: handles}}}
		b.sweep(context.Background())
		klog.Flush()
		want := []string{`"cluster tag backfill sweep finished" tagged=0 failed=0 needsAdministrator=22 remaining=0 needsAdministratorResources=["` + strings.Join(admin, `","`) + `"] needsAdministratorTotal=22`}
		if got := logMessages(logs.String()); !reflect.DeepEqual(got, want) {
			t.Fatalf("log lines:\n got %q\nwant %q", got, want)
		}
	})
}

// n returns tagged, failed, needsAdministrator and remaining.
func (c backfillCounts) n() [4]int {
	return [4]int{c.tagged, c.failed, c.needsAdministrator, c.remaining}
}

// logMessages returns the structured part (after "] ") of each klog line in out.
func logMessages(out string) []string {
	var lines []string
	for _, l := range strings.Split(strings.TrimSpace(out), "\n") {
		if _, msg, ok := strings.Cut(l, "] "); ok {
			lines = append(lines, msg)
		}
	}
	return lines
}

func copyTags(tags map[string]string) map[string]string {
	out := make(map[string]string, len(tags))
	for k, v := range tags {
		out[k] = v
	}
	return out
}

func TestKubernetesReferenceLister(t *testing.T) {
	client := fake.NewSimpleClientset(
		&corev1.PersistentVolume{
			ObjectMeta: metav1.ObjectMeta{Name: "pv-1", Annotations: map[string]string{provisionedByAnnotation: DriverName}},
			Spec: corev1.PersistentVolumeSpec{PersistentVolumeSource: corev1.PersistentVolumeSource{
				CSI: &corev1.CSIPersistentVolumeSource{Driver: DriverName, VolumeHandle: "fsvol-1"},
			}},
		},
		&corev1.PersistentVolume{
			ObjectMeta: metav1.ObjectMeta{Name: "pv-static"},
			Spec: corev1.PersistentVolumeSpec{PersistentVolumeSource: corev1.PersistentVolumeSource{
				CSI: &corev1.CSIPersistentVolumeSource{Driver: DriverName, VolumeHandle: "fs-1"},
			}},
		},
		&corev1.PersistentVolume{
			ObjectMeta: metav1.ObjectMeta{Name: "pv-other-driver"},
			Spec: corev1.PersistentVolumeSpec{PersistentVolumeSource: corev1.PersistentVolumeSource{
				CSI: &corev1.CSIPersistentVolumeSource{Driver: "ebs.csi.aws.com", VolumeHandle: "vol-1"},
			}},
		},
		&storagev1.StorageClass{ObjectMeta: metav1.ObjectMeta{Name: "sc-1"}, Provisioner: DriverName, Parameters: map[string]string{"ParentVolumeId": `"fsvol-parent"`}},
		&storagev1.StorageClass{ObjectMeta: metav1.ObjectMeta{Name: "sc-2"}, Provisioner: "other", Parameters: map[string]string{"ParentVolumeId": `"fsvol-x"`}},
	)
	dynamic := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "snapshot.storage.k8s.io/v1",
		"kind":       "VolumeSnapshotContent",
		"metadata":   map[string]interface{}{"name": "snapcontent-1"},
		"spec":       map[string]interface{}{"driver": DriverName, "source": map[string]interface{}{"volumeHandle": "fsvol-1"}},
		"status":     map[string]interface{}{"snapshotHandle": "fsvolsnap-1"},
	}}
	static := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "snapshot.storage.k8s.io/v1",
		"kind":       "VolumeSnapshotContent",
		"metadata":   map[string]interface{}{"name": "snapcontent-2"},
		"spec":       map[string]interface{}{"driver": DriverName, "source": map[string]interface{}{"snapshotHandle": "fsvolsnap-2"}},
	}}
	dyn := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(),
		map[schema.GroupVersionResource]string{volumeSnapshotContentResource: "VolumeSnapshotContentList"}, dynamic, static)
	l := &kubernetesReferenceLister{client: client, dynamic: dyn}
	refs, err := l.List(context.Background())
	if err != nil {
		t.Fatalf("List() error: %v", err)
	}
	wantHandles := []handle{
		{ID: "fsvol-1", Provisioned: true, Name: "pv-1"},
		{ID: "fs-1", Name: "pv-static"},
		{ID: "fsvolsnap-1", Provisioned: true},
		{ID: "fsvolsnap-2"},
	}
	sort.Slice(refs.Handles, func(i, j int) bool { return refs.Handles[i].ID < refs.Handles[j].ID })
	sort.Slice(wantHandles, func(i, j int) bool { return wantHandles[i].ID < wantHandles[j].ID })
	if !reflect.DeepEqual(refs.Handles, wantHandles) || strings.Join(refs.ParentVolumes, ",") != "fsvol-parent" {
		t.Fatalf("List() = %+v", refs)
	}
}
