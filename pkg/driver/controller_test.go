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
	"google.golang.org/protobuf/types/known/timestamppb"
	"sigs.k8s.io/aws-fsx-openzfs-csi-driver/pkg/driver/internal"
	"testing"
	"time"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/golang/mock/gomock"
	"sigs.k8s.io/aws-fsx-openzfs-csi-driver/pkg/cloud"
	"sigs.k8s.io/aws-fsx-openzfs-csi-driver/pkg/driver/mocks"
)

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
