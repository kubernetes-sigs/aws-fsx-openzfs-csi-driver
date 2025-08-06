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
	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/golang/mock/gomock"
	"github.com/kubernetes-sigs/aws-fsx-openzfs-csi-driver/pkg/cloud"
	"github.com/kubernetes-sigs/aws-fsx-openzfs-csi-driver/pkg/driver/internal"
	"github.com/kubernetes-sigs/aws-fsx-openzfs-csi-driver/pkg/driver/mocks"
	"github.com/kubernetes-sigs/aws-fsx-openzfs-csi-driver/pkg/util"
	"google.golang.org/protobuf/types/known/timestamppb"
	"testing"
	"time"
)

func TestCreateVolume(t *testing.T) {
	var (
		fileSystemParameters         map[string]string
		requiredFileSystemParameters map[string]string
		volumeParameters             map[string]string
		requiredVolumeParameters     map[string]string
		snapshotVolumeParameters     map[string]string
		filesystemId                       = "filesystemId"
		volumeId                           = "volumeId"
		storageCapacity              int32 = 64
		dnsName                            = "dnsName"
		snapshotId                         = "fsvolsnap-1234"
		volumePath                         = "/"
		snapshotArn                        = "arn:"
		creationTime                       = time.Now()
		stdVolCap                          = &csi.VolumeCapability{
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
			name: "success: filesystem all parameters",
			testFunc: func(t *testing.T) {
				mockCtl := gomock.NewController(t)
				mockCloud := mocks.NewMockCloud(mockCtl)

				driver := controllerService{
					cloud:         mockCloud,
					inFlight:      internal.NewInFlight(),
					driverOptions: &DriverOptions{},
				}

				req := &csi.CreateVolumeRequest{
					Name: filesystemId,
					CapacityRange: &csi.CapacityRange{
						RequiredBytes: util.GiBToBytes(storageCapacity),
						LimitBytes:    util.GiBToBytes(storageCapacity),
					},
					VolumeCapabilities: []*csi.VolumeCapability{stdVolCap},
					Parameters:         fileSystemParameters,
				}

				ctx := context.Background()
				filesystem := &cloud.FileSystem{
					DnsName:         dnsName,
					FileSystemId:    filesystemId,
					StorageCapacity: int32(storageCapacity),
				}

				mockCloud.EXPECT().CreateFileSystem(gomock.Eq(ctx), gomock.Any()).Return(filesystem, nil)
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

				if resp.Volume.VolumeContext[volumeContextResourceType] != "filesystem" {
					t.Fatalf("volumeContextResourceType mismatches. actual: %v expected %v", resp.Volume.VolumeContext[volumeContextResourceType], "filesystem")
				}

				if resp.Volume.VolumeContext[volumeContextDnsName] != dnsName {
					t.Fatalf("volumeContextDnsName mismatches. actual: %v expected %v", resp.Volume.VolumeContext[volumeContextDnsName], "filesystem")
				}

				mockCtl.Finish()
			},
		},
		{
			name: "success: filesystem required parameters",
			testFunc: func(t *testing.T) {
				mockCtl := gomock.NewController(t)
				mockCloud := mocks.NewMockCloud(mockCtl)

				driver := controllerService{
					cloud:         mockCloud,
					inFlight:      internal.NewInFlight(),
					driverOptions: &DriverOptions{},
				}

				req := &csi.CreateVolumeRequest{
					Name: filesystemId,
					CapacityRange: &csi.CapacityRange{
						RequiredBytes: util.GiBToBytes(storageCapacity),
						LimitBytes:    util.GiBToBytes(storageCapacity),
					},
					VolumeCapabilities: []*csi.VolumeCapability{stdVolCap},
					Parameters:         requiredFileSystemParameters,
				}

				ctx := context.Background()
				filesystem := &cloud.FileSystem{
					DnsName:         dnsName,
					FileSystemId:    filesystemId,
					StorageCapacity: int32(storageCapacity),
				}

				mockCloud.EXPECT().CreateFileSystem(gomock.Eq(ctx), gomock.Any()).Return(filesystem, nil)
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

				if resp.Volume.VolumeContext[volumeContextResourceType] != "filesystem" {
					t.Fatalf("volumeContextResourceType mismatches. actual: %v expected %v", resp.Volume.VolumeContext[volumeContextResourceType], "filesystem")
				}

				if resp.Volume.VolumeContext[volumeContextDnsName] != dnsName {
					t.Fatalf("volumeContextDnsName mismatches. actual: %v expected %v", resp.Volume.VolumeContext[volumeContextDnsName], "filesystem")
				}

				mockCtl.Finish()
			},
		},
		{
			name: "success: volume all parameters",
			testFunc: func(t *testing.T) {
				mockCtl := gomock.NewController(t)
				mockCloud := mocks.NewMockCloud(mockCtl)

				driver := controllerService{
					cloud:         mockCloud,
					inFlight:      internal.NewInFlight(),
					driverOptions: &DriverOptions{},
				}

				req := &csi.CreateVolumeRequest{
					Name: volumeId,
					CapacityRange: &csi.CapacityRange{
						RequiredBytes: util.GiBToBytes(1),
						LimitBytes:    util.GiBToBytes(1),
					},
					VolumeCapabilities: []*csi.VolumeCapability{stdVolCap},
					Parameters:         volumeParameters,
				}
				volume := &cloud.Volume{
					FileSystemId: filesystemId,
					VolumePath:   volumePath,
					VolumeId:     volumeId,
				}
				filesystem := &cloud.FileSystem{
					DnsName:         dnsName,
					FileSystemId:    filesystemId,
					StorageCapacity: int32(storageCapacity),
				}

				ctx := context.Background()
				mockCloud.EXPECT().CreateVolume(gomock.Eq(ctx), gomock.Any()).Return(volume, nil)
				mockCloud.EXPECT().WaitForVolumeAvailable(gomock.Eq(ctx), gomock.Eq(volumeId)).Return(nil)
				mockCloud.EXPECT().DescribeFileSystem(gomock.Eq(ctx), gomock.Eq(filesystemId)).Return(filesystem, nil)

				resp, err := driver.CreateVolume(ctx, req)
				if err != nil {
					t.Fatalf("CreateVolume failed: %v", err)
				}

				if resp.Volume == nil {
					t.Fatal("resp.Volume is nil")
				}

				if resp.Volume.CapacityBytes != util.GiBToBytes(1) {
					t.Fatalf("CapacityBytes mismatches. actual: %v expected %v", resp.Volume.CapacityBytes, util.GiBToBytes(storageCapacity))
				}

				if resp.Volume.VolumeId != volumeId {
					t.Fatalf("VolumeId mismatches. actual: %v expected %v", resp.Volume.VolumeId, volumeId)
				}

				if resp.Volume.VolumeContext[volumeContextResourceType] != "volume" {
					t.Fatalf("volumeContextResourceType mismatches. actual: %v expected %v", resp.Volume.VolumeContext[volumeContextResourceType], "filesystem")
				}

				if resp.Volume.VolumeContext[volumeContextDnsName] != dnsName {
					t.Fatalf("volumeContextDnsName mismatches. actual: %v expected %v", resp.Volume.VolumeContext[volumeContextDnsName], dnsName)
				}

				if resp.Volume.VolumeContext[volumeContextVolumePath] != volumePath {
					t.Fatalf("volumeContextVolumePath mismatches. actual: %v expected %v", resp.Volume.VolumeContext[volumeContextVolumePath], volumePath)
				}

				mockCtl.Finish()
			},
		},
		{
			name: "success: volume required parameters",
			testFunc: func(t *testing.T) {
				mockCtl := gomock.NewController(t)
				mockCloud := mocks.NewMockCloud(mockCtl)

				driver := controllerService{
					cloud:         mockCloud,
					inFlight:      internal.NewInFlight(),
					driverOptions: &DriverOptions{},
				}

				req := &csi.CreateVolumeRequest{
					Name: volumeId,
					CapacityRange: &csi.CapacityRange{
						RequiredBytes: util.GiBToBytes(1),
						LimitBytes:    util.GiBToBytes(1),
					},
					VolumeCapabilities: []*csi.VolumeCapability{stdVolCap},
					Parameters:         requiredVolumeParameters,
				}
				volume := &cloud.Volume{
					FileSystemId: filesystemId,
					VolumePath:   volumePath,
					VolumeId:     volumeId,
				}
				filesystem := &cloud.FileSystem{
					DnsName:         dnsName,
					FileSystemId:    filesystemId,
					StorageCapacity: int32(storageCapacity),
				}

				ctx := context.Background()
				mockCloud.EXPECT().CreateVolume(gomock.Eq(ctx), gomock.Any()).Return(volume, nil)
				mockCloud.EXPECT().WaitForVolumeAvailable(gomock.Eq(ctx), gomock.Eq(volumeId)).Return(nil)
				mockCloud.EXPECT().DescribeFileSystem(gomock.Eq(ctx), gomock.Eq(filesystemId)).Return(filesystem, nil)

				resp, err := driver.CreateVolume(ctx, req)
				if err != nil {
					t.Fatalf("CreateVolume failed: %v", err)
				}

				if resp.Volume == nil {
					t.Fatal("resp.Volume is nil")
				}

				if resp.Volume.CapacityBytes != util.GiBToBytes(1) {
					t.Fatalf("CapacityBytes mismatches. actual: %v expected %v", resp.Volume.CapacityBytes, util.GiBToBytes(storageCapacity))
				}

				if resp.Volume.VolumeId != volumeId {
					t.Fatalf("VolumeId mismatches. actual: %v expected %v", resp.Volume.VolumeId, volumeId)
				}

				if resp.Volume.VolumeContext[volumeContextResourceType] != "volume" {
					t.Fatalf("volumeContextResourceType mismatches. actual: %v expected %v", resp.Volume.VolumeContext[volumeContextResourceType], "filesystem")
				}

				if resp.Volume.VolumeContext[volumeContextDnsName] != dnsName {
					t.Fatalf("volumeContextDnsName mismatches. actual: %v expected %v", resp.Volume.VolumeContext[volumeContextDnsName], dnsName)
				}

				if resp.Volume.VolumeContext[volumeContextVolumePath] != volumePath {
					t.Fatalf("volumeContextVolumePath mismatches. actual: %v expected %v", resp.Volume.VolumeContext[volumeContextVolumePath], volumePath)
				}

				mockCtl.Finish()
			},
		},
		{
			name: "success: volume snapshot",
			testFunc: func(t *testing.T) {
				mockCtl := gomock.NewController(t)
				mockCloud := mocks.NewMockCloud(mockCtl)

				driver := controllerService{
					cloud:         mockCloud,
					inFlight:      internal.NewInFlight(),
					driverOptions: &DriverOptions{},
				}

				req := &csi.CreateVolumeRequest{
					Name: volumeId,
					CapacityRange: &csi.CapacityRange{
						RequiredBytes: util.GiBToBytes(1),
						LimitBytes:    util.GiBToBytes(1),
					},
					VolumeCapabilities: []*csi.VolumeCapability{stdVolCap},
					Parameters:         snapshotVolumeParameters,
					VolumeContentSource: &csi.VolumeContentSource{
						Type: &csi.VolumeContentSource_Snapshot{
							Snapshot: &csi.VolumeContentSource_SnapshotSource{
								SnapshotId: snapshotId,
							},
						},
					},
				}
				volume := &cloud.Volume{
					FileSystemId: filesystemId,
					VolumePath:   volumePath,
					VolumeId:     volumeId,
				}
				filesystem := &cloud.FileSystem{
					DnsName:         dnsName,
					FileSystemId:    filesystemId,
					StorageCapacity: int32(storageCapacity),
				}
				snapshot := &cloud.Snapshot{
					SnapshotID:     snapshotId,
					SourceVolumeID: volumeId,
					ResourceARN:    snapshotArn,
					CreationTime:   creationTime,
				}

				ctx := context.Background()
				mockCloud.EXPECT().DescribeSnapshot(gomock.Eq(ctx), gomock.Eq(snapshotId)).Return(snapshot, nil)
				mockCloud.EXPECT().CreateVolume(gomock.Eq(ctx), gomock.Any()).Return(volume, nil)
				mockCloud.EXPECT().WaitForVolumeAvailable(gomock.Eq(ctx), gomock.Eq(volumeId)).Return(nil)
				mockCloud.EXPECT().DescribeFileSystem(gomock.Eq(ctx), gomock.Eq(filesystemId)).Return(filesystem, nil)

				resp, err := driver.CreateVolume(ctx, req)
				if err != nil {
					t.Fatalf("CreateVolume failed: %v", err)
				}

				if resp.Volume == nil {
					t.Fatal("resp.Volume is nil")
				}

				if resp.Volume.CapacityBytes != util.GiBToBytes(1) {
					t.Fatalf("CapacityBytes mismatches. actual: %v expected %v", resp.Volume.CapacityBytes, util.GiBToBytes(storageCapacity))
				}

				if resp.Volume.VolumeId != volumeId {
					t.Fatalf("VolumeId mismatches. actual: %v expected %v", resp.Volume.VolumeId, volumeId)
				}

				if resp.Volume.VolumeContext[volumeContextResourceType] != "volume" {
					t.Fatalf("volumeContextResourceType mismatches. actual: %v expected %v", resp.Volume.VolumeContext[volumeContextResourceType], "filesystem")
				}

				if resp.Volume.VolumeContext[volumeContextDnsName] != dnsName {
					t.Fatalf("volumeContextDnsName mismatches. actual: %v expected %v", resp.Volume.VolumeContext[volumeContextDnsName], dnsName)
				}

				if resp.Volume.VolumeContext[volumeContextVolumePath] != volumePath {
					t.Fatalf("volumeContextVolumePath mismatches. actual: %v expected %v", resp.Volume.VolumeContext[volumeContextVolumePath], volumePath)
				}

				mockCtl.Finish()
			},
		},
		{
			name: "fail: ResourceType not valid",
			testFunc: func(t *testing.T) {
				mockCtl := gomock.NewController(t)
				mockCloud := mocks.NewMockCloud(mockCtl)

				driver := controllerService{
					cloud:         mockCloud,
					inFlight:      internal.NewInFlight(),
					driverOptions: &DriverOptions{},
				}

				badParameters := util.MapCopy(requiredFileSystemParameters)
				badParameters["ResourceType"] = `type`

				req := &csi.CreateVolumeRequest{
					Name: filesystemId,
					CapacityRange: &csi.CapacityRange{
						RequiredBytes: util.GiBToBytes(storageCapacity),
						LimitBytes:    util.GiBToBytes(storageCapacity),
					},
					VolumeCapabilities: []*csi.VolumeCapability{stdVolCap},
					Parameters:         badParameters,
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
			name: "fail: parameters contains driver defined variable",
			testFunc: func(t *testing.T) {
				mockCtl := gomock.NewController(t)
				mockCloud := mocks.NewMockCloud(mockCtl)

				driver := controllerService{
					cloud:         mockCloud,
					inFlight:      internal.NewInFlight(),
					driverOptions: &DriverOptions{},
				}

				badParameters := util.MapCopy(requiredFileSystemParameters)
				badParameters["StorageCapacity"] = `100`

				req := &csi.CreateVolumeRequest{
					Name: filesystemId,
					CapacityRange: &csi.CapacityRange{
						RequiredBytes: util.GiBToBytes(storageCapacity),
						LimitBytes:    util.GiBToBytes(storageCapacity),
					},
					VolumeCapabilities: []*csi.VolumeCapability{stdVolCap},
					Parameters:         badParameters,
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
			name: "fail: filesystem snapshot",
			testFunc: func(t *testing.T) {
				mockCtl := gomock.NewController(t)
				mockCloud := mocks.NewMockCloud(mockCtl)

				driver := controllerService{
					cloud:         mockCloud,
					inFlight:      internal.NewInFlight(),
					driverOptions: &DriverOptions{},
				}

				req := &csi.CreateVolumeRequest{
					Name: filesystemId,
					CapacityRange: &csi.CapacityRange{
						RequiredBytes: util.GiBToBytes(storageCapacity),
						LimitBytes:    util.GiBToBytes(storageCapacity),
					},
					VolumeCapabilities: []*csi.VolumeCapability{stdVolCap},
					Parameters:         fileSystemParameters,
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
			name: "fail: volume size not 1Gi",
			testFunc: func(t *testing.T) {
				mockCtl := gomock.NewController(t)
				mockCloud := mocks.NewMockCloud(mockCtl)

				driver := controllerService{
					cloud:         mockCloud,
					inFlight:      internal.NewInFlight(),
					driverOptions: &DriverOptions{},
				}

				req := &csi.CreateVolumeRequest{
					CapacityRange: &csi.CapacityRange{
						RequiredBytes: util.GiBToBytes(100),
						LimitBytes:    util.GiBToBytes(100),
					},
					VolumeCapabilities: []*csi.VolumeCapability{stdVolCap},
					Parameters:         volumeParameters,
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
			name: "fail: SkipFinalBackupOnDeletion not provided",
			testFunc: func(t *testing.T) {
				mockCtl := gomock.NewController(t)
				mockCloud := mocks.NewMockCloud(mockCtl)

				driver := controllerService{
					cloud:         mockCloud,
					inFlight:      internal.NewInFlight(),
					driverOptions: &DriverOptions{},
				}

				badParameters := util.MapCopy(requiredFileSystemParameters)
				delete(badParameters, "SkipFinalBackupOnDeletion")

				req := &csi.CreateVolumeRequest{
					Name: filesystemId,
					CapacityRange: &csi.CapacityRange{
						RequiredBytes: util.GiBToBytes(storageCapacity),
						LimitBytes:    util.GiBToBytes(storageCapacity),
					},
					VolumeCapabilities: []*csi.VolumeCapability{stdVolCap},
					Parameters:         badParameters,
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
			name: "fail: unknown deletion parameter",
			testFunc: func(t *testing.T) {
				mockCtl := gomock.NewController(t)
				mockCloud := mocks.NewMockCloud(mockCtl)

				driver := controllerService{
					cloud:         mockCloud,
					inFlight:      internal.NewInFlight(),
					driverOptions: &DriverOptions{},
				}

				badParameters := util.MapCopy(requiredFileSystemParameters)
				badParameters["TestOnDeletion"] = `true`

				req := &csi.CreateVolumeRequest{
					Name: filesystemId,
					CapacityRange: &csi.CapacityRange{
						RequiredBytes: util.GiBToBytes(storageCapacity),
						LimitBytes:    util.GiBToBytes(storageCapacity),
					},
					VolumeCapabilities: []*csi.VolumeCapability{stdVolCap},
					Parameters:         badParameters,
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

				driver := controllerService{
					cloud:         mockCloud,
					inFlight:      internal.NewInFlight(),
					driverOptions: &DriverOptions{},
				}

				req := &csi.CreateVolumeRequest{
					Name: filesystemId,
					CapacityRange: &csi.CapacityRange{
						RequiredBytes: util.GiBToBytes(storageCapacity),
						LimitBytes:    util.GiBToBytes(storageCapacity),
					},
					VolumeCapabilities: []*csi.VolumeCapability{stdVolCap},
					Parameters:         requiredFileSystemParameters,
				}

				ctx := context.Background()
				mockCloud.EXPECT().CreateFileSystem(gomock.Eq(ctx), gomock.Any()).Return(nil, cloud.ErrAlreadyExists)

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

				driver := controllerService{
					cloud:         mockCloud,
					inFlight:      internal.NewInFlight(),
					driverOptions: &DriverOptions{},
				}

				req := &csi.CreateVolumeRequest{
					Name: filesystemId,
					CapacityRange: &csi.CapacityRange{
						RequiredBytes: util.GiBToBytes(storageCapacity),
						LimitBytes:    util.GiBToBytes(storageCapacity),
					},
					VolumeCapabilities: []*csi.VolumeCapability{stdVolCap},
					Parameters:         requiredFileSystemParameters,
				}

				ctx := context.Background()
				filesystem := &cloud.FileSystem{
					DnsName:         dnsName,
					FileSystemId:    filesystemId,
					StorageCapacity: int32(storageCapacity),
				}
				mockCloud.EXPECT().CreateFileSystem(gomock.Eq(ctx), gomock.Any()).Return(filesystem, nil)
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

				driver := controllerService{
					cloud:         mockCloud,
					inFlight:      internal.NewInFlight(),
					driverOptions: &DriverOptions{},
				}

				req := &csi.CreateVolumeRequest{
					Name: volumeId,
					CapacityRange: &csi.CapacityRange{
						RequiredBytes: util.GiBToBytes(1),
						LimitBytes:    util.GiBToBytes(1),
					},
					VolumeCapabilities: []*csi.VolumeCapability{stdVolCap},
					Parameters:         requiredVolumeParameters,
				}

				ctx := context.Background()
				mockCloud.EXPECT().CreateVolume(gomock.Eq(ctx), gomock.Any()).Return(nil, cloud.ErrAlreadyExists)

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

				driver := controllerService{
					cloud:         mockCloud,
					inFlight:      internal.NewInFlight(),
					driverOptions: &DriverOptions{},
				}

				req := &csi.CreateVolumeRequest{
					Name: volumeId,
					CapacityRange: &csi.CapacityRange{
						RequiredBytes: util.GiBToBytes(1),
						LimitBytes:    util.GiBToBytes(1),
					},
					VolumeCapabilities: []*csi.VolumeCapability{stdVolCap},
					Parameters:         requiredVolumeParameters,
				}

				ctx := context.Background()
				volume := &cloud.Volume{
					FileSystemId: filesystemId,
					VolumePath:   "/",
					VolumeId:     volumeId,
				}
				mockCloud.EXPECT().CreateVolume(gomock.Eq(ctx), gomock.Any()).Return(volume, nil)
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

				driver := controllerService{
					cloud:         mockCloud,
					inFlight:      internal.NewInFlight(),
					driverOptions: &DriverOptions{},
				}

				req := &csi.CreateVolumeRequest{
					Name: volumeId,
					CapacityRange: &csi.CapacityRange{
						RequiredBytes: util.GiBToBytes(1),
						LimitBytes:    util.GiBToBytes(1),
					},
					VolumeCapabilities: []*csi.VolumeCapability{stdVolCap},
					Parameters:         requiredVolumeParameters,
				}
				volume := &cloud.Volume{
					FileSystemId: filesystemId,
					VolumePath:   volumePath,
					VolumeId:     volumeId,
				}

				ctx := context.Background()
				mockCloud.EXPECT().CreateVolume(gomock.Eq(ctx), gomock.Any()).Return(volume, nil)
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
			name: "fail: filesystem parameter not a valid json",
			testFunc: func(t *testing.T) {
				mockCtl := gomock.NewController(t)
				mockCloud := mocks.NewMockCloud(mockCtl)

				driver := controllerService{
					cloud:         mockCloud,
					inFlight:      internal.NewInFlight(),
					driverOptions: &DriverOptions{},
				}

				badParameters := util.MapCopy(requiredFileSystemParameters)
				badParameters["SubnetIds"] = `{invalid`

				req := &csi.CreateVolumeRequest{
					Name: filesystemId,
					CapacityRange: &csi.CapacityRange{
						RequiredBytes: util.GiBToBytes(storageCapacity),
						LimitBytes:    util.GiBToBytes(storageCapacity),
					},
					VolumeCapabilities: []*csi.VolumeCapability{stdVolCap},
					Parameters:         badParameters,
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
			name: "success: INTELLIGENT_TIERING filesystem with 1Gi",
			testFunc: func(t *testing.T) {
				mockCtl := gomock.NewController(t)
				mockCloud := mocks.NewMockCloud(mockCtl)

				driver := controllerService{
					cloud:         mockCloud,
					inFlight:      internal.NewInFlight(),
					driverOptions: &DriverOptions{},
				}

				intelligentTieringParams := map[string]string{
					"ResourceType":              "filesystem",
					"StorageType":               `"INTELLIGENT_TIERING"`,
					"DeploymentType":            `"MULTI_AZ_1"`,
					"ThroughputCapacity":        `160`,
					"SubnetIds":                 `["subnet-test"]`,
					"SkipFinalBackupOnDeletion": `true`,
				}

				req := &csi.CreateVolumeRequest{
					Name: filesystemId,
					CapacityRange: &csi.CapacityRange{
						RequiredBytes: util.GiBToBytes(1),
						LimitBytes:    util.GiBToBytes(1),
					},
					VolumeCapabilities: []*csi.VolumeCapability{stdVolCap},
					Parameters:         intelligentTieringParams,
				}

				ctx := context.Background()
				filesystem := &cloud.FileSystem{
					DnsName:         dnsName,
					FileSystemId:    filesystemId,
					StorageCapacity: 1,
				}

				mockCloud.EXPECT().CreateFileSystem(gomock.Eq(ctx), gomock.Any()).DoAndReturn(
					func(ctx context.Context, params map[string]string) (*cloud.FileSystem, error) {
						if _, exists := params["StorageCapacity"]; exists {
							t.Error("StorageCapacity should be removed for INTELLIGENT_TIERING")
						}
						return filesystem, nil
					})
				mockCloud.EXPECT().WaitForFileSystemAvailable(gomock.Eq(ctx), gomock.Eq(filesystemId)).Return(nil)

				_, err := driver.CreateVolume(ctx, req)
				if err != nil {
					t.Fatalf("CreateVolume failed: %v", err)
				}

				mockCtl.Finish()
			},
		},
		{
			name: "fail: INTELLIGENT_TIERING filesystem with non-1Gi",
			testFunc: func(t *testing.T) {
				mockCtl := gomock.NewController(t)
				mockCloud := mocks.NewMockCloud(mockCtl)

				driver := controllerService{
					cloud:         mockCloud,
					inFlight:      internal.NewInFlight(),
					driverOptions: &DriverOptions{},
				}

				intelligentTieringParams := map[string]string{
					"ResourceType":              "filesystem",
					"StorageType":               `"INTELLIGENT_TIERING"`,
					"DeploymentType":            `"MULTI_AZ_1"`,
					"ThroughputCapacity":        `160`,
					"SubnetIds":                 `["subnet-test"]`,
					"SkipFinalBackupOnDeletion": `true`,
				}

				req := &csi.CreateVolumeRequest{
					Name: filesystemId,
					CapacityRange: &csi.CapacityRange{
						RequiredBytes: util.GiBToBytes(100),
						LimitBytes:    util.GiBToBytes(100),
					},
					VolumeCapabilities: []*csi.VolumeCapability{stdVolCap},
					Parameters:         intelligentTieringParams,
				}

				ctx := context.Background()

				_, err := driver.CreateVolume(ctx, req)
				if err == nil {
					t.Fatal("CreateVolume should have failed")
				}

				mockCtl.Finish()
			},
		},
		{
			name: "fail: volume parameter not a valid json",
			testFunc: func(t *testing.T) {
				mockCtl := gomock.NewController(t)
				mockCloud := mocks.NewMockCloud(mockCtl)

				driver := controllerService{
					cloud:         mockCloud,
					inFlight:      internal.NewInFlight(),
					driverOptions: &DriverOptions{},
				}

				badParameters := util.MapCopy(requiredVolumeParameters)
				badParameters["ParentVolumeId"] = `{invalid`

				req := &csi.CreateVolumeRequest{
					Name: filesystemId,
					CapacityRange: &csi.CapacityRange{
						RequiredBytes: util.GiBToBytes(1),
						LimitBytes:    util.GiBToBytes(1),
					},
					VolumeCapabilities: []*csi.VolumeCapability{stdVolCap},
					Parameters:         badParameters,
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
		fileSystemParameters = map[string]string{
			"ResourceType":                  "filesystem",
			"DeploymentType":                `"SINGLE_AZ_1"`,
			"ThroughputCapacity":            `64`,
			"SubnetIds":                     `["subnet-0eabfaa81fb22bcaf"]`,
			"SkipFinalBackupOnDeletion":     `true`,
			"OptionsOnDeletion":             `["DELETE_CHILD_VOLUMES_AND_SNAPSHOTS"]`,
			"KmsKeyId":                      `"1234abcd-12ab-34cd-56ef-1234567890ab"`,
			"AutomaticBackupRetentionDays":  `1`,
			"CopyTagsToBackups":             `false`,
			"CopyTagsToVolumes":             `false`,
			"DailyAutomaticBackupStartTime": `"00:00"`,
			"DiskIopsConfiguration":         `{"Iops": 300, "Mode": "USER_PROVISIONED"}`,
			"RootVolumeConfiguration":       `{"CopyTagsToSnapshots": false, "DataCompressionType": "NONE", "NfsExports": [{"ClientConfigurations": [{"Clients": "*", "Options": ["rw","crossmnt"]}]}], "ReadOnly": true, "RecordSize": 128, "UserAndGroupQuotas": [{"Type": "User", "Id": 1, "StorageCapacityQuotaGiB": 10}]}`,
			"WeeklyMaintenanceStartTime":    `"7:09:00"`,
			"SecurityGroupIds":              `["sg-068000ccf82dfba88"]`,
			"Tags":                          `[{"Key": "OPENZFS", "Value": "OPENZFS"}]`,
		}
		requiredFileSystemParameters = map[string]string{
			"ResourceType":              fileSystemParameters["ResourceType"],
			"DeploymentType":            fileSystemParameters["DeploymentType"],
			"ThroughputCapacity":        fileSystemParameters["ThroughputCapacity"],
			"SubnetIds":                 fileSystemParameters["SubnetIds"],
			"SkipFinalBackupOnDeletion": fileSystemParameters["SkipFinalBackupOnDeletion"],
		}
		volumeParameters = map[string]string{
			"ResourceType":        "volume",
			"CopyTagsToSnapshots": `false`,
			"DataCompressionType": `"NONE"`,
			"NfsExports":          `[{"ClientConfigurations": [{"Clients": "*", "Options": ["rw","crossmnt"]}]}]`,
			"ParentVolumeId":      `"fsvol-03062e7ff37662dff"`,
			"ReadOnly":            `false`,
			"RecordSizeKiB":       `128`,
			"UserAndGroupQuotas":  `[{"Type": "User","Id": 1, "StorageCapacityQuotaGiB": 10}]`,
			"Tags":                `[{"Key": "OPENZFS", "Value": "OPENZFS"}]`,
			"OptionsOnDeletion":   `["DELETE_CHILD_VOLUMES_AND_SNAPSHOTS"]`,
		}
		requiredVolumeParameters = map[string]string{
			"ResourceType":   volumeParameters["ResourceType"],
			"ParentVolumeId": volumeParameters["ParentVolumeId"],
		}
		snapshotVolumeParameters = map[string]string{
			"ResourceType":   volumeParameters["ResourceType"],
			"ParentVolumeId": volumeParameters["ParentVolumeId"],
			"OriginSnapshot": `{"CopyStrategy": "CLONE"}`,
		}

		t.Run(tc.name, tc.testFunc)
	}
}

func TestDeleteVolume(t *testing.T) {
	var (
		filesystemParameters map[string]string
		volumeParameters     map[string]string
		fileSystemId         = "fs-1234"
		volumeId             = "fsvol-1234"
	)
	testCases := []struct {
		name     string
		testFunc func(t *testing.T)
	}{
		{
			name: "success: filesystem all parameters",
			testFunc: func(t *testing.T) {
				mockCtl := gomock.NewController(t)
				mockCloud := mocks.NewMockCloud(mockCtl)

				driver := controllerService{
					cloud:         mockCloud,
					inFlight:      internal.NewInFlight(),
					driverOptions: &DriverOptions{},
				}

				req := &csi.DeleteVolumeRequest{
					VolumeId: fileSystemId,
				}

				ctx := context.Background()
				mockCloud.EXPECT().GetDeleteParameters(gomock.Eq(ctx), gomock.Any()).Return(filesystemParameters, nil)
				mockCloud.EXPECT().DeleteFileSystem(gomock.Eq(ctx), gomock.Any()).Return(nil)

				_, err := driver.DeleteVolume(ctx, req)
				if err != nil {
					t.Fatalf("DeleteVolume is failed: %v", err)
				}

				mockCtl.Finish()
			},
		},
		{
			name: "success: filesystem no parameters",
			testFunc: func(t *testing.T) {
				mockCtl := gomock.NewController(t)
				mockCloud := mocks.NewMockCloud(mockCtl)

				driver := controllerService{
					cloud:         mockCloud,
					inFlight:      internal.NewInFlight(),
					driverOptions: &DriverOptions{},
				}

				req := &csi.DeleteVolumeRequest{
					VolumeId: fileSystemId,
				}

				ctx := context.Background()
				mockCloud.EXPECT().GetDeleteParameters(gomock.Eq(ctx), gomock.Any()).Return(map[string]string{}, nil)
				mockCloud.EXPECT().DeleteFileSystem(gomock.Eq(ctx), gomock.Any()).Return(nil)

				_, err := driver.DeleteVolume(ctx, req)
				if err != nil {
					t.Fatalf("DeleteVolume is failed: %v", err)
				}

				mockCtl.Finish()
			},
		},
		{
			name: "success: volume all parameters",
			testFunc: func(t *testing.T) {
				mockCtl := gomock.NewController(t)
				mockCloud := mocks.NewMockCloud(mockCtl)

				driver := controllerService{
					cloud:         mockCloud,
					inFlight:      internal.NewInFlight(),
					driverOptions: &DriverOptions{},
				}

				req := &csi.DeleteVolumeRequest{
					VolumeId: volumeId,
				}

				ctx := context.Background()
				mockCloud.EXPECT().GetDeleteParameters(gomock.Eq(ctx), gomock.Any()).Return(volumeParameters, nil)
				mockCloud.EXPECT().DeleteVolume(gomock.Eq(ctx), gomock.Any()).Return(nil)

				_, err := driver.DeleteVolume(ctx, req)
				if err != nil {
					t.Fatalf("DeleteVolume is failed: %v", err)
				}

				mockCtl.Finish()
			},
		},
		{
			name: "success: volume no parameters",
			testFunc: func(t *testing.T) {
				mockCtl := gomock.NewController(t)
				mockCloud := mocks.NewMockCloud(mockCtl)

				driver := controllerService{
					cloud:         mockCloud,
					inFlight:      internal.NewInFlight(),
					driverOptions: &DriverOptions{},
				}

				req := &csi.DeleteVolumeRequest{
					VolumeId: volumeId,
				}

				ctx := context.Background()
				mockCloud.EXPECT().GetDeleteParameters(gomock.Eq(ctx), gomock.Any()).Return(map[string]string{}, nil)
				mockCloud.EXPECT().DeleteVolume(gomock.Eq(ctx), gomock.Any()).Return(nil)

				_, err := driver.DeleteVolume(ctx, req)
				if err != nil {
					t.Fatalf("DeleteVolume is failed: %v", err)
				}

				mockCtl.Finish()
			},
		},
		{
			name: "fail: GetDeleteParameters returns error",
			testFunc: func(t *testing.T) {
				mockCtl := gomock.NewController(t)
				mockCloud := mocks.NewMockCloud(mockCtl)

				driver := controllerService{
					cloud:         mockCloud,
					inFlight:      internal.NewInFlight(),
					driverOptions: &DriverOptions{},
				}

				req := &csi.DeleteVolumeRequest{
					VolumeId: fileSystemId,
				}

				ctx := context.Background()
				mockCloud.EXPECT().GetDeleteParameters(gomock.Eq(ctx), gomock.Any()).Return(nil, errors.New(""))

				_, err := driver.DeleteVolume(ctx, req)
				if err == nil {
					t.Fatal("DeleteVolume is not failed")
				}

				mockCtl.Finish()
			},
		},
		{
			name: "success: DeleteFileSystem returns ErrNotFound",
			testFunc: func(t *testing.T) {
				mockCtl := gomock.NewController(t)
				mockCloud := mocks.NewMockCloud(mockCtl)

				driver := controllerService{
					cloud:         mockCloud,
					inFlight:      internal.NewInFlight(),
					driverOptions: &DriverOptions{},
				}

				req := &csi.DeleteVolumeRequest{
					VolumeId: fileSystemId,
				}

				ctx := context.Background()
				mockCloud.EXPECT().GetDeleteParameters(gomock.Eq(ctx), gomock.Any()).Return(volumeParameters, nil)
				mockCloud.EXPECT().DeleteFileSystem(gomock.Eq(ctx), gomock.Any()).Return(cloud.ErrNotFound)

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

				driver := controllerService{
					cloud:         mockCloud,
					inFlight:      internal.NewInFlight(),
					driverOptions: &DriverOptions{},
				}

				req := &csi.DeleteVolumeRequest{
					VolumeId: fileSystemId,
				}

				ctx := context.Background()
				mockCloud.EXPECT().GetDeleteParameters(gomock.Eq(ctx), gomock.Any()).Return(volumeParameters, nil)
				mockCloud.EXPECT().DeleteFileSystem(gomock.Eq(ctx), gomock.Any()).Return(errors.New(""))

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

				driver := controllerService{
					cloud:         mockCloud,
					inFlight:      internal.NewInFlight(),
					driverOptions: &DriverOptions{},
				}

				req := &csi.DeleteVolumeRequest{
					VolumeId: volumeId,
				}

				ctx := context.Background()
				mockCloud.EXPECT().GetDeleteParameters(gomock.Eq(ctx), gomock.Any()).Return(volumeParameters, nil)
				mockCloud.EXPECT().DeleteVolume(gomock.Eq(ctx), gomock.Any()).Return(cloud.ErrNotFound)

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

				driver := controllerService{
					cloud:         mockCloud,
					inFlight:      internal.NewInFlight(),
					driverOptions: &DriverOptions{},
				}

				req := &csi.DeleteVolumeRequest{
					VolumeId: volumeId,
				}

				ctx := context.Background()
				mockCloud.EXPECT().GetDeleteParameters(gomock.Eq(ctx), gomock.Any()).Return(volumeParameters, nil)
				mockCloud.EXPECT().DeleteVolume(gomock.Eq(ctx), gomock.Any()).Return(errors.New(""))

				_, err := driver.DeleteVolume(ctx, req)
				if err == nil {
					t.Fatal("DeleteVolume is not failed")
				}

				mockCtl.Finish()
			},
		},
	}

	for _, tc := range testCases {
		filesystemParameters = map[string]string{
			"SkipFinalBackup": `true`,
			"Options":         `["DELETE_CHILD_VOLUMES_AND_SNAPSHOTS"]`,
		}
		volumeParameters = map[string]string{
			"Options": `["DELETE_CHILD_VOLUMES_AND_SNAPSHOTS"]`,
		}
		t.Run(tc.name, tc.testFunc)
	}
}

func TestControllerGetCapabilities(t *testing.T) {
	mockCtl := gomock.NewController(t)
	mockCloud := mocks.NewMockCloud(mockCtl)

	driver := controllerService{
		cloud:         mockCloud,
		inFlight:      internal.NewInFlight(),
		driverOptions: &DriverOptions{},
	}

	ctx := context.Background()
	_, err := driver.ControllerGetCapabilities(ctx, &csi.ControllerGetCapabilitiesRequest{})
	if err != nil {
		t.Fatalf("ControllerGetCapabilities is failed: %v", err)
	}
}

func TestValidateVolumeCapabilities(t *testing.T) {
	var (
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

				driver := controllerService{
					cloud:         mockCloud,
					inFlight:      internal.NewInFlight(),
					driverOptions: &DriverOptions{},
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
						volumeParamsResourceType: "filesystem",
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

				driver := controllerService{
					cloud:         mockCloud,
					inFlight:      internal.NewInFlight(),
					driverOptions: &DriverOptions{},
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
						volumeParamsResourceType: "filesystem",
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

				driver := controllerService{
					cloud:         mockCloud,
					inFlight:      internal.NewInFlight(),
					driverOptions: &DriverOptions{},
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

				driver := controllerService{
					cloud:         mockCloud,
					inFlight:      internal.NewInFlight(),
					driverOptions: &DriverOptions{},
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
		snapshotName   = "snapshot-1234abcd-12ab-34cd-56ef-123456abcdef"
		volVolumeId    = "fsvol-1234"
		fsVolumeId     = "fs-1234"
		fsRootVolumeId = "fsvol-5678"
		snapshotId     = "fsvolsnap-1234"
		parameters     map[string]string
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

				driver := controllerService{
					cloud:         mockCloud,
					inFlight:      internal.NewInFlight(),
					driverOptions: &DriverOptions{},
				}

				req := &csi.CreateSnapshotRequest{
					SourceVolumeId: volVolumeId,
					Name:           snapshotName,
					Parameters:     parameters,
				}

				ctx := context.Background()
				snapshot := &cloud.Snapshot{
					SnapshotID:     snapshotId,
					SourceVolumeID: volVolumeId,
					CreationTime:   creationTime,
				}
				mockCloud.EXPECT().GetVolumeId(gomock.Eq(ctx), volVolumeId).Return(volVolumeId, nil)
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

				driver := controllerService{
					cloud:         mockCloud,
					inFlight:      internal.NewInFlight(),
					driverOptions: &DriverOptions{},
				}

				req := &csi.CreateSnapshotRequest{
					SourceVolumeId: fsVolumeId,
					Name:           snapshotName,
					Parameters:     parameters,
				}

				ctx := context.Background()
				snapshot := &cloud.Snapshot{
					SnapshotID:     snapshotId,
					SourceVolumeID: fsRootVolumeId,
					CreationTime:   creationTime,
				}
				mockCloud.EXPECT().GetVolumeId(gomock.Eq(ctx), fsVolumeId).Return(fsRootVolumeId, nil)
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
			name: "success: required parameters",
			testFunc: func(t *testing.T) {
				mockCtl := gomock.NewController(t)
				mockCloud := mocks.NewMockCloud(mockCtl)

				driver := controllerService{
					cloud:         mockCloud,
					inFlight:      internal.NewInFlight(),
					driverOptions: &DriverOptions{},
				}

				req := &csi.CreateSnapshotRequest{
					SourceVolumeId: fsVolumeId,
					Name:           snapshotName,
					Parameters:     map[string]string{},
				}

				ctx := context.Background()
				snapshot := &cloud.Snapshot{
					SnapshotID:     snapshotId,
					SourceVolumeID: fsRootVolumeId,
					CreationTime:   creationTime,
				}
				mockCloud.EXPECT().GetVolumeId(gomock.Eq(ctx), fsVolumeId).Return(fsRootVolumeId, nil)
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

				driver := controllerService{
					cloud:         mockCloud,
					inFlight:      internal.NewInFlight(),
					driverOptions: &DriverOptions{},
				}

				req := &csi.CreateSnapshotRequest{
					SourceVolumeId: fsVolumeId,
					Parameters:     map[string]string{},
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

				driver := controllerService{
					cloud:         mockCloud,
					inFlight:      internal.NewInFlight(),
					driverOptions: &DriverOptions{},
				}

				req := &csi.CreateSnapshotRequest{
					Name:       snapshotName,
					Parameters: map[string]string{},
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
			name: "fail: invalid parameter",
			testFunc: func(t *testing.T) {
				mockCtl := gomock.NewController(t)
				mockCloud := mocks.NewMockCloud(mockCtl)

				driver := controllerService{
					cloud:         mockCloud,
					inFlight:      internal.NewInFlight(),
					driverOptions: &DriverOptions{},
				}

				req := &csi.CreateSnapshotRequest{
					Name: snapshotName,
					Parameters: map[string]string{
						"BadParam": "{test",
					},
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
		parameters = map[string]string{
			"Tags": `[{"Key": "OPENZFS", "Value": "OPENZFS"}]`,
		}
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
			name: "success: normal delete",
			testFunc: func(t *testing.T) {
				mockCtl := gomock.NewController(t)
				mockCloud := mocks.NewMockCloud(mockCtl)

				driver := controllerService{
					cloud:         mockCloud,
					inFlight:      internal.NewInFlight(),
					driverOptions: &DriverOptions{},
				}

				req := &csi.DeleteSnapshotRequest{
					SnapshotId: snapshotId,
				}

				ctx := context.Background()
				mockCloud.EXPECT().DeleteSnapshot(gomock.Eq(ctx), gomock.Any()).Return(nil)

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

				driver := controllerService{
					cloud:         mockCloud,
					inFlight:      internal.NewInFlight(),
					driverOptions: &DriverOptions{},
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

func TestControllerExpandVolume(t *testing.T) {
	var (
		dnsName               = "dnsName"
		filesystemId          = "fs-1234"
		volumeId              = "fsvol-1234"
		storageCapacity int32 = 100
		currentBytes          = util.GiBToBytes(storageCapacity)
		newCapacity     int32 = 150
		requiredBytes         = util.GiBToBytes(newCapacity)
		limitBytes            = util.GiBToBytes(newCapacity)
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

				driver := controllerService{
					cloud:         mockCloud,
					inFlight:      internal.NewInFlight(),
					driverOptions: &DriverOptions{},
				}

				req := &csi.ControllerExpandVolumeRequest{
					VolumeId: filesystemId,
					CapacityRange: &csi.CapacityRange{
						RequiredBytes: requiredBytes,
						LimitBytes:    limitBytes,
					},
				}
				filesystem := &cloud.FileSystem{
					DnsName:         dnsName,
					FileSystemId:    filesystemId,
					StorageCapacity: int32(storageCapacity),
				}

				ctx := context.Background()
				newCapacityInt32 := int32(newCapacity)
				mockCloud.EXPECT().DescribeFileSystem(gomock.Eq(ctx), gomock.Eq(filesystemId)).Return(filesystem, nil)
				mockCloud.EXPECT().ResizeFileSystem(gomock.Eq(ctx), gomock.Eq(filesystemId), gomock.Any()).Return(&newCapacityInt32, nil)
				mockCloud.EXPECT().WaitForFileSystemResize(gomock.Eq(ctx), gomock.Eq(filesystemId), newCapacityInt32).Return(nil)

				resp, err := driver.ControllerExpandVolume(ctx, req)
				if err != nil {
					t.Fatalf("ControllerExpandVolume failed: %v", err)
				}

				if resp.CapacityBytes != requiredBytes {
					t.Fatal("resp.CapacityBytes is not equal to requiredBytes")
				}

				mockCtl.Finish()
			},
		},
		{
			name: "failure: volume",
			testFunc: func(t *testing.T) {
				mockCtl := gomock.NewController(t)
				mockCloud := mocks.NewMockCloud(mockCtl)

				driver := controllerService{
					cloud:         mockCloud,
					inFlight:      internal.NewInFlight(),
					driverOptions: &DriverOptions{},
				}

				req := &csi.ControllerExpandVolumeRequest{
					VolumeId: volumeId,
					CapacityRange: &csi.CapacityRange{
						RequiredBytes: requiredBytes,
						LimitBytes:    limitBytes,
					},
				}

				ctx := context.Background()

				_, err := driver.ControllerExpandVolume(ctx, req)
				if err == nil {
					t.Fatal("ControllerExpandVolume success")
				}

				mockCtl.Finish()
			},
		},
		{
			name: "success: newCapacity less than currentCapacity",
			testFunc: func(t *testing.T) {
				mockCtl := gomock.NewController(t)
				mockCloud := mocks.NewMockCloud(mockCtl)
				var lowerBytes int64 = 1

				driver := controllerService{
					cloud:         mockCloud,
					inFlight:      internal.NewInFlight(),
					driverOptions: &DriverOptions{},
				}

				req := &csi.ControllerExpandVolumeRequest{
					VolumeId: filesystemId,
					CapacityRange: &csi.CapacityRange{
						RequiredBytes: lowerBytes,
						LimitBytes:    limitBytes,
					},
				}
				filesystem := &cloud.FileSystem{
					DnsName:         dnsName,
					FileSystemId:    filesystemId,
					StorageCapacity: int32(storageCapacity),
				}

				ctx := context.Background()
				mockCloud.EXPECT().DescribeFileSystem(gomock.Eq(ctx), gomock.Eq(filesystemId)).Return(filesystem, nil)

				resp, err := driver.ControllerExpandVolume(ctx, req)
				if err != nil {
					t.Fatalf("ControllerExpandVolume failed: %v", err)
				}

				if resp.CapacityBytes != currentBytes {
					t.Fatal("resp.CapacityBytes is not equal to currentBytes")
				}

				mockCtl.Finish()
			},
		},
		{
			name: "fail: required capacity greater than limit",
			testFunc: func(t *testing.T) {
				mockCtl := gomock.NewController(t)
				mockCloud := mocks.NewMockCloud(mockCtl)

				driver := controllerService{
					cloud:         mockCloud,
					inFlight:      internal.NewInFlight(),
					driverOptions: &DriverOptions{},
				}

				req := &csi.ControllerExpandVolumeRequest{
					VolumeId: filesystemId,
					CapacityRange: &csi.CapacityRange{
						RequiredBytes: requiredBytes,
						LimitBytes:    1,
					},
				}

				ctx := context.Background()

				_, err := driver.ControllerExpandVolume(ctx, req)
				if err == nil {
					t.Fatalf("ControllerExpandVolume success: %v", err)
				}

				mockCtl.Finish()
			},
		},
		{
			name: "fail: missing volumeId",
			testFunc: func(t *testing.T) {
				mockCtl := gomock.NewController(t)
				mockCloud := mocks.NewMockCloud(mockCtl)

				driver := controllerService{
					cloud:         mockCloud,
					inFlight:      internal.NewInFlight(),
					driverOptions: &DriverOptions{},
				}

				req := &csi.ControllerExpandVolumeRequest{
					CapacityRange: &csi.CapacityRange{
						RequiredBytes: requiredBytes,
						LimitBytes:    limitBytes,
					},
				}

				ctx := context.Background()

				_, err := driver.ControllerExpandVolume(ctx, req)
				if err == nil {
					t.Fatalf("ControllerExpandVolume success: %v", err)
				}

				mockCtl.Finish()
			},
		},
		{
			name: "fail: missing capacity range",
			testFunc: func(t *testing.T) {
				mockCtl := gomock.NewController(t)
				mockCloud := mocks.NewMockCloud(mockCtl)

				driver := controllerService{
					cloud:         mockCloud,
					inFlight:      internal.NewInFlight(),
					driverOptions: &DriverOptions{},
				}

				req := &csi.ControllerExpandVolumeRequest{
					VolumeId: filesystemId,
				}

				ctx := context.Background()

				_, err := driver.ControllerExpandVolume(ctx, req)
				if err == nil {
					t.Fatalf("ControllerExpandVolume success: %v", err)
				}

				mockCtl.Finish()
			},
		},
		{
			name: "fail: describe throws error",
			testFunc: func(t *testing.T) {
				mockCtl := gomock.NewController(t)
				mockCloud := mocks.NewMockCloud(mockCtl)

				driver := controllerService{
					cloud:         mockCloud,
					inFlight:      internal.NewInFlight(),
					driverOptions: &DriverOptions{},
				}

				req := &csi.ControllerExpandVolumeRequest{
					VolumeId: filesystemId,
					CapacityRange: &csi.CapacityRange{
						RequiredBytes: requiredBytes,
						LimitBytes:    limitBytes,
					},
				}

				ctx := context.Background()
				mockCloud.EXPECT().DescribeFileSystem(gomock.Eq(ctx), gomock.Eq(filesystemId)).Return(nil, errors.New(""))

				_, err := driver.ControllerExpandVolume(ctx, req)
				if err == nil {
					t.Fatalf("ControllerExpandVolume success: %v", err)
				}

				mockCtl.Finish()
			},
		},
		{
			name: "fail: resize throws error",
			testFunc: func(t *testing.T) {
				mockCtl := gomock.NewController(t)
				mockCloud := mocks.NewMockCloud(mockCtl)

				driver := controllerService{
					cloud:         mockCloud,
					inFlight:      internal.NewInFlight(),
					driverOptions: &DriverOptions{},
				}

				req := &csi.ControllerExpandVolumeRequest{
					VolumeId: filesystemId,
					CapacityRange: &csi.CapacityRange{
						RequiredBytes: requiredBytes,
						LimitBytes:    limitBytes,
					},
				}
				filesystem := &cloud.FileSystem{
					DnsName:         dnsName,
					FileSystemId:    filesystemId,
					StorageCapacity: int32(storageCapacity),
				}

				ctx := context.Background()
				mockCloud.EXPECT().DescribeFileSystem(gomock.Eq(ctx), gomock.Eq(filesystemId)).Return(filesystem, nil)
				mockCloud.EXPECT().ResizeFileSystem(gomock.Eq(ctx), gomock.Eq(filesystemId), gomock.Any()).Return(nil, errors.New(""))

				_, err := driver.ControllerExpandVolume(ctx, req)
				if err == nil {
					t.Fatalf("ControllerExpandVolume success: %v", err)
				}

				mockCtl.Finish()
			},
		},
		{
			name: "fail: wait throws error",
			testFunc: func(t *testing.T) {
				mockCtl := gomock.NewController(t)
				mockCloud := mocks.NewMockCloud(mockCtl)

				driver := controllerService{
					cloud:         mockCloud,
					inFlight:      internal.NewInFlight(),
					driverOptions: &DriverOptions{},
				}

				req := &csi.ControllerExpandVolumeRequest{
					VolumeId: filesystemId,
					CapacityRange: &csi.CapacityRange{
						RequiredBytes: requiredBytes,
						LimitBytes:    limitBytes,
					},
				}
				filesystem := &cloud.FileSystem{
					DnsName:         dnsName,
					FileSystemId:    filesystemId,
					StorageCapacity: int32(storageCapacity),
				}

				ctx := context.Background()
				newCapacityInt32 := int32(newCapacity)
				mockCloud.EXPECT().DescribeFileSystem(gomock.Eq(ctx), gomock.Eq(filesystemId)).Return(filesystem, nil)
				mockCloud.EXPECT().ResizeFileSystem(gomock.Eq(ctx), gomock.Eq(filesystemId), gomock.Any()).Return(&newCapacityInt32, nil)
				mockCloud.EXPECT().WaitForFileSystemResize(gomock.Eq(ctx), gomock.Eq(filesystemId), newCapacityInt32).Return(errors.New(""))

				_, err := driver.ControllerExpandVolume(ctx, req)
				if err == nil {
					t.Fatalf("ControllerExpandVolume success: %v", err)
				}

				mockCtl.Finish()
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, tc.testFunc)
	}
}
