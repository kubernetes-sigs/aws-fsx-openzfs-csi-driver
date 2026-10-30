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
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/golang/mock/gomock"
	"github.com/kubernetes-sigs/aws-fsx-openzfs-csi-driver/pkg/cloud"
	"github.com/kubernetes-sigs/aws-fsx-openzfs-csi-driver/pkg/driver/internal"
	"github.com/kubernetes-sigs/aws-fsx-openzfs-csi-driver/pkg/driver/mocks"
	"github.com/kubernetes-sigs/aws-fsx-openzfs-csi-driver/pkg/util"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/klog/v2"
)

const (
	testTagKey      = "example.com/cluster"
	testClusterName = "my-cluster"
)

func testClusterTagOptions(isolation ClusterTagIsolation, extra map[string]string) *ClusterTagOptions {
	return &ClusterTagOptions{ClusterName: testClusterName, TagKey: testTagKey, Isolation: isolation, ExtraTags: extra}
}

func TestClusterTagOptionsValidate(t *testing.T) {
	testCases := []struct {
		name    string
		options ClusterTagOptions
		want    ClusterTagIsolation
		wantErr string
	}{
		{name: "nothing set means disable", options: ClusterTagOptions{}, want: ClusterTagIsolationDisable},
		{name: "disable with cluster tag", options: ClusterTagOptions{ClusterName: testClusterName, TagKey: testTagKey, Isolation: "disable"}, want: ClusterTagIsolationDisable},
		{name: "backfill", options: ClusterTagOptions{ClusterName: testClusterName, TagKey: testTagKey, Isolation: "backfill"}, want: ClusterTagIsolationBackfill},
		{name: "enforce", options: ClusterTagOptions{ClusterName: testClusterName, TagKey: testTagKey, Isolation: "enforce"}, want: ClusterTagIsolationEnforce},
		{name: "unknown state", options: ClusterTagOptions{Isolation: "strict"}, wantErr: invalidIsolationError("strict")},
		{name: "off", options: ClusterTagOptions{Isolation: "off"}, wantErr: invalidIsolationError("off")},
		{name: "OFF", options: ClusterTagOptions{Isolation: "OFF"}, wantErr: invalidIsolationError("OFF")},
		{name: "leading space", options: ClusterTagOptions{Isolation: " disable"}, wantErr: invalidIsolationError(" disable")},
		{name: "false is not disable", options: ClusterTagOptions{Isolation: "false"}, wantErr: invalidIsolationError("false")},
		{name: "true", options: ClusterTagOptions{Isolation: "true"}, wantErr: invalidIsolationError("true")},
		{name: "on", options: ClusterTagOptions{Isolation: "on"}, wantErr: invalidIsolationError("on")},
		{name: "backfill without cluster name", options: ClusterTagOptions{TagKey: testTagKey, Isolation: "backfill"}, wantErr: "--cluster-tag-isolation=backfill requires --cluster-name and --cluster-tag-key"},
		{name: "enforce without key", options: ClusterTagOptions{ClusterName: testClusterName, Isolation: "enforce"}, wantErr: "--cluster-tag-isolation=enforce requires --cluster-name and --cluster-tag-key"},
		{name: "cluster name without key is disable", options: ClusterTagOptions{ClusterName: testClusterName}, want: ClusterTagIsolationDisable},
		{name: "key without cluster name is disable", options: ClusterTagOptions{TagKey: testTagKey}, want: ClusterTagIsolationDisable},
		{name: "enforce with whitespace cluster name", options: ClusterTagOptions{ClusterName: "  ", TagKey: testTagKey, Isolation: "enforce"}, wantErr: "--cluster-tag-isolation=enforce requires --cluster-name and --cluster-tag-key"},
		{name: "backfill with whitespace key", options: ClusterTagOptions{ClusterName: testClusterName, TagKey: "\t", Isolation: "backfill"}, wantErr: "--cluster-tag-isolation=backfill requires --cluster-name and --cluster-tag-key"},
		{name: "aws: key", options: ClusterTagOptions{ClusterName: testClusterName, TagKey: "aws:cluster"}, wantErr: "must not start with"},
		{name: "driver tag key", options: ClusterTagOptions{ClusterName: testClusterName, TagKey: AwsFsxOpenZfsDriverTagKey}, wantErr: "is set by the driver"},
		{name: "deletion key", options: ClusterTagOptions{ClusterName: testClusterName, TagKey: "OptionsOnDeletion"}, wantErr: "must not end with"},
		{name: "extra tags", options: ClusterTagOptions{ExtraTags: map[string]string{"team": "a"}}, want: ClusterTagIsolationDisable},
		{name: "extra tag sets cluster tag key", options: ClusterTagOptions{ClusterName: testClusterName, TagKey: testTagKey, ExtraTags: map[string]string{testTagKey: "other"}}, wantErr: "must not contain the cluster tag key"},
		{name: "extra tag sets deletion option", options: ClusterTagOptions{ExtraTags: map[string]string{"OptionsOnDeletion": "x"}}, wantErr: "must not end with"},
		{name: "extra tag sets driver tag", options: ClusterTagOptions{ExtraTags: map[string]string{AwsFsxOpenZfsDriverTagKey: "false"}}, wantErr: "is set by the driver"},
		{name: "extra tag with aws: prefix", options: ClusterTagOptions{ExtraTags: map[string]string{"AWS:x": "y"}}, wantErr: "must not start with"},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.options.Validate()
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("Validate() error = %v, want %q", err, tc.wantErr)
				}
				if strings.HasPrefix(tc.wantErr, "invalid --cluster-tag-isolation ") && err.Error() != tc.wantErr {
					t.Fatalf("Validate() error = %q, want exactly %q", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("Validate() unexpected error: %v", err)
			}
			if tc.options.Isolation != tc.want {
				t.Fatalf("Isolation = %q, want %q", tc.options.Isolation, tc.want)
			}
		})
	}
}

