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
	"fmt"
	"github.com/container-storage-interface/spec/lib/go/csi"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
	"k8s.io/klog/v2"
	"sigs.k8s.io/aws-fsx-openzfs-csi-driver/pkg/cloud"
	"sigs.k8s.io/aws-fsx-openzfs-csi-driver/pkg/driver/internal"
	"sigs.k8s.io/aws-fsx-openzfs-csi-driver/pkg/util"
	"strings"
)

var (
	// controllerCaps represents the capability of controller service
	controllerCaps = []csi.ControllerServiceCapability_RPC_Type{
		csi.ControllerServiceCapability_RPC_CREATE_DELETE_VOLUME,
		csi.ControllerServiceCapability_RPC_CREATE_DELETE_SNAPSHOT,
	}
)

const (
	volumeContextVolumeType                   = "volumeType"
	volumeContextDnsName                      = "dnsName"
	volumeContextVolumePath                   = "volumePath"
	volumeContextSkipFinalBackup              = "skipFinalBackup"
	volumeParamsVolumeType                    = "volumeType"
	volumeParamsKmsKeyId                      = "kmsKeyId"
	volumeParamsAutomaticBackupRetentionDays  = "automaticBackupRetentionDays"
	volumeParamsCopyTagsToBackups             = "copyTagsToBackups"
	volumeParamsCopyTagsToVolumes             = "copyTagsToVolumes"
	volumeParamsDailyAutomaticBackupStartTime = "dailyAutomaticBackupStartTime"
	volumeParamsDeploymentType                = "deploymentType"
	volumeParamsDiskIopsConfiguration         = "diskIopsConfiguration"
	volumeParamsRootVolumeConfiguration       = "rootVolumeConfiguration"
	volumeParamsThroughputCapacity            = "throughputCapacity"
	volumeParamsWeeklyMaintenanceStartTime    = "weeklyMaintenanceStartTime"
	volumeParamsSecurityGroupIds              = "securityGroupIds"
	volumeParamsSubnetIds                     = "subnetIds"
	volumeParamsTags                          = "tags"
	volumeParamsSkipFinalBackup               = "skipFinalBackup"
	volumeParamsCopyTagsToSnapshots           = "copyTagsToSnapshots"
	volumeParamsDataCompressionType           = "dataCompressionType"
	volumeParamsNfsExports                    = "NfsExports"
	volumeParamsParentVolumeId                = "parentVolumeId"
	volumeParamsReadOnly                      = "readOnly"
	volumeParamsRecordSizeKiB                 = "recordSizeKiB"
	volumeParamsUserAndGroupQuotas            = "userAndGroupQuotas"
)

const (
	fsVolumeType   = "filesystem"
	volVolumeType  = "volume"
	rootVolumePath = "fsx"
)

