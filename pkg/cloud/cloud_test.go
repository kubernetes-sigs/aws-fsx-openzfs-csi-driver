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
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/fsx"
	"github.com/golang/mock/gomock"
	"reflect"
	"sigs.k8s.io/aws-fsx-openzfs-csi-driver/pkg/cloud/mocks"
	"testing"
	"time"
)

func TestCreateFileSystem(t *testing.T) {
	var (
		kmsKeyId                      = aws.String("1234abcd-12ab-34cd-56ef-1234567890ab")
		automaticBackupRetentionDays  = aws.Int64(1)
		copyTagsToBackups             = aws.Bool(false)
		copyTagsToVolumes             = aws.Bool(false)
		dailyAutomaticBackupStartTime = aws.String("00:00")
		deploymentType                = aws.String("SINGLE_AZ_1")
		diskIopsConfiguration         = aws.String("Mode=USER_PROVISIONED,Iops=300")
		rootVolumeConfiguration       = aws.String("CopyTagsToSnapshots=false,DataCompressionType=NONE,NfsExports=[{ClientConfigurations=[{Clients=*,Options=[crossmnt]}]}],ReadOnly=false,RecordSizeKiB=128,UserAndGroupQuotas=[{Id=1,StorageCapacityQuotaGiB=10,Type=USER}]")
		throughputCapacity            = aws.Int64(64)
		weeklyMaintenanceStartTime    = aws.String("7:09:00")
		securityGroupIds              = []string{"sg-068000ccf82dfba88"}
		storageCapacity               = aws.Int64(64)
		subnetIds                     = []string{"subnet-0eabfaa81fb22bcaf"}
		tags                          = aws.String("Tag1=Value1,Tag2=Value2")
		fileSystemId                  = aws.String("fs-1234")
		dnsName                       = aws.String("https://aws.com")
		volumeName                    = "volumeName"
		optionsOnDeletion             = aws.String("[DELETE_CHILD_VOLUMES_AND_SNAPSHOTS]")
		skipFinalBackupOnDeletion     = aws.Bool(true)
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

				req := FileSystemOptions{
					KmsKeyId:                      kmsKeyId,
					AutomaticBackupRetentionDays:  automaticBackupRetentionDays,
					CopyTagsToBackups:             copyTagsToBackups,
					CopyTagsToVolumes:             copyTagsToVolumes,
					DailyAutomaticBackupStartTime: dailyAutomaticBackupStartTime,
					DeploymentType:                deploymentType,
					DiskIopsConfiguration:         diskIopsConfiguration,
					RootVolumeConfiguration:       rootVolumeConfiguration,
					ThroughputCapacity:            throughputCapacity,
					WeeklyMaintenanceStartTime:    weeklyMaintenanceStartTime,
					SecurityGroupIds:              securityGroupIds,
					StorageCapacity:               storageCapacity,
					SubnetIds:                     subnetIds,
					Tags:                          tags,
					OptionsOnDeletion:             optionsOnDeletion,
					SkipFinalBackupOnDeletion:     skipFinalBackupOnDeletion,
				}
				output := &fsx.CreateFileSystemOutput{
					FileSystem: &fsx.FileSystem{
						DNSName:         dnsName,
						FileSystemId:    fileSystemId,
						StorageCapacity: storageCapacity,
					},
				}

				ctx := context.Background()
				mockFSx.EXPECT().CreateFileSystemWithContext(gomock.Eq(ctx), gomock.Any()).Return(output, nil)
				resp, err := c.CreateFileSystem(ctx, volumeName, req)
				if err != nil {
					t.Fatalf("CreateFileSystem is failed: %v", err)
				}

				if resp == nil {
					t.Fatal("resp is nil")
				}

				if resp.DnsName != aws.StringValue(dnsName) {
					t.Fatalf("DnsName mismatches. actual: %v expected: %v", resp.DnsName, dnsName)
				}

				if resp.FileSystemId != aws.StringValue(fileSystemId) {
					t.Fatalf("FileSystemId mismatches. actual: %v expected: %v", resp.FileSystemId, fileSystemId)
				}

				if resp.StorageCapacity != aws.Int64Value(storageCapacity) {
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

				req := FileSystemOptions{
					DeploymentType:     deploymentType,
					ThroughputCapacity: throughputCapacity,
					StorageCapacity:    storageCapacity,
					SubnetIds:          subnetIds,
				}
				output := &fsx.CreateFileSystemOutput{
					FileSystem: &fsx.FileSystem{
						DNSName:         dnsName,
						FileSystemId:    fileSystemId,
						StorageCapacity: storageCapacity,
					},
				}

				ctx := context.Background()
				mockFSx.EXPECT().CreateFileSystemWithContext(gomock.Eq(ctx), gomock.Any()).Return(output, nil)
				resp, err := c.CreateFileSystem(ctx, volumeName, req)
				if err != nil {
					t.Fatalf("CreateFileSystem is failed: %v", err)
				}

				if resp == nil {
					t.Fatal("resp is nil")
				}

				if resp.DnsName != aws.StringValue(dnsName) {
					t.Fatalf("DnsName mismatches. actual: %v expected: %v", resp.DnsName, dnsName)
				}

				if resp.FileSystemId != aws.StringValue(fileSystemId) {
					t.Fatalf("FileSystemId mismatches. actual: %v expected: %v", resp.FileSystemId, fileSystemId)
				}

				if resp.StorageCapacity != aws.Int64Value(storageCapacity) {
					t.Fatalf("StorageCapacity mismatches. actual: %v expected: %v", resp.StorageCapacity, storageCapacity)
				}

				mockCtl.Finish()
			},
		},
		{
			name: "fail: bad diskIops",
			testFunc: func(t *testing.T) {
				mockCtl := gomock.NewController(t)
				mockFSx := mocks.NewMockFSx(mockCtl)
				c := &cloud{
					fsx: mockFSx,
				}

				badDiskIops := "Mode=USER_PROVISIONED,Iops=VALUE1"
				req := FileSystemOptions{
					DeploymentType:        deploymentType,
					ThroughputCapacity:    throughputCapacity,
					StorageCapacity:       storageCapacity,
					SubnetIds:             subnetIds,
					DiskIopsConfiguration: aws.String(badDiskIops),
				}

				ctx := context.Background()
				_, err := c.CreateFileSystem(ctx, volumeName, req)

				if err == nil {
					t.Fatal("CreateFileSystem is not failed")
				}

				mockCtl.Finish()
			},
		},
		{
			name: "fail: bad rootVolumeConfig",
			testFunc: func(t *testing.T) {
				mockCtl := gomock.NewController(t)
				mockFSx := mocks.NewMockFSx(mockCtl)
				c := &cloud{
					fsx: mockFSx,
				}

				badRootVolumeConfig := "ReadOnly=INVALID"
				req := FileSystemOptions{
					DeploymentType:          deploymentType,
					ThroughputCapacity:      throughputCapacity,
					StorageCapacity:         storageCapacity,
					SubnetIds:               subnetIds,
					RootVolumeConfiguration: aws.String(badRootVolumeConfig),
				}

				ctx := context.Background()
				_, err := c.CreateFileSystem(ctx, volumeName, req)

				if err == nil {
					t.Fatal("CreateFileSystem is not failed")
				}

				mockCtl.Finish()
			},
		},
		{
			name: "fail: CreateFileSystemWithContext return ErrCodeIncompatibleParameterError error",
			testFunc: func(t *testing.T) {
				mockCtl := gomock.NewController(t)
				mockFSx := mocks.NewMockFSx(mockCtl)
				c := &cloud{
					fsx: mockFSx,
				}

				req := FileSystemOptions{
					DeploymentType:     deploymentType,
					ThroughputCapacity: throughputCapacity,
					StorageCapacity:    storageCapacity,
					SubnetIds:          subnetIds,
				}

				ctx := context.Background()
				mockFSx.EXPECT().CreateFileSystemWithContext(gomock.Eq(ctx), gomock.Any()).Return(nil, errors.New(fsx.ErrCodeIncompatibleParameterError))
				_, err := c.CreateFileSystem(ctx, volumeName, req)
				if err == nil {
					t.Fatal("CreateFileSystem is not failed")
				}

				mockCtl.Finish()
			},
		},
		{
			name: "fail: CreateFileSystemWithContext return error",
			testFunc: func(t *testing.T) {
				mockCtl := gomock.NewController(t)
				mockFSx := mocks.NewMockFSx(mockCtl)
				c := &cloud{
					fsx: mockFSx,
				}

				req := FileSystemOptions{
					DeploymentType:     deploymentType,
					ThroughputCapacity: throughputCapacity,
					StorageCapacity:    storageCapacity,
					SubnetIds:          subnetIds,
				}

				ctx := context.Background()
				mockFSx.EXPECT().CreateFileSystemWithContext(gomock.Eq(ctx), gomock.Any()).Return(nil, errors.New("CreateFileSystemWithContext failed"))
				_, err := c.CreateFileSystem(ctx, volumeName, req)
				if err == nil {
					t.Fatal("CreateFileSystem is not failed")
				}

				mockCtl.Finish()
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, tc.testFunc)
	}
}

