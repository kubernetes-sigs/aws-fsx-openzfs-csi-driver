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
	"strconv"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/fsx"
	"github.com/aws/aws-sdk-go-v2/service/fsx/types"
	"github.com/golang/mock/gomock"
	"github.com/kubernetes-sigs/aws-fsx-openzfs-csi-driver/pkg/cloud/mocks"
	"github.com/kubernetes-sigs/aws-fsx-openzfs-csi-driver/pkg/util"
)

func TestCreateFileSystem(t *testing.T) {
	var (
		fileSystemId       = aws.String("fs-1234")
		dnsName            = aws.String("https://aws.com")
		storageCapacity    = aws.Int32(64)
		parameters         map[string]string
		requiredParameters map[string]string
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
						StorageCapacity: storageCapacity,
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

				if resp.StorageCapacity != aws.ToInt32(storageCapacity) {
					t.Fatalf("StorageCapacity mismatches. actual: %v expected: %v", resp.StorageCapacity, storageCapacity)
				}

				mockCtl.Finish()
			},
		},
		{
			name: "success: required variables",
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
						StorageCapacity: storageCapacity,
					},
				}

				ctx := context.Background()
				mockFSx.EXPECT().CreateFileSystem(gomock.Eq(ctx), gomock.Any()).Return(output, nil)
				resp, err := c.CreateFileSystem(ctx, requiredParameters)
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

				if resp.StorageCapacity != aws.ToInt32(storageCapacity) {
					t.Fatalf("StorageCapacity mismatches. actual: %v expected: %v", resp.StorageCapacity, storageCapacity)
				}

				mockCtl.Finish()
			},
		},
		{
			name: "fail: parameter value not a json",
			testFunc: func(t *testing.T) {
				mockCtl := gomock.NewController(t)
				mockFSx := mocks.NewMockFSx(mockCtl)
				c := &cloud{
					fsx: mockFSx,
				}

				badParameters := requiredParameters
				badParameters["Tags"] = "{"

				ctx := context.Background()
				_, err := c.CreateFileSystem(ctx, badParameters)
				if err == nil {
					t.Fatal("CreateFileSystem is not failed")
				}

				mockCtl.Finish()
			},
		},
		{
			name: "fail: CreateFileSystem return ErrCodeIncompatibleParameterError error",
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
					t.Fatal("CreateFileSystem is not ErrAlreadyExists")
				}

				mockCtl.Finish()
			},
		},
		{
			name: "fail: CreateFileSystem return error",
			testFunc: func(t *testing.T) {
				mockCtl := gomock.NewController(t)
				mockFSx := mocks.NewMockFSx(mockCtl)
				c := &cloud{
					fsx: mockFSx,
				}

				ctx := context.Background()
				mockFSx.EXPECT().CreateFileSystem(gomock.Eq(ctx), gomock.Any()).Return(nil, errors.New("CreateFileSystem failed"))
				_, err := c.CreateFileSystem(ctx, parameters)
				if err == nil {
					t.Fatal("CreateFileSystem is not failed")
				}

				mockCtl.Finish()
			},
		},
	}

	for _, tc := range testCases {
		parameters = map[string]string{
			"ClientRequestToken":   `"Test"`,
			"FileSystemType":       `"OPENZFS"`,
			"KmsKeyId":             `"1234abcd-12ab-34cd-56ef-1234567890ab"`,
			"OpenZFSConfiguration": `{"AutomaticBackupRetentionDays":1,"CopyTagsToBackups":false,"CopyTagsToVolumes":false,"DailyAutomaticBackupStartTime":"00:00","DeploymentType":"SINGLE_AZ_1","DiskIopsConfiguration":{"Iops":300,"Mode":"USER_PROVISIONED"},"RootVolumeConfiguration":{"CopyTagsToSnapshots":false,"DataCompressionType":"NONE","NfsExports":[{"ClientConfigurations":[{"Clients":"*","Options":["rw","crossmnt"]}]}],"ReadOnly":true,"RecordSizeKiB":null,"UserAndGroupQuotas":[{"Id":1,"StorageCapacityQuotaGiB":10,"Type":"User"}]},"ThroughputCapacity":64,"WeeklyMaintenanceStartTime":"7:09:00"}`,
			"SecurityGroupIds":     `["test","test2"]`,
			"StorageCapacity":      strconv.Itoa(int(*storageCapacity)),
			"StorageType":          `"SSD"`,
			"SubnetIds":            `["test","test2"]`,
			"Tags":                 `[{"Key": "OPENZFS", "Value": "TRUE"}]`,
		}
		requiredParameters = map[string]string{
			"ClientRequestToken":   `"Test"`,
			"FileSystemType":       `"OPENZFS"`,
			"OpenZFSConfiguration": `{"DeploymentType":"SINGLE_AZ_1","ThroughputCapacity":64}`,
			"StorageCapacity":      strconv.Itoa(int(*storageCapacity)),
			"SubnetIds":            `["test","test2"]`,
		}

		t.Run(tc.name, tc.testFunc)
	}
}