func (d *Driver) CreateVolume(ctx context.Context, req *csi.CreateVolumeRequest) (*csi.CreateVolumeResponse, error) {
	klog.V(4).Infof("CreateVolume: called with args %#v", req)

	volName := req.GetName()
	if len(volName) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Volume name not provided")
	}

	volCaps := req.GetVolumeCapabilities()
	if len(volCaps) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Volume capabilities not provided")
	}

	if !d.isValidVolumeCapabilities(volCaps) {
		return nil, status.Error(codes.InvalidArgument, "Volume capabilities not supported")
	}

	volumeParams := req.GetParameters()
	volumeType := volumeParams[volumeParamsVolumeType]

	var storageCapacity *int64

	if req.GetCapacityRange() != nil {
		convertedCapacity := util.BytesToGiB(req.GetCapacityRange().GetRequiredBytes())
		storageCapacity = &convertedCapacity
	}

	if volumeType == fsVolumeType {
		if req.GetVolumeContentSource() != nil {
			return nil, status.Error(codes.InvalidArgument, "Snapshots are not available at the filesystem level. Set volumeType to volume.")
		}

		if _, ok := volumeParams[volumeParamsSkipFinalBackup]; !ok {
			return nil, status.Errorf(codes.InvalidArgument, "invalid parameter of %s: %s", volumeParamsSkipFinalBackup, "field required")
		}

		fsOptions := cloud.FileSystemOptions{
			StorageCapacity: storageCapacity,
		}

		for key, value := range volumeParams {
			switch key {
			case volumeParamsKmsKeyId:
				fsOptions.KmsKeyId = util.StringToStringPointer(value)
			case volumeParamsAutomaticBackupRetentionDays:
				i, err := util.StringToIntPointer(value)
				if err != nil {
					return nil, status.Errorf(codes.InvalidArgument, "invalid parameter of %s: %s", volumeParamsAutomaticBackupRetentionDays, err)
				}
				fsOptions.AutomaticBackupRetentionDays = i
			case volumeParamsCopyTagsToBackups:
				boolVal, err := util.StringToBoolPointer(value)
				if err != nil {
					return nil, status.Errorf(codes.InvalidArgument, "invalid parameter of %s: %s", volumeParamsCopyTagsToBackups, err)
				}
				fsOptions.CopyTagsToBackups = boolVal
			case volumeParamsCopyTagsToVolumes:
				boolVal, err := util.StringToBoolPointer(value)
				if err != nil {
					return nil, status.Errorf(codes.InvalidArgument, "invalid parameter of %s: %s", volumeParamsCopyTagsToVolumes, err)
				}
				fsOptions.CopyTagsToVolumes = boolVal
			case volumeParamsDailyAutomaticBackupStartTime:
				fsOptions.DailyAutomaticBackupStartTime = util.StringToStringPointer(value)
			case volumeParamsDeploymentType:
				fsOptions.DeploymentType = util.StringToStringPointer(value)
			case volumeParamsDiskIopsConfiguration:
				fsOptions.DiskIopsConfiguration = util.StringToStringPointer(value)
			case volumeParamsRootVolumeConfiguration:
				fsOptions.RootVolumeConfiguration = util.StringToStringPointer(value)
			case volumeParamsThroughputCapacity:
				i, err := util.StringToIntPointer(value)
				if err != nil {
					return nil, status.Errorf(codes.InvalidArgument, "invalid parameter of %s: %s", volumeParamsThroughputCapacity, err)
				}
				fsOptions.ThroughputCapacity = i
			case volumeParamsWeeklyMaintenanceStartTime:
				fsOptions.WeeklyMaintenanceStartTime = util.StringToStringPointer(value)
			case volumeParamsSecurityGroupIds:
				fsOptions.SecurityGroupIds = util.SplitCommasAndRemoveOuterBrackets(value)
			case volumeParamsSubnetIds:
				fsOptions.SubnetIds = util.SplitCommasAndRemoveOuterBrackets(value)
			case volumeParamsTags:
				fsOptions.Tags = util.StringToStringPointer(value)
			case volumeParamsSkipFinalBackup:
				boolVal, err := util.StringToBoolPointer(value)
				if err != nil {
					return nil, status.Errorf(codes.InvalidArgument, "invalid parameter of %s: %s", volumeParamsSkipFinalBackup, err)
				}
				fsOptions.SkipFinalBackup = boolVal
			}
		}

		fs, err := d.cloud.CreateFileSystem(ctx, volName, fsOptions)
		if err != nil {
			klog.V(4).Infof("CreateFileSystem error: ", err.Error())
			switch {
			case errors.Is(err, cloud.ErrInvalidParameter):
				return nil, status.Error(codes.InvalidArgument, err.Error())
			case errors.Is(err, cloud.ErrAlreadyExists):
				return nil, status.Error(codes.AlreadyExists, err.Error())
			default:
				return nil, status.Errorf(codes.Internal, "Could not create volume %q: %v", volName, err)
			}
		}

		err = d.cloud.WaitForFileSystemAvailable(ctx, fs.FileSystemId)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "Filesystem is not ready: %v", err)
		}

		return &csi.CreateVolumeResponse{
			Volume: &csi.Volume{
				CapacityBytes: util.GiBToBytes(fs.StorageCapacity),
				VolumeId:      fs.FileSystemId,
				VolumeContext: map[string]string{
					volumeContextDnsName:         fs.DnsName,
					volumeContextVolumeType:      volumeType,
					volumeContextSkipFinalBackup: volumeParams[volumeParamsSkipFinalBackup],
				},
			},
		}, nil
	}
	if volumeType == volVolumeType {
		var snapshotArn *string
		volumeSource := req.GetVolumeContentSource()
		if volumeSource != nil {
			if _, ok := volumeSource.GetType().(*csi.VolumeContentSource_Snapshot); !ok {
				return nil, status.Error(codes.InvalidArgument, "Unsupported volumeContentSource type")
			}
			sourceSnapshot := volumeSource.GetSnapshot()
			if sourceSnapshot == nil {
				return nil, status.Error(codes.InvalidArgument, "Error retrieving snapshot from the volumeContentSource")
			}

			id := sourceSnapshot.GetSnapshotId()
			snapshot, err := d.cloud.DescribeSnapshot(ctx, id)
			if err != nil {
				return nil, err
			}
			snapshotArn = &snapshot.ResourceARN
		}

		volOptions := cloud.VolumeOptions{
			Name:                          &volName,
			StorageCapacityQuotaGiB:       storageCapacity,
			StorageCapacityReservationGiB: storageCapacity,
			SnapshotARN:                   snapshotArn,
		}

		for key, value := range volumeParams {
			switch key {
			case volumeParamsCopyTagsToSnapshots:
				boolVal, err := util.StringToBoolPointer(value)
				if err != nil {
					return nil, status.Errorf(codes.InvalidArgument, "invalid parameter of %s: %s", volumeParamsCopyTagsToSnapshots, err)
				}
				volOptions.CopyTagsToSnapshots = boolVal
			case volumeParamsDataCompressionType:
				volOptions.DataCompressionType = util.StringToStringPointer(value)
			case volumeParamsNfsExports:
				volOptions.NfsExports = util.StringToStringPointer(value)
			case volumeParamsParentVolumeId:
				volOptions.ParentVolumeId = util.StringToStringPointer(value)
			case volumeParamsReadOnly:
				boolVal, err := util.StringToBoolPointer(value)
				if err != nil {
					return nil, status.Errorf(codes.InvalidArgument, "invalid parameter of %s: %s", volumeParamsReadOnly, err)
				}
				volOptions.ReadOnly = boolVal
			case volumeParamsRecordSizeKiB:
				i, err := util.StringToIntPointer(value)
				if err != nil {
					return nil, status.Errorf(codes.InvalidArgument, "invalid parameter of %s: %s", volumeParamsRecordSizeKiB, err)
				}
				volOptions.RecordSizeKiB = i
			case volumeParamsUserAndGroupQuotas:
				volOptions.UserAndGroupQuotas = util.StringToStringPointer(value)
			case volumeParamsTags:
				volOptions.Tags = util.StringToStringPointer(value)
			}
		}

		v, err := d.cloud.CreateVolume(ctx, volName, volOptions)
		if err != nil {
			klog.V(4).Infof("CreateVolume error: ", err.Error())
			switch {
			case errors.Is(err, cloud.ErrInvalidParameter):
				return nil, status.Error(codes.InvalidArgument, err.Error())
			case errors.Is(err, cloud.ErrAlreadyExists):
				return nil, status.Error(codes.AlreadyExists, err.Error())
			default:
				return nil, status.Errorf(codes.Internal, "Could not create volume %q: %v", volName, err)
			}
		}

		err = d.cloud.WaitForVolumeAvailable(ctx, v.VolumeId)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "Volume is not ready: %v", err)
		}

		fileSystem, err := d.cloud.DescribeFileSystem(ctx, v.FileSystemId)
		if err != nil {
			return nil, err
		}

		return &csi.CreateVolumeResponse{
			Volume: &csi.Volume{
				CapacityBytes: util.GiBToBytes(v.StorageCapacityReservationGiB),
				VolumeId:      v.VolumeId,
				VolumeContext: map[string]string{
					volumeContextDnsName:    fileSystem.DnsName,
					volumeContextVolumeType: volumeType,
					volumeContextVolumePath: v.VolumePath,
				},
			},
		}, nil
	}
	return nil, status.Error(codes.InvalidArgument, "Volume type not supported")
}