func invalidIsolationError(v string) string {
	return `invalid --cluster-tag-isolation "` + v + `": must be "disable", "backfill" or "enforce"`
}

func TestClusterTagSettingsLogLine(t *testing.T) {
	logs := captureLogs(t)
	testClusterTagOptions(ClusterTagIsolationBackfill, nil).logSettings()
	testClusterTagOptions(ClusterTagIsolationEnforce, nil).logSettings()
	(*ClusterTagOptions)(nil).logSettings()
	(&ClusterTagOptions{ClusterName: testClusterName}).logSettings()
	klog.Flush()
	want := []string{
		`"cluster tag isolation settings" clusterName="my-cluster" tagKey="example.com/cluster" clusterTagIsolation="backfill" backfillInterval="1h0m0s"`,
		`"cluster tag isolation settings" clusterName="my-cluster" tagKey="example.com/cluster" clusterTagIsolation="enforce"`,
		`"cluster tag isolation settings" clusterName="" tagKey="" clusterTagIsolation="disable" backfillInterval="1h0m0s"`,
		`"cluster tag isolation settings" clusterName="my-cluster" tagKey="" clusterTagIsolation="disable" backfillInterval="1h0m0s"`,
		`"only one of --cluster-name and --cluster-tag-key is set; no cluster tag is written"`,
	}
	if got := logMessages(logs.String()); !reflect.DeepEqual(got, want) {
		t.Fatalf("log lines:\n got %q\nwant %q", got, want)
	}
}