func TestResizeFileSystem(t *testing.T) {
	var (
		fileSystemId         = "fs-1234"
		currentSizeGiB int64 = 100
		newSizeGiB     int64 = 150
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
					FileSystem: &fsx.FileSystem{
						FileSystemId:    aws.String(fileSystemId),
						StorageCapacity: aws.Int64(newSizeGiB),
					},
				}

				ctx := context.Background()
				mockFSx.EXPECT().UpdateFileSystemWithContext(gomock.Eq(ctx), gomock.Any()).Return(updateOutput, nil)
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
					FileSystems: []*fsx.FileSystem{
						{
							AdministrativeActions: []*fsx.AdministrativeAction{
								{
									AdministrativeActionType: aws.String(fsx.AdministrativeActionTypeFileSystemUpdate),
									TargetFileSystemValues: &fsx.FileSystem{
										StorageCapacity: aws.Int64(newSizeGiB),
									},
								},
							},
							FileSystemId:    aws.String(fileSystemId),
							StorageCapacity: aws.Int64(currentSizeGiB),
						},
					},
				}

				ctx := context.Background()
				mockFSx.EXPECT().UpdateFileSystemWithContext(gomock.Eq(ctx), gomock.Any()).Return(nil, awserr.New(fsx.ErrCodeBadRequest, "Unable to perform the storage capacity update. There is an update already in progress.", errors.New("")))
				mockFSx.EXPECT().DescribeFileSystemsWithContext(gomock.Eq(ctx), gomock.Any()).Return(describeOutput, nil)
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
				mockFSx.EXPECT().UpdateFileSystemWithContext(gomock.Eq(ctx), gomock.Any()).Return(nil, errors.New(""))
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
					FileSystems: []*fsx.FileSystem{
						{
							AdministrativeActions: []*fsx.AdministrativeAction{
								{
									AdministrativeActionType: aws.String(fsx.AdministrativeActionTypeFileSystemUpdate),
									TargetFileSystemValues: &fsx.FileSystem{
										StorageCapacity: aws.Int64(currentSizeGiB),
									},
								},
							},
							FileSystemId:    aws.String(fileSystemId),
							StorageCapacity: aws.Int64(currentSizeGiB),
						},
					},
				}

				ctx := context.Background()
				mockFSx.EXPECT().UpdateFileSystemWithContext(gomock.Eq(ctx), gomock.Any()).Return(nil, awserr.New(fsx.ErrCodeBadRequest, "Unable to perform the storage capacity update. There is an update already in progress.", errors.New("")))
				mockFSx.EXPECT().DescribeFileSystemsWithContext(gomock.Eq(ctx), gomock.Any()).Return(describeOutput, nil)
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
		fileSystemId              = "fs-123456789abcdefgh"
		optionsOnDeletion         = "DELETE_CHILD_VOLUMES_AND_SNAPSHOTS"
		skipFinalBackupOnDeletion = "true"
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

				output := &fsx.DeleteFileSystemOutput{}
				describeOutput := &fsx.DescribeFileSystemsOutput{
					FileSystems: []*fsx.FileSystem{
						{
							FileSystemId: &fileSystemId,
							Tags: []*fsx.Tag{
								{
									Key:   aws.String(OptionsOnDeletionTagKey),
									Value: aws.String(optionsOnDeletion),
								},
								{
									Key:   aws.String(SkipFinalBackupOnDeletionTagKey),
									Value: aws.String(skipFinalBackupOnDeletion),
								},
							},
						},
					},
				}

				ctx := context.Background()
				mockFSx.EXPECT().DescribeFileSystemsWithContext(gomock.Eq(ctx), gomock.Any()).Return(describeOutput, nil)
				mockFSx.EXPECT().DeleteFileSystemWithContext(gomock.Eq(ctx), gomock.Any()).Return(output, nil)
				err := c.DeleteFileSystem(ctx, fileSystemId)
				if err != nil {
					t.Fatalf("DeleteFileSystem is failed: %v", err)
				}

				mockCtl.Finish()
			},
		},
		{
			name: "success: missing optionsOnDeletion tag",
			testFunc: func(t *testing.T) {
				mockCtl := gomock.NewController(t)
				mockFSx := mocks.NewMockFSx(mockCtl)
				c := &cloud{
					fsx: mockFSx,
				}

				output := &fsx.DeleteFileSystemOutput{}
				describeOutput := &fsx.DescribeFileSystemsOutput{
					FileSystems: []*fsx.FileSystem{
						{
							FileSystemId: &fileSystemId,
							Tags: []*fsx.Tag{
								{
									Key:   aws.String(SkipFinalBackupOnDeletionTagKey),
									Value: aws.String(skipFinalBackupOnDeletion),
								},
							},
						},
					},
				}

				ctx := context.Background()
				mockFSx.EXPECT().DescribeFileSystemsWithContext(gomock.Eq(ctx), gomock.Any()).Return(describeOutput, nil)
				mockFSx.EXPECT().DeleteFileSystemWithContext(gomock.Eq(ctx), gomock.Any()).Return(output, nil)
				err := c.DeleteFileSystem(ctx, fileSystemId)
				if err != nil {
					t.Fatalf("DeleteFileSystem is failed: %v", err)
				}

				mockCtl.Finish()
			},
		},
		{
			name: "success: missing skipFinalBackupOnDeletion tag",
			testFunc: func(t *testing.T) {
				mockCtl := gomock.NewController(t)
				mockFSx := mocks.NewMockFSx(mockCtl)
				c := &cloud{
					fsx: mockFSx,
				}

				output := &fsx.DeleteFileSystemOutput{}
				describeOutput := &fsx.DescribeFileSystemsOutput{
					FileSystems: []*fsx.FileSystem{
						{
							FileSystemId: &fileSystemId,
							Tags: []*fsx.Tag{
								{
									Key:   aws.String(OptionsOnDeletionTagKey),
									Value: aws.String(optionsOnDeletion),
								},
							},
						},
					},
				}

				ctx := context.Background()
				mockFSx.EXPECT().DescribeFileSystemsWithContext(gomock.Eq(ctx), gomock.Any()).Return(describeOutput, nil)
				mockFSx.EXPECT().DeleteFileSystemWithContext(gomock.Eq(ctx), gomock.Any()).Return(output, nil)
				err := c.DeleteFileSystem(ctx, fileSystemId)
				if err != nil {
					t.Fatalf("DeleteFileSystem is failed: %v", err)
				}

				mockCtl.Finish()
			},
		},
		{
			name: "success: no tags",
			testFunc: func(t *testing.T) {
				mockCtl := gomock.NewController(t)
				mockFSx := mocks.NewMockFSx(mockCtl)
				c := &cloud{
					fsx: mockFSx,
				}

				output := &fsx.DeleteFileSystemOutput{}
				describeOutput := &fsx.DescribeFileSystemsOutput{
					FileSystems: []*fsx.FileSystem{
						{
							FileSystemId: &fileSystemId,
						},
					},
				}

				ctx := context.Background()
				mockFSx.EXPECT().DescribeFileSystemsWithContext(gomock.Eq(ctx), gomock.Any()).Return(describeOutput, nil)
				mockFSx.EXPECT().DeleteFileSystemWithContext(gomock.Eq(ctx), gomock.Any()).Return(output, nil)
				err := c.DeleteFileSystem(ctx, fileSystemId)
				if err != nil {
					t.Fatalf("DeleteFileSystem is failed: %v", err)
				}

				mockCtl.Finish()
			},
		},
		{
			name: "fail: DeleteFileSystemWithContext return error",
			testFunc: func(t *testing.T) {
				mockCtl := gomock.NewController(t)
				mockFSx := mocks.NewMockFSx(mockCtl)
				c := &cloud{
					fsx: mockFSx,
				}

				describeOutput := &fsx.DescribeFileSystemsOutput{
					FileSystems: []*fsx.FileSystem{
						{
							FileSystemId: &fileSystemId,
							Tags: []*fsx.Tag{
								{
									Key:   aws.String(OptionsOnDeletionTagKey),
									Value: aws.String(optionsOnDeletion),
								},
								{
									Key:   aws.String(SkipFinalBackupOnDeletionTagKey),
									Value: aws.String(skipFinalBackupOnDeletion),
								},
							},
						},
					},
				}

				ctx := context.Background()
				mockFSx.EXPECT().DescribeFileSystemsWithContext(gomock.Eq(ctx), gomock.Any()).Return(describeOutput, nil)
				mockFSx.EXPECT().DeleteFileSystemWithContext(gomock.Eq(ctx), gomock.Any()).Return(nil, errors.New("DeleteFileSystemWithContext failed"))
				err := c.DeleteFileSystem(ctx, fileSystemId)
				if err == nil {
					t.Fatal("DeleteFileSystem is not failed")
				}

				mockCtl.Finish()
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, tc.testFunc)
	}
}

