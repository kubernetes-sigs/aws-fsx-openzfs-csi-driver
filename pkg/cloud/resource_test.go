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

package cloud

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/fsx"
	"github.com/aws/aws-sdk-go-v2/service/fsx/types"
	"github.com/golang/mock/gomock"
	"github.com/kubernetes-sigs/aws-fsx-openzfs-csi-driver/pkg/cloud/mocks"
)

func TestDescribeResource(t *testing.T) {
	tags := []types.Tag{{Key: aws.String("k"), Value: aws.String("v")}}
	mockCtl := gomock.NewController(t)
	mockFSx := mocks.NewMockFSx(mockCtl)
	c := &cloud{fsx: mockFSx}
	ctx := context.Background()

	mockFSx.EXPECT().DescribeFileSystems(ctx, &fsx.DescribeFileSystemsInput{FileSystemIds: []string{"fs-1"}}).Return(&fsx.DescribeFileSystemsOutput{FileSystems: []types.FileSystem{{
		ResourceARN: aws.String("arn:fs-1"), OpenZFSConfiguration: &types.OpenZFSFileSystemConfiguration{RootVolumeId: aws.String("fsvol-root")},
	}}}, nil)
	mockFSx.EXPECT().DescribeVolumes(ctx, &fsx.DescribeVolumesInput{VolumeIds: []string{"fsvol-1"}}).Return(&fsx.DescribeVolumesOutput{Volumes: []types.Volume{{
		ResourceARN: aws.String("arn:fsvol-1"), Name: aws.String("pvc-1"), OpenZFSConfiguration: &types.OpenZFSVolumeConfiguration{ParentVolumeId: aws.String("fsvol-parent")},
	}}}, nil)
	mockFSx.EXPECT().DescribeSnapshots(ctx, &fsx.DescribeSnapshotsInput{SnapshotIds: []string{"fsvolsnap-1"}}).Return(&fsx.DescribeSnapshotsOutput{Snapshots: []types.Snapshot{{
		ResourceARN: aws.String("arn:fsvolsnap-1"), Name: aws.String("snapshot-1"),
	}}}, nil)
	for _, arn := range []string{"arn:fs-1", "arn:fsvol-1", "arn:fsvolsnap-1"} {
		mockFSx.EXPECT().ListTagsForResource(ctx, &fsx.ListTagsForResourceInput{ResourceARN: aws.String(arn)}).Return(&fsx.ListTagsForResourceOutput{Tags: tags}, nil)
	}
	mockFSx.EXPECT().DescribeVolumes(ctx, &fsx.DescribeVolumesInput{VolumeIds: []string{"fsvol-gone"}}).Return(nil, &types.VolumeNotFound{})

	want := map[string]*Resource{
		"fs-1":        {ID: "fs-1", ARN: "arn:fs-1", Tags: map[string]string{"k": "v"}, RootVolumeID: "fsvol-root"},
		"fsvol-1":     {ID: "fsvol-1", ARN: "arn:fsvol-1", Name: "pvc-1", Tags: map[string]string{"k": "v"}, ParentVolumeID: "fsvol-parent"},
		"fsvolsnap-1": {ID: "fsvolsnap-1", ARN: "arn:fsvolsnap-1", Name: "snapshot-1", Tags: map[string]string{"k": "v"}},
	}
	for id, w := range want {
		got, err := c.DescribeResource(ctx, id)
		if err != nil {
			t.Fatalf("DescribeResource(%s) error: %v", id, err)
		}
		if !reflect.DeepEqual(got, w) {
			t.Fatalf("DescribeResource(%s) = %+v, want %+v", id, got, w)
		}
	}
	if _, err := c.DescribeResource(ctx, "fsvol-gone"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("DescribeResource(fsvol-gone) error = %v, want ErrNotFound", err)
	}
	if _, err := c.DescribeResource(ctx, "bad"); err == nil {
		t.Fatal("DescribeResource(bad) succeeded")
	}
	mockCtl.Finish()
}

func TestTagResource(t *testing.T) {
	mockCtl := gomock.NewController(t)
	mockFSx := mocks.NewMockFSx(mockCtl)
	c := &cloud{fsx: mockFSx}
	ctx := context.Background()
	mockFSx.EXPECT().TagResource(ctx, &fsx.TagResourceInput{
		ResourceARN: aws.String("arn:fsvol-1"),
		Tags:        []types.Tag{{Key: aws.String("example.com/cluster"), Value: aws.String("my-cluster")}},
	}).Return(&fsx.TagResourceOutput{}, nil)
	if err := c.TagResource(ctx, "arn:fsvol-1", map[string]string{"example.com/cluster": "my-cluster"}); err != nil {
		t.Fatalf("TagResource() error: %v", err)
	}
	mockCtl.Finish()
}