func TestMergeTags(t *testing.T) {
	marker := map[string]string{"Key": AwsFsxOpenZfsDriverTagKey, "Value": "true"}
	clusterTag := map[string]string{"Key": testTagKey, "Value": testClusterName}
	testCases := []struct {
		name    string
		options *ClusterTagOptions
		tags    []map[string]string
		want    []map[string]string
	}{
		{name: "nil options change nothing", options: nil, tags: []map[string]string{marker}, want: []map[string]string{marker}},
		{name: "empty options change nothing", options: &ClusterTagOptions{}, tags: []map[string]string{marker}, want: []map[string]string{marker}},
		{name: "cluster tag added in disable", options: testClusterTagOptions(ClusterTagIsolationDisable, nil), tags: []map[string]string{marker}, want: []map[string]string{marker, clusterTag}},
		{name: "cluster tag added in backfill", options: testClusterTagOptions(ClusterTagIsolationBackfill, nil), tags: nil, want: []map[string]string{clusterTag}},
		{name: "cluster tag added in enforce", options: testClusterTagOptions(ClusterTagIsolationEnforce, nil), tags: nil, want: []map[string]string{clusterTag}},
		{
			name:    "extra tags added, provided tags override them",
			options: testClusterTagOptions(ClusterTagIsolationDisable, map[string]string{"team": "a", "env": "prod"}),
			tags:    []map[string]string{{"Key": "team", "Value": "b"}},
			want:    []map[string]string{{"Key": "team", "Value": "b"}, {"Key": "env", "Value": "prod"}, clusterTag},
		},
		{
			name:    "extra tags without cluster tag",
			options: &ClusterTagOptions{ExtraTags: map[string]string{"team": "a"}},
			tags:    nil,
			want:    []map[string]string{{"Key": "team", "Value": "a"}},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.options.mergeTags(tc.tags); !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("mergeTags() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestValidateProvidedTags(t *testing.T) {
	testCases := []struct {
		name    string
		options *ClusterTagOptions
		tags    []map[string]string
		wantErr bool
	}{
		{name: "other tags pass", options: testClusterTagOptions(ClusterTagIsolationDisable, nil), tags: []map[string]string{{"Key": "team", "Value": "a"}}},
		{name: "nil options pass other tags", options: nil, tags: []map[string]string{{"Key": testTagKey, "Value": "x"}}},
		{name: "cluster tag with the same value", options: testClusterTagOptions(ClusterTagIsolationDisable, nil), tags: []map[string]string{{"Key": testTagKey, "Value": testClusterName}}, wantErr: true},
		{name: "cluster tag with another value", options: testClusterTagOptions(ClusterTagIsolationEnforce, nil), tags: []map[string]string{{"Key": testTagKey, "Value": "other"}}, wantErr: true},
		{name: "driver tag", options: testClusterTagOptions(ClusterTagIsolationDisable, nil), tags: []map[string]string{{"Key": AwsFsxOpenZfsDriverTagKey, "Value": "true"}}, wantErr: true},
		{name: "driver tag without options", options: nil, tags: []map[string]string{{"Key": AwsFsxOpenZfsDriverTagKey, "Value": "true"}}, wantErr: true},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.options.validateProvidedTags(tc.tags)
			if tc.wantErr != errors.Is(err, errClusterTagConflict) {
				t.Fatalf("validateProvidedTags() error = %v, want error %v", err, tc.wantErr)
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("validateProvidedTags() unexpected error: %v", err)
			}
		})
	}
}

func TestCreateVolumeClusterTags(t *testing.T) {
	stdVolCap := &csi.VolumeCapability{
		AccessType: &csi.VolumeCapability_Mount{Mount: &csi.VolumeCapability_MountVolume{}},
		AccessMode: &csi.VolumeCapability_AccessMode{Mode: csi.VolumeCapability_AccessMode_MULTI_NODE_MULTI_WRITER},
	}
	volumeRequest := func(tags string) *csi.CreateVolumeRequest {
		params := map[string]string{
			"ResourceType":   "volume",
			"ParentVolumeId": `"fsvol-03062e7ff37662dff"`,
		}
		if tags != "" {
			params["Tags"] = tags
		}
		return &csi.CreateVolumeRequest{
			Name:               "pvc-1",
			CapacityRange:      &csi.CapacityRange{RequiredBytes: util.GiBToBytes(1)},
			VolumeCapabilities: []*csi.VolumeCapability{stdVolCap},
			Parameters:         params,
		}
	}
	for _, isolation := range []ClusterTagIsolation{ClusterTagIsolationDisable, ClusterTagIsolationBackfill, ClusterTagIsolationEnforce} {
		t.Run("volume gets cluster and extra tags in "+string(isolation), func(t *testing.T) {
			mockCtl := gomock.NewController(t)
			mockCloud := mocks.NewMockCloud(mockCtl)
			driver := controllerService{
				cloud:         mockCloud,
				inFlight:      internal.NewInFlight(),
				driverOptions: &DriverOptions{clusterTag: testClusterTagOptions(isolation, map[string]string{"team": "a"})},
			}
			want := `[{"Key":"user","Value":"x"},{"Key":"fsx.openzfs.csi.aws.com/cluster","Value":"true"},{"Key":"team","Value":"a"},{"Key":"example.com/cluster","Value":"my-cluster"}]`
			mockCloud.EXPECT().CreateVolume(gomock.Eq(context.Background()), gomock.Any()).DoAndReturn(
				func(_ context.Context, params map[string]string) (*cloud.Volume, error) {
					if params["Tags"] != want {
						t.Fatalf("Tags = %s, want %s", params["Tags"], want)
					}
					return &cloud.Volume{VolumeId: "fsvol-1", FileSystemId: "fs-1"}, nil
				})
			mockCloud.EXPECT().WaitForVolumeAvailable(gomock.Any(), "fsvol-1").Return(nil)
			mockCloud.EXPECT().DescribeFileSystem(gomock.Any(), "fs-1").Return(&cloud.FileSystem{DnsName: "dns"}, nil)
			if _, err := driver.CreateVolume(context.Background(), volumeRequest(`[{"Key":"user","Value":"x"}]`)); err != nil {
				t.Fatalf("CreateVolume() error: %v", err)
			}
			mockCtl.Finish()
		})
	}
	t.Run("nothing set keeps the default tags", func(t *testing.T) {
		mockCtl := gomock.NewController(t)
		mockCloud := mocks.NewMockCloud(mockCtl)
		driver := controllerService{cloud: mockCloud, inFlight: internal.NewInFlight(), driverOptions: &DriverOptions{}}
		want := `[{"Key":"fsx.openzfs.csi.aws.com/cluster","Value":"true"}]`
		mockCloud.EXPECT().CreateVolume(gomock.Any(), gomock.Any()).DoAndReturn(
			func(_ context.Context, params map[string]string) (*cloud.Volume, error) {
				if params["Tags"] != want {
					t.Fatalf("Tags = %s, want %s", params["Tags"], want)
				}
				return &cloud.Volume{VolumeId: "fsvol-1", FileSystemId: "fs-1"}, nil
			})
		mockCloud.EXPECT().WaitForVolumeAvailable(gomock.Any(), "fsvol-1").Return(nil)
		mockCloud.EXPECT().DescribeFileSystem(gomock.Any(), "fs-1").Return(&cloud.FileSystem{DnsName: "dns"}, nil)
		if _, err := driver.CreateVolume(context.Background(), volumeRequest("")); err != nil {
			t.Fatalf("CreateVolume() error: %v", err)
		}
		mockCtl.Finish()
	})
	t.Run("StorageClass Tags with the driver tag fail", func(t *testing.T) {
		mockCtl := gomock.NewController(t)
		mockCloud := mocks.NewMockCloud(mockCtl)
		driver := controllerService{cloud: mockCloud, inFlight: internal.NewInFlight(), driverOptions: &DriverOptions{}}
		_, err := driver.CreateVolume(context.Background(), volumeRequest(`[{"Key":"fsx.openzfs.csi.aws.com/cluster","Value":"true"}]`))
		if status.Code(err) != codes.InvalidArgument {
			t.Fatalf("CreateVolume() error = %v, want InvalidArgument", err)
		}
		mockCtl.Finish()
	})
	t.Run("StorageClass Tags with the cluster tag fail", func(t *testing.T) {
		mockCtl := gomock.NewController(t)
		mockCloud := mocks.NewMockCloud(mockCtl)
		driver := controllerService{cloud: mockCloud, inFlight: internal.NewInFlight(), driverOptions: &DriverOptions{clusterTag: testClusterTagOptions(ClusterTagIsolationDisable, nil)}}
		_, err := driver.CreateVolume(context.Background(), volumeRequest(`[{"Key":"example.com/cluster","Value":"my-cluster"}]`))
		if status.Code(err) != codes.InvalidArgument {
			t.Fatalf("CreateVolume() error = %v, want InvalidArgument", err)
		}
		mockCtl.Finish()
	})
	t.Run("StorageClass Tags with another cluster tag value fail", func(t *testing.T) {
		mockCtl := gomock.NewController(t)
		mockCloud := mocks.NewMockCloud(mockCtl)
		driver := controllerService{cloud: mockCloud, inFlight: internal.NewInFlight(), driverOptions: &DriverOptions{clusterTag: testClusterTagOptions(ClusterTagIsolationDisable, nil)}}
		_, err := driver.CreateVolume(context.Background(), volumeRequest(`[{"Key":"example.com/cluster","Value":"other"}]`))
		if status.Code(err) != codes.InvalidArgument {
			t.Fatalf("CreateVolume() error = %v, want InvalidArgument", err)
		}
		mockCtl.Finish()
	})
	t.Run("final backup tags with another cluster tag value fail", func(t *testing.T) {
		mockCtl := gomock.NewController(t)
		mockCloud := mocks.NewMockCloud(mockCtl)
		driver := controllerService{cloud: mockCloud, inFlight: internal.NewInFlight(), driverOptions: &DriverOptions{clusterTag: testClusterTagOptions(ClusterTagIsolationDisable, nil)}}
		req := &csi.CreateVolumeRequest{
			Name:               "pvc-2",
			CapacityRange:      &csi.CapacityRange{RequiredBytes: util.GiBToBytes(64)},
			VolumeCapabilities: []*csi.VolumeCapability{stdVolCap},
			Parameters: map[string]string{
				"ResourceType":              "filesystem",
				"SkipFinalBackupOnDeletion": "false",
				"FinalBackupTagsOnDeletion": `[{"Key":"example.com/cluster","Value":"other"}]`,
			},
		}
		_, err := driver.CreateVolume(context.Background(), req)
		if status.Code(err) != codes.InvalidArgument {
			t.Fatalf("CreateVolume() error = %v, want InvalidArgument", err)
		}
		mockCtl.Finish()
	})
}

func TestCreateSnapshotClusterTags(t *testing.T) {
	mockCtl := gomock.NewController(t)
	mockCloud := mocks.NewMockCloud(mockCtl)
	driver := controllerService{cloud: mockCloud, inFlight: internal.NewInFlight(), driverOptions: &DriverOptions{clusterTag: testClusterTagOptions(ClusterTagIsolationDisable, map[string]string{"team": "a"})}}
	want := `[{"Key":"fsx.openzfs.csi.aws.com/cluster","Value":"true"},{"Key":"team","Value":"a"},{"Key":"example.com/cluster","Value":"my-cluster"}]`
	mockCloud.EXPECT().GetVolumeId(gomock.Any(), "fsvol-1").Return("fsvol-1", nil)
	mockCloud.EXPECT().CreateSnapshot(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, params map[string]string) (*cloud.Snapshot, error) {
			if params["Tags"] != want {
				t.Fatalf("Tags = %s, want %s", params["Tags"], want)
			}
			return &cloud.Snapshot{SnapshotID: "fsvolsnap-1", SourceVolumeID: "fsvol-1"}, nil
		})
	mockCloud.EXPECT().WaitForSnapshotAvailable(gomock.Any(), "fsvolsnap-1").Return(nil)
	if _, err := driver.CreateSnapshot(context.Background(), &csi.CreateSnapshotRequest{Name: "snap-1", SourceVolumeId: "fsvol-1"}); err != nil {
		t.Fatalf("CreateSnapshot() error: %v", err)
	}
	mockCtl.Finish()
}

