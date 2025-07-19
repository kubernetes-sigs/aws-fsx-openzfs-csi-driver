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
	"strconv"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/fsx"
	"github.com/aws/aws-sdk-go-v2/service/fsx/types"
	"github.com/golang/mock/gomock"
	"github.com/kubernetes-sigs/aws-fsx-openzfs-csi-driver/pkg/cloud/mocks"
)

func TestCreateFileSystem(t *testing.T) {
	var (
		fileSystemId    = aws.String("fs-1234")
		dnsName         = aws.String("https://aws.com")
		storageCapacity = aws.Int32(64)
		parameters      map[string]string
	)
	testCases := []struct {
		name     string
		testFunc func(t *testing.T)
	}{
		{
			name: "success: all variables",
			testFunc: func(t *testing.T) {
				mockCtl := gomock.NewController(t)
				mockFSx := mocks.NewMockFSx(mockCtl)
				c := &cloud{
					fsx: mockFSx,
				}

				output := &fsx.CreateFileSystemOutput{
					FileSystem: &types.FileSystem{
						DNSName:         dnsName,
						FileSystemId:    fileSystemId,
						StorageCapacity: aws.Int32(64),
					},
				}

				ctx := context.Background()
				mockFSx.EXPECT().CreateFileSystem(gomock.Eq(ctx), gomock.Any()).Return(output, nil)
				resp, err := c.CreateFileSystem(ctx, parameters)
				if err != nil {
					t.Fatalf("CreateFileSystem is failed: %v", err)
				}

				if resp == nil {
					t.Fatal("resp is nil")
				}

				if resp.DnsName != aws.ToString(dnsName) {
					t.Fatalf("DnsName mismatches. actual: %v expected: %v", resp.DnsName, dnsName)
				}

				if resp.FileSystemId != aws.ToString(fileSystemId) {
					t.Fatalf("FileSystemId mismatches. actual: %v expected: %v", resp.FileSystemId, fileSystemId)
				}

				if resp.StorageCapacity != 64 {
					t.Fatalf("StorageCapacity mismatches. actual: %v expected: %v", resp.StorageCapacity, 64)
				}

				mockCtl.Finish()
			},
		},
		{
			name: "fail: CreateFileSystem return IncompatibleParameterError",
			testFunc: func(t *testing.T) {
				mockCtl := gomock.NewController(t)
				mockFSx := mocks.NewMockFSx(mockCtl)
				c := &cloud{
					fsx: mockFSx,
				}

				ctx := context.Background()
				mockFSx.EXPECT().CreateFileSystem(gomock.Eq(ctx), gomock.Any()).Return(nil, &types.IncompatibleParameterError{})
				_, err := c.CreateFileSystem(ctx, parameters)
				if !errors.Is(err, ErrAlreadyExists) {
					t.Fatal("CreateVolume is not ErrAlreadyExists")
				}

				mockCtl.Finish()
			},
		},
	}

	for _, tc := range testCases {
		parameters = map[string]string{
			"ClientRequestToken": `"Test"`,
			"FileSystemType":     `"OPENZFS"`,
			"StorageCapacity":    strconv.Itoa(int(*storageCapacity)),
			"SubnetIds":          `["test","test2"]`,
		}

		t.Run(tc.name, tc.testFunc)
	}
}
func TestCreateVolume(t *testing.T) {
	var (
		fileSystemId = aws.String("fs-1234")
		volumePath   = aws.String("/subVolume")
		volumeId     = aws.String("fsvol-0987654321abcdefg")
		parameters   map[string]string
	)
	testCases := []struct {
		name     string
		testFunc func(t *testing.T)
	}{
		{
			name: "success: all variables",
			testFunc: func(t *testing.T) {
				mockCtl := gomock.NewController(t)
				mockFSx := mocks.NewMockFSx(mockCtl)
				c := &cloud{
					fsx: mockFSx,
				}

				output := &fsx.CreateVolumeOutput{
					Volume: &types.Volume{
						FileSystemId: fileSystemId,
						OpenZFSConfiguration: &types.OpenZFSVolumeConfiguration{
							VolumePath: volumePath,
						},
						VolumeId: volumeId,
					},
				}

				ctx := context.Background()
				mockFSx.EXPECT().CreateVolume(gomock.Eq(ctx), gomock.Any()).Return(output, nil)
				resp, err := c.CreateVolume(ctx, parameters)
				if err != nil {
					t.Fatalf("CreateVolume is failed: %v", err)
				}

				if resp == nil {
					t.Fatal("resp is nil")
				}

				if resp.FileSystemId != aws.ToString(fileSystemId) {
					t.Fatalf("FileSystemId mismatches. actual: %v expected: %v", resp.FileSystemId, fileSystemId)
				}

				if resp.VolumePath != aws.ToString(volumePath) {
					t.Fatalf("VolumePath mismatches. actual: %v expected: %v", resp.VolumePath, volumePath)
				}

				if resp.VolumeId != aws.ToString(volumeId) {
					t.Fatalf("VolumeId mismatches. actual: %v expected: %v", resp.VolumeId, volumeId)
				}

				mockCtl.Finish()
			},
		},
		{
			name: "fail: CreateVolume return IncompatibleParameterError",
			testFunc: func(t *testing.T) {
				mockCtl := gomock.NewController(t)
				mockFSx := mocks.NewMockFSx(mockCtl)
				c := &cloud{
					fsx: mockFSx,
				}

				ctx := context.Background()
				mockFSx.EXPECT().CreateVolume(gomock.Eq(ctx), gomock.Any()).Return(nil, &types.IncompatibleParameterError{})
				_, err := c.CreateVolume(ctx, parameters)
				if !errors.Is(err, ErrAlreadyExists) {
					t.Fatal("CreateVolume is not ErrAlreadyExists")
				}

				mockCtl.Finish()
			},
		},
	}

	for _, tc := range testCases {
		parameters = map[string]string{
			"ClientRequestToken":   `"test"`,
			"Name":                 `"fsx"`,
			"OpenZFSConfiguration": `{"ParentVolumeId":"fsvol-03062e7ff37662dff"}`,
			"VolumeType":           `"OPENZFS"`,
		}

		t.Run(tc.name, tc.testFunc)
	}
}