func (d *Driver) DeleteVolume(ctx context.Context, req *csi.DeleteVolumeRequest) (*csi.DeleteVolumeResponse, error) {
	var err error
	volumeID := req.GetVolumeId()
	if len(volumeID) == 0 {
		return nil, status.Error(codes.InvalidArgument, "VolumeId is empty")
	}

	splitVolumeId := strings.SplitN(volumeID, "-", 2)
	if splitVolumeId[0] == cloud.FilesystemPrefix {
		err = d.cloud.DeleteFileSystem(ctx, volumeID)
	}
	if splitVolumeId[0] == cloud.VolumePrefix {
		err = d.cloud.DeleteVolume(ctx, volumeID)
	}

	if err != nil {
		if err == cloud.ErrNotFound {
			klog.V(4).Infof("DeleteVolume: volume not found, returning with success")
			return &csi.DeleteVolumeResponse{}, nil
		}
		return nil, status.Errorf(codes.Internal, "Could not delete volume ID %q: %v", volumeID, err)
	}

	return &csi.DeleteVolumeResponse{}, nil
}

func (d *Driver) ControllerPublishVolume(ctx context.Context, req *csi.ControllerPublishVolumeRequest) (*csi.ControllerPublishVolumeResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

func (d *Driver) ControllerUnpublishVolume(ctx context.Context, req *csi.ControllerUnpublishVolumeRequest) (*csi.ControllerUnpublishVolumeResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

func (d *Driver) ValidateVolumeCapabilities(ctx context.Context, req *csi.ValidateVolumeCapabilitiesRequest) (*csi.ValidateVolumeCapabilitiesResponse, error) {
	klog.V(4).Infof("ValidateVolumeCapabilities: called with args %#v", req)
	var err error

	volumeID := req.GetVolumeId()
	if len(volumeID) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Volume ID not provided")
	}

	volCaps := req.GetVolumeCapabilities()
	if len(volCaps) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Volume capabilities not provided")
	}

	splitVolumeId := strings.SplitN(volumeID, "-", 2)
	if splitVolumeId[0] == cloud.FilesystemPrefix {
		_, err = d.cloud.DescribeFileSystem(ctx, volumeID)
	} else if splitVolumeId[0] == cloud.VolumePrefix {
		_, err = d.cloud.DescribeVolume(ctx, volumeID)
	} else {
		err = cloud.ErrNotFound
	}

	if err != nil {
		if err == cloud.ErrNotFound {
			return nil, status.Error(codes.NotFound, "Volume not found")
		}
		return nil, status.Errorf(codes.Internal, "Could not get volume with ID %q: %v", volumeID, err)
	}

	confirmed := d.isValidVolumeCapabilities(volCaps)
	if confirmed {
		return &csi.ValidateVolumeCapabilitiesResponse{
			Confirmed: &csi.ValidateVolumeCapabilitiesResponse_Confirmed{
				// TODO if volume context is provided, should validate it too
				//  VolumeContext:      req.GetVolumeContext(),
				VolumeCapabilities: volCaps,
				// TODO if parameters are provided, should validate them too
				//  Parameters:      req.GetParameters(),
			},
		}, nil
	} else {
		return &csi.ValidateVolumeCapabilitiesResponse{}, nil
	}
}