func TestDeleteFileSystemFinalBackupTags(t *testing.T) {
	testCases := []struct {
		name       string
		deleteTags map[string]string
		want       string
	}{
		{name: "final backup gets cluster and extra tags", deleteTags: map[string]string{"SkipFinalBackup": "false", "FinalBackupTags": `[{"Key":"user","Value":"x"}]`}, want: `"FinalBackupTags":[{"Key":"user","Value":"x"},{"Key":"team","Value":"a"},{"Key":"example.com/cluster","Value":"my-cluster"}]`},
		{name: "no final backup, no tags", deleteTags: map[string]string{"SkipFinalBackup": "true"}, want: `"FinalBackupTags":null`},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockCtl := gomock.NewController(t)
			mockCloud := mocks.NewMockCloud(mockCtl)
			driver := controllerService{cloud: mockCloud, inFlight: internal.NewInFlight(), driverOptions: &DriverOptions{clusterTag: testClusterTagOptions(ClusterTagIsolationDisable, map[string]string{"team": "a"})}}
			mockCloud.EXPECT().GetDeleteParameters(gomock.Any(), "fs-1").Return(tc.deleteTags, nil)
			mockCloud.EXPECT().DeleteFileSystem(gomock.Any(), gomock.Any()).DoAndReturn(
				func(_ context.Context, params map[string]string) error {
					if !strings.Contains(params["OpenZFSConfiguration"], tc.want) {
						t.Fatalf("OpenZFSConfiguration = %s, want it to contain %s", params["OpenZFSConfiguration"], tc.want)
					}
					return nil
				})
			if _, err := driver.DeleteVolume(context.Background(), &csi.DeleteVolumeRequest{VolumeId: "fs-1"}); err != nil {
				t.Fatalf("DeleteVolume() error: %v", err)
			}
			mockCtl.Finish()
		})
	}
}

