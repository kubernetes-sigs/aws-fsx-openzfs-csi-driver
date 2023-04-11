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
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/fsx"
	"github.com/golang/mock/gomock"
	"reflect"
	"sigs.k8s.io/aws-fsx-openzfs-csi-driver/pkg/cloud/mocks"
	"testing"
	"time"
)

func TestCreateSnapshot(t *testing.T) {
	var (
		snapshotName   = "snapshot-1234abcd-12ab-34cd-56ef-123456abcdef"
		volVolumeId    = "fsvol-1234"
		fsVolumeId     = "fs-1234"
		fsRootVolumeId = "fsvol-5678"
		snapshotId     = "fsvolsnap-1234"
		tags           = "Tag1=Value1,Tag2=Value2"
		creationTime   = time.Now()
	)
	testCases := []struct {
		name     string
		testFunc func(t *testing.T)
	}{
		{
			name: "success: normal volume snapshot",
			testFunc: func(t *testing.T) {
				mockCtl := gomock.NewController(t)
				mockFSx := mocks.NewMockFSx(mockCtl)
				c := &cloud{
					fsx: mockFSx,
				}

				req := SnapshotOptions{
					SnapshotName:   &snapshotName,
					SourceVolumeId: &volVolumeId,
					Tags:           &tags,
				}

				output := &fsx.CreateSnapshotOutput{
					Snapshot: &fsx.Snapshot{
						CreationTime: aws.Time(creationTime),
						Name:         aws.String(snapshotName),
						SnapshotId:   aws.String(snapshotId),
						VolumeId:     aws.String(volVolumeId),
					},
				}
				ctx := context.Background()
				mockFSx.EXPECT().CreateSnapshotWithContext(gomock.Eq(ctx), gomock.Any()).Return(output, nil)
				resp, err := c.CreateSnapshot(ctx, req)
				if err != nil {
					t.Fatalf("CreateSnapshot failed: %v", err)
				}

				if resp == nil {
					t.Fatal("resp is nil")
				}

				if resp.SnapshotID != snapshotId {
					t.Fatalf("Snapshot id mismatches. actual: %v expected: %v", resp.SnapshotID, snapshotId)
				}

				if resp.SourceVolumeID != volVolumeId {
					t.Fatalf("Source volume id mismatches. actual: %v expected: %v", resp.SourceVolumeID, volVolumeId)
				}

				if resp.CreationTime != creationTime {
					t.Fatalf("Creation time mismatches. actual: %v expected: %v", resp.CreationTime, creationTime)
				}

				mockCtl.Finish()
			},
		},
		{
			name: "success: normal volume snapshot of file system root volume",
			testFunc: func(t *testing.T) {
				mockCtl := gomock.NewController(t)
				mockFSx := mocks.NewMockFSx(mockCtl)
				c := &cloud{
					fsx: mockFSx,
				}

				req := SnapshotOptions{
					SnapshotName:   &snapshotName,
					SourceVolumeId: &fsVolumeId,
					Tags:           &tags,
				}

				fs := &fsx.DescribeFileSystemsOutput{
					FileSystems: []*fsx.FileSystem{
						{
							OpenZFSConfiguration: &fsx.OpenZFSFileSystemConfiguration{
								RootVolumeId: aws.String(fsRootVolumeId),
							},
						},
					},
				}

				output := &fsx.CreateSnapshotOutput{
					Snapshot: &fsx.Snapshot{
						CreationTime: aws.Time(creationTime),
						Name:         aws.String(snapshotName),
						SnapshotId:   aws.String(snapshotId),
						VolumeId:     aws.String(fsRootVolumeId),
					},
				}
				ctx := context.Background()
				mockFSx.EXPECT().DescribeFileSystemsWithContext(gomock.Eq(ctx), gomock.Any()).Return(fs, nil)
				mockFSx.EXPECT().CreateSnapshotWithContext(gomock.Eq(ctx), gomock.Any()).Return(output, nil)
				resp, err := c.CreateSnapshot(ctx, req)
				if err != nil {
					t.Fatalf("CreateSnapshot failed: %v", err)
				}

				if resp == nil {
					t.Fatal("resp is nil")
				}

				if resp.SnapshotID != snapshotId {
					t.Fatalf("Snapshot id mismatches. actual: %v expected: %v", resp.SnapshotID, snapshotId)
				}

				if resp.SourceVolumeID != fsRootVolumeId {
					t.Fatalf("Source volume id mismatches. actual: %v expected: %v", resp.SourceVolumeID, fsRootVolumeId)
				}

				if resp.CreationTime != creationTime {
					t.Fatalf("Creation time mismatches. actual: %v expected: %v", resp.CreationTime, creationTime)
				}

				mockCtl.Finish()
			},
		},
		{
			name: "fail: missing snapshot name",
			testFunc: func(t *testing.T) {
				mockCtl := gomock.NewController(t)
				mockFSx := mocks.NewMockFSx(mockCtl)
				c := &cloud{
					fsx: mockFSx,
				}

				req := SnapshotOptions{
					SourceVolumeId: &volVolumeId,
					Tags:           &tags,
				}

				ctx := context.Background()

				_, err := c.CreateSnapshot(ctx, req)
				if err == nil {
					t.Fatal("CreateSnapshot is not failed")
				}

				mockCtl.Finish()
			},
		},
		{
			name: "fail: missing source volume id",
			testFunc: func(t *testing.T) {
				mockCtl := gomock.NewController(t)
				mockFSx := mocks.NewMockFSx(mockCtl)
				c := &cloud{
					fsx: mockFSx,
				}

				req := SnapshotOptions{
					SnapshotName: &snapshotName,
					Tags:         &tags,
				}

				ctx := context.Background()

				_, err := c.CreateSnapshot(ctx, req)
				if err == nil {
					t.Fatal("CreateSnapshot is not failed")
				}

				mockCtl.Finish()
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, tc.testFunc)
	}
}

func TestDeleteSnapshot(t *testing.T) {
	var (
		snapshotId = "fsvolsnap-1234"
	)
	testCases := []struct {
		name     string
		testFunc func(t *testing.T)
	}{
		{
			name: "success: normal",
			testFunc: func(t *testing.T) {
				mockCtl := gomock.NewController(t)
				mockFSx := mocks.NewMockFSx(mockCtl)
				c := &cloud{
					fsx: mockFSx,
				}
				output := &fsx.DeleteSnapshotOutput{}
				ctx := context.Background()
				mockFSx.EXPECT().DeleteSnapshotWithContext(gomock.Eq(ctx), gomock.Any()).Return(output, nil)
				err := c.DeleteSnapshot(ctx, snapshotId)
				if err != nil {
					t.Fatalf("DeleteSnapshot failed: %v", err)
				}

				mockCtl.Finish()
			},
		},
		{
			name: "fail: missing snapshot id",
			testFunc: func(t *testing.T) {
				mockCtl := gomock.NewController(t)
				mockFSx := mocks.NewMockFSx(mockCtl)
				c := &cloud{
					fsx: mockFSx,
				}
				ctx := context.Background()
				err := c.DeleteSnapshot(ctx, "")
				if err == nil {
					t.Fatal("DeleteSnapshot is not failed")
				}

				mockCtl.Finish()
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, tc.testFunc)
	}
}

func TestWaitForSnapshotAvailable(t *testing.T) {
	var (
		snapshotId = "fsvolsnap-1234"
	)
	testCases := []struct {
		name     string
		testFunc func(t *testing.T)
	}{
		{
			name: "success: snapshot available",
			testFunc: func(t *testing.T) {
				mockCtl := gomock.NewController(t)
				mockFSx := mocks.NewMockFSx(mockCtl)
				c := &cloud{
					fsx: mockFSx,
				}
				input := &fsx.DescribeSnapshotsInput{
					SnapshotIds: []*string{aws.String(snapshotId)},
				}
				output := &fsx.DescribeSnapshotsOutput{
					Snapshots: []*fsx.Snapshot{
						{
							SnapshotId: aws.String(snapshotId),
							Lifecycle:  aws.String(fsx.SnapshotLifecycleAvailable),
						},
					},
				}
				ctx := context.Background()
				mockFSx.EXPECT().DescribeSnapshotsWithContext(gomock.Eq(ctx), gomock.Eq(input)).Return(output, nil)
				err := c.WaitForSnapshotAvailable(ctx, snapshotId)
				if err != nil {
					t.Fatalf("WaitForSnapshotAvailable failed: %v", err)
				}

				mockCtl.Finish()
			},
		},
		{
			name: "fail: snapshot creation failed",
			testFunc: func(t *testing.T) {
				mockCtl := gomock.NewController(t)
				mockFSx := mocks.NewMockFSx(mockCtl)
				c := &cloud{
					fsx: mockFSx,
				}
				input := &fsx.DescribeSnapshotsInput{
					SnapshotIds: []*string{aws.String(snapshotId)},
				}
				output := &fsx.DescribeSnapshotsOutput{
					Snapshots: []*fsx.Snapshot{
						{
							SnapshotId: aws.String(snapshotId),
							Lifecycle:  aws.String(fsx.StatusFailed),
						},
					},
				}
				ctx := context.Background()
				mockFSx.EXPECT().DescribeSnapshotsWithContext(gomock.Eq(ctx), gomock.Eq(input)).Return(output, nil)
				err := c.WaitForSnapshotAvailable(ctx, snapshotId)
				if err == nil {
					t.Fatal("WaitForSnapshotAvailable is not failed")
				}

				mockCtl.Finish()
			},
		},
		{
			name: "fail: missing snapshot id",
			testFunc: func(t *testing.T) {
				mockCtl := gomock.NewController(t)
				mockFSx := mocks.NewMockFSx(mockCtl)
				c := &cloud{
					fsx: mockFSx,
				}
				ctx := context.Background()
				err := c.WaitForSnapshotAvailable(ctx, "")
				if err == nil {
					t.Fatal("WaitForSnapshotAvailable is not failed")
				}

				mockCtl.Finish()
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, tc.testFunc)
	}
}

func TestParseTags(t *testing.T) {
	testCases := []struct {
		name     string
		tags     string
		expected []*fsx.Tag
	}{
		{
			name: "ParseTags normal tags",
			tags: "Key1=Value1,Key2=Value2",
			expected: []*fsx.Tag{
				{
					Key:   aws.String("Key1"),
					Value: aws.String("Value1"),
				},
				{
					Key:   aws.String("Key2"),
					Value: aws.String("Value2"),
				},
			},
		},
		{
			name: "ParseTags one improperly formatted tag",
			tags: "Key1=Value1,Key2Value2",
			expected: []*fsx.Tag{
				{
					Key:   aws.String("Key1"),
					Value: aws.String("Value1"),
				},
			},
		},
		{
			name: "ParseTags missing value",
			tags: "Key1=",
			expected: []*fsx.Tag{
				{
					Key:   aws.String("Key1"),
					Value: aws.String(""),
				},
			},
		},
		{
			name:     "ParseTags empty tags",
			tags:     "",
			expected: nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actual := parseTags(tc.tags)

			if !reflect.DeepEqual(actual, tc.expected) {
				t.Fatalf("ParseTags got wrong result. actual: %s, expected: %s", actual, tc.expected)
			}
		})
	}
}