func TestDescribeFileSystem(t *testing.T) {
	var (
		deploymentType     = aws.String("SINGLE_AZ_1")
		throughputCapacity = aws.Int64(64)
		storageCapacity    = aws.Int64(64)
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
					FileSystems: []*fsx.FileSystem{
						{
							DNSName:         dnsName,
							FileSystemId:    fileSystemId,
							StorageCapacity: storageCapacity,
							OpenZFSConfiguration: &fsx.OpenZFSFileSystemConfiguration{
								DeploymentType:     deploymentType,
								RootVolumeId:       rootVolumeId,
								ThroughputCapacity: throughputCapacity,
							},
						},
					},
				}

				ctx := context.Background()
				mockFSx.EXPECT().DescribeFileSystemsWithContext(gomock.Eq(ctx), gomock.Any()).Return(output, nil)
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
				mockFSx.EXPECT().DescribeFileSystemsWithContext(gomock.Eq(ctx), gomock.Any()).Return(nil, errors.New("DescribeFileSystemsWithContext failed"))
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
					FileSystemIds: []*string{aws.String(filesystemId)},
				}
				output := &fsx.DescribeFileSystemsOutput{
					FileSystems: []*fsx.FileSystem{
						{
							FileSystemId: aws.String(filesystemId),
							Lifecycle:    aws.String(fsx.FileSystemLifecycleAvailable),
						},
					},
				}

				ctx := context.Background()
				mockFSx.EXPECT().DescribeFileSystemsWithContext(gomock.Eq(ctx), gomock.Eq(input)).Return(output, nil)
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
					FileSystemIds: []*string{aws.String(filesystemId)},
				}
				output := &fsx.DescribeFileSystemsOutput{
					FileSystems: []*fsx.FileSystem{
						{
							FileSystemId: aws.String(filesystemId),
							Lifecycle:    aws.String(fsx.FileSystemLifecycleFailed),
						},
					},
				}

				ctx := context.Background()
				mockFSx.EXPECT().DescribeFileSystemsWithContext(gomock.Eq(ctx), gomock.Eq(input)).Return(output, nil)
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
		resizeGiB    int64 = 100
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
					FileSystems: []*fsx.FileSystem{
						{
							AdministrativeActions: []*fsx.AdministrativeAction{
								{
									AdministrativeActionType: aws.String(fsx.AdministrativeActionTypeFileSystemUpdate),
									Status:                   aws.String(fsx.StatusCompleted),
									TargetFileSystemValues: &fsx.FileSystem{
										StorageCapacity: aws.Int64(resizeGiB),
									},
								},
							},
							FileSystemId: aws.String(filesystemId),
						},
					},
				}

				ctx := context.Background()
				mockFSx.EXPECT().DescribeFileSystemsWithContext(gomock.Eq(ctx), gomock.Any()).Return(output, nil)
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
					FileSystems: []*fsx.FileSystem{
						{
							AdministrativeActions: []*fsx.AdministrativeAction{
								{
									AdministrativeActionType: aws.String(fsx.AdministrativeActionTypeFileSystemUpdate),
									Status:                   aws.String(fsx.StatusFailed),
									TargetFileSystemValues: &fsx.FileSystem{
										StorageCapacity: aws.Int64(resizeGiB),
									},
									FailureDetails: &fsx.AdministrativeActionFailureDetails{
										Message: aws.String("Update failed"),
									},
								},
							},
							FileSystemId: aws.String(filesystemId),
						},
					},
				}

				ctx := context.Background()
				mockFSx.EXPECT().DescribeFileSystemsWithContext(gomock.Eq(ctx), gomock.Any()).Return(output, nil)
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
		name                          = aws.String("subVolume")
		copyTagsToSnapshots           = aws.Bool(false)
		dataCompressionType           = aws.String("NONE")
		nfsExports                    = aws.String("[{ClientConfigurations=[{Clients=*,Options=[crossmnt]}]}]")
		parentVolumeId                = aws.String("fsvol-03062e7ff37662dff")
		readOnly                      = aws.Bool(false)
		recordSizeKiB                 = aws.Int64(128)
		storageCapacityQuotaGiB       = aws.Int64(10)
		storageCapacityReservationGiB = aws.Int64(10)
		userAndGroupQuotas            = aws.String("[{Id=1,StorageCapacityQuotaGiB=10,Type=USER}]")
		tags                          = aws.String("Tag1=Value1,Tag2=Value2")
		volumeName                    = "volumeName"
		fileSystemId                  = aws.String("fs-1234")
		volumePath                    = aws.String("/subVolume")
		volumeId                      = aws.String("fsvol-0987654321abcdefg")
		dnsName                       = aws.String("https://aws.com")
		snapshotResourceArn           = aws.String("arn:")
		optionsOnDeletion             = aws.String("[DELETE_CHILD_VOLUMES_AND_SNAPSHOTS]")
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

				req := VolumeOptions{
					Name:                          name,
					CopyTagsToSnapshots:           copyTagsToSnapshots,
					DataCompressionType:           dataCompressionType,
					NfsExports:                    nfsExports,
					ParentVolumeId:                parentVolumeId,
					ReadOnly:                      readOnly,
					RecordSizeKiB:                 recordSizeKiB,
					StorageCapacityQuotaGiB:       storageCapacityQuotaGiB,
					StorageCapacityReservationGiB: storageCapacityReservationGiB,
					UserAndGroupQuotas:            userAndGroupQuotas,
					Tags:                          tags,
					OptionsOnDeletion:             optionsOnDeletion,
				}
				output := &fsx.CreateVolumeOutput{
					Volume: &fsx.Volume{
						FileSystemId: fileSystemId,
						OpenZFSConfiguration: &fsx.OpenZFSVolumeConfiguration{
							ParentVolumeId:                parentVolumeId,
							StorageCapacityQuotaGiB:       storageCapacityQuotaGiB,
							StorageCapacityReservationGiB: storageCapacityReservationGiB,
							VolumePath:                    volumePath,
						},
						VolumeId: volumeId,
					},
				}

				ctx := context.Background()
				mockFSx.EXPECT().CreateVolumeWithContext(gomock.Eq(ctx), gomock.Any()).Return(output, nil)
				resp, err := c.CreateVolume(ctx, volumeName, req)
				if err != nil {
					t.Fatalf("CreateVolume is failed: %v", err)
				}

				if resp == nil {
					t.Fatal("resp is nil")
				}

				if resp.FileSystemId != aws.StringValue(fileSystemId) {
					t.Fatalf("FileSystemId mismatches. actual: %v expected: %v", resp.FileSystemId, dnsName)
				}

				if resp.StorageCapacityQuotaGiB != aws.Int64Value(storageCapacityQuotaGiB) {
					t.Fatalf("StorageCapacityQuotaGiB mismatches. actual: %v expected: %v", resp.StorageCapacityQuotaGiB, storageCapacityQuotaGiB)
				}

				if resp.StorageCapacityReservationGiB != aws.Int64Value(storageCapacityReservationGiB) {
					t.Fatalf("StorageCapacityReservationGiB mismatches. actual: %v expected: %v", resp.StorageCapacityReservationGiB, storageCapacityReservationGiB)
				}

				if resp.VolumePath != aws.StringValue(volumePath) {
					t.Fatalf("VolumePath mismatches. actual: %v expected: %v", resp.VolumePath, volumePath)
				}

				if resp.VolumeId != aws.StringValue(volumeId) {
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

				req := VolumeOptions{
					Name:           name,
					ParentVolumeId: parentVolumeId,
				}

				output := &fsx.CreateVolumeOutput{
					Volume: &fsx.Volume{
						FileSystemId: fileSystemId,
						OpenZFSConfiguration: &fsx.OpenZFSVolumeConfiguration{
							ParentVolumeId:                parentVolumeId,
							StorageCapacityQuotaGiB:       storageCapacityQuotaGiB,
							StorageCapacityReservationGiB: storageCapacityReservationGiB,
							VolumePath:                    volumePath,
						},
						VolumeId: volumeId,
					},
				}

				ctx := context.Background()
				mockFSx.EXPECT().CreateVolumeWithContext(gomock.Eq(ctx), gomock.Any()).Return(output, nil)
				resp, err := c.CreateVolume(ctx, volumeName, req)
				if err != nil {
					t.Fatalf("CreateVolume is failed: %v", err)
				}

				if resp == nil {
					t.Fatal("resp is nil")
				}

				if resp.FileSystemId != aws.StringValue(fileSystemId) {
					t.Fatalf("FileSystemId mismatches. actual: %v expected: %v", resp.FileSystemId, dnsName)
				}

				if resp.StorageCapacityQuotaGiB != aws.Int64Value(storageCapacityQuotaGiB) {
					t.Fatalf("StorageCapacityQuotaGiB mismatches. actual: %v expected: %v", resp.StorageCapacityQuotaGiB, storageCapacityQuotaGiB)
				}

				if resp.StorageCapacityReservationGiB != aws.Int64Value(storageCapacityReservationGiB) {
					t.Fatalf("StorageCapacityReservationGiB mismatches. actual: %v expected: %v", resp.StorageCapacityReservationGiB, storageCapacityReservationGiB)
				}

				if resp.VolumePath != aws.StringValue(volumePath) {
					t.Fatalf("VolumePath mismatches. actual: %v expected: %v", resp.VolumePath, volumePath)
				}

				if resp.VolumeId != aws.StringValue(volumeId) {
					t.Fatalf("VolumeId mismatches. actual: %v expected: %v", resp.VolumeId, volumeId)
				}

				mockCtl.Finish()
			},
		},
		{
			name: "success: snapshot",
			testFunc: func(t *testing.T) {
				mockCtl := gomock.NewController(t)
				mockFSx := mocks.NewMockFSx(mockCtl)
				c := &cloud{
					fsx: mockFSx,
				}

				req := VolumeOptions{
					Name:           name,
					ParentVolumeId: parentVolumeId,
					SnapshotARN:    snapshotResourceArn,
				}

				output := &fsx.CreateVolumeOutput{
					Volume: &fsx.Volume{
						FileSystemId: fileSystemId,
						OpenZFSConfiguration: &fsx.OpenZFSVolumeConfiguration{
							ParentVolumeId:                parentVolumeId,
							StorageCapacityQuotaGiB:       storageCapacityQuotaGiB,
							StorageCapacityReservationGiB: storageCapacityReservationGiB,
							VolumePath:                    volumePath,
						},
						VolumeId: volumeId,
					},
				}

				ctx := context.Background()
				mockFSx.EXPECT().CreateVolumeWithContext(gomock.Eq(ctx), gomock.Any()).Return(output, nil)
				resp, err := c.CreateVolume(ctx, volumeName, req)
				if err != nil {
					t.Fatalf("CreateVolume is failed: %v", err)
				}

				if resp == nil {
					t.Fatal("resp is nil")
				}

				if resp.FileSystemId != aws.StringValue(fileSystemId) {
					t.Fatalf("FileSystemId mismatches. actual: %v expected: %v", resp.FileSystemId, dnsName)
				}

				if resp.StorageCapacityQuotaGiB != aws.Int64Value(storageCapacityQuotaGiB) {
					t.Fatalf("StorageCapacityQuotaGiB mismatches. actual: %v expected: %v", resp.StorageCapacityQuotaGiB, storageCapacityQuotaGiB)
				}

				if resp.StorageCapacityReservationGiB != aws.Int64Value(storageCapacityReservationGiB) {
					t.Fatalf("StorageCapacityReservationGiB mismatches. actual: %v expected: %v", resp.StorageCapacityReservationGiB, storageCapacityReservationGiB)
				}

				if resp.VolumePath != aws.StringValue(volumePath) {
					t.Fatalf("VolumePath mismatches. actual: %v expected: %v", resp.VolumePath, volumePath)
				}

				if resp.VolumeId != aws.StringValue(volumeId) {
					t.Fatalf("VolumeId mismatches. actual: %v expected: %v", resp.VolumeId, volumeId)
				}

				mockCtl.Finish()
			},
		},
		{
			name: "fail: bad userAndGroupQuotas",
			testFunc: func(t *testing.T) {
				mockCtl := gomock.NewController(t)
				mockFSx := mocks.NewMockFSx(mockCtl)
				c := &cloud{
					fsx: mockFSx,
				}

				badUserAndGroupQuotas := "[{Type=User,Id=INVALID,StorageCapacityQuotaGib=10}]"
				req := VolumeOptions{
					Name:               name,
					ParentVolumeId:     parentVolumeId,
					UserAndGroupQuotas: aws.String(badUserAndGroupQuotas),
				}

				ctx := context.Background()
				_, err := c.CreateVolume(ctx, volumeName, req)

				if err == nil {
					t.Fatal("CreateVolume is not failed")
				}

				mockCtl.Finish()
			},
		},
		{
			name: "fail: CreateVolumeWithContext return ErrCodeIncompatibleParameterError error",
			testFunc: func(t *testing.T) {
				mockCtl := gomock.NewController(t)
				mockFSx := mocks.NewMockFSx(mockCtl)
				c := &cloud{
					fsx: mockFSx,
				}

				req := VolumeOptions{
					Name:           name,
					ParentVolumeId: parentVolumeId,
				}

				ctx := context.Background()
				mockFSx.EXPECT().CreateVolumeWithContext(gomock.Eq(ctx), gomock.Any()).Return(nil, errors.New(fsx.ErrCodeIncompatibleParameterError))
				_, err := c.CreateVolume(ctx, volumeName, req)
				if err == nil {
					t.Fatal("CreateVolume is not failed")
				}

				mockCtl.Finish()
			},
		},
		{
			name: "fail: CreateVolumeWithContext return error",
			testFunc: func(t *testing.T) {
				mockCtl := gomock.NewController(t)
				mockFSx := mocks.NewMockFSx(mockCtl)
				c := &cloud{
					fsx: mockFSx,
				}

				req := VolumeOptions{
					Name:           name,
					ParentVolumeId: parentVolumeId,
				}

				ctx := context.Background()
				mockFSx.EXPECT().CreateVolumeWithContext(gomock.Eq(ctx), gomock.Any()).Return(nil, errors.New("CreateFileSystemWithContext failed"))
				_, err := c.CreateVolume(ctx, volumeName, req)
				if err == nil {
					t.Fatal("CreateVolume is not failed")
				}

				mockCtl.Finish()
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, tc.testFunc)
	}
}