func TestDeleteVolume(t *testing.T) {
	var (
		parameters map[string]string
	)
	testCases := []struct {
		name     string
		testFunc func(t *testing.T)
	}{
		{
			name: "success: all variables",
			testFunc: func(t *testing.T) {
				mockCtl := gomock.NewController(t)
				mockFSx := mocks.NewMockFSx(mockCtl)
				c := &cloud{
					fsx: mockFSx,
				}

				ctx := context.Background()
				mockFSx.EXPECT().DeleteVolume(gomock.Eq(ctx), gomock.Any()).Return(nil, nil)
				err := c.DeleteVolume(ctx, parameters)
				if err != nil {
					t.Fatalf("DeleteVolume is failed: %v", err)
				}

				mockCtl.Finish()
			},
		},
		{
			name: "fail: DeleteVolume return error",
			testFunc: func(t *testing.T) {
				mockCtl := gomock.NewController(t)
				mockFSx := mocks.NewMockFSx(mockCtl)
				c := &cloud{
					fsx: mockFSx,
				}

				ctx := context.Background()
				mockFSx.EXPECT().DeleteVolume(gomock.Eq(ctx), gomock.Any()).Return(nil, errors.New("DeleteVolume failed"))
				err := c.DeleteVolume(ctx, parameters)
				if err == nil {
					t.Fatal("DeleteVolume is not failed")
				}

				mockCtl.Finish()
			},
		},
	}

	for _, tc := range testCases {
		parameters = map[string]string{
			"VolumeId": `"fsvol-0987654321abcdefg"`,
		}
		t.Run(tc.name, tc.testFunc)
	}
}

func TestCreateSnapshot(t *testing.T) {
	var (
		volumeId     = "fsvol-1234567890abcdefg"
		snapshotId   = "fsvolsnap-1234"
		creationTime = time.Now()
		resourceARN  = "arn:"
		parameters   map[string]string
	)
	testCases := []struct {
		name     string
		testFunc func(t *testing.T)
	}{
		{
			name: "success: all parameters",
			testFunc: func(t *testing.T) {
				mockCtl := gomock.NewController(t)
				mockFSx := mocks.NewMockFSx(mockCtl)
				c := &cloud{
					fsx: mockFSx,
				}

				output := &fsx.CreateSnapshotOutput{
					Snapshot: &types.Snapshot{
						CreationTime: aws.Time(creationTime),
						SnapshotId:   aws.String(snapshotId),
						VolumeId:     aws.String(volumeId),
						ResourceARN:  aws.String(resourceARN),
					},
				}
				ctx := context.Background()
				mockFSx.EXPECT().CreateSnapshot(gomock.Eq(ctx), gomock.Any()).Return(output, nil)
				resp, err := c.CreateSnapshot(ctx, parameters)
				if err != nil {
					t.Fatalf("CreateSnapshot failed: %v", err)
				}

				if resp == nil {
					t.Fatal("resp is nil")
				}

				if resp.SnapshotID != snapshotId {
					t.Fatalf("Snapshot id mismatches. actual: %v expected: %v", resp.SnapshotID, snapshotId)
				}

				if resp.SourceVolumeID != volumeId {
					t.Fatalf("Source volume id mismatches. actual: %v expected: %v", resp.SourceVolumeID, volumeId)
				}

				if resp.ResourceARN != resourceARN {
					t.Fatalf("Resource ARN mismatches. actual: %v expected: %v", resp.ResourceARN, resourceARN)
				}

				if resp.CreationTime != creationTime {
					t.Fatalf("Creation time mismatches. actual: %v expected: %v", resp.CreationTime, creationTime)
				}

				mockCtl.Finish()
			},
		},
		{
			name: "fail: CreateSnapshot return IncompatibleParameterError",
			testFunc: func(t *testing.T) {
				mockCtl := gomock.NewController(t)
				mockFSx := mocks.NewMockFSx(mockCtl)
				c := &cloud{
					fsx: mockFSx,
				}

				ctx := context.Background()
				mockFSx.EXPECT().CreateSnapshot(gomock.Eq(ctx), gomock.Any()).Return(nil, &types.IncompatibleParameterError{})
				_, err := c.CreateSnapshot(ctx, parameters)
				if !errors.Is(err, ErrAlreadyExists) {
					t.Fatal("CreateSnapshot is not ErrAlreadyExists")
				}

				mockCtl.Finish()
			},
		},
	}

	for _, tc := range testCases {
		parameters = map[string]string{
			"Name":     `"OPENZFS"`,
			"VolumeId": `"fsvol-1234567890abcdefg"`,
		}

		t.Run(tc.name, tc.testFunc)
	}
}