func TestEnforceLogLine(t *testing.T) {
	logs := captureLogs(t)
	mockCloud := mocks.NewMockCloud(gomock.NewController(t))
	d := controllerService{cloud: mockCloud, driverOptions: &DriverOptions{clusterTag: testClusterTagOptions(ClusterTagIsolationEnforce, nil)}}
	mockCloud.EXPECT().DescribeResource(gomock.Any(), "fsvol-2").Return(&cloud.Resource{ID: "fsvol-2", Tags: map[string]string{testTagKey: "other"}}, nil)
	mockCloud.EXPECT().DescribeResource(gomock.Any(), "fsvol-3").Return(&cloud.Resource{ID: "fsvol-3", Tags: map[string]string{}}, nil)
	_ = d.checkClusterTag(context.Background(), "fsvol-2")
	_ = d.checkClusterTag(context.Background(), "fsvol-3")
	klog.Flush()
	want := []string{
		`"ignoring resource not tagged for this cluster" resource="fsvol-2" tagKey="example.com/cluster" clusterName="my-cluster" tagValue="other"`,
		`"ignoring resource not tagged for this cluster" resource="fsvol-3" tagKey="example.com/cluster" clusterName="my-cluster" tagValue=""`,
	}
	if got := logMessages(logs.String()); !reflect.DeepEqual(got, want) {
		t.Fatalf("log lines:\n got %q\nwant %q", got, want)
	}
}