func TestResizeFileSystem(t *testing.T) {
	var (
		fileSystemId         = "fs-1234"
		currentSizeGiB int32 = 100
		newSizeGiB     int32 = 150
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

				updateOutput := &fsx.UpdateFileSystemOutput{
					FileSystem: &types.FileSystem{
						FileSystemId:    aws.String(fileSystemId),
						StorageCapacity: aws.Int32(newSizeGiB),
					},
				}

				ctx := context.Background()
				mockFSx.EXPECT().UpdateFileSystem(gomock.Eq(ctx), gomock.Any()).Return(updateOutput, nil)
				resp, err := c.ResizeFileSystem(ctx, fileSystemId, newSizeGiB)
				if err != nil {
					t.Fatalf("ResizeFileSystem is failed: %v", err)
				}

				if resp == nil {
					t.Fatal("resp is nil")
				}

				if *resp != newSizeGiB {
					t.Fatalf("newSizeGiB mismatches. actual: %v expected: %v", resp, newSizeGiB)
				}

				mockCtl.Finish()
			},
		},
		{
			name: "success: existing administrativeAction",
			testFunc: func(t *testing.T) {
				mockCtl := gomock.NewController(t)
				mockFSx := mocks.NewMockFSx(mockCtl)
				c := &cloud{
					fsx: mockFSx,
				}

				describeOutput := &fsx.DescribeFileSystemsOutput{
					FileSystems: []types.FileSystem{
						{
							AdministrativeActions: []types.AdministrativeAction{
								{
									AdministrativeActionType: types.AdministrativeActionTypeFileSystemUpdate,
									TargetFileSystemValues: &types.FileSystem{
										StorageCapacity: aws.Int32(newSizeGiB),
									},
								},
							},
							FileSystemId:    aws.String(fileSystemId),
							StorageCapacity: aws.Int32(currentSizeGiB),
						},
					},
				}

				ctx := context.Background()
				badRequestErr := &types.BadRequest{
					Message: aws.String("Unable to perform the storage capacity update. There is an update already in progress."),
				}
				mockFSx.EXPECT().UpdateFileSystem(gomock.Eq(ctx), gomock.Any()).Return(nil, badRequestErr)
				mockFSx.EXPECT().DescribeFileSystems(gomock.Eq(ctx), gomock.Any()).Return(describeOutput, nil)
				resp, err := c.ResizeFileSystem(ctx, fileSystemId, newSizeGiB)
				if err != nil {
					t.Fatalf("ResizeFileSystem is failed: %v", err)
				}

				if resp == nil {
					t.Fatal("resp is nil")
				}

				if *resp != newSizeGiB {
					t.Fatalf("newSizeGiB mismatches. actual: %v expected: %v", resp, newSizeGiB)
				}

				mockCtl.Finish()
			},
		},
		{
			name: "fail: update error",
			testFunc: func(t *testing.T) {
				mockCtl := gomock.NewController(t)
				mockFSx := mocks.NewMockFSx(mockCtl)
				c := &cloud{
					fsx: mockFSx,
				}

				ctx := context.Background()
				mockFSx.EXPECT().UpdateFileSystem(gomock.Eq(ctx), gomock.Any()).Return(nil, errors.New(""))
				_, err := c.ResizeFileSystem(ctx, fileSystemId, newSizeGiB)
				if err == nil {
					t.Fatalf("ResizeFileSystem is not failed")
				}

				mockCtl.Finish()
			},
		},
		{
			name: "fail: existing administrativeAction with incorrect capacity",
			testFunc: func(t *testing.T) {
				mockCtl := gomock.NewController(t)
				mockFSx := mocks.NewMockFSx(mockCtl)
				c := &cloud{
					fsx: mockFSx,
				}

				describeOutput := &fsx.DescribeFileSystemsOutput{
					FileSystems: []types.FileSystem{
						{
							AdministrativeActions: []types.AdministrativeAction{
								{
									AdministrativeActionType: types.AdministrativeActionTypeFileSystemUpdate,
									TargetFileSystemValues: &types.FileSystem{
										StorageCapacity: aws.Int32(currentSizeGiB),
									},
								},
							},
							FileSystemId:    aws.String(fileSystemId),
							StorageCapacity: aws.Int32(currentSizeGiB),
						},
					},
				}

				ctx := context.Background()
				badRequestErr := &types.BadRequest{
					Message: aws.String("Unable to perform the storage capacity update. There is an update already in progress."),
				}
				mockFSx.EXPECT().UpdateFileSystem(gomock.Eq(ctx), gomock.Any()).Return(nil, badRequestErr)
				mockFSx.EXPECT().DescribeFileSystems(gomock.Eq(ctx), gomock.Any()).Return(describeOutput, nil)
				_, err := c.ResizeFileSystem(ctx, fileSystemId, newSizeGiB)
				if err == nil {
					t.Fatalf("ResizeFileSystem is not failed")
				}

				mockCtl.Finish()
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, tc.testFunc)
	}
}