func TestDeleteSnapshot(t *testing.T) {
	var (
		parameters map[string]string
	)
	testCases := []struct {
		name     string
		testFunc func(t *testing.T)
	}{
		{
			name: "success: required parameters",
			testFunc: func(t *testing.T) {
				mockCtl := gomock.NewController(t)
				mockFSx := mocks.NewMockFSx(mockCtl)
				c := &cloud{
					fsx: mockFSx,
				}

				ctx := context.Background()
				mockFSx.EXPECT().DeleteSnapshot(gomock.Eq(ctx), gomock.Any()).Return(nil, nil)
				err := c.DeleteSnapshot(ctx, parameters)
				if err != nil {
					t.Fatalf("DeleteSnapshot failed: %v", err)
				}

				mockCtl.Finish()
			},
		},
		{
			name: "fail: DeleteSnapshot return error",
			testFunc: func(t *testing.T) {
				mockCtl := gomock.NewController(t)
				mockFSx := mocks.NewMockFSx(mockCtl)
				c := &cloud{
					fsx: mockFSx,
				}

				ctx := context.Background()
				mockFSx.EXPECT().DeleteSnapshot(gomock.Eq(ctx), gomock.Any()).Return(nil, errors.New(""))
				err := c.DeleteSnapshot(ctx, parameters)
				if err == nil {
					t.Fatal("DeleteSnapshot is not failed")
				}

				mockCtl.Finish()
			},
		},
	}

	for _, tc := range testCases {
		parameters = map[string]string{
			"SnapshotId": `"fsvolsnap-1234"`,
		}

		t.Run(tc.name, tc.testFunc)
	}
}

func TestWaitForVolumeAvailable(t *testing.T) {
	var (
		volumeId = "fsvol-1234"
	)
	testCases := []struct {
		name     string
		testFunc func(t *testing.T)
	}{
		{
			name: "success: volume available",
			testFunc: func(t *testing.T) {
				mockCtl := gomock.NewController(t)
				mockFSx := mocks.NewMockFSx(mockCtl)
				c := &cloud{
					fsx: mockFSx,
				}

				input := &fsx.DescribeVolumesInput{
					VolumeIds: []string{volumeId},
				}
				output := &fsx.DescribeVolumesOutput{
					Volumes: []types.Volume{
						{
							VolumeId:  aws.String(volumeId),
							Lifecycle: types.VolumeLifecycleAvailable,
						},
					},
				}

				ctx := context.Background()
				mockFSx.EXPECT().DescribeVolumes(gomock.Eq(ctx), gomock.Eq(input)).Return(output, nil)
				err := c.WaitForVolumeAvailable(ctx, volumeId)
				if err != nil {
					t.Fatalf("WaitForVolumeAvailable failed: %v", err)
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
					SnapshotIds: []string{snapshotId},
				}
				output := &fsx.DescribeSnapshotsOutput{
					Snapshots: []types.Snapshot{
						{
							SnapshotId: aws.String(snapshotId),
							Lifecycle:  types.SnapshotLifecycleAvailable,
						},
					},
				}
				ctx := context.Background()
				mockFSx.EXPECT().DescribeSnapshots(gomock.Eq(ctx), gomock.Eq(input)).Return(output, nil)
				err := c.WaitForSnapshotAvailable(ctx, snapshotId)
				if err != nil {
					t.Fatalf("WaitForSnapshotAvailable failed: %v", err)
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