func (d *Driver) ListVolumes(ctx context.Context, req *csi.ListVolumesRequest) (*csi.ListVolumesResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

func (d *Driver) GetCapacity(ctx context.Context, req *csi.GetCapacityRequest) (*csi.GetCapacityResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

func (d *Driver) ControllerGetCapabilities(ctx context.Context, req *csi.ControllerGetCapabilitiesRequest) (*csi.ControllerGetCapabilitiesResponse, error) {
	klog.V(4).Infof("ControllerGetCapabilities: called with args %#v", req)
	var caps []*csi.ControllerServiceCapability
	for _, cap := range controllerCaps {
		c := &csi.ControllerServiceCapability{
			Type: &csi.ControllerServiceCapability_Rpc{
				Rpc: &csi.ControllerServiceCapability_RPC{
					Type: cap,
				},
			},
		}
		caps = append(caps, c)
	}
	return &csi.ControllerGetCapabilitiesResponse{Capabilities: caps}, nil
}

func (d *Driver) CreateSnapshot(ctx context.Context, req *csi.CreateSnapshotRequest) (*csi.CreateSnapshotResponse, error) {
	klog.V(4).Infof("CreateSnapshot: called with args %#v", req)

	if len(req.GetName()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Snapshot name not provided")
	}

	if len(req.GetSourceVolumeId()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Snapshot volume source ID not provided")
	}

	snapshotName := req.GetName()
	volumeID := req.GetSourceVolumeId()
	tags := req.Parameters["tags"]

	// check if a request is already in-flight
	if ok := d.inFlight.Insert(snapshotName); !ok {
		msg := fmt.Sprintf("CreateSnapshot: "+internal.SnapshotOperationAlreadyExistsErrorMsg, snapshotName)
		return nil, status.Error(codes.Aborted, msg)
	}
	defer d.inFlight.Delete(snapshotName)

	opts := cloud.SnapshotOptions{
		SnapshotName:   &snapshotName,
		SourceVolumeId: &volumeID,
		Tags:           &tags,
	}

	snapshot, err := d.cloud.CreateSnapshot(ctx, opts)
	if err != nil {
		switch {
		case errors.Is(err, cloud.ErrInvalidParameter):
			return nil, status.Error(codes.InvalidArgument, err.Error())
		case errors.Is(err, cloud.ErrAlreadyExists):
			return nil, status.Error(codes.AlreadyExists, err.Error())
		default:
			return nil, status.Errorf(codes.Internal, "CreateSnapshot: Failed to create snapshot %q with error %v", snapshotName, err)
		}
	}

	err = d.cloud.WaitForSnapshotAvailable(ctx, snapshot.SnapshotID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Snapshot %s is not ready: %v", snapshotName, err)
	}

	return &csi.CreateSnapshotResponse{
		Snapshot: &csi.Snapshot{
			SnapshotId:     snapshot.SnapshotID,
			SourceVolumeId: snapshot.SourceVolumeID,
			CreationTime:   timestamppb.New(snapshot.CreationTime),
			ReadyToUse:     true,
		},
	}, nil
}

func (d *Driver) DeleteSnapshot(ctx context.Context, req *csi.DeleteSnapshotRequest) (*csi.DeleteSnapshotResponse, error) {
	klog.V(4).Infof("DeleteSnapshot: called with args %#v", req)

	if len(req.GetSnapshotId()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Snapshot ID not provided")
	}

	snapshotId := req.GetSnapshotId()

	// check if a request is already in-flight
	if ok := d.inFlight.Insert(snapshotId); !ok {
		msg := fmt.Sprintf("DeleteSnapshot: "+internal.SnapshotOperationAlreadyExistsErrorMsg, snapshotId)
		return nil, status.Error(codes.Aborted, msg)
	}
	defer d.inFlight.Delete(snapshotId)

	if err := d.cloud.DeleteSnapshot(ctx, snapshotId); err != nil {
		if strings.Contains(err.Error(), "Unable to find snapshot") {
			klog.V(4).Infof("DeleteSnapshot: Snapshot %s not found, returning with success", snapshotId)
			return &csi.DeleteSnapshotResponse{}, nil
		}
		return nil, status.Errorf(codes.Internal, "DeleteSnapshot: Could not delete snapshot %s, received error %v", snapshotId, err)
	}

	return &csi.DeleteSnapshotResponse{}, nil
}

func (d *Driver) ListSnapshots(ctx context.Context, req *csi.ListSnapshotsRequest) (*csi.ListSnapshotsResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

func (d *Driver) ControllerExpandVolume(ctx context.Context, req *csi.ControllerExpandVolumeRequest) (*csi.ControllerExpandVolumeResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

func (d *Driver) ControllerGetVolume(ctx context.Context, req *csi.ControllerGetVolumeRequest) (*csi.ControllerGetVolumeResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}