func TestResizeVolume(t *testing.T) {
	var (
		volumeId             = "fsvol-1234"
		currentSizeGiB int64 = 100
		newSizeGiB     int64 = 150
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

				updateOutput := &fsx.UpdateVolumeOutput{
					Volume: &fsx.Volume{
						VolumeId: aws.String(volumeId),
						OpenZFSConfiguration: &fsx.OpenZFSVolumeConfiguration{
							StorageCapacityQuotaGiB:       aws.Int64(newSizeGiB),
							StorageCapacityReservationGiB: aws.Int64(newSizeGiB),
						},
					},
				}

				ctx := context.Background()
				mockFSx.EXPECT().UpdateVolumeWithContext(gomock.Eq(ctx), gomock.Any()).Return(updateOutput, nil)
				resp, err := c.ResizeVolume(ctx, volumeId, newSizeGiB)
				if err != nil {
					t.Fatalf("ResizeVolume is failed: %v", err)
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

				describeOutput := &fsx.DescribeVolumesOutput{
					Volumes: []*fsx.Volume{
						{
							AdministrativeActions: []*fsx.AdministrativeAction{
								{
									AdministrativeActionType: aws.String(fsx.AdministrativeActionTypeVolumeUpdate),
									TargetVolumeValues: &fsx.Volume{
										OpenZFSConfiguration: &fsx.OpenZFSVolumeConfiguration{
											StorageCapacityQuotaGiB:       aws.Int64(newSizeGiB),
											StorageCapacityReservationGiB: aws.Int64(newSizeGiB),
										},
									},
								},
							},
							FileSystemId: aws.String(volumeId),
						},
					},
				}

				ctx := context.Background()
				mockFSx.EXPECT().UpdateVolumeWithContext(gomock.Eq(ctx), gomock.Any()).Return(nil, awserr.New(fsx.ErrCodeBadRequest, "Unable to update the volume because there are existing pending actions for the volume", errors.New("")))
				mockFSx.EXPECT().DescribeVolumesWithContext(gomock.Eq(ctx), gomock.Any()).Return(describeOutput, nil)
				resp, err := c.ResizeVolume(ctx, volumeId, newSizeGiB)
				if err != nil {
					t.Fatalf("ResizeVolume is failed: %v", err)
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
				mockFSx.EXPECT().UpdateVolumeWithContext(gomock.Eq(ctx), gomock.Any()).Return(nil, errors.New(""))
				_, err := c.ResizeVolume(ctx, volumeId, newSizeGiB)
				if err == nil {
					t.Fatalf("ResizeVolume is not failed")
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

				describeOutput := &fsx.DescribeVolumesOutput{
					Volumes: []*fsx.Volume{
						{
							AdministrativeActions: []*fsx.AdministrativeAction{
								{
									AdministrativeActionType: aws.String(fsx.AdministrativeActionTypeVolumeUpdate),
									TargetVolumeValues: &fsx.Volume{
										OpenZFSConfiguration: &fsx.OpenZFSVolumeConfiguration{
											StorageCapacityQuotaGiB:       aws.Int64(currentSizeGiB),
											StorageCapacityReservationGiB: aws.Int64(currentSizeGiB),
										},
									},
								},
							},
							FileSystemId: aws.String(volumeId),
						},
					},
				}

				ctx := context.Background()
				mockFSx.EXPECT().UpdateVolumeWithContext(gomock.Eq(ctx), gomock.Any()).Return(nil, awserr.New(fsx.ErrCodeBadRequest, "Unable to update the volume because there are existing pending actions for the volume", errors.New("")))
				mockFSx.EXPECT().DescribeVolumesWithContext(gomock.Eq(ctx), gomock.Any()).Return(describeOutput, nil)
				_, err := c.ResizeVolume(ctx, volumeId, newSizeGiB)
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

func TestDeleteVolume(t *testing.T) {
	var (
		volumeId          = "fsvol-0987654321abcdefg"
		resourceARN       = "arn:aws:fsx:us-east-1:123456789012:volume/fs-1234abcd5678efgh9/fsvol-0987654321abcdefg"
		optionsOnDeletion = "DELETE_CHILD_VOLUMES_AND_SNAPSHOTS"
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

				output := &fsx.DeleteVolumeOutput{}
				describeOutput := &fsx.DescribeVolumesOutput{
					Volumes: []*fsx.Volume{
						{
							VolumeId:    aws.String(volumeId),
							ResourceARN: aws.String(resourceARN),
						},
					},
				}
				listTagsOutput := &fsx.ListTagsForResourceOutput{
					Tags: []*fsx.Tag{
						{
							Key:   aws.String(OptionsOnDeletionTagKey),
							Value: aws.String(optionsOnDeletion),
						},
					},
				}

				ctx := context.Background()
				mockFSx.EXPECT().DescribeVolumesWithContext(gomock.Eq(ctx), gomock.Any()).Return(describeOutput, nil)
				mockFSx.EXPECT().ListTagsForResource(gomock.Any()).Return(listTagsOutput, nil)
				mockFSx.EXPECT().DeleteVolumeWithContext(gomock.Eq(ctx), gomock.Any()).Return(output, nil)
				err := c.DeleteVolume(ctx, volumeId)
				if err != nil {
					t.Fatalf("DeleteVolume is failed: %v", err)
				}

				mockCtl.Finish()
			},
		},
		{
			name: "success: no tags",
			testFunc: func(t *testing.T) {
				mockCtl := gomock.NewController(t)
				mockFSx := mocks.NewMockFSx(mockCtl)
				c := &cloud{
					fsx: mockFSx,
				}

				output := &fsx.DeleteVolumeOutput{}
				describeOutput := &fsx.DescribeVolumesOutput{
					Volumes: []*fsx.Volume{
						{
							VolumeId:    aws.String(volumeId),
							ResourceARN: aws.String(resourceARN),
						},
					},
				}
				listTagsOutput := &fsx.ListTagsForResourceOutput{
					Tags: []*fsx.Tag{},
				}

				ctx := context.Background()
				mockFSx.EXPECT().DescribeVolumesWithContext(gomock.Eq(ctx), gomock.Any()).Return(describeOutput, nil)
				mockFSx.EXPECT().ListTagsForResource(gomock.Any()).Return(listTagsOutput, nil)
				mockFSx.EXPECT().DeleteVolumeWithContext(gomock.Eq(ctx), gomock.Any()).Return(output, nil)
				err := c.DeleteVolume(ctx, volumeId)
				if err != nil {
					t.Fatalf("DeleteVolume is failed: %v", err)
				}

				mockCtl.Finish()
			},
		},
		{
			name: "fail: DeleteVolumeWithContext return error",
			testFunc: func(t *testing.T) {
				mockCtl := gomock.NewController(t)
				mockFSx := mocks.NewMockFSx(mockCtl)
				c := &cloud{
					fsx: mockFSx,
				}

				describeOutput := &fsx.DescribeVolumesOutput{
					Volumes: []*fsx.Volume{
						{
							VolumeId:    aws.String(volumeId),
							ResourceARN: aws.String(resourceARN),
						},
					},
				}
				listTagsOutput := &fsx.ListTagsForResourceOutput{
					Tags: []*fsx.Tag{
						{
							Key:   aws.String(OptionsOnDeletionTagKey),
							Value: aws.String(optionsOnDeletion),
						},
					},
				}

				ctx := context.Background()
				mockFSx.EXPECT().DescribeVolumesWithContext(gomock.Eq(ctx), gomock.Any()).Return(describeOutput, nil)
				mockFSx.EXPECT().ListTagsForResource(gomock.Any()).Return(listTagsOutput, nil)
				mockFSx.EXPECT().DeleteVolumeWithContext(gomock.Eq(ctx), gomock.Any()).Return(nil, errors.New("DeleteVolumeWithContext failed"))
				err := c.DeleteVolume(ctx, volumeId)
				if err == nil {
					t.Fatal("DeleteVolume is not failed")
				}

				mockCtl.Finish()
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, tc.testFunc)
	}
}

func TestDescribeVolume(t *testing.T) {
	var (
		fileSystemId                  = aws.String("fs-1234567890abcdefgh")
		parentVolumeId                = aws.String("fsvol-03062e7ff37662dff")
		storageCapacityQuotaGiB       = aws.Int64(10)
		storageCapacityReservationGiB = aws.Int64(10)
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
					Volumes: []*fsx.Volume{
						{
							FileSystemId: fileSystemId,
							OpenZFSConfiguration: &fsx.OpenZFSVolumeConfiguration{
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
				mockFSx.EXPECT().DescribeVolumesWithContext(gomock.Eq(ctx), gomock.Any()).Return(output, nil)
				_, err := c.DescribeVolume(ctx, *volumeId)
				if err != nil {
					t.Fatalf("DescribeVolume is failed: %v", err)
				}

				mockCtl.Finish()
			},
		},
		{
			name: "fail: DescribeVolumesWithContext return error",
			testFunc: func(t *testing.T) {
				mockCtl := gomock.NewController(t)
				mockFSx := mocks.NewMockFSx(mockCtl)
				c := &cloud{
					fsx: mockFSx,
				}

				ctx := context.Background()
				mockFSx.EXPECT().DescribeVolumesWithContext(gomock.Eq(ctx), gomock.Any()).Return(nil, errors.New("DescribeVolumesWithContext failed"))
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
					VolumeIds: []*string{aws.String(volumeId)},
				}
				output := &fsx.DescribeVolumesOutput{
					Volumes: []*fsx.Volume{
						{
							VolumeId:  aws.String(volumeId),
							Lifecycle: aws.String(fsx.VolumeLifecycleAvailable),
						},
					},
				}

				ctx := context.Background()
				mockFSx.EXPECT().DescribeVolumesWithContext(gomock.Eq(ctx), gomock.Eq(input)).Return(output, nil)
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
					VolumeIds: []*string{aws.String(volumeId)},
				}
				output := &fsx.DescribeVolumesOutput{
					Volumes: []*fsx.Volume{
						{
							FileSystemId: aws.String(volumeId),
							Lifecycle:    aws.String(fsx.VolumeLifecycleFailed),
						},
					},
				}

				ctx := context.Background()
				mockFSx.EXPECT().DescribeVolumesWithContext(gomock.Eq(ctx), gomock.Eq(input)).Return(output, nil)
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
		resizeGiB int64 = 100
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
					Volumes: []*fsx.Volume{
						{
							AdministrativeActions: []*fsx.AdministrativeAction{
								{
									AdministrativeActionType: aws.String(fsx.AdministrativeActionTypeVolumeUpdate),
									Status:                   aws.String(fsx.StatusCompleted),
									TargetVolumeValues: &fsx.Volume{
										OpenZFSConfiguration: &fsx.OpenZFSVolumeConfiguration{
											StorageCapacityQuotaGiB:       aws.Int64(resizeGiB),
											StorageCapacityReservationGiB: aws.Int64(resizeGiB),
										},
									},
								},
							},
							VolumeId: aws.String(volumeId),
						},
					},
				}

				ctx := context.Background()
				mockFSx.EXPECT().DescribeVolumesWithContext(gomock.Eq(ctx), gomock.Any()).Return(output, nil)
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
					Volumes: []*fsx.Volume{
						{
							AdministrativeActions: []*fsx.AdministrativeAction{
								{
									AdministrativeActionType: aws.String(fsx.AdministrativeActionTypeVolumeUpdate),
									Status:                   aws.String(fsx.StatusFailed),
									TargetVolumeValues: &fsx.Volume{
										OpenZFSConfiguration: &fsx.OpenZFSVolumeConfiguration{
											StorageCapacityQuotaGiB:       aws.Int64(resizeGiB),
											StorageCapacityReservationGiB: aws.Int64(resizeGiB),
										},
									},
									FailureDetails: &fsx.AdministrativeActionFailureDetails{
										Message: aws.String("Update failed"),
									},
								},
							},
							VolumeId: aws.String(volumeId),
						},
					},
				}

				ctx := context.Background()
				mockFSx.EXPECT().DescribeVolumesWithContext(gomock.Eq(ctx), gomock.Any()).Return(output, nil)
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
					Snapshots: []*fsx.Snapshot{
						{
							CreationTime: creationTime,
							ResourceARN:  resourceARN,
							SnapshotId:   snapshotId,
							VolumeId:     volumeId,
						},
					},
				}

				ctx := context.Background()
				mockFSx.EXPECT().DescribeSnapshotsWithContext(gomock.Eq(ctx), gomock.Any()).Return(output, nil)
				_, err := c.DescribeSnapshot(ctx, *snapshotId)
				if err != nil {
					t.Fatalf("DescribeSnapshot is failed: %v", err)
				}

				mockCtl.Finish()
			},
		},
		{
			name: "fail: DescribeSnapshotsWithContext return error",
			testFunc: func(t *testing.T) {
				mockCtl := gomock.NewController(t)
				mockFSx := mocks.NewMockFSx(mockCtl)
				c := &cloud{
					fsx: mockFSx,
				}

				ctx := context.Background()
				mockFSx.EXPECT().DescribeSnapshotsWithContext(gomock.Eq(ctx), gomock.Any()).Return(nil, errors.New("DescribeSnapshotsWithContext failed"))
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

func TestParseDiskIopsConfiguration(t *testing.T) {
	testCases := []struct {
		name     string
		input    string
		expected *fsx.DiskIopsConfiguration
		error    bool
	}{
		{
			name:  "success: normal configuration",
			input: "Mode=USER_PROVISIONED,Iops=300",
			expected: &fsx.DiskIopsConfiguration{
				Iops: aws.Int64(300),
				Mode: aws.String(fsx.DiskIopsConfigurationModeUserProvisioned),
			},
			error: false,
		},
		{
			name:  "failure: invalid format",
			input: "ModeUSER_PROVISIONED,Iops=INVALID",
			error: true,
		},
		{
			name:  "failure: iops not a number",
			input: "Mode=USER_PROVISIONED,Iops=INVALID",
			error: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockCtl := gomock.NewController(t)
			mockFSx := mocks.NewMockFSx(mockCtl)
			c := &cloud{
				fsx: mockFSx,
			}

			actual, err := c.parseDiskIopsConfiguration(tc.input)

			if err != nil && tc.error == false || err == nil && tc.error == true {
				t.Fatalf("ParseDiskIopsConfiguration got wrong result. Error mismatch.")
			}

			if !reflect.DeepEqual(actual, tc.expected) {
				t.Fatalf("ParseDiskIopsConfiguration got wrong result. actual: %s, expected: %s", actual, tc.expected)
			}
		})
	}
}

func TestParseRootVolumeConfiguration(t *testing.T) {
	testCases := []struct {
		name     string
		input    string
		expected *fsx.OpenZFSCreateRootVolumeConfiguration
		error    bool
	}{
		{
			name:  "success: normal configuration",
			input: "CopyTagsToSnapshots=true,DataCompressionType=NONE,ReadOnly=false,RecordSizeKiB=128",
			expected: &fsx.OpenZFSCreateRootVolumeConfiguration{
				CopyTagsToSnapshots: aws.Bool(true),
				DataCompressionType: aws.String("NONE"),
				ReadOnly:            aws.Bool(false),
				RecordSizeKiB:       aws.Int64(128),
			},
			error: false,
		},
		{
			name:  "failure: invalid format",
			input: "CopyTagsToSnapshotDataCompressionType=NONE,ReadOnly=128,RecordSizeKiB=128",
			error: true,
		},
		{
			name:  "failure: CopyTagsToSnapshots not a bool",
			input: "CopyTagsToSnapshots=INVALID",
			error: true,
		},
		{
			name:  "failure: ReadOnly not a bool",
			input: "ReadOnly=INVALID",
			error: true,
		},
		{
			name:  "failure: RecordSizeKiB not a number",
			input: "RecordSizeKiB=INVALID",
			error: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockCtl := gomock.NewController(t)
			mockFSx := mocks.NewMockFSx(mockCtl)
			c := &cloud{
				fsx: mockFSx,
			}

			actual, err := c.parseRootVolumeConfiguration(tc.input)

			if err != nil && tc.error == false || err == nil && tc.error == true {
				t.Fatalf("ParseRootVolumeConfiguration got wrong result. Error mismatch.")
			}

			if !reflect.DeepEqual(actual, tc.expected) {
				t.Fatalf("ParseRootVolumeConfiguration got wrong result. actual: %s, expected: %s", actual, tc.expected)
			}
		})
	}
}

func TestParseNfsExports(t *testing.T) {
	testCases := []struct {
		name     string
		input    string
		expected []*fsx.OpenZFSNfsExport
		error    bool
	}{
		{
			name:  "success: normal configuration",
			input: "[{ClientConfigurations=[{Clients=*,Options=[sync,crossmnt]},{Clients=192.0.2.0/24,Options=[async]}]},{ClientConfigurations=[{Clients=*,Options=[sync,crossmnt]}]}]",
			expected: []*fsx.OpenZFSNfsExport{
				{
					ClientConfigurations: []*fsx.OpenZFSClientConfiguration{
						{
							Clients: aws.String("*"),
							Options: []*string{aws.String("sync"), aws.String("crossmnt")},
						},
						{
							Clients: aws.String("192.0.2.0/24"),
							Options: []*string{aws.String("async")},
						},
					},
				},
				{
					ClientConfigurations: []*fsx.OpenZFSClientConfiguration{
						{
							Clients: aws.String("*"),
							Options: []*string{aws.String("sync"), aws.String("crossmnt")},
						},
					},
				},
			},
			error: false,
		},
		{
			name:  "failure: invalid format",
			input: "NfsExports=[[[[{ClientConfigurations=[{Clients=*,Options=[rw,crossmnt]}]}]",
			error: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockCtl := gomock.NewController(t)
			mockFSx := mocks.NewMockFSx(mockCtl)
			c := &cloud{
				fsx: mockFSx,
			}

			actual, err := c.parseNfsExports(tc.input)

			if err != nil && tc.error == false || err == nil && tc.error == true {
				t.Fatalf("ParseNfsExports got wrong result. Error mismatch.")
			}

			if !reflect.DeepEqual(actual, tc.expected) {
				t.Fatalf("ParseNfsExports got wrong result. actual: %s, expected: %s", actual, tc.expected)
			}
		})
	}
}

func TestParseUserAndGroupQuotas(t *testing.T) {
	testCases := []struct {
		name     string
		input    string
		expected []*fsx.OpenZFSUserOrGroupQuota
		error    bool
	}{
		{
			name:  "success: normal configuration",
			input: "[{Id=1,StorageCapacityQuotaGiB=10,Type=User},{Id=2,StorageCapacityQuotaGiB=5,Type=Group}]",
			expected: []*fsx.OpenZFSUserOrGroupQuota{
				{
					Id:                      aws.Int64(1),
					StorageCapacityQuotaGiB: aws.Int64(10),
					Type:                    aws.String("User"),
				},
				{
					Id:                      aws.Int64(2),
					StorageCapacityQuotaGiB: aws.Int64(5),
					Type:                    aws.String("Group"),
				},
			},
			error: false,
		},
		{
			name:  "failure: invalid format",
			input: "[{Id:1,StorageCapacityQuotaGiB=10,Type=User}]",
			error: true,
		},
		{
			name:  "failure: Id not a number",
			input: "[{Id=INVALID,StorageCapacityQuotaGiB=10,Type=User}]",
			error: true,
		},
		{
			name:  "failure: StorageCapacityQuotaGiB not a number",
			input: "[{Id=INVALID,StorageCapacityQuotaGiB=INVALID,Type=User}]",
			error: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockCtl := gomock.NewController(t)
			mockFSx := mocks.NewMockFSx(mockCtl)
			c := &cloud{
				fsx: mockFSx,
			}

			actual, err := c.parseUserAndGroupQuotas(tc.input)

			if err != nil && tc.error == false || err == nil && tc.error == true {
				t.Fatalf("ParseUserAndGroupQuotas got wrong result. Error mismatch.")
			}

			if !reflect.DeepEqual(actual, tc.expected) {
				t.Fatalf("ParseUserAndGroupQuotas got wrong result. actual: %s, expected: %s", actual, tc.expected)
			}
		})
	}
}

func TestParseTags(t *testing.T) {
	testCases := []struct {
		name     string
		tags     string
		expected []*fsx.Tag
		error    bool
	}{
		{
			name: "success: normal tags",
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
			error: false,
		},
		{
			name: "success: missing value",
			tags: "Key1=",
			expected: []*fsx.Tag{
				{
					Key:   aws.String("Key1"),
					Value: nil,
				},
			},
			error: false,
		},
		{
			name:     "failure: empty tags",
			tags:     "",
			expected: nil,
			error:    true,
		},
		{
			name:     "failure: improperly formatted tag",
			tags:     "Key1=Value1,Key2Value2",
			expected: nil,
			error:    true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockCtl := gomock.NewController(t)
			mockFSx := mocks.NewMockFSx(mockCtl)
			c := &cloud{
				fsx: mockFSx,
			}

			actual, err := c.parseTags(tc.tags)
			if err != nil && tc.error == false || err == nil && tc.error == true {
				t.Fatalf("ParseTags got wrong result. Error mismatch.")
			}

			if !reflect.DeepEqual(actual, tc.expected) {
				t.Fatalf("ParseTags got wrong result. actual: %s, expected: %s", actual, tc.expected)
			}
		})
	}
}
