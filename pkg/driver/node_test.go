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
	"fmt"
	"testing"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/golang/mock/gomock"
	"sigs.k8s.io/aws-fsx-openzfs-csi-driver/pkg/driver/mocks"
)

func TestNodePublishVolume(t *testing.T) {

	var (
		endpoint        = "endpoint"
		nodeID          = "nodeID"
		fsVolumeId      = "fs-0a2d0632b5ff567e9"
		volumeId        = "fsvol-0efb292807cc770ff"
		dnsname         = "fs-0a2d0632b5ff567e9.fsx.us-west-2.amazonaws.com"
		volumemountname = "/fsx/testVol"
		targetPath      = "/target/path"
		stdVolCap       = &csi.VolumeCapability{
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
			name: "success: normal filesystem nfs mount",
			testFunc: func(t *testing.T) {
				mockCtrl := gomock.NewController(t)
				mockMounter := mocks.NewMockMounter(mockCtrl)
				driver := &Driver{
					endpoint: endpoint,
					nodeID:   nodeID,
					mounter:  mockMounter,
				}
				source := fmt.Sprintf("%s:%s", dnsname, rootVolumePath)

				ctx := context.Background()
				req := &csi.NodePublishVolumeRequest{
					VolumeId: fsVolumeId,
					VolumeContext: map[string]string{
						"dnsname":    dnsname,
						"mountname":  rootVolumePath,
						"volumeType": fsVolumeType,
					},
					VolumeCapability: stdVolCap,
					TargetPath:       targetPath,
				}

				mockMounter.EXPECT().MakeDir(gomock.Eq(targetPath)).Return(nil)
				mockMounter.EXPECT().IsLikelyNotMountPoint(gomock.Eq(targetPath)).Return(true, nil)
				mockMounter.EXPECT().Mount(gomock.Eq(source), gomock.Eq(targetPath), gomock.Eq("nfs"), gomock.Any()).Return(nil)
				_, err := driver.NodePublishVolume(ctx, req)
				if err != nil {
					t.Fatalf("NodePublishVolume is failed: %v", err)
				}

				mockCtrl.Finish()
			},
		},
		{
			name: "success: normal volume nfs mount",
			testFunc: func(t *testing.T) {
				mockCtrl := gomock.NewController(t)
				mockMounter := mocks.NewMockMounter(mockCtrl)
				driver := &Driver{
					endpoint: endpoint,
					nodeID:   nodeID,
					mounter:  mockMounter,
				}
				source := fmt.Sprintf("%s:%s", dnsname, volumemountname)

				ctx := context.Background()
				req := &csi.NodePublishVolumeRequest{
					VolumeId: volumeId,
					VolumeContext: map[string]string{
						"dnsname":    dnsname,
						"mountname":  volumemountname,
						"volumeType": volVolumeType,
					},
					VolumeCapability: stdVolCap,
					TargetPath:       targetPath,
				}

				mockMounter.EXPECT().MakeDir(gomock.Eq(targetPath)).Return(nil)
				mockMounter.EXPECT().IsLikelyNotMountPoint(gomock.Eq(targetPath)).Return(true, nil)
				mockMounter.EXPECT().Mount(gomock.Eq(source), gomock.Eq(targetPath), gomock.Eq("nfs"), gomock.Any()).Return(nil)
				_, err := driver.NodePublishVolume(ctx, req)
				if err != nil {
					t.Fatalf("NodePublishVolume is failed: %v", err)
				}

				mockCtrl.Finish()
			},
		},
		{
			name: "success: missing mountname for file system mount in static provisioning, default 'fsx' used",
			testFunc: func(t *testing.T) {
				mockCtrl := gomock.NewController(t)
				mockMounter := mocks.NewMockMounter(mockCtrl)
				driver := &Driver{
					endpoint: endpoint,
					nodeID:   nodeID,
					mounter:  mockMounter,
				}
				source := fmt.Sprintf("%s:%s", dnsname, rootVolumePath)

				ctx := context.Background()
				req := &csi.NodePublishVolumeRequest{
					VolumeId: fsVolumeId,
					VolumeContext: map[string]string{
						"dnsname":    dnsname,
						"volumeType": fsVolumeType,
					},
					VolumeCapability: stdVolCap,
					TargetPath:       targetPath,
				}

				mockMounter.EXPECT().MakeDir(gomock.Eq(targetPath)).Return(nil)
				mockMounter.EXPECT().IsLikelyNotMountPoint(gomock.Eq(targetPath)).Return(true, nil)
				mockMounter.EXPECT().Mount(gomock.Eq(source), gomock.Eq(targetPath), gomock.Eq("nfs"), gomock.Any()).Return(nil)
				_, err := driver.NodePublishVolume(ctx, req)
				if err != nil {
					t.Fatalf("NodePublishVolume is failed: %v", err)
				}

				mockCtrl.Finish()
			},
		},
		{
			name: "success: normal nfs mount with read only mount",
			testFunc: func(t *testing.T) {
				mockCtrl := gomock.NewController(t)
				mockMounter := mocks.NewMockMounter(mockCtrl)
				driver := &Driver{
					endpoint: endpoint,
					nodeID:   nodeID,
					mounter:  mockMounter,
				}

				source := fmt.Sprintf("%s:%s", dnsname, rootVolumePath)

				ctx := context.Background()
				req := &csi.NodePublishVolumeRequest{
					VolumeId: fsVolumeId,
					VolumeContext: map[string]string{
						"dnsname":    dnsname,
						"mountname":  rootVolumePath,
						"volumeType": fsVolumeType,
					},
					VolumeCapability: stdVolCap,
					TargetPath:       targetPath,
					Readonly:         true,
				}

				mockMounter.EXPECT().MakeDir(gomock.Eq(targetPath)).Return(nil)
				mockMounter.EXPECT().IsLikelyNotMountPoint(gomock.Eq(targetPath)).Return(true, nil)
				mockMounter.EXPECT().Mount(gomock.Eq(source), gomock.Eq(targetPath), gomock.Eq("nfs"), gomock.Eq([]string{"ro"})).Return(nil)
				_, err := driver.NodePublishVolume(ctx, req)
				if err != nil {
					t.Fatalf("NodePublishVolume is failed: %v", err)
				}

				mockCtrl.Finish()
			},
		},
		{
			name: "success: normal nfs mount with mount options",
			testFunc: func(t *testing.T) {
				mockCtrl := gomock.NewController(t)
				mockMounter := mocks.NewMockMounter(mockCtrl)
				driver := &Driver{
					endpoint: endpoint,
					nodeID:   nodeID,
					mounter:  mockMounter,
				}

				source := fmt.Sprintf("%s:%s", dnsname, rootVolumePath)

				ctx := context.Background()
				req := &csi.NodePublishVolumeRequest{
					VolumeId: "volumeId",
					VolumeContext: map[string]string{
						"dnsname":    dnsname,
						"mountname":  rootVolumePath,
						"volumeType": fsVolumeType,
					},
					VolumeCapability: &csi.VolumeCapability{
						AccessType: &csi.VolumeCapability_Mount{
							Mount: &csi.VolumeCapability_MountVolume{
								MountFlags: []string{"nfsvers=4.1", "rsize=1048576", "wsize=1048576", "timeo=600"},
							},
						},
						AccessMode: &csi.VolumeCapability_AccessMode{
							Mode: csi.VolumeCapability_AccessMode_MULTI_NODE_MULTI_WRITER,
						},
					},
					TargetPath: targetPath,
				}

				mockMounter.EXPECT().MakeDir(gomock.Eq(targetPath)).Return(nil)
				mockMounter.EXPECT().IsLikelyNotMountPoint(gomock.Eq(targetPath)).Return(true, nil)
				mockMounter.EXPECT().Mount(gomock.Eq(source), gomock.Eq(targetPath), gomock.Eq("nfs"), gomock.Eq([]string{"nfsvers=4.1", "rsize=1048576", "wsize=1048576", "timeo=600"})).Return(nil)
				_, err := driver.NodePublishVolume(ctx, req)
				if err != nil {
					t.Fatalf("NodePublishVolume is failed: %v", err)
				}

				mockCtrl.Finish()
			},
		},
		{
			name: "fail: missing volume id",
			testFunc: func(t *testing.T) {
				mockCtrl := gomock.NewController(t)
				mockMounter := mocks.NewMockMounter(mockCtrl)
				driver := &Driver{
					endpoint: endpoint,
					nodeID:   nodeID,
					mounter:  mockMounter,
				}

				ctx := context.Background()
				req := &csi.NodePublishVolumeRequest{
					VolumeContext: map[string]string{
						"dnsname":    dnsname,
						"mountname":  rootVolumePath,
						"volumeType": fsVolumeType,
					},
					VolumeCapability: stdVolCap,
					TargetPath:       targetPath,
				}

				_, err := driver.NodePublishVolume(ctx, req)
				if err == nil {
					t.Fatalf("NodePublishVolume is not failed: %v", err)
				}

				mockCtrl.Finish()
			},
		},
		{
			name: "fail: missing volume capability",
			testFunc: func(t *testing.T) {
				mockCtrl := gomock.NewController(t)
				mockMounter := mocks.NewMockMounter(mockCtrl)
				driver := &Driver{
					endpoint: endpoint,
					nodeID:   nodeID,
					mounter:  mockMounter,
				}

				ctx := context.Background()
				req := &csi.NodePublishVolumeRequest{
					VolumeId: "volumeId",
					VolumeContext: map[string]string{
						"dnsname":    dnsname,
						"mountname":  rootVolumePath,
						"volumeType": fsVolumeType,
					},
					TargetPath: targetPath,
				}

				_, err := driver.NodePublishVolume(ctx, req)
				if err == nil {
					t.Fatalf("NodePublishVolume is not failed: %v", err)
				}

				mockCtrl.Finish()
			},
		},
		{
			name: "fail: unsupported volume capability",
			testFunc: func(t *testing.T) {
				mockCtrl := gomock.NewController(t)
				mockMounter := mocks.NewMockMounter(mockCtrl)
				driver := &Driver{
					endpoint: endpoint,
					nodeID:   nodeID,
					mounter:  mockMounter,
				}

				ctx := context.Background()
				req := &csi.NodePublishVolumeRequest{
					VolumeId: fsVolumeId,
					VolumeContext: map[string]string{
						"dnsname":    dnsname,
						"mountname":  rootVolumePath,
						"volumeType": fsVolumeType,
					},
					VolumeCapability: &csi.VolumeCapability{
						AccessType: &csi.VolumeCapability_Mount{
							Mount: &csi.VolumeCapability_MountVolume{},
						},
						AccessMode: &csi.VolumeCapability_AccessMode{
							Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_READER_ONLY,
						},
					},
					TargetPath: targetPath,
				}

				_, err := driver.NodePublishVolume(ctx, req)
				if err == nil {
					t.Fatalf("NodePublishVolume is not failed: %v", err)
				}

				mockCtrl.Finish()
			},
		},
		{
			name: "fail: missing dns name",
			testFunc: func(t *testing.T) {
				mockCtrl := gomock.NewController(t)
				mockMounter := mocks.NewMockMounter(mockCtrl)
				driver := &Driver{
					endpoint: endpoint,
					nodeID:   nodeID,
					mounter:  mockMounter,
				}

				ctx := context.Background()
				req := &csi.NodePublishVolumeRequest{
					VolumeId: fsVolumeId,
					VolumeContext: map[string]string{
						"mountname":  rootVolumePath,
						"volumeType": fsVolumeType,
					},
					VolumeCapability: stdVolCap,
					TargetPath:       targetPath,
				}

				_, err := driver.NodePublishVolume(ctx, req)
				if err == nil {
					t.Fatalf("NodePublishVolume is not failed: %v", err)
				}

				mockCtrl.Finish()
			},
		},
		{
			name: "fail: invalid volume type",
			testFunc: func(t *testing.T) {
				mockCtrl := gomock.NewController(t)
				mockMounter := mocks.NewMockMounter(mockCtrl)
				driver := &Driver{
					endpoint: endpoint,
					nodeID:   nodeID,
					mounter:  mockMounter,
				}

				ctx := context.Background()
				req := &csi.NodePublishVolumeRequest{
					VolumeId: fsVolumeId,
					VolumeContext: map[string]string{
						"dnsname":    dnsname,
						"mountname":  rootVolumePath,
						"volumeType": "voolume",
					},
					VolumeCapability: stdVolCap,
					TargetPath:       targetPath,
				}

				_, err := driver.NodePublishVolume(ctx, req)
				if err == nil {
					t.Fatalf("NodePublishVolume is not failed: %v", err)
				}

				mockCtrl.Finish()
			},
		},
		{
			name: "fail: missing mountname when mounting volume",
			testFunc: func(t *testing.T) {
				mockCtrl := gomock.NewController(t)
				mockMounter := mocks.NewMockMounter(mockCtrl)
				driver := &Driver{
					endpoint: endpoint,
					nodeID:   nodeID,
					mounter:  mockMounter,
				}

				ctx := context.Background()
				req := &csi.NodePublishVolumeRequest{
					VolumeId: volumeId,
					VolumeContext: map[string]string{
						"dnsname":    dnsname,
						"volumeType": volVolumeType,
					},
					VolumeCapability: stdVolCap,
					TargetPath:       targetPath,
				}

				_, err := driver.NodePublishVolume(ctx, req)
				if err == nil {
					t.Fatalf("NodePublishVolume is not failed: %v", err)
				}

				mockCtrl.Finish()
			},
		},
		{
			name: "fail: invalid mountname when mounting a file system",
			testFunc: func(t *testing.T) {
				mockCtrl := gomock.NewController(t)
				mockMounter := mocks.NewMockMounter(mockCtrl)
				driver := &Driver{
					endpoint: endpoint,
					nodeID:   nodeID,
					mounter:  mockMounter,
				}

				ctx := context.Background()
				req := &csi.NodePublishVolumeRequest{
					VolumeId: fsVolumeId,
					VolumeContext: map[string]string{
						"dnsname":    dnsname,
						"mountname":  volumemountname,
						"volumeType": fsVolumeType,
					},
					VolumeCapability: stdVolCap,
					TargetPath:       targetPath,
				}

				_, err := driver.NodePublishVolume(ctx, req)
				if err == nil {
					t.Fatalf("NodePublishVolume is not failed: %v", err)
				}

				mockCtrl.Finish()
			},
		},
		{
			name: "fail: missing target path",
			testFunc: func(t *testing.T) {
				mockCtrl := gomock.NewController(t)
				mockMounter := mocks.NewMockMounter(mockCtrl)
				driver := &Driver{
					endpoint: endpoint,
					nodeID:   nodeID,
					mounter:  mockMounter,
				}

				ctx := context.Background()
				req := &csi.NodePublishVolumeRequest{
					VolumeId: fsVolumeId,
					VolumeContext: map[string]string{
						"dnsname":    dnsname,
						"mountname":  rootVolumePath,
						"volumeType": fsVolumeType,
					},
					VolumeCapability: stdVolCap,
				}

				_, err := driver.NodePublishVolume(ctx, req)
				if err == nil {
					t.Fatalf("NodePublishVolume is not failed: %v", err)
				}

				mockCtrl.Finish()
			},
		},
		{
			name: "fail: mounter failed to MakeDir",
			testFunc: func(t *testing.T) {
				mockCtrl := gomock.NewController(t)
				mockMounter := mocks.NewMockMounter(mockCtrl)
				driver := &Driver{
					endpoint: endpoint,
					nodeID:   nodeID,
					mounter:  mockMounter,
				}

				ctx := context.Background()
				req := &csi.NodePublishVolumeRequest{
					VolumeId: fsVolumeId,
					VolumeContext: map[string]string{
						"dnsname":    dnsname,
						"mountname":  rootVolumePath,
						"volumeType": fsVolumeType,
					},
					VolumeCapability: stdVolCap,
					TargetPath:       targetPath,
				}

				err := fmt.Errorf("failed to MakeDir")
				mockMounter.EXPECT().MakeDir(gomock.Eq(targetPath)).Return(err)

				_, err = driver.NodePublishVolume(ctx, req)
				if err == nil {
					t.Fatalf("NodePublishVolume is not failed: %v", err)
				}

				mockCtrl.Finish()
			},
		},
		{
			name: "fail: mounter failed to Mount",
			testFunc: func(t *testing.T) {
				mockCtrl := gomock.NewController(t)
				mockMounter := mocks.NewMockMounter(mockCtrl)
				driver := &Driver{
					endpoint: endpoint,
					nodeID:   nodeID,
					mounter:  mockMounter,
				}

				ctx := context.Background()
				req := &csi.NodePublishVolumeRequest{
					VolumeId: fsVolumeId,
					VolumeContext: map[string]string{
						"dnsname":    dnsname,
						"mountname":  rootVolumePath,
						"volumeType": fsVolumeType,
					},
					VolumeCapability: stdVolCap,
					TargetPath:       targetPath,
				}

				source := fmt.Sprintf("%s:%s", dnsname, rootVolumePath)
				err := fmt.Errorf("failed to Mount")
				mockMounter.EXPECT().MakeDir(gomock.Eq(targetPath)).Return(nil)
				mockMounter.EXPECT().IsLikelyNotMountPoint(gomock.Eq(targetPath)).Return(true, nil)
				mockMounter.EXPECT().Mount(gomock.Eq(source), gomock.Eq(targetPath), gomock.Eq("nfs"), gomock.Any()).Return(err)

				_, err = driver.NodePublishVolume(ctx, req)
				if err == nil {
					t.Fatalf("NodePublishVolume is not failed: %v", err)
				}

				mockCtrl.Finish()
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, tc.testFunc)
	}
}

func TestNodeUnpublishVolume(t *testing.T) {

	var (
		endpoint   = "endpoint"
		nodeID     = "nodeID"
		targetPath = "/target/path"
		volumeId   = "fsvol-0efb292807cc770ff"
	)

	testCases := []struct {
		name     string
		testFunc func(t *testing.T)
	}{
		{
			name: "success: normal",
			testFunc: func(t *testing.T) {
				mockCtrl := gomock.NewController(t)
				mockMounter := mocks.NewMockMounter(mockCtrl)
				driver := &Driver{
					endpoint: endpoint,
					nodeID:   nodeID,
					mounter:  mockMounter,
				}

				ctx := context.Background()
				req := &csi.NodeUnpublishVolumeRequest{
					VolumeId:   volumeId,
					TargetPath: targetPath,
				}

				mockMounter.EXPECT().IsLikelyNotMountPoint(gomock.Eq(targetPath)).Return(false, nil)
				mockMounter.EXPECT().Unmount(gomock.Eq(targetPath)).Return(nil)

				_, err := driver.NodeUnpublishVolume(ctx, req)
				if err != nil {
					t.Fatalf("NodeUnpublishVolume is failed: %v", err)
				}
			},
		},
		{
			name: "success: target already unmounted",
			testFunc: func(t *testing.T) {
				mockCtrl := gomock.NewController(t)
				mockMounter := mocks.NewMockMounter(mockCtrl)
				driver := &Driver{
					endpoint: endpoint,
					nodeID:   nodeID,
					mounter:  mockMounter,
				}

				ctx := context.Background()
				req := &csi.NodeUnpublishVolumeRequest{
					VolumeId:   volumeId,
					TargetPath: targetPath,
				}

				mockMounter.EXPECT().IsLikelyNotMountPoint(gomock.Eq(targetPath)).Return(true, nil)

				_, err := driver.NodeUnpublishVolume(ctx, req)
				if err != nil {
					t.Fatalf("NodeUnpublishVolume is failed: %v", err)
				}
			},
		},
		{
			name: "fail: targetPath is missing",
			testFunc: func(t *testing.T) {
				mockCtrl := gomock.NewController(t)
				mockMounter := mocks.NewMockMounter(mockCtrl)
				driver := &Driver{
					endpoint: endpoint,
					nodeID:   nodeID,
					mounter:  mockMounter,
				}

				ctx := context.Background()
				req := &csi.NodeUnpublishVolumeRequest{
					VolumeId: "volumeId",
				}

				_, err := driver.NodeUnpublishVolume(ctx, req)
				if err == nil {
					t.Fatalf("NodeUnpublishVolume is not failed: %v", err)
				}
			},
		},
		{
			name: "fail: mounter failed to umount",
			testFunc: func(t *testing.T) {
				mockCtrl := gomock.NewController(t)
				mockMounter := mocks.NewMockMounter(mockCtrl)
				driver := &Driver{
					endpoint: endpoint,
					nodeID:   nodeID,
					mounter:  mockMounter,
				}

				ctx := context.Background()
				req := &csi.NodeUnpublishVolumeRequest{
					VolumeId:   volumeId,
					TargetPath: targetPath,
				}

				mockMounter.EXPECT().IsLikelyNotMountPoint(gomock.Eq(targetPath)).Return(false, nil)
				mountErr := fmt.Errorf("Unmount failed")
				mockMounter.EXPECT().Unmount(gomock.Eq(targetPath)).Return(mountErr)

				_, err := driver.NodeUnpublishVolume(ctx, req)
				if err == nil {
					t.Fatalf("NodeUnpublishVolume is not failed: %v", err)
				}
			},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, tc.testFunc)
	}
}