func TestDeleteFileSystem(t *testing.T) {
	var (
		parameters         map[string]string
		requiredParameters map[string]string
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

				ctx := context.Background()
				mockFSx.EXPECT().DeleteFileSystem(gomock.Eq(ctx), gomock.Any()).Return(nil, nil)
				err := c.DeleteFileSystem(ctx, parameters)
				if err != nil {
					t.Fatalf("DeleteFileSystem is failed: %v", err)
				}

				mockCtl.Finish()
			},
		},
		{
			name: "success: required parameters",
			testFunc: func(t *testing.T) {
				mockCtl := gomock.NewController(t)
				mockFSx := mocks.NewMockFSx(mockCtl)
				c := &cloud{
					fsx: mockFSx,
				}

				ctx := context.Background()
				mockFSx.EXPECT().DeleteFileSystem(gomock.Eq(ctx), gomock.Any()).Return(nil, nil)
				err := c.DeleteFileSystem(ctx, requiredParameters)
				if err != nil {
					t.Fatalf("DeleteFileSystem is failed: %v", err)
				}

				mockCtl.Finish()
			},
		},
		{
			name: "fail: DeleteFileSystem return error",
			testFunc: func(t *testing.T) {
				mockCtl := gomock.NewController(t)
				mockFSx := mocks.NewMockFSx(mockCtl)
				c := &cloud{
					fsx: mockFSx,
				}

				ctx := context.Background()
				mockFSx.EXPECT().DeleteFileSystem(gomock.Eq(ctx), gomock.Any()).Return(nil, errors.New("DeleteFileSystem failed"))
				err := c.DeleteFileSystem(ctx, requiredParameters)
				if err == nil {
					t.Fatal("DeleteFileSystem is not failed")
				}

				mockCtl.Finish()
			},
		},
	}

	for _, tc := range testCases {
		parameters = map[string]string{
			"FileSystemId":         `"fs-123456789abcdefgh"`,
			"OpenZFSConfiguration": `{"FinalBackupTags": [{"Key": "OPENZFS", "Value": "OPENZFS"}], "Options": ["DELETE_CHILD_VOLUMES_AND_SNAPSHOTS"], "SkipFinalBackup": true}`,
		}
		requiredParameters = map[string]string{
			"FileSystemId": `"fs-123456789abcdefgh"`,
		}

		t.Run(tc.name, tc.testFunc)
	}
}

func TestDescribeFileSystem(t *testing.T) {
	var (
		deploymentType     = types.OpenZFSDeploymentTypeSingleAz1
		throughputCapacity = aws.Int32(64)
		storageCapacity    = aws.Int32(64)
		fileSystemId       = aws.String("fs-1234567890abcdefgh")
		dnsName            = aws.String("https://aws.com")
		rootVolumeId       = aws.String("fsvol-03062e7ff37662dff")
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

				output := &fsx.DescribeFileSystemsOutput{
					FileSystems: []types.FileSystem{
						{
							DNSName:         dnsName,
							FileSystemId:    fileSystemId,
							StorageCapacity: storageCapacity,
							OpenZFSConfiguration: &types.OpenZFSFileSystemConfiguration{
								DeploymentType:     deploymentType,
								RootVolumeId:       rootVolumeId,
								ThroughputCapacity: throughputCapacity,
							},
						},
					},
				}

				ctx := context.Background()
				mockFSx.EXPECT().DescribeFileSystems(gomock.Eq(ctx), gomock.Any()).Return(output, nil)
				_, err := c.DescribeFileSystem(ctx, *fileSystemId)
				if err != nil {
					t.Fatalf("DeleteFileSystem is failed: %v", err)
				}

				mockCtl.Finish()
			},
		},
		{
			name: "fail: DescribeFileSystemWithContext return error",
			testFunc: func(t *testing.T) {
				mockCtl := gomock.NewController(t)
				mockFSx := mocks.NewMockFSx(mockCtl)
				c := &cloud{
					fsx: mockFSx,
				}

				ctx := context.Background()
				mockFSx.EXPECT().DescribeFileSystems(gomock.Eq(ctx), gomock.Any()).Return(nil, errors.New("DescribeFileSystems failed"))
				_, err := c.DescribeFileSystem(ctx, *fileSystemId)
				if err == nil {
					t.Fatal("DescribeFileSystem is not failed")
				}

				mockCtl.Finish()
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, tc.testFunc)
	}
}

