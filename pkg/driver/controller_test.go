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
	"google.golang.org/protobuf/types/known/timestamppb"
	"sigs.k8s.io/aws-fsx-openzfs-csi-driver/pkg/driver/internal"
	"sigs.k8s.io/aws-fsx-openzfs-csi-driver/pkg/util"
	"testing"
	"time"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/golang/mock/gomock"
	"sigs.k8s.io/aws-fsx-openzfs-csi-driver/pkg/cloud"
	"sigs.k8s.io/aws-fsx-openzfs-csi-driver/pkg/driver/mocks"
)

func TestCreateVolume(t *testing.T) {

	var (
		filesystemId                        = "filesystemId"
		volumeId                            = "volumeId"
		endpoint                            = "endpoint"
		storageCapacity               int64 = 64
		dnsName                             = "dnsName"
		kmsKeyId                            = "1234abcd-12ab-34cd-56ef-1234567890ab"
		automaticBackupRetentionDays        = "1"
		copyTagsToBackups                   = "false"
		copyTagsToVolumes                   = "false"
		dailyAutomaticBackupStartTime       = "00:00"
		deploymentType                      = "SINGLE_AZ_1"
		diskIopsConfiguration               = "Mode=USER_PROVISIONED,Iops=300"
		rootVolumeConfiguration             = "RecordSizeKiB=128,DataCompressionType=NONE,NfsExports=[{ClientConfigurations=[{Clients=*,Options=[rw,crossmnt]}]}]"
		throughputCapacity                  = "64"
		weeklyMaintenanceStartTime          = "7:09:00"
		securityGroupIds                    = "sg-068000ccf82dfba88"
		subnetIds                           = "subnet-0eabfaa81fb22bcaf"
		tags                                = "Tag1=Value1,Tag2=Value2"
		skipFinalBackup                     = "true"
		copyTagsToSnapshots                 = "false"
		dataCompressionType                 = "NONE"
		nfsExports                          = "[{ClientConfigurations=[{Clients=*,Options=[rw,crossmnt]}]}]"
		snapshotId                          = "fsvolsnap-1234"
		parentVolumeId                      = "fsvol-03062e7ff37662dff"
		readOnly                            = "false"
		recordSizeKiB                       = "128"
		userAndGroupQuotas                  = "[{Type=User,Id=1,StorageCapacityQuotaGiB=10}]"
		volumePath                          = "/"
		snapshotArn                         = "arn:"
		creationTime                        = time.Now()
		stdVolCap                           = &csi.VolumeCapability{
			AccessType: &csi.VolumeCapability_Mount{
				Mount: &csi.VolumeCapability_MountVolume{},
			},
			AccessMode: &csi.VolumeCapability_AccessMode{
				Mode: csi.VolumeCapability_AccessMode_MULTI_NODE_MULTI_WRITER,
			},
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
				mockCloud := mocks.NewMockCloud(mockCtl)

				driver := &Driver{
					endpoint: endpoint,
					cloud:    mockCloud,
				}

				req := &csi.CreateVolumeRequest{
					Name: filesystemId,
					CapacityRange: &csi.CapacityRange{
						RequiredBytes: util.GiBToBytes(storageCapacity),
						LimitBytes:    util.GiBToBytes(storageCapacity),
					},
					VolumeCapabilities: []*csi.VolumeCapability{stdVolCap},
					Parameters: map[string]string{
						volumeParamsVolumeType:                    "filesystem",
						volumeParamsKmsKeyId:                      kmsKeyId,
						volumeParamsAutomaticBackupRetentionDays:  automaticBackupRetentionDays,
						volumeParamsCopyTagsToBackups:             copyTagsToBackups,
						volumeParamsCopyTagsToVolumes:             copyTagsToVolumes,
						volumeParamsDailyAutomaticBackupStartTime: dailyAutomaticBackupStartTime,
						volumeParamsDeploymentType:                deploymentType,
						volumeParamsDiskIopsConfiguration:         diskIopsConfiguration,
						volumeParamsRootVolumeConfiguration:       rootVolumeConfiguration,
						volumeParamsThroughputCapacity:            throughputCapacity,
						volumeParamsWeeklyMaintenanceStartTime:    weeklyMaintenanceStartTime,
						volumeParamsSecurityGroupIds:              securityGroupIds,
						volumeParamsSubnetIds:                     subnetIds,
						volumeParamsTags:                          tags,
						volumeParamsSkipFinalBackup:               skipFinalBackup,
					},
				}

				ctx := context.Background()
				filesystem := &cloud.FileSystem{
					DnsName:         dnsName,
					FileSystemId:    filesystemId,
					StorageCapacity: storageCapacity,
				}
				mockCloud.EXPECT().CreateFileSystem(gomock.Eq(ctx), gomock.Eq(filesystemId), gomock.Any()).Return(filesystem, nil)
				mockCloud.EXPECT().WaitForFileSystemAvailable(gomock.Eq(ctx), gomock.Eq(filesystemId)).Return(nil)

				resp, err := driver.CreateVolume(ctx, req)
				if err != nil {
					t.Fatalf("CreateVolume failed: %v", err)
				}

				if resp.Volume == nil {
					t.Fatal("resp.Volume is nil")
				}

				if resp.Volume.CapacityBytes != util.GiBToBytes(storageCapacity) {
					t.Fatalf("CapacityBytes mismatches. actual: %v expected %v", resp.Volume.CapacityBytes, util.GiBToBytes(storageCapacity))
				}

				if resp.Volume.VolumeId != filesystemId {
					t.Fatalf("VolumeId mismatches. actual: %v expected %v", resp.Volume.VolumeId, filesystemId)
				}

				if resp.Volume.VolumeContext[volumeContextVolumeType] != "filesystem" {
					t.Fatalf("volumeContextVolumeType mismatches. actual: %v expected %v", resp.Volume.VolumeContext[volumeContextVolumeType], "filesystem")
				}

				if resp.Volume.VolumeContext[volumeContextDnsName] != dnsName {
					t.Fatalf("volumeContextDnsName mismatches. actual: %v expected %v", resp.Volume.VolumeContext[volumeContextDnsName], "filesystem")
				}

				if resp.Volume.VolumeContext[volumeContextSkipFinalBackup] != skipFinalBackup {
					t.Fatalf("volumeContextSkipFinalBackup mismatches. actual: %v expected %v", resp.Volume.VolumeContext[volumeContextSkipFinalBackup], skipFinalBackup)
				}

				mockCtl.Finish()
			},
		},
		{
			name: "success: volume",
			testFunc: func(t *testing.T) {
				mockCtl := gomock.NewController(t)
				mockCloud := mocks.NewMockCloud(mockCtl)

				driver := &Driver{
					endpoint: endpoint,
					cloud:    mockCloud,
				}

				req := &csi.CreateVolumeRequest{
					Name: volumeId,
					CapacityRange: &csi.CapacityRange{
						RequiredBytes: util.GiBToBytes(storageCapacity),
						LimitBytes:    util.GiBToBytes(storageCapacity),
					},
					VolumeCapabilities: []*csi.VolumeCapability{stdVolCap},
					Parameters: map[string]string{
						volumeParamsVolumeType:          "volume",
						volumeParamsCopyTagsToSnapshots: copyTagsToSnapshots,
						volumeParamsDataCompressionType: dataCompressionType,
						volumeParamsNfsExports:          nfsExports,
						volumeParamsParentVolumeId:      parentVolumeId,
						volumeParamsReadOnly:            readOnly,
						volumeParamsRecordSizeKiB:       recordSizeKiB,
						volumeParamsUserAndGroupQuotas:  userAndGroupQuotas,
						volumeParamsTags:                tags,
					},
				}
				volume := &cloud.Volume{
					FileSystemId:                  filesystemId,
					StorageCapacityQuotaGiB:       storageCapacity,
					StorageCapacityReservationGiB: storageCapacity,
					VolumePath:                    volumePath,
					VolumeId:                      volumeId,
				}
				filesystem := &cloud.FileSystem{
					DnsName:         dnsName,
					FileSystemId:    filesystemId,
					StorageCapacity: storageCapacity,
				}

				ctx := context.Background()
				mockCloud.EXPECT().CreateVolume(gomock.Eq(ctx), gomock.Eq(volumeId), gomock.Any()).Return(volume, nil)
				mockCloud.EXPECT().WaitForVolumeAvailable(gomock.Eq(ctx), gomock.Eq(volumeId)).Return(nil)
				mockCloud.EXPECT().DescribeFileSystem(gomock.Eq(ctx), gomock.Eq(filesystemId)).Return(filesystem, nil)

				resp, err := driver.CreateVolume(ctx, req)
				if err != nil {
					t.Fatalf("CreateVolume failed: %v", err)
				}

				if resp.Volume == nil {
					t.Fatal("resp.Volume is nil")
				}

				if resp.Volume.CapacityBytes != util.GiBToBytes(storageCapacity) {
					t.Fatalf("CapacityBytes mismatches. actual: %v expected %v", resp.Volume.CapacityBytes, util.GiBToBytes(storageCapacity))
				}

				if resp.Volume.VolumeId != volumeId {
					t.Fatalf("VolumeId mismatches. actual: %v expected %v", resp.Volume.VolumeId, volumeId)
				}

				if resp.Volume.VolumeContext[volumeContextVolumeType] != "volume" {
					t.Fatalf("volumeContextVolumeType mismatches. actual: %v expected %v", resp.Volume.VolumeContext[volumeContextVolumeType], "filesystem")
				}

				if resp.Volume.VolumeContext[volumeContextDnsName] != dnsName {
					t.Fatalf("volumeContextVolumeType mismatches. actual: %v expected %v", resp.Volume.VolumeContext[volumeContextVolumeType], "filesystem")
				}

				if resp.Volume.VolumeContext[volumeContextVolumePath] != volumePath {
					t.Fatalf("volumeContextVolumeType mismatches. actual: %v expected %v", resp.Volume.VolumeContext[volumeContextVolumeType], "filesystem")
				}

				mockCtl.Finish()
			},
		},
		{
			name: "success: volume snapshot",
			testFunc: func(t *testing.T) {
				mockCtl := gomock.NewController(t)
				mockCloud := mocks.NewMockCloud(mockCtl)

				driver := &Driver{
					endpoint: endpoint,
					cloud:    mockCloud,
				}

				req := &csi.CreateVolumeRequest{
					Name: volumeId,
					CapacityRange: &csi.CapacityRange{
						RequiredBytes: util.GiBToBytes(storageCapacity),
						LimitBytes:    util.GiBToBytes(storageCapacity),
					},
					VolumeCapabilities: []*csi.VolumeCapability{stdVolCap},
					Parameters: map[string]string{
						volumeParamsVolumeType:          "volume",
						volumeParamsCopyTagsToSnapshots: copyTagsToSnapshots,
						volumeParamsDataCompressionType: dataCompressionType,
						volumeParamsNfsExports:          nfsExports,
						volumeParamsParentVolumeId:      parentVolumeId,
						volumeParamsReadOnly:            readOnly,
						volumeParamsRecordSizeKiB:       recordSizeKiB,
						volumeParamsUserAndGroupQuotas:  userAndGroupQuotas,
						volumeParamsTags:                tags,
					},
					VolumeContentSource: &csi.VolumeContentSource{
						Type: &csi.VolumeContentSource_Snapshot{
							Snapshot: &csi.VolumeContentSource_SnapshotSource{
								SnapshotId: snapshotId,
							},
						},
					},
				}
				filesystem := &cloud.FileSystem{
					DnsName:         dnsName,
					FileSystemId:    filesystemId,
					StorageCapacity: storageCapacity,
				}
				volume := &cloud.Volume{
					FileSystemId:                  filesystemId,
					StorageCapacityQuotaGiB:       storageCapacity,
					StorageCapacityReservationGiB: storageCapacity,
					VolumePath:                    "/",
					VolumeId:                      volumeId,
				}
				snapshot := &cloud.Snapshot{
					SnapshotID:     snapshotId,
					SourceVolumeID: volumeId,
					ResourceARN:    snapshotArn,
					CreationTime:   creationTime,
				}

				ctx := context.Background()
				mockCloud.EXPECT().DescribeSnapshot(gomock.Eq(ctx), gomock.Eq(snapshotId)).Return(snapshot, nil)
				mockCloud.EXPECT().CreateVolume(gomock.Eq(ctx), gomock.Eq(volumeId), gomock.Any()).Return(volume, nil)
				mockCloud.EXPECT().WaitForVolumeAvailable(gomock.Eq(ctx), gomock.Eq(volumeId)).Return(nil)
				mockCloud.EXPECT().DescribeFileSystem(gomock.Eq(ctx), gomock.Eq(filesystemId)).Return(filesystem, nil)

				resp, err := driver.CreateVolume(ctx, req)
				if err != nil {
					t.Fatalf("CreateVolume failed: %v", err)
				}

				if resp.Volume == nil {
					t.Fatal("resp.Volume is nil")
				}

				if resp.Volume.CapacityBytes != util.GiBToBytes(storageCapacity) {
					t.Fatalf("CapacityBytes mismatches. actual: %v expected %v", resp.Volume.CapacityBytes, util.GiBToBytes(storageCapacity))
				}

				if resp.Volume.VolumeId != volumeId {
					t.Fatalf("VolumeId mismatches. actual: %v expected %v", resp.Volume.VolumeId, volumeId)
				}

				if resp.Volume.VolumeContext[volumeContextVolumeType] != "volume" {
					t.Fatalf("volumeContextVolumeType mismatches. actual: %v expected %v", resp.Volume.VolumeContext[volumeContextVolumeType], "filesystem")
				}

				if resp.Volume.VolumeContext[volumeContextDnsName] != dnsName {
					t.Fatalf("volumeContextDnsName mismatches. actual: %v expected %v", resp.Volume.VolumeContext[volumeContextDnsName], dnsName)
				}

				if resp.Volume.VolumeContext[volumeContextVolumePath] != volumePath {
					t.Fatalf("volumeContextVolumeType mismatches. actual: %v expected %v", resp.Volume.VolumeContext[volumeContextVolumeType], "filesystem")
				}

				mockCtl.Finish()
			},
		},
		{
			name: "fail: volume name missing",
			testFunc: func(t *testing.T) {
				mockCtl := gomock.NewController(t)
				mockCloud := mocks.NewMockCloud(mockCtl)

				driver := &Driver{
					endpoint: endpoint,
					cloud:    mockCloud,
				}

				req := &csi.CreateVolumeRequest{
					CapacityRange: &csi.CapacityRange{
						RequiredBytes: util.GiBToBytes(storageCapacity),
						LimitBytes:    util.GiBToBytes(storageCapacity),
					},
					VolumeCapabilities: []*csi.VolumeCapability{stdVolCap},
					Parameters: map[string]string{
						volumeParamsVolumeType:                    "filesystem",
						volumeParamsKmsKeyId:                      kmsKeyId,
						volumeParamsAutomaticBackupRetentionDays:  automaticBackupRetentionDays,
						volumeParamsCopyTagsToBackups:             copyTagsToBackups,
						volumeParamsCopyTagsToVolumes:             copyTagsToVolumes,
						volumeParamsDailyAutomaticBackupStartTime: dailyAutomaticBackupStartTime,
						volumeParamsDeploymentType:                deploymentType,
						volumeParamsDiskIopsConfiguration:         diskIopsConfiguration,
						volumeParamsRootVolumeConfiguration:       rootVolumeConfiguration,
						volumeParamsThroughputCapacity:            throughputCapacity,
						volumeParamsWeeklyMaintenanceStartTime:    weeklyMaintenanceStartTime,
						volumeParamsSecurityGroupIds:              securityGroupIds,
						volumeParamsSubnetIds:                     subnetIds,
						volumeParamsTags:                          tags,
					},
				}

				ctx := context.Background()
				_, err := driver.CreateVolume(ctx, req)
				if err == nil {
					t.Fatal("CreateVolume is not failed")
				}

				mockCtl.Finish()
			},
		},
		{
			name: "fail: filesystem invalid parameter",
			testFunc: func(t *testing.T) {
				mockCtl := gomock.NewController(t)
				mockCloud := mocks.NewMockCloud(mockCtl)

				driver := &Driver{
					endpoint: endpoint,
					cloud:    mockCloud,
				}

				req := &csi.CreateVolumeRequest{
					Name: volumeId,
					CapacityRange: &csi.CapacityRange{
						RequiredBytes: util.GiBToBytes(storageCapacity),
						LimitBytes:    util.GiBToBytes(storageCapacity),
					},
					VolumeCapabilities: []*csi.VolumeCapability{stdVolCap},
					Parameters: map[string]string{
						volumeParamsVolumeType:                    "filesystem",
						volumeParamsKmsKeyId:                      kmsKeyId,
						volumeParamsAutomaticBackupRetentionDays:  automaticBackupRetentionDays,
						volumeParamsCopyTagsToBackups:             copyTagsToBackups,
						volumeParamsCopyTagsToVolumes:             copyTagsToVolumes,
						volumeParamsDailyAutomaticBackupStartTime: dailyAutomaticBackupStartTime,
						volumeParamsDeploymentType:                deploymentType,
						volumeParamsDiskIopsConfiguration:         diskIopsConfiguration,
						volumeParamsRootVolumeConfiguration:       rootVolumeConfiguration,
						volumeParamsThroughputCapacity:            throughputCapacity,
						volumeParamsWeeklyMaintenanceStartTime:    weeklyMaintenanceStartTime,
						volumeParamsSecurityGroupIds:              securityGroupIds,
						volumeParamsSubnetIds:                     subnetIds,
						volumeParamsTags:                          tags,
						"key":                                     "value",
					},
				}

				ctx := context.Background()

				_, err := driver.CreateVolume(ctx, req)
				if err == nil {
					t.Fatal("CreateVolume is not failed")
				}

				mockCtl.Finish()
			},
		},
		{
			name: "fail: CreateFileSystem return error",
			testFunc: func(t *testing.T) {
				mockCtl := gomock.NewController(t)
				mockCloud := mocks.NewMockCloud(mockCtl)

				driver := &Driver{
					endpoint: endpoint,
					cloud:    mockCloud,
				}

				req := &csi.CreateVolumeRequest{
					Name: filesystemId,
					CapacityRange: &csi.CapacityRange{
						RequiredBytes: util.GiBToBytes(storageCapacity),
						LimitBytes:    util.GiBToBytes(storageCapacity),
					},
					VolumeCapabilities: []*csi.VolumeCapability{stdVolCap},
					Parameters: map[string]string{
						volumeParamsVolumeType:                    "filesystem",
						volumeParamsKmsKeyId:                      kmsKeyId,
						volumeParamsAutomaticBackupRetentionDays:  automaticBackupRetentionDays,
						volumeParamsCopyTagsToBackups:             copyTagsToBackups,
						volumeParamsCopyTagsToVolumes:             copyTagsToVolumes,
						volumeParamsDailyAutomaticBackupStartTime: dailyAutomaticBackupStartTime,
						volumeParamsDeploymentType:                deploymentType,
						volumeParamsDiskIopsConfiguration:         diskIopsConfiguration,
						volumeParamsRootVolumeConfiguration:       rootVolumeConfiguration,
						volumeParamsThroughputCapacity:            throughputCapacity,
						volumeParamsWeeklyMaintenanceStartTime:    weeklyMaintenanceStartTime,
						volumeParamsSecurityGroupIds:              securityGroupIds,
						volumeParamsSubnetIds:                     subnetIds,
						volumeParamsTags:                          tags,
						volumeParamsSkipFinalBackup:               skipFinalBackup,
					},
				}

				ctx := context.Background()
				mockCloud.EXPECT().CreateFileSystem(gomock.Eq(ctx), gomock.Eq(filesystemId), gomock.Any()).Return(nil, cloud.ErrAlreadyExists)

				_, err := driver.CreateVolume(ctx, req)
				if err == nil {
					t.Fatal("CreateVolume is not failed")
				}

				mockCtl.Finish()
			},
		},
		{
			name: "fail: WaitForFileSystem return error",
			testFunc: func(t *testing.T) {
				mockCtl := gomock.NewController(t)
				mockCloud := mocks.NewMockCloud(mockCtl)

				driver := &Driver{
					endpoint: endpoint,
					cloud:    mockCloud,
				}

				req := &csi.CreateVolumeRequest{
					Name: filesystemId,
					CapacityRange: &csi.CapacityRange{
						RequiredBytes: util.GiBToBytes(storageCapacity),
						LimitBytes:    util.GiBToBytes(storageCapacity),
					},
					VolumeCapabilities: []*csi.VolumeCapability{stdVolCap},
					Parameters: map[string]string{
						volumeParamsVolumeType:                    "filesystem",
						volumeParamsKmsKeyId:                      kmsKeyId,
						volumeParamsAutomaticBackupRetentionDays:  automaticBackupRetentionDays,
						volumeParamsCopyTagsToBackups:             copyTagsToBackups,
						volumeParamsCopyTagsToVolumes:             copyTagsToVolumes,
						volumeParamsDailyAutomaticBackupStartTime: dailyAutomaticBackupStartTime,
						volumeParamsDeploymentType:                deploymentType,
						volumeParamsDiskIopsConfiguration:         diskIopsConfiguration,
						volumeParamsRootVolumeConfiguration:       rootVolumeConfiguration,
						volumeParamsThroughputCapacity:            throughputCapacity,
						volumeParamsWeeklyMaintenanceStartTime:    weeklyMaintenanceStartTime,
						volumeParamsSecurityGroupIds:              securityGroupIds,
						volumeParamsSubnetIds:                     subnetIds,
						volumeParamsTags:                          tags,
						volumeParamsSkipFinalBackup:               skipFinalBackup,
					},
				}

				ctx := context.Background()
				filesystem := &cloud.FileSystem{
					DnsName:         dnsName,
					FileSystemId:    filesystemId,
					StorageCapacity: storageCapacity,
				}
				mockCloud.EXPECT().CreateFileSystem(gomock.Eq(ctx), gomock.Eq(filesystemId), gomock.Any()).Return(filesystem, nil)
				mockCloud.EXPECT().WaitForFileSystemAvailable(gomock.Eq(ctx), gomock.Eq(filesystemId)).Return(errors.New("error"))

				_, err := driver.CreateVolume(ctx, req)
				if err == nil {
					t.Fatal("CreateVolume is not failed")
				}

				mockCtl.Finish()
			},
		},
		{
			name: "fail: CreateVolume return error",
			testFunc: func(t *testing.T) {
				mockCtl := gomock.NewController(t)
				mockCloud := mocks.NewMockCloud(mockCtl)

				driver := &Driver{
					endpoint: endpoint,
					cloud:    mockCloud,
				}

				req := &csi.CreateVolumeRequest{
					Name: volumeId,
					CapacityRange: &csi.CapacityRange{
						RequiredBytes: util.GiBToBytes(storageCapacity),
						LimitBytes:    util.GiBToBytes(storageCapacity),
					},
					VolumeCapabilities: []*csi.VolumeCapability{stdVolCap},
					Parameters: map[string]string{
						volumeParamsVolumeType:          "volume",
						volumeParamsCopyTagsToSnapshots: copyTagsToSnapshots,
						volumeParamsDataCompressionType: dataCompressionType,
						volumeParamsNfsExports:          nfsExports,
						volumeParamsParentVolumeId:      parentVolumeId,
						volumeParamsReadOnly:            readOnly,
						volumeParamsRecordSizeKiB:       recordSizeKiB,
						volumeParamsUserAndGroupQuotas:  userAndGroupQuotas,
						volumeParamsTags:                tags,
					},
				}

				ctx := context.Background()
				mockCloud.EXPECT().CreateVolume(gomock.Eq(ctx), gomock.Eq(volumeId), gomock.Any()).Return(nil, cloud.ErrAlreadyExists)

				_, err := driver.CreateVolume(ctx, req)
				if err == nil {
					t.Fatal("CreateVolume is not failed")
				}

				mockCtl.Finish()
			},
		},
		{
			name: "fail: WaitForVolume return error",
			testFunc: func(t *testing.T) {
				mockCtl := gomock.NewController(t)
				mockCloud := mocks.NewMockCloud(mockCtl)

				driver := &Driver{
					endpoint: endpoint,
					cloud:    mockCloud,
				}

				req := &csi.CreateVolumeRequest{
					Name: volumeId,
					CapacityRange: &csi.CapacityRange{
						RequiredBytes: util.GiBToBytes(storageCapacity),
						LimitBytes:    util.GiBToBytes(storageCapacity),
					},
					VolumeCapabilities: []*csi.VolumeCapability{stdVolCap},
					Parameters: map[string]string{
						volumeParamsVolumeType:          "volume",
						volumeParamsCopyTagsToSnapshots: copyTagsToSnapshots,
						volumeParamsDataCompressionType: dataCompressionType,
						volumeParamsNfsExports:          nfsExports,
						volumeParamsParentVolumeId:      parentVolumeId,
						volumeParamsReadOnly:            readOnly,
						volumeParamsRecordSizeKiB:       recordSizeKiB,
						volumeParamsUserAndGroupQuotas:  userAndGroupQuotas,
						volumeParamsTags:                tags,
					},
				}

				ctx := context.Background()
				volume := &cloud.Volume{
					FileSystemId:                  filesystemId,
					StorageCapacityQuotaGiB:       storageCapacity,
					StorageCapacityReservationGiB: storageCapacity,
					VolumePath:                    "/",
					VolumeId:                      volumeId,
				}
				mockCloud.EXPECT().CreateVolume(gomock.Eq(ctx), gomock.Eq(volumeId), gomock.Any()).Return(volume, nil)
				mockCloud.EXPECT().WaitForVolumeAvailable(gomock.Eq(ctx), gomock.Eq(volumeId)).Return(errors.New("error"))

				_, err := driver.CreateVolume(ctx, req)
				if err == nil {
					t.Fatal("CreateVolume is not failed")
				}

				mockCtl.Finish()
			},
		},
		{
			name: "fail: DescribeFileSystem return error",
			testFunc: func(t *testing.T) {
				mockCtl := gomock.NewController(t)
				mockCloud := mocks.NewMockCloud(mockCtl)

				driver := &Driver{
					endpoint: endpoint,
					cloud:    mockCloud,
				}

				req := &csi.CreateVolumeRequest{
					Name: volumeId,
					CapacityRange: &csi.CapacityRange{
						RequiredBytes: util.GiBToBytes(storageCapacity),
						LimitBytes:    util.GiBToBytes(storageCapacity),
					},
					VolumeCapabilities: []*csi.VolumeCapability{stdVolCap},
					Parameters: map[string]string{
						volumeParamsVolumeType:          "volume",
						volumeParamsCopyTagsToSnapshots: copyTagsToSnapshots,
						volumeParamsDataCompressionType: dataCompressionType,
						volumeParamsNfsExports:          nfsExports,
						volumeParamsParentVolumeId:      parentVolumeId,
						volumeParamsReadOnly:            readOnly,
						volumeParamsRecordSizeKiB:       recordSizeKiB,
						volumeParamsUserAndGroupQuotas:  userAndGroupQuotas,
						volumeParamsTags:                tags,
					},
				}
				volume := &cloud.Volume{
					FileSystemId:                  filesystemId,
					StorageCapacityQuotaGiB:       storageCapacity,
					StorageCapacityReservationGiB: storageCapacity,
					VolumePath:                    volumePath,
					VolumeId:                      volumeId,
				}

				ctx := context.Background()
				mockCloud.EXPECT().CreateVolume(gomock.Eq(ctx), gomock.Eq(volumeId), gomock.Any()).Return(volume, nil)
				mockCloud.EXPECT().WaitForVolumeAvailable(gomock.Eq(ctx), gomock.Eq(volumeId)).Return(nil)
				mockCloud.EXPECT().DescribeFileSystem(gomock.Eq(ctx), gomock.Eq(filesystemId)).Return(nil, errors.New(""))

				_, err := driver.CreateVolume(ctx, req)
				if err == nil {
					t.Fatal("CreateVolume is not failed")
				}

				mockCtl.Finish()
			},
		},
		{
			name: "fail: filesystem snapshot",
			testFunc: func(t *testing.T) {
				mockCtl := gomock.NewController(t)
				mockCloud := mocks.NewMockCloud(mockCtl)

				driver := &Driver{
					endpoint: endpoint,
					cloud:    mockCloud,
				}

				req := &csi.CreateVolumeRequest{
					Name: filesystemId,
					CapacityRange: &csi.CapacityRange{
						RequiredBytes: util.GiBToBytes(storageCapacity),
						LimitBytes:    util.GiBToBytes(storageCapacity),
					},
					VolumeCapabilities: []*csi.VolumeCapability{stdVolCap},
					Parameters: map[string]string{
						volumeParamsVolumeType:                    "filesystem",
						volumeParamsKmsKeyId:                      kmsKeyId,
						volumeParamsAutomaticBackupRetentionDays:  automaticBackupRetentionDays,
						volumeParamsCopyTagsToBackups:             copyTagsToBackups,
						volumeParamsCopyTagsToVolumes:             copyTagsToVolumes,
						volumeParamsDailyAutomaticBackupStartTime: dailyAutomaticBackupStartTime,
						volumeParamsDeploymentType:                deploymentType,
						volumeParamsDiskIopsConfiguration:         diskIopsConfiguration,
						volumeParamsRootVolumeConfiguration:       rootVolumeConfiguration,
						volumeParamsThroughputCapacity:            throughputCapacity,
						volumeParamsWeeklyMaintenanceStartTime:    weeklyMaintenanceStartTime,
						volumeParamsSecurityGroupIds:              securityGroupIds,
						volumeParamsSubnetIds:                     subnetIds,
						volumeParamsTags:                          tags,
					},
					VolumeContentSource: &csi.VolumeContentSource{
						Type: &csi.VolumeContentSource_Snapshot{
							Snapshot: &csi.VolumeContentSource_SnapshotSource{
								SnapshotId: snapshotId,
							},
						},
					},
				}

				ctx := context.Background()

				_, err := driver.CreateVolume(ctx, req)
				if err == nil {
					t.Fatal("CreateVolume is not failed")
				}

				mockCtl.Finish()
			},
		},
		{
			name: "fail: skipFinalBackup not provided",
			testFunc: func(t *testing.T) {
				mockCtl := gomock.NewController(t)
				mockCloud := mocks.NewMockCloud(mockCtl)

				driver := &Driver{
					endpoint: endpoint,
					cloud:    mockCloud,
				}

				req := &csi.CreateVolumeRequest{
					Name: filesystemId,
					CapacityRange: &csi.CapacityRange{
						RequiredBytes: util.GiBToBytes(storageCapacity),
						LimitBytes:    util.GiBToBytes(storageCapacity),
					},
					VolumeCapabilities: []*csi.VolumeCapability{stdVolCap},
					Parameters: map[string]string{
						volumeParamsVolumeType:                    "filesystem",
						volumeParamsKmsKeyId:                      kmsKeyId,
						volumeParamsAutomaticBackupRetentionDays:  automaticBackupRetentionDays,
						volumeParamsCopyTagsToBackups:             copyTagsToBackups,
						volumeParamsCopyTagsToVolumes:             copyTagsToVolumes,
						volumeParamsDailyAutomaticBackupStartTime: dailyAutomaticBackupStartTime,
						volumeParamsDeploymentType:                deploymentType,
						volumeParamsDiskIopsConfiguration:         diskIopsConfiguration,
						volumeParamsRootVolumeConfiguration:       rootVolumeConfiguration,
						volumeParamsThroughputCapacity:            throughputCapacity,
						volumeParamsWeeklyMaintenanceStartTime:    weeklyMaintenanceStartTime,
						volumeParamsSecurityGroupIds:              securityGroupIds,
						volumeParamsSubnetIds:                     subnetIds,
						volumeParamsTags:                          tags,
					},
				}

				ctx := context.Background()

				_, err := driver.CreateVolume(ctx, req)
				if err == nil {
					t.Fatal("CreateVolume is not failed")
				}

				mockCtl.Finish()
			},
		},
		{
			name: "fail: volumeType not provided",
			testFunc: func(t *testing.T) {
				mockCtl := gomock.NewController(t)
				mockCloud := mocks.NewMockCloud(mockCtl)

				driver := &Driver{
					endpoint: endpoint,
					cloud:    mockCloud,
				}

				req := &csi.CreateVolumeRequest{
					Name: filesystemId,
					CapacityRange: &csi.CapacityRange{
						RequiredBytes: util.GiBToBytes(storageCapacity),
						LimitBytes:    util.GiBToBytes(storageCapacity),
					},
					VolumeCapabilities: []*csi.VolumeCapability{stdVolCap},
					Parameters:         map[string]string{},
				}

				ctx := context.Background()

				_, err := driver.CreateVolume(ctx, req)
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

func TestDeleteVolume(t *testing.T) {
	var (
		endpoint     = "endpoint"
		fileSystemId = "fs-1234"
		volumeId     = "fsvol-1234"
	)
	testCases := []struct {
		name     string
		testFunc func(t *testing.T)
	}{
		{
			name: "success: filesystem",
			testFunc: func(t *testing.T) {
				mockCtl := gomock.NewController(t)
				mockCloud := mocks.NewMockCloud(mockCtl)

				driver := &Driver{
					endpoint: endpoint,
					cloud:    mockCloud,
				}

				req := &csi.DeleteVolumeRequest{
					VolumeId: fileSystemId,
				}

				ctx := context.Background()
				mockCloud.EXPECT().DeleteFileSystem(gomock.Eq(ctx), gomock.Eq(fileSystemId)).Return(nil)

				_, err := driver.DeleteVolume(ctx, req)
				if err != nil {
					t.Fatalf("DeleteVolume is failed: %v", err)
				}

				mockCtl.Finish()
			},
		},
		{
			name: "success: volume",
			testFunc: func(t *testing.T) {
				mockCtl := gomock.NewController(t)
				mockCloud := mocks.NewMockCloud(mockCtl)

				driver := &Driver{
					endpoint: endpoint,
					cloud:    mockCloud,
				}

				req := &csi.DeleteVolumeRequest{
					VolumeId: volumeId,
				}

				ctx := context.Background()
				mockCloud.EXPECT().DeleteVolume(gomock.Eq(ctx), gomock.Eq(volumeId)).Return(nil)

				_, err := driver.DeleteVolume(ctx, req)
				if err != nil {
					t.Fatalf("DeleteVolume is failed: %v", err)
				}

				mockCtl.Finish()
			},
		},
		{
			name: "success: DeleteFileSystem returns ErrNotFound",
			testFunc: func(t *testing.T) {
				mockCtl := gomock.NewController(t)
				mockCloud := mocks.NewMockCloud(mockCtl)

				driver := &Driver{
					endpoint: endpoint,
					cloud:    mockCloud,
				}

				req := &csi.DeleteVolumeRequest{
					VolumeId: fileSystemId,
				}

				ctx := context.Background()
				mockCloud.EXPECT().DeleteFileSystem(gomock.Eq(ctx), gomock.Eq(fileSystemId)).Return(cloud.ErrNotFound)

				_, err := driver.DeleteVolume(ctx, req)
				if err != nil {
					t.Fatalf("DeleteVolume is failed: %v", err)
				}

				mockCtl.Finish()
			},
		},
		{
			name: "fail: DeleteFileSystem returns other error",
			testFunc: func(t *testing.T) {
				mockCtl := gomock.NewController(t)
				mockCloud := mocks.NewMockCloud(mockCtl)

				driver := &Driver{
					endpoint: endpoint,
					cloud:    mockCloud,
				}

				req := &csi.DeleteVolumeRequest{
					VolumeId: fileSystemId,
				}

				ctx := context.Background()
				mockCloud.EXPECT().DeleteFileSystem(gomock.Eq(ctx), gomock.Eq(fileSystemId)).Return(errors.New("DeleteFileSystem failed"))

				_, err := driver.DeleteVolume(ctx, req)
				if err == nil {
					t.Fatal("DeleteVolume is not failed")
				}

				mockCtl.Finish()
			},
		},
		{
			name: "success: DeleteVolume returns ErrNotFound",
			testFunc: func(t *testing.T) {
				mockCtl := gomock.NewController(t)
				mockCloud := mocks.NewMockCloud(mockCtl)

				driver := &Driver{
					endpoint: endpoint,
					cloud:    mockCloud,
				}

				req := &csi.DeleteVolumeRequest{
					VolumeId: volumeId,
				}

				ctx := context.Background()
				mockCloud.EXPECT().DeleteVolume(gomock.Eq(ctx), gomock.Eq(volumeId)).Return(cloud.ErrNotFound)

				_, err := driver.DeleteVolume(ctx, req)
				if err != nil {
					t.Fatalf("DeleteVolume is failed: %v", err)
				}

				mockCtl.Finish()
			},
		},
		{
			name: "fail: DeleteVolume returns other error",
			testFunc: func(t *testing.T) {
				mockCtl := gomock.NewController(t)
				mockCloud := mocks.NewMockCloud(mockCtl)

				driver := &Driver{
					endpoint: endpoint,
					cloud:    mockCloud,
				}

				req := &csi.DeleteVolumeRequest{
					VolumeId: volumeId,
				}

				ctx := context.Background()
				mockCloud.EXPECT().DeleteVolume(gomock.Eq(ctx), gomock.Eq(volumeId)).Return(errors.New("DeleteVolume failed"))

				_, err := driver.DeleteVolume(ctx, req)
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

func TestControllerGetCapabilities(t *testing.T) {
	mockCtl := gomock.NewController(t)
	mockCloud := mocks.NewMockCloud(mockCtl)
	endpoint := "endpoint"

	driver := &Driver{
		endpoint: endpoint,
		cloud:    mockCloud,
	}

	ctx := context.Background()
	_, err := driver.ControllerGetCapabilities(ctx, &csi.ControllerGetCapabilitiesRequest{})
	if err != nil {
		t.Fatalf("ControllerGetCapabilities is failed: %v", err)
	}
}

func TestValidateVolumeCapabilities(t *testing.T) {

	var (
		endpoint     = "endpoint"
		fileSystemId = "fs-12345"
		stdVolCap    = &csi.VolumeCapability{
			AccessType: &csi.VolumeCapability_Mount{
				Mount: &csi.VolumeCapability_MountVolume{},
			},
			AccessMode: &csi.VolumeCapability_AccessMode{
				Mode: csi.VolumeCapability_AccessMode_MULTI_NODE_MULTI_WRITER,
			},
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
				mockCloud := mocks.NewMockCloud(mockCtl)

				driver := &Driver{
					endpoint: endpoint,
					cloud:    mockCloud,
				}

				ctx := context.Background()

				fs := &cloud.FileSystem{}
				mockCloud.EXPECT().DescribeFileSystem(gomock.Eq(ctx), gomock.Eq(fileSystemId)).Return(fs, nil)

				req := &csi.ValidateVolumeCapabilitiesRequest{
					VolumeId: fileSystemId,
					VolumeCapabilities: []*csi.VolumeCapability{
						stdVolCap,
					},
					VolumeContext: map[string]string{
						volumeParamsVolumeType: "filesystem",
					},
				}

				resp, err := driver.ValidateVolumeCapabilities(ctx, req)
				if err != nil {
					t.Fatalf("ControllerGetCapabilities is failed: %v", err)
				}
				if resp.Confirmed == nil {
					t.Fatal("capability is not supported")
				}
				mockCtl.Finish()
			},
		},
		{
			name: "success: volume",
			testFunc: func(t *testing.T) {
				mockCtl := gomock.NewController(t)
				mockCloud := mocks.NewMockCloud(mockCtl)

				driver := &Driver{
					endpoint: endpoint,
					cloud:    mockCloud,
				}

				ctx := context.Background()

				fs := &cloud.FileSystem{}
				mockCloud.EXPECT().DescribeFileSystem(gomock.Eq(ctx), gomock.Eq(fileSystemId)).Return(fs, nil)

				req := &csi.ValidateVolumeCapabilitiesRequest{
					VolumeId: fileSystemId,
					VolumeCapabilities: []*csi.VolumeCapability{
						stdVolCap,
					},
					VolumeContext: map[string]string{
						volumeParamsVolumeType: "filesystem",
					},
				}

				resp, err := driver.ValidateVolumeCapabilities(ctx, req)
				if err != nil {
					t.Fatalf("ControllerGetCapabilities is failed: %v", err)
				}
				if resp.Confirmed == nil {
					t.Fatal("capability is not supported")
				}
				mockCtl.Finish()
			},
		},
		{
			name: "fail: volume ID is missing",
			testFunc: func(t *testing.T) {
				mockCtl := gomock.NewController(t)
				mockCloud := mocks.NewMockCloud(mockCtl)

				driver := &Driver{
					endpoint: endpoint,
					cloud:    mockCloud,
				}

				ctx := context.Background()
				req := &csi.ValidateVolumeCapabilitiesRequest{
					VolumeCapabilities: []*csi.VolumeCapability{
						stdVolCap,
					},
				}

				_, err := driver.ValidateVolumeCapabilities(ctx, req)
				if err == nil {
					t.Fatal("ControllerGetCapabilities is not failed")
				}

				mockCtl.Finish()
			},
		},
		{
			name: "fail: volume capability is missing",
			testFunc: func(t *testing.T) {
				mockCtl := gomock.NewController(t)
				mockCloud := mocks.NewMockCloud(mockCtl)

				driver := &Driver{
					endpoint: endpoint,
					cloud:    mockCloud,
				}

				ctx := context.Background()
				req := &csi.ValidateVolumeCapabilitiesRequest{
					VolumeId: fileSystemId,
				}

				_, err := driver.ValidateVolumeCapabilities(ctx, req)
				if err == nil {
					t.Fatal("ControllerGetCapabilities is not failed")
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
		endpoint       = "endpoint"
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
				mockCloud := mocks.NewMockCloud(mockCtl)

				driver := &Driver{
					endpoint: endpoint,
					inFlight: internal.NewInFlight(),
					cloud:    mockCloud,
				}

				req := &csi.CreateSnapshotRequest{
					SourceVolumeId: volVolumeId,
					Name:           snapshotName,
					Parameters:     map[string]string{"tags": tags},
				}

				ctx := context.Background()
				snapshot := &cloud.Snapshot{
					SnapshotID:     snapshotId,
					SourceVolumeID: volVolumeId,
					CreationTime:   creationTime,
				}
				mockCloud.EXPECT().CreateSnapshot(gomock.Eq(ctx), gomock.Any()).Return(snapshot, nil)
				mockCloud.EXPECT().WaitForSnapshotAvailable(gomock.Eq(ctx), gomock.Eq(snapshotId)).Return(nil)

				resp, err := driver.CreateSnapshot(ctx, req)
				if err != nil {
					t.Fatalf("CreateSnapshot failed: %v", err)
				}

				if resp.Snapshot == nil {
					t.Fatal("resp.Snapshot is nil")
				}

				if resp.Snapshot.SnapshotId != snapshotId {
					t.Fatalf("SnapshotId mismatches. actual: %v expected %v", resp.Snapshot.SnapshotId, snapshotId)
				}

				if resp.Snapshot.SourceVolumeId != volVolumeId {
					t.Fatalf("SourceVolumeId mismatches. actual: %v expected %v", resp.Snapshot.SourceVolumeId, volVolumeId)
				}

				if resp.Snapshot.CreationTime.String() != timestamppb.New(creationTime).String() {
					t.Fatalf("CreationTime mismatches. actual: %v expected %v", resp.Snapshot.CreationTime, timestamppb.New(creationTime))
				}

				if !resp.Snapshot.ReadyToUse {
					t.Fatal("Snapshot is not ready to use")
				}

				mockCtl.Finish()
			},
		},
		{
			name: "success: normal volume snapshot of file system root volume",
			testFunc: func(t *testing.T) {
				mockCtl := gomock.NewController(t)
				mockCloud := mocks.NewMockCloud(mockCtl)

				driver := &Driver{
					endpoint: endpoint,
					inFlight: internal.NewInFlight(),
					cloud:    mockCloud,
				}

				req := &csi.CreateSnapshotRequest{
					SourceVolumeId: fsVolumeId,
					Name:           snapshotName,
					Parameters:     map[string]string{"tags": tags},
				}

				ctx := context.Background()
				snapshot := &cloud.Snapshot{
					SnapshotID:     snapshotId,
					SourceVolumeID: fsRootVolumeId,
					CreationTime:   creationTime,
				}
				mockCloud.EXPECT().CreateSnapshot(gomock.Eq(ctx), gomock.Any()).Return(snapshot, nil)
				mockCloud.EXPECT().WaitForSnapshotAvailable(gomock.Eq(ctx), gomock.Eq(snapshotId)).Return(nil)

				resp, err := driver.CreateSnapshot(ctx, req)
				if err != nil {
					t.Fatalf("CreateSnapshot failed: %v", err)
				}

				if resp.Snapshot == nil {
					t.Fatal("resp.Snapshot is nil")
				}

				if resp.Snapshot.SnapshotId != snapshotId {
					t.Fatalf("SnapshotId mismatches. actual: %v expected %v", resp.Snapshot.SnapshotId, snapshotId)
				}

				if resp.Snapshot.SourceVolumeId != fsRootVolumeId {
					t.Fatalf("SourceVolumeId mismatches. actual: %v expected %v", resp.Snapshot.SourceVolumeId, fsRootVolumeId)
				}

				if resp.Snapshot.CreationTime.String() != timestamppb.New(creationTime).String() {
					t.Fatalf("CreationTime mismatches. actual: %v expected %v", resp.Snapshot.CreationTime, timestamppb.New(creationTime))
				}

				if !resp.Snapshot.ReadyToUse {
					t.Fatal("Snapshot is not ready to use")
				}

				mockCtl.Finish()
			},
		},
		{
			name: "fail: snapshot name not provided",
			testFunc: func(t *testing.T) {
				mockCtl := gomock.NewController(t)
				mockCloud := mocks.NewMockCloud(mockCtl)

				driver := &Driver{
					endpoint: endpoint,
					inFlight: internal.NewInFlight(),
					cloud:    mockCloud,
				}

				req := &csi.CreateSnapshotRequest{
					SourceVolumeId: fsVolumeId,
					Parameters:     map[string]string{"tags": tags},
				}

				ctx := context.Background()
				_, err := driver.CreateSnapshot(ctx, req)
				if err == nil {
					t.Fatal("CreateSnapshot is not failed")
				}

				mockCtl.Finish()
			},
		},
		{
			name: "fail: source volume id not provided",
			testFunc: func(t *testing.T) {
				mockCtl := gomock.NewController(t)
				mockCloud := mocks.NewMockCloud(mockCtl)

				driver := &Driver{
					endpoint: endpoint,
					inFlight: internal.NewInFlight(),
					cloud:    mockCloud,
				}

				req := &csi.CreateSnapshotRequest{
					Name:       snapshotName,
					Parameters: map[string]string{"tags": tags},
				}

				ctx := context.Background()
				_, err := driver.CreateSnapshot(ctx, req)
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
		endpoint   = "endpoint"
		snapshotId = "fsvolsnap-1234"
	)
	testCases := []struct {
		name     string
		testFunc func(t *testing.T)
	}{
		{
			name: "success: normal delete",
			testFunc: func(t *testing.T) {
				mockCtl := gomock.NewController(t)
				mockCloud := mocks.NewMockCloud(mockCtl)

				driver := &Driver{
					endpoint: endpoint,
					inFlight: internal.NewInFlight(),
					cloud:    mockCloud,
				}

				req := &csi.DeleteSnapshotRequest{
					SnapshotId: snapshotId,
				}

				ctx := context.Background()
				mockCloud.EXPECT().DeleteSnapshot(gomock.Eq(ctx), snapshotId).Return(nil)

				_, err := driver.DeleteSnapshot(ctx, req)
				if err != nil {
					t.Fatalf("DeleteSnapshot failed: %v", err)
				}

				mockCtl.Finish()
			},
		},
		{
			name: "fail: snapshot id not provided",
			testFunc: func(t *testing.T) {
				mockCtl := gomock.NewController(t)
				mockCloud := mocks.NewMockCloud(mockCtl)

				driver := &Driver{
					endpoint: endpoint,
					inFlight: internal.NewInFlight(),
					cloud:    mockCloud,
				}

				req := &csi.DeleteSnapshotRequest{}

				ctx := context.Background()
				_, err := driver.DeleteSnapshot(ctx, req)
				if err == nil {
					t.Fatalf("DeleteSnapshot is not failed: %v", err)
				}

				mockCtl.Finish()
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, tc.testFunc)
	}
}