func TestEnforceIgnoresResourcesOfOtherClusters(t *testing.T) {
	ours := &cloud.Resource{ID: "fsvol-1", ARN: "arn:aws:fsx:us-east-1:111122223333:volume/fs-1/fsvol-1", Tags: map[string]string{testTagKey: testClusterName}}
	theirs := &cloud.Resource{ID: "fsvol-2", ARN: "arn:aws:fsx:us-east-1:111122223333:volume/fs-1/fsvol-2", Tags: map[string]string{testTagKey: "other"}}
	untagged := &cloud.Resource{ID: "fsvol-3", ARN: "arn:aws:fsx:us-east-1:111122223333:volume/fs-1/fsvol-3", Tags: map[string]string{AwsFsxOpenZfsDriverTagKey: "true"}}

	testCases := []struct {
		name      string
		isolation ClusterTagIsolation
		resource  *cloud.Resource
		readErr   error
		wantCode  codes.Code
		deletes   bool
	}{
		{name: "enforce deletes own volume", isolation: ClusterTagIsolationEnforce, resource: ours, wantCode: codes.OK, deletes: true},
		{name: "enforce ignores other cluster's volume", isolation: ClusterTagIsolationEnforce, resource: theirs, wantCode: codes.FailedPrecondition},
		{name: "enforce ignores untagged volume", isolation: ClusterTagIsolationEnforce, resource: untagged, wantCode: codes.FailedPrecondition},
		{name: "enforce continues when the tag read fails", isolation: ClusterTagIsolationEnforce, readErr: errors.New("AccessDenied"), wantCode: codes.OK, deletes: true},
		{name: "enforce continues when the resource is gone", isolation: ClusterTagIsolationEnforce, readErr: cloud.ErrNotFound, wantCode: codes.OK, deletes: true},
		{name: "backfill does not filter", isolation: ClusterTagIsolationBackfill, resource: theirs, wantCode: codes.OK, deletes: true},
		{name: "disable does not filter", isolation: ClusterTagIsolationDisable, resource: theirs, wantCode: codes.OK, deletes: true},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockCtl := gomock.NewController(t)
			mockCloud := mocks.NewMockCloud(mockCtl)
			driver := controllerService{cloud: mockCloud, inFlight: internal.NewInFlight(), driverOptions: &DriverOptions{clusterTag: testClusterTagOptions(tc.isolation, nil)}}
			id := "fsvol-1"
			if tc.resource != nil {
				id = tc.resource.ID
			}
			if tc.isolation == ClusterTagIsolationEnforce {
				mockCloud.EXPECT().DescribeResource(gomock.Any(), id).Return(tc.resource, tc.readErr)
			}
			if tc.deletes {
				mockCloud.EXPECT().GetDeleteParameters(gomock.Any(), id).Return(map[string]string{}, nil)
				mockCloud.EXPECT().DeleteVolume(gomock.Any(), gomock.Any()).Return(nil)
			}
			_, err := driver.DeleteVolume(context.Background(), &csi.DeleteVolumeRequest{VolumeId: id})
			if status.Code(err) != tc.wantCode {
				t.Fatalf("DeleteVolume() error = %v, want code %v", err, tc.wantCode)
			}
			mockCtl.Finish()
		})
	}

	t.Run("enforce ignores other cluster's snapshot", func(t *testing.T) {
		mockCtl := gomock.NewController(t)
		mockCloud := mocks.NewMockCloud(mockCtl)
		driver := controllerService{cloud: mockCloud, inFlight: internal.NewInFlight(), driverOptions: &DriverOptions{clusterTag: testClusterTagOptions(ClusterTagIsolationEnforce, nil)}}
		mockCloud.EXPECT().DescribeResource(gomock.Any(), "fsvolsnap-1").Return(&cloud.Resource{ID: "fsvolsnap-1", Tags: map[string]string{}}, nil)
		_, err := driver.DeleteSnapshot(context.Background(), &csi.DeleteSnapshotRequest{SnapshotId: "fsvolsnap-1"})
		if status.Code(err) != codes.FailedPrecondition {
			t.Fatalf("DeleteSnapshot() error = %v, want FailedPrecondition", err)
		}
		mockCtl.Finish()
	})
	t.Run("enforce ignores snapshots of other cluster's volume", func(t *testing.T) {
		mockCtl := gomock.NewController(t)
		mockCloud := mocks.NewMockCloud(mockCtl)
		driver := controllerService{cloud: mockCloud, inFlight: internal.NewInFlight(), driverOptions: &DriverOptions{clusterTag: testClusterTagOptions(ClusterTagIsolationEnforce, nil)}}
		mockCloud.EXPECT().GetVolumeId(gomock.Any(), "fsvol-2").Return("fsvol-2", nil)
		mockCloud.EXPECT().DescribeResource(gomock.Any(), "fsvol-2").Return(theirs, nil)
		_, err := driver.CreateSnapshot(context.Background(), &csi.CreateSnapshotRequest{Name: "snap", SourceVolumeId: "fsvol-2"})
		if status.Code(err) != codes.FailedPrecondition {
			t.Fatalf("CreateSnapshot() error = %v, want FailedPrecondition", err)
		}
		mockCtl.Finish()
	})
}