func TestWaitForFileSystemAvailable(t *testing.T) {
	var (
		filesystemId = "fs-1234"
	)
	testCases := []struct {
		name     string
		testFunc func(t *testing.T)
	}{
		{
			name: "success: filesystem available",
			testFunc: func(t *testing.T) {
				mockCtl := gomock.NewController(t)
				mockFSx := mocks.NewMockFSx(mockCtl)
				c := &cloud{
					fsx: mockFSx,
				}
				input := &fsx.DescribeFileSystemsInput{
					FileSystemIds: []string{filesystemId},
				}
				output := &fsx.DescribeFileSystemsOutput{
					FileSystems: []types.FileSystem{
						{
							FileSystemId: aws.String(filesystemId),
							Lifecycle:    types.FileSystemLifecycleAvailable,
						},
					},
				}

				ctx := context.Background()
				mockFSx.EXPECT().DescribeFileSystems(gomock.Eq(ctx), gomock.Eq(input)).Return(output, nil)
				err := c.WaitForFileSystemAvailable(ctx, filesystemId)
				if err != nil {
					t.Fatalf("WaitForFileSystemAvailable failed: %v", err)
				}

				mockCtl.Finish()
			},
		},
		{
			name: "fail: filesystem failed",
			testFunc: func(t *testing.T) {
				mockCtl := gomock.NewController(t)
				mockFSx := mocks.NewMockFSx(mockCtl)
				c := &cloud{
					fsx: mockFSx,
				}
				input := &fsx.DescribeFileSystemsInput{
					FileSystemIds: []string{filesystemId},
				}
				output := &fsx.DescribeFileSystemsOutput{
					FileSystems: []types.FileSystem{
						{
							FileSystemId: aws.String(filesystemId),
							Lifecycle:    types.FileSystemLifecycleFailed,
						},
					},
				}

				ctx := context.Background()
				mockFSx.EXPECT().DescribeFileSystems(gomock.Eq(ctx), gomock.Eq(input)).Return(output, nil)
				err := c.WaitForFileSystemAvailable(ctx, filesystemId)
				if err == nil {
					t.Fatal("WaitForFileSystemAvailable is not failed")
				}

				mockCtl.Finish()
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, tc.testFunc)
	}
}

func TestWaitForFileSystemResize(t *testing.T) {
	var (
		filesystemId       = "fs-1234"
		resizeGiB    int32 = 100
	)
	testCases := []struct {
		name     string
		testFunc func(t *testing.T)
	}{
		{
			name: "success: resize complete",
			testFunc: func(t *testing.T) {
				mockCtl := gomock.NewController(t)
				mockFSx := mocks.NewMockFSx(mockCtl)
				c := &cloud{
					fsx: mockFSx,
				}

				output := &fsx.DescribeFileSystemsOutput{
					FileSystems: []types.FileSystem{
						{
							AdministrativeActions: []types.AdministrativeAction{
								{
									AdministrativeActionType: types.AdministrativeActionTypeFileSystemUpdate,
									Status:                   types.StatusCompleted,
									TargetFileSystemValues: &types.FileSystem{
										StorageCapacity: aws.Int32(resizeGiB),
									},
								},
							},
							FileSystemId: aws.String(filesystemId),
						},
					},
				}

				ctx := context.Background()
				mockFSx.EXPECT().DescribeFileSystems(gomock.Eq(ctx), gomock.Any()).Return(output, nil)
				err := c.WaitForFileSystemResize(ctx, filesystemId, resizeGiB)
				if err != nil {
					t.Fatalf("WaitForFileSystemResize failed: %v", err)
				}

				mockCtl.Finish()
			},
		},
		{
			name: "fail: resize failed",
			testFunc: func(t *testing.T) {
				mockCtl := gomock.NewController(t)
				mockFSx := mocks.NewMockFSx(mockCtl)
				c := &cloud{
					fsx: mockFSx,
				}

				output := &fsx.DescribeFileSystemsOutput{
					FileSystems: []types.FileSystem{
						{
							AdministrativeActions: []types.AdministrativeAction{
								{
									AdministrativeActionType: types.AdministrativeActionTypeFileSystemUpdate,
									Status:                   types.StatusFailed,
									TargetFileSystemValues: &types.FileSystem{
										StorageCapacity: aws.Int32(resizeGiB),
									},
									FailureDetails: &types.AdministrativeActionFailureDetails{
										Message: aws.String("Update failed"),
									},
								},
							},
							FileSystemId: aws.String(filesystemId),
						},
					},
				}

				ctx := context.Background()
				mockFSx.EXPECT().DescribeFileSystems(gomock.Eq(ctx), gomock.Any()).Return(output, nil)
				err := c.WaitForFileSystemResize(ctx, filesystemId, resizeGiB)
				if err == nil {
					t.Fatalf("WaitForFileSystemResize is not failed: %v", err)
				}

				mockCtl.Finish()
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, tc.testFunc)
	}
}

func TestCreateVolume(t *testing.T) {
	var (
		fileSystemId       = aws.String("fs-1234")
		volumePath         = aws.String("/subVolume")
		volumeId           = aws.String("fsvol-0987654321abcdefg")
		parameters         map[string]string
		snapshotParameters map[string]string
		requiredParameters map[string]string
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
			name: "success: required variables",
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
				resp, err := c.CreateVolume(ctx, requiredParameters)
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
			name: "success: snapshot variables",
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
				resp, err := c.CreateVolume(ctx, snapshotParameters)
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
			name: "fail: parameter value not a json",
			testFunc: func(t *testing.T) {
				mockCtl := gomock.NewController(t)
				mockFSx := mocks.NewMockFSx(mockCtl)
				c := &cloud{
					fsx: mockFSx,
				}

				badParameters := requiredParameters
				badParameters["Tags"] = "{"

				ctx := context.Background()
				_, err := c.CreateFileSystem(ctx, badParameters)
				if err == nil {
					t.Fatal("CreateFileSystem is not failed")
				}

				mockCtl.Finish()
			},
		},
		{
			name: "fail: CreateVolume return ErrCodeIncompatibleParameterError error",
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
		{
			name: "fail: CreateVolume return error",
			testFunc: func(t *testing.T) {
				mockCtl := gomock.NewController(t)
				mockFSx := mocks.NewMockFSx(mockCtl)
				c := &cloud{
					fsx: mockFSx,
				}

				ctx := context.Background()
				mockFSx.EXPECT().CreateVolume(gomock.Eq(ctx), gomock.Any()).Return(nil, errors.New("CreateFileSystem failed"))
				_, err := c.CreateVolume(ctx, parameters)
				if err == nil {
					t.Fatal("CreateVolume is not failed")
				}

				mockCtl.Finish()
			},
		},
	}

	for _, tc := range testCases {
		parameters = map[string]string{
			"ClientRequestToken":   `"test"`,
			"Name":                 `"fsx"`,
			"OpenZFSConfiguration": `{"CopyTagsToSnapshots":false,"DataCompressionType":"NONE","NfsExports":[{"ClientConfigurations":[{"Clients":"*","Options":["rw","crossmnt"]}]}],"OriginSnapshot":null,"ParentVolumeId":"fsvol-03062e7ff37662dff","ReadOnly":false,"RecordSizeKiB":128,"StorageCapacityQuotaGiB":null,"StorageCapacityReservationGiB":null,"UserAndGroupQuotas":[{"Id":1,"StorageCapacityQuotaGiB":10,"Type":"User"}]}`,
			"Tags":                 `[{"Key": "OPENZFS", "Value": "TRUE"}]`,
			"VolumeType":           `"OPENZFS"`,
		}
		snapshotParameters = map[string]string{
			"ClientRequestToken":   `"test"`,
			"Name":                 `"fsx"`,
			"OpenZFSConfiguration": `{"CopyTagsToSnapshots":null,"DataCompressionType":null,"NfsExports":null,"OriginSnapshot":{"CopyStrategy":"CLONE","SnapshotARN":"arn:"},"ParentVolumeId":"fsvol-03062e7ff37662dff","ReadOnly":null,"RecordSizeKiB":null,"StorageCapacityQuotaGiB":null,"StorageCapacityReservationGiB":null,"UserAndGroupQuotas":null}`,
			"VolumeType":           `"OPENZFS"`,
		}
		requiredParameters = map[string]string{
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
		parameters         map[string]string
		requiredParameters map[string]string
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
			name: "success: required variables",
			testFunc: func(t *testing.T) {
				mockCtl := gomock.NewController(t)
				mockFSx := mocks.NewMockFSx(mockCtl)
				c := &cloud{
					fsx: mockFSx,
				}

				ctx := context.Background()
				mockFSx.EXPECT().DeleteVolume(gomock.Eq(ctx), gomock.Any()).Return(nil, nil)
				err := c.DeleteVolume(ctx, requiredParameters)
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
			"VolumeId":             `"fsvol-0987654321abcdefg"`,
			"OpenZFSConfiguration": `{"FinalBackupTags":null,"Options":["DELETE_CHILD_VOLUMES_AND_SNAPSHOTS"],"SkipFinalBackup":null}`,
		}
		requiredParameters = map[string]string{
			"VolumeId": `"fsvol-0987654321abcdefg"`,
		}
		t.Run(tc.name, tc.testFunc)
	}
}

func TestDescribeVolume(t *testing.T) {
	var (
		fileSystemId                  = aws.String("fs-1234567890abcdefgh")
		parentVolumeId                = aws.String("fsvol-03062e7ff37662dff")
		storageCapacityQuotaGiB       = aws.Int32(10)
		storageCapacityReservationGiB = aws.Int32(10)
		volumePath                    = aws.String("/subVolume")
		volumeId                      = aws.String("fsvol-0987654321abcdefg")
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

				output := &fsx.DescribeVolumesOutput{
					Volumes: []types.Volume{
						{
							FileSystemId: fileSystemId,
							OpenZFSConfiguration: &types.OpenZFSVolumeConfiguration{
								ParentVolumeId:                parentVolumeId,
								StorageCapacityQuotaGiB:       storageCapacityQuotaGiB,
								StorageCapacityReservationGiB: storageCapacityReservationGiB,
								VolumePath:                    volumePath,
							},
							VolumeId: volumeId,
						},
					},
				}

				ctx := context.Background()
				mockFSx.EXPECT().DescribeVolumes(gomock.Eq(ctx), gomock.Any()).Return(output, nil)
				_, err := c.DescribeVolume(ctx, *volumeId)
				if err != nil {
					t.Fatalf("DescribeVolume is failed: %v", err)
				}

				mockCtl.Finish()
			},
		},
		{
			name: "fail: DescribeVolumes return error",
			testFunc: func(t *testing.T) {
				mockCtl := gomock.NewController(t)
				mockFSx := mocks.NewMockFSx(mockCtl)
				c := &cloud{
					fsx: mockFSx,
				}

				ctx := context.Background()
				mockFSx.EXPECT().DescribeVolumes(gomock.Eq(ctx), gomock.Any()).Return(nil, errors.New("DescribeVolumes failed"))
				_, err := c.DescribeVolume(ctx, *volumeId)
				if err == nil {
					t.Fatal("DescribeVolume is not failed")
				}

				mockCtl.Finish()
			},
		},
	}

	for _, tc := range testCases {
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
		{
			name: "fail: volume failed",
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
							FileSystemId: aws.String(volumeId),
							Lifecycle:    types.VolumeLifecycleFailed,
						},
					},
				}

				ctx := context.Background()
				mockFSx.EXPECT().DescribeVolumes(gomock.Eq(ctx), gomock.Eq(input)).Return(output, nil)
				err := c.WaitForVolumeAvailable(ctx, volumeId)
				if err == nil {
					t.Fatal("WaitForVolumeAvailable is not failed")
				}

				mockCtl.Finish()
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, tc.testFunc)
	}
}

func TestWaitForVolumeResize(t *testing.T) {
	var (
		volumeId        = "fsvol-1234"
		resizeGiB int32 = 100
	)
	testCases := []struct {
		name     string
		testFunc func(t *testing.T)
	}{
		{
			name: "success: resize complete",
			testFunc: func(t *testing.T) {
				mockCtl := gomock.NewController(t)
				mockFSx := mocks.NewMockFSx(mockCtl)
				c := &cloud{
					fsx: mockFSx,
				}

				output := &fsx.DescribeVolumesOutput{
					Volumes: []types.Volume{
						{
							AdministrativeActions: []types.AdministrativeAction{
								{
									AdministrativeActionType: types.AdministrativeActionTypeVolumeUpdate,
									Status:                   types.StatusCompleted,
									TargetVolumeValues: &types.Volume{
										OpenZFSConfiguration: &types.OpenZFSVolumeConfiguration{
											StorageCapacityQuotaGiB:       aws.Int32(resizeGiB),
											StorageCapacityReservationGiB: aws.Int32(resizeGiB),
										},
									},
								},
							},
							VolumeId: aws.String(volumeId),
						},
					},
				}

				ctx := context.Background()
				mockFSx.EXPECT().DescribeVolumes(gomock.Eq(ctx), gomock.Any()).Return(output, nil)
				err := c.WaitForVolumeResize(ctx, volumeId, resizeGiB)
				if err != nil {
					t.Fatalf("WaitForVolumeResize failed: %v", err)
				}

				mockCtl.Finish()
			},
		},
		{
			name: "fail: resize failed",
			testFunc: func(t *testing.T) {
				mockCtl := gomock.NewController(t)
				mockFSx := mocks.NewMockFSx(mockCtl)
				c := &cloud{
					fsx: mockFSx,
				}

				output := &fsx.DescribeVolumesOutput{
					Volumes: []types.Volume{
						{
							AdministrativeActions: []types.AdministrativeAction{
								{
									AdministrativeActionType: types.AdministrativeActionTypeVolumeUpdate,
									Status:                   types.StatusFailed,
									TargetVolumeValues: &types.Volume{
										OpenZFSConfiguration: &types.OpenZFSVolumeConfiguration{
											StorageCapacityQuotaGiB:       aws.Int32(resizeGiB),
											StorageCapacityReservationGiB: aws.Int32(resizeGiB),
										},
									},
									FailureDetails: &types.AdministrativeActionFailureDetails{
										Message: aws.String("Update failed"),
									},
								},
							},
							VolumeId: aws.String(volumeId),
						},
					},
				}

				ctx := context.Background()
				mockFSx.EXPECT().DescribeVolumes(gomock.Eq(ctx), gomock.Any()).Return(output, nil)
				err := c.WaitForVolumeResize(ctx, volumeId, resizeGiB)
				if err == nil {
					t.Fatalf("WaitForVolumeResize is not failed: %v", err)
				}

				mockCtl.Finish()
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, tc.testFunc)
	}
}

func TestCreateSnapshot(t *testing.T) {
	var (
		volumeId           = "fsvol-1234567890abcdefg"
		snapshotId         = "fsvolsnap-1234"
		creationTime       = time.Now()
		resourceARN        = "arn:"
		parameters         map[string]string
		requiredParameters map[string]string
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
					t.Fatalf("Source volume id mismatches. actual: %v expected: %v", resp.SourceVolumeID, volumeId)
				}

				if resp.CreationTime != creationTime {
					t.Fatalf("Creation time mismatches. actual: %v expected: %v", resp.CreationTime, creationTime)
				}

				mockCtl.Finish()
			},
		},
		{
			name: "success: required parameters",
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
				resp, err := c.CreateSnapshot(ctx, requiredParameters)
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
					t.Fatalf("Source volume id mismatches. actual: %v expected: %v", resp.SourceVolumeID, volumeId)
				}

				if resp.CreationTime != creationTime {
					t.Fatalf("Creation time mismatches. actual: %v expected: %v", resp.CreationTime, creationTime)
				}

				mockCtl.Finish()
			},
		},
		{
			name: "fail: parameter value not a json",
			testFunc: func(t *testing.T) {
				mockCtl := gomock.NewController(t)
				mockFSx := mocks.NewMockFSx(mockCtl)
				c := &cloud{
					fsx: mockFSx,
				}

				badParameters := requiredParameters
				badParameters["Tags"] = "{"

				ctx := context.Background()

				_, err := c.CreateSnapshot(ctx, badParameters)
				if err == nil {
					t.Fatal("CreateSnapshot is not failed")
				}

				mockCtl.Finish()
			},
		},
		{
			name: "fail: CreateSnapshot return ErrCodeIncompatibleParameterError error",
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
		{
			name: "fail: CreateSnapshot return error",
			testFunc: func(t *testing.T) {
				mockCtl := gomock.NewController(t)
				mockFSx := mocks.NewMockFSx(mockCtl)
				c := &cloud{
					fsx: mockFSx,
				}

				ctx := context.Background()

				mockFSx.EXPECT().CreateSnapshot(gomock.Eq(ctx), gomock.Any()).Return(nil, errors.New(""))
				_, err := c.CreateSnapshot(ctx, parameters)
				if err == nil {
					t.Fatal("CreateSnapshot is not failed")
				}

				mockCtl.Finish()
			},
		},
	}

	for _, tc := range testCases {
		parameters = map[string]string{
			"ClientRequestToken": `"test"`,
			"Name":               `"OPENZFS"`,
			"Tags":               `[{"Key": "OPENZFS", "Value": "TRUE"}]`,
			"VolumeId":           `"fsvol-1234567890abcdefg"`,
		}
		requiredParameters = map[string]string{
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

func TestDescribeSnapshot(t *testing.T) {
	var (
		creationTime = aws.Time(time.Now())
		resourceARN  = aws.String("arn:")
		snapshotId   = aws.String("fsvolsnap-1234")
		volumeId     = aws.String("fsvol-0987654321abcdefg")
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

				output := &fsx.DescribeSnapshotsOutput{
					Snapshots: []types.Snapshot{
						{
							CreationTime: creationTime,
							ResourceARN:  resourceARN,
							SnapshotId:   snapshotId,
							VolumeId:     volumeId,
						},
					},
				}

				ctx := context.Background()
				mockFSx.EXPECT().DescribeSnapshots(gomock.Eq(ctx), gomock.Any()).Return(output, nil)
				_, err := c.DescribeSnapshot(ctx, *snapshotId)
				if err != nil {
					t.Fatalf("DescribeSnapshot is failed: %v", err)
				}

				mockCtl.Finish()
			},
		},
		{
			name: "fail: DescribeSnapshots return error",
			testFunc: func(t *testing.T) {
				mockCtl := gomock.NewController(t)
				mockFSx := mocks.NewMockFSx(mockCtl)
				c := &cloud{
					fsx: mockFSx,
				}

				ctx := context.Background()
				mockFSx.EXPECT().DescribeSnapshots(gomock.Eq(ctx), gomock.Any()).Return(nil, errors.New("DescribeSnapshots failed"))
				_, err := c.DescribeSnapshot(ctx, *snapshotId)
				if err == nil {
					t.Fatal("DescribeSnapshot is not failed")
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
			name: "fail: snapshot creation failed",
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
							Lifecycle:  "FAILED",
						},
					},
				}
				ctx := context.Background()
				mockFSx.EXPECT().DescribeSnapshots(gomock.Eq(ctx), gomock.Eq(input)).Return(output, nil)
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

func TestGetDeleteParameters(t *testing.T) {
	var (
		fileSystemId         = "fs-12345"
		volumeId             = "fsvol-12345"
		resourceArn          = aws.String("arn:")
		filesystemParameters = map[string]string{
			"SkipFinalBackup": `true`,
			"Options":         `["DELETE_CHILD_VOLUMES_AND_SNAPSHOTS"]`,
		}
		volumeParameters = map[string]string{
			"Options": `["DELETE_CHILD_VOLUMES_AND_SNAPSHOTS"]`,
		}
	)
	testCases := []struct {
		name     string
		testFunc func(t *testing.T)
	}{
		{
			name: "success: filesystem",
			testFunc: func(t *testing.T) {
				mockCtl := gomock.NewController(t)
				mockFSx := mocks.NewMockFSx(mockCtl)
				c := &cloud{
					fsx: mockFSx,
				}

				describeOutput := &fsx.DescribeFileSystemsOutput{FileSystems: []types.FileSystem{
					{
						ResourceARN: resourceArn,
					},
				}}
				listOutput := &fsx.ListTagsForResourceOutput{
					Tags: []types.Tag{
						{
							Key:   aws.String("SkipFinalBackupOnDeletion"),
							Value: aws.String("true"),
						},
						{
							Key:   aws.String("OptionsOnDeletion"),
							Value: aws.String(" -  @ DELETE_CHILD_VOLUMES_AND_SNAPSHOTS @  _ "),
						},
					},
				}

				ctx := context.Background()
				mockFSx.EXPECT().DescribeFileSystems(gomock.Eq(ctx), gomock.Any()).Return(describeOutput, nil)
				mockFSx.EXPECT().ListTagsForResource(gomock.Eq(ctx), gomock.Any()).Return(listOutput, nil)
				resp, err := c.GetDeleteParameters(ctx, fileSystemId)
				if err != nil {
					t.Fatalf("CreateFileSystem is failed: %v", err)
				}

				if resp == nil {
					t.Fatal("resp is nil")
				}

				if !reflect.DeepEqual(resp, filesystemParameters) {
					t.Fatalf("Parameters mismatches. actual: %v expected: %v", resp, filesystemParameters)
				}

				mockCtl.Finish()
			},
		},
		{
			name: "success: volume",
			testFunc: func(t *testing.T) {
				mockCtl := gomock.NewController(t)
				mockFSx := mocks.NewMockFSx(mockCtl)
				c := &cloud{
					fsx: mockFSx,
				}

				describeOutput := &fsx.DescribeVolumesOutput{Volumes: []types.Volume{
					{
						ResourceARN: resourceArn,
					},
				}}
				listOutput := &fsx.ListTagsForResourceOutput{
					Tags: []types.Tag{
						{
							Key:   aws.String("OptionsOnDeletion"),
							Value: aws.String(" -  @ DELETE_CHILD_VOLUMES_AND_SNAPSHOTS @  _ "),
						},
					},
				}

				ctx := context.Background()
				mockFSx.EXPECT().DescribeVolumes(gomock.Eq(ctx), gomock.Any()).Return(describeOutput, nil)
				mockFSx.EXPECT().ListTagsForResource(gomock.Eq(ctx), gomock.Any()).Return(listOutput, nil)
				resp, err := c.GetDeleteParameters(ctx, volumeId)
				if err != nil {
					t.Fatalf("CreateFileSystem is failed: %v", err)
				}

				if resp == nil {
					t.Fatal("resp is nil")
				}

				if !reflect.DeepEqual(resp, volumeParameters) {
					t.Fatalf("Parameters mismatches. actual: %v expected: %v", resp, volumeParameters)
				}

				mockCtl.Finish()
			},
		},
		{
			name: "success: ignore bad and random parameters",
			testFunc: func(t *testing.T) {
				mockCtl := gomock.NewController(t)
				mockFSx := mocks.NewMockFSx(mockCtl)
				c := &cloud{
					fsx: mockFSx,
				}

				describeOutput := &fsx.DescribeFileSystemsOutput{FileSystems: []types.FileSystem{
					{
						ResourceARN: resourceArn,
					},
				}}
				listOutput := &fsx.ListTagsForResourceOutput{
					Tags: []types.Tag{
						{
							Key:   aws.String("SkipFinalBackupOnDeletion"),
							Value: aws.String("fail"),
						},
						{
							Key:   aws.String("OptionsOnDeletion"),
							Value: aws.String(" -  @ DELETE_CHILD_VOLUMES_AND_SNAPSHOTS @  _ "),
						},
						{
							Key:   aws.String("TestOnDeletion"),
							Value: aws.String("true"),
						},
					},
				}

				newParameters := util.MapCopy(volumeParameters)
				delete(newParameters, "SkipFinalBackupOnDeletion")

				ctx := context.Background()
				mockFSx.EXPECT().DescribeFileSystems(gomock.Eq(ctx), gomock.Any()).Return(describeOutput, nil)
				mockFSx.EXPECT().ListTagsForResource(gomock.Eq(ctx), gomock.Any()).Return(listOutput, nil)
				resp, err := c.GetDeleteParameters(ctx, fileSystemId)
				if err != nil {
					t.Fatalf("CreateFileSystem is failed: %v", err)
				}

				if resp == nil {
					t.Fatal("resp is nil")
				}

				if !reflect.DeepEqual(resp, newParameters) {
					t.Fatalf("Parameters mismatches. actual: %v expected: %v", resp, newParameters)
				}

				mockCtl.Finish()
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, tc.testFunc)
	}
}
