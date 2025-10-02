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
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/fsx"
	"github.com/aws/aws-sdk-go-v2/service/fsx/types"
	"github.com/kubernetes-sigs/aws-fsx-openzfs-csi-driver/pkg/util"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/klog/v2"
	"os"
	"strings"
	"time"
)

// Polling
const (
	// PollCheckInterval specifies the interval to check if the resource is ready;
	// this needs to be shorter than the timeout.
	PollCheckInterval = 15 * time.Second
	// PollCheckTimeout specifies the time limit for polling the describe API for a completed create/update operation.
	PollCheckTimeout = 15 * time.Minute
)

// Set during build time via -ldflags
var driverVersion string

// Errors
var (
	ErrAlreadyExists = errors.New("resource already exists with the given name and different parameters")
	ErrInvalidInput  = errors.New("invalid input")
	ErrMultipleFound = errors.New("multiple resource found with the given ID")
	ErrNotFound      = errors.New("resource could not be found using the respective ID")
)

// Prefixes used to parse the volume id.
const (
	FilesystemPrefix = "fs"
	VolumePrefix     = "fsvol"
)

// FileSystem represents an OpenZFS filesystem
type FileSystem struct {
	DnsName         string
	FileSystemId    string
	StorageCapacity int32
	StorageType     types.StorageType
}

// Volume represents an OpenZFS volume
type Volume struct {
	FileSystemId string
	VolumeId     string
	VolumePath   string
}

// Snapshot represents an OpenZFS volume snapshot
type Snapshot struct {
	SnapshotID     string
	SourceVolumeID string
	ResourceARN    string
	CreationTime   time.Time
}

// FSx abstracts FSx client to facilitate its mocking.
// See https://pkg.go.dev/github.com/aws/aws-sdk-go-v2/service/fsx for details
type FSx interface {
	CreateFileSystem(context.Context, *fsx.CreateFileSystemInput, ...func(*fsx.Options)) (*fsx.CreateFileSystemOutput, error)
	UpdateFileSystem(context.Context, *fsx.UpdateFileSystemInput, ...func(*fsx.Options)) (*fsx.UpdateFileSystemOutput, error)
	DeleteFileSystem(context.Context, *fsx.DeleteFileSystemInput, ...func(*fsx.Options)) (*fsx.DeleteFileSystemOutput, error)
	DescribeFileSystems(context.Context, *fsx.DescribeFileSystemsInput, ...func(*fsx.Options)) (*fsx.DescribeFileSystemsOutput, error)
	CreateVolume(context.Context, *fsx.CreateVolumeInput, ...func(*fsx.Options)) (*fsx.CreateVolumeOutput, error)
	DeleteVolume(context.Context, *fsx.DeleteVolumeInput, ...func(*fsx.Options)) (*fsx.DeleteVolumeOutput, error)
	DescribeVolumes(context.Context, *fsx.DescribeVolumesInput, ...func(*fsx.Options)) (*fsx.DescribeVolumesOutput, error)
	CreateSnapshot(context.Context, *fsx.CreateSnapshotInput, ...func(*fsx.Options)) (*fsx.CreateSnapshotOutput, error)
	DeleteSnapshot(context.Context, *fsx.DeleteSnapshotInput, ...func(*fsx.Options)) (*fsx.DeleteSnapshotOutput, error)
	DescribeSnapshots(context.Context, *fsx.DescribeSnapshotsInput, ...func(*fsx.Options)) (*fsx.DescribeSnapshotsOutput, error)
	ListTagsForResource(context.Context, *fsx.ListTagsForResourceInput, ...func(*fsx.Options)) (*fsx.ListTagsForResourceOutput, error)
}

type Cloud interface {
	CreateFileSystem(ctx context.Context, parameters map[string]string) (*FileSystem, error)
	ResizeFileSystem(ctx context.Context, fileSystemId string, newSizeGiB int32) (*int32, error)
	DeleteFileSystem(ctx context.Context, parameters map[string]string) error
	DescribeFileSystem(ctx context.Context, fileSystemId string) (*FileSystem, error)
	WaitForFileSystemAvailable(ctx context.Context, fileSystemId string) error
	WaitForFileSystemResize(ctx context.Context, fileSystemId string, resizeGiB int32) error
	CreateVolume(ctx context.Context, parameters map[string]string) (*Volume, error)
	DeleteVolume(ctx context.Context, parameters map[string]string) error
	DescribeVolume(ctx context.Context, volumeId string) (*Volume, error)
	WaitForVolumeAvailable(ctx context.Context, volumeId string) error
	WaitForVolumeResize(ctx context.Context, volumeId string, resizeGiB int32) error
	CreateSnapshot(ctx context.Context, options map[string]string) (*Snapshot, error)
	DeleteSnapshot(ctx context.Context, parameters map[string]string) error
	DescribeSnapshot(ctx context.Context, snapshotId string) (*Snapshot, error)
	WaitForSnapshotAvailable(ctx context.Context, snapshotId string) error
	GetDeleteParameters(ctx context.Context, id string) (map[string]string, error)
	GetVolumeId(ctx context.Context, volumeId string) (string, error)
}

type cloud struct {
	region string
	fsx    FSx
}

// NewCloud returns a new instance of AWS cloud
func NewCloud(region string) (Cloud, error) {
	cfg, err := config.LoadDefaultConfig(context.Background(),
		config.WithRegion(region),
		// Set MaxRetries to a high value. It will be "overwritten" if context deadline comes sooner.
		config.WithRetryMaxAttempts(8),
	)
	if err != nil {
		return nil, err
	}

	os.Setenv("AWS_EXECUTION_ENV", "aws-fsx-openzfs-csi-driver-"+driverVersion)

	svc := fsx.NewFromConfig(cfg)
	return &cloud{
		region: region,
		fsx:    svc,
	}, nil
}

func (c *cloud) CreateFileSystem(ctx context.Context, parameters map[string]string) (*FileSystem, error) {
	input := fsx.CreateFileSystemInput{}
	err := util.StrictRemoveParametersAndPopulateObject(parameters, &input)
	if err != nil {
		return nil, err
	}

	output, err := c.fsx.CreateFileSystem(ctx, &input)
	if err != nil {
		var incompatibleParamErr *types.IncompatibleParameterError
		if errors.As(err, &incompatibleParamErr) {
			return nil, ErrAlreadyExists
		}
		return nil, err
	}

	return &FileSystem{
		DnsName:         aws.ToString(output.FileSystem.DNSName),
		FileSystemId:    aws.ToString(output.FileSystem.FileSystemId),
		StorageCapacity: aws.ToInt32(output.FileSystem.StorageCapacity),
		StorageType:     output.FileSystem.StorageType,
	}, nil
}

func (c *cloud) ResizeFileSystem(ctx context.Context, fileSystemId string, newSizeGiB int32) (*int32, error) {
	input := &fsx.UpdateFileSystemInput{
		FileSystemId:    aws.String(fileSystemId),
		StorageCapacity: aws.Int32(newSizeGiB),
	}

	_, err := c.fsx.UpdateFileSystem(ctx, input)
	if err != nil {
		var badRequestErr *types.BadRequest
		if errors.As(err, &badRequestErr) &&
			badRequestErr.ErrorMessage() == "Unable to perform the storage capacity update. There is an update already in progress." {
			// If a previous request timed out and was successful, don't error
			_, err = c.getUpdateResizeFilesystemAdministrativeAction(ctx, fileSystemId, newSizeGiB)
			if err != nil {
				return nil, err
			}
			return &newSizeGiB, nil
		}
		return nil, fmt.Errorf("UpdateFileSystem failed: %v", err)
	}

	return &newSizeGiB, nil
}

func (c *cloud) DeleteFileSystem(ctx context.Context, parameters map[string]string) error {
	input := fsx.DeleteFileSystemInput{}
	err := util.RemoveParametersAndPopulateObject(parameters, &input)
	if err != nil {
		return err
	}

	_, err = c.fsx.DeleteFileSystem(ctx, &input)
	if err != nil {
		var fileSystemNotFoundErr *types.FileSystemNotFound
		if errors.As(err, &fileSystemNotFoundErr) {
			return ErrNotFound
		}
		return err
	}

	return nil
}

func (c *cloud) DescribeFileSystem(ctx context.Context, fileSystemId string) (*FileSystem, error) {
	fs, err := c.getFileSystem(ctx, fileSystemId)
	if err != nil {
		return nil, err
	}

	return &FileSystem{
		DnsName:         aws.ToString(fs.DNSName),
		FileSystemId:    aws.ToString(fs.FileSystemId),
		StorageCapacity: aws.ToInt32(fs.StorageCapacity),
		StorageType:     fs.StorageType,
	}, nil
}

func (c *cloud) WaitForFileSystemAvailable(ctx context.Context, fileSystemId string) error {
	err := wait.Poll(PollCheckInterval, PollCheckTimeout, func() (done bool, err error) {
		fs, err := c.getFileSystem(ctx, fileSystemId)
		if err != nil {
			return true, err
		}
		klog.V(2).InfoS("WaitForFileSystemAvailable", "filesystem", fileSystemId, "lifecycle", fs.Lifecycle)
		switch fs.Lifecycle {
		case types.FileSystemLifecycleAvailable:
			return true, nil
		case types.FileSystemLifecycleCreating:
			return false, nil
		default:
			return true, fmt.Errorf("unexpected state for filesystem %s: %q", fileSystemId, fs.Lifecycle)
		}
	})

	return err
}

func (c *cloud) WaitForFileSystemResize(ctx context.Context, fileSystemId string, resizeGiB int32) error {
	err := wait.Poll(PollCheckInterval, PollCheckTimeout, func() (done bool, err error) {
		updateAction, err := c.getUpdateResizeFilesystemAdministrativeAction(ctx, fileSystemId, resizeGiB)
		if err != nil {
			return true, err
		}
		klog.V(2).InfoS("WaitForFileSystemResize", "filesystem", fileSystemId, "update status", updateAction.Status)
		switch updateAction.Status {
		case types.StatusPending, types.StatusInProgress:
			return false, nil
		case types.StatusUpdatedOptimizing, types.StatusCompleted:
			return true, nil
		default:
			return true, fmt.Errorf("update failed for filesystem %s: %q", fileSystemId, *updateAction.FailureDetails.Message)
		}
	})

	return err
}

func (c *cloud) CreateVolume(ctx context.Context, parameters map[string]string) (*Volume, error) {
	input := fsx.CreateVolumeInput{}
	err := util.StrictRemoveParametersAndPopulateObject(parameters, &input)
	if err != nil {
		return nil, err
	}

	if len(parameters) != 0 {
		return nil, ErrInvalidInput
	}

	output, err := c.fsx.CreateVolume(ctx, &input)
	if err != nil {
		var incompatibleParamErr *types.IncompatibleParameterError
		if errors.As(err, &incompatibleParamErr) {
			return nil, ErrAlreadyExists
		}
		return nil, err
	}

	return &Volume{
		FileSystemId: aws.ToString(output.Volume.FileSystemId),
		VolumeId:     aws.ToString(output.Volume.VolumeId),
		VolumePath:   aws.ToString(output.Volume.OpenZFSConfiguration.VolumePath),
	}, nil
}

func (c *cloud) DeleteVolume(ctx context.Context, parameters map[string]string) error {
	input := fsx.DeleteVolumeInput{}
	err := util.RemoveParametersAndPopulateObject(parameters, &input)
	if err != nil {
		return err
	}

	_, err = c.fsx.DeleteVolume(ctx, &input)
	if err != nil {
		var volumeNotFoundErr *types.VolumeNotFound
		if errors.As(err, &volumeNotFoundErr) {
			return ErrNotFound
		}
		return err
	}

	return nil
}

func (c *cloud) DescribeVolume(ctx context.Context, volumeId string) (*Volume, error) {
	v, err := c.getVolume(ctx, volumeId)
	if err != nil {
		return nil, err
	}

	return &Volume{
		VolumeId:   aws.ToString(v.VolumeId),
		VolumePath: aws.ToString(v.OpenZFSConfiguration.VolumePath),
	}, nil
}

func (c *cloud) WaitForVolumeAvailable(ctx context.Context, volumeId string) error {
	err := wait.Poll(PollCheckInterval, PollCheckTimeout, func() (done bool, err error) {
		v, err := c.getVolume(ctx, volumeId)
		if err != nil {
			return true, err
		}
		klog.V(2).InfoS("WaitForVolumeAvailable", "volume", volumeId, "lifecycle", v.Lifecycle)
		switch v.Lifecycle {
		case types.VolumeLifecycleAvailable:
			return true, nil
		case types.VolumeLifecyclePending, types.VolumeLifecycleCreating:
			return false, nil
		default:
			return true, fmt.Errorf("unexpected state for volume %s: %q", volumeId, v.Lifecycle)
		}
	})

	return err
}

// WaitForVolumeResize TODO: Remove this function and its associated tests.
func (c *cloud) WaitForVolumeResize(ctx context.Context, volumeId string, resizeGiB int32) error {
	err := wait.Poll(PollCheckInterval, PollCheckTimeout, func() (done bool, err error) {
		updateAction, err := c.getUpdateResizeVolumeAdministrativeAction(ctx, volumeId, resizeGiB)
		if err != nil {
			return true, err
		}
		klog.V(2).InfoS("WaitForVolumeResize", "volume", volumeId, "update status", updateAction.Status)
		switch updateAction.Status {
		case types.StatusPending, types.StatusInProgress:
			return false, nil
		case types.StatusUpdatedOptimizing, types.StatusCompleted:
			return true, nil
		default:
			return true, fmt.Errorf("update failed for volume %s: %q", volumeId, *updateAction.FailureDetails.Message)
		}
	})

	return err
}

func (c *cloud) CreateSnapshot(ctx context.Context, parameters map[string]string) (*Snapshot, error) {
	input := fsx.CreateSnapshotInput{}
	err := util.StrictRemoveParametersAndPopulateObject(parameters, &input)
	if err != nil {
		return nil, err
	}

	output, err := c.fsx.CreateSnapshot(ctx, &input)
	if err != nil {
		var incompatibleParamErr *types.IncompatibleParameterError
		if errors.As(err, &incompatibleParamErr) {
			return nil, ErrAlreadyExists
		}
		return nil, fmt.Errorf("error creating snapshot of volume %s: %w", *input.VolumeId, err)
	}
	if output == nil {
		return nil, fmt.Errorf("nil CreateSnapshotResponse")
	}
	return &Snapshot{
		SnapshotID:     aws.ToString(output.Snapshot.SnapshotId),
		SourceVolumeID: aws.ToString(output.Snapshot.VolumeId),
		ResourceARN:    aws.ToString(output.Snapshot.ResourceARN),
		CreationTime:   aws.ToTime(output.Snapshot.CreationTime),
	}, nil
}

func (c *cloud) DeleteSnapshot(ctx context.Context, parameters map[string]string) error {
	input := fsx.DeleteSnapshotInput{}
	err := util.RemoveParametersAndPopulateObject(parameters, &input)
	if err != nil {
		return err
	}

	if _, err = c.fsx.DeleteSnapshot(ctx, &input); err != nil {
		var snapshotNotFoundErr *types.SnapshotNotFound
		if errors.As(err, &snapshotNotFoundErr) {
			return fmt.Errorf("DeleteSnapshot: Unable to find snapshot %s", *input.SnapshotId)
		}
		return fmt.Errorf("DeleteSnapshot: Failed to delete snapshot %s, received error %v", *input.SnapshotId, err)
	}
	return nil
}

func (c *cloud) DescribeSnapshot(ctx context.Context, snapshotId string) (*Snapshot, error) {
	snapshot, err := c.getSnapshot(ctx, snapshotId)
	if err != nil {
		return nil, err
	}

	return &Snapshot{
		SnapshotID:     aws.ToString(snapshot.SnapshotId),
		SourceVolumeID: aws.ToString(snapshot.VolumeId),
		ResourceARN:    aws.ToString(snapshot.ResourceARN),
		CreationTime:   aws.ToTime(snapshot.CreationTime),
	}, nil
}

func (c *cloud) WaitForSnapshotAvailable(ctx context.Context, snapshotId string) error {
	if len(snapshotId) == 0 {
		return fmt.Errorf("snapshot id not provided")
	}

	err := wait.Poll(PollCheckInterval, PollCheckTimeout, func() (done bool, err error) {
		snapshot, err := c.getSnapshot(ctx, snapshotId)
		if err != nil {
			return true, err
		}
		klog.V(2).InfoS("WaitForSnapshotAvailable", "snapshot", snapshotId, "lifecycle", snapshot.Lifecycle)
		switch snapshot.Lifecycle {
		case types.SnapshotLifecycleAvailable:
			return true, nil
		case types.SnapshotLifecyclePending, types.SnapshotLifecycleCreating:
			return false, nil
		default:
			return true, fmt.Errorf("WaitForSnapshotAvailable: Snapshot %s has unexpected status %q", snapshotId, snapshot.Lifecycle)
		}
	})

	return err
}

func (c *cloud) GetDeleteParameters(ctx context.Context, id string) (map[string]string, error) {
	parameters := make(map[string]string)
	resourceArn := ""

	splitVolumeId := strings.SplitN(id, "-", 2)
	if splitVolumeId[0] == FilesystemPrefix {
		f, err := c.getFileSystem(ctx, id)
		if err != nil {
			return nil, err
		}
		resourceArn = *f.ResourceARN
	}
	if splitVolumeId[0] == VolumePrefix {
		v, err := c.getVolume(ctx, id)
		if err != nil {
			return nil, err
		}
		resourceArn = *v.ResourceARN
	}

	tags, err := c.getTagsForResource(ctx, resourceArn)
	if err != nil {
		return nil, err
	}

	for _, tag := range tags {
		if tag.Key == nil || tag.Value == nil {
			continue
		}
		if strings.HasSuffix(*tag.Key, "OnDeletion") {
			deleteKey := strings.TrimSuffix(*tag.Key, "OnDeletion")
			deleteValue := util.DecodeDeletionTag(*tag.Value)
			deleteMap := map[string]string{
				deleteKey: deleteValue,
			}
			if splitVolumeId[0] == FilesystemPrefix {
				err = util.StrictRemoveParametersAndPopulateObject(deleteMap, &types.DeleteFileSystemOpenZFSConfiguration{})
			}
			if splitVolumeId[0] == VolumePrefix {
				err = util.StrictRemoveParametersAndPopulateObject(deleteMap, &types.DeleteVolumeOpenZFSConfiguration{})
			}

			if err != nil {
				continue
			}
			parameters[deleteKey] = deleteValue
		}
	}
	return parameters, nil
}

func (c *cloud) getFileSystem(ctx context.Context, fileSystemId string) (types.FileSystem, error) {
	input := &fsx.DescribeFileSystemsInput{
		FileSystemIds: []string{fileSystemId},
	}

	output, err := c.fsx.DescribeFileSystems(ctx, input)
	if err != nil {
		return types.FileSystem{}, err
	}

	if len(output.FileSystems) == 0 {
		return types.FileSystem{}, ErrNotFound
	}

	if len(output.FileSystems) > 1 {
		return types.FileSystem{}, ErrMultipleFound
	}

	return output.FileSystems[0], nil
}

func (c *cloud) getVolume(ctx context.Context, volumeId string) (types.Volume, error) {
	input := &fsx.DescribeVolumesInput{
		VolumeIds: []string{volumeId},
	}

	output, err := c.fsx.DescribeVolumes(ctx, input)
	if err != nil {
		return types.Volume{}, err
	}

	if len(output.Volumes) == 0 {
		return types.Volume{}, ErrNotFound
	}

	if len(output.Volumes) > 1 {
		return types.Volume{}, ErrMultipleFound
	}

	return output.Volumes[0], nil
}

func (c *cloud) getSnapshot(ctx context.Context, snapshotId string) (types.Snapshot, error) {
	input := &fsx.DescribeSnapshotsInput{
		SnapshotIds: []string{snapshotId},
	}

	output, err := c.fsx.DescribeSnapshots(ctx, input)
	if err != nil {
		return types.Snapshot{}, err
	}

	if len(output.Snapshots) == 0 {
		return types.Snapshot{}, ErrNotFound
	}

	if len(output.Snapshots) > 1 {
		return types.Snapshot{}, ErrMultipleFound
	}

	return output.Snapshots[0], nil
}

func (c *cloud) getTagsForResource(ctx context.Context, resourceARN string) ([]types.Tag, error) {
	input := &fsx.ListTagsForResourceInput{
		ResourceARN: aws.String(resourceARN),
	}

	output, err := c.fsx.ListTagsForResource(ctx, input)
	if err != nil {
		return nil, err
	}

	return output.Tags, nil
}

// GetVolumeId getVolumeId Parses the volumeId to determine if it is the id of a file system or an OpenZFS volume.
// If the volumeId references a file system, this function retrieves the file system's root volume id.
// If the volumeId references an OpenZFS volume, this function returns a pointer to the volumeId.
func (c *cloud) GetVolumeId(ctx context.Context, volumeId string) (string, error) {
	splitVolumeId := strings.Split(volumeId, "-")
	if len(splitVolumeId) != 2 {
		return "", fmt.Errorf("volume id %s is improperly formatted", volumeId)
	}
	idPrefix := splitVolumeId[0]
	if idPrefix == FilesystemPrefix {
		filesystem, err := c.getFileSystem(ctx, volumeId)
		if err != nil {
			return "", err
		}
		if filesystem.OpenZFSConfiguration != nil && len(*filesystem.OpenZFSConfiguration.RootVolumeId) > 0 {
			return *filesystem.OpenZFSConfiguration.RootVolumeId, nil
		}
		return "", fmt.Errorf("failed to retrieve root volume id for file system %s", volumeId)
	} else if idPrefix == VolumePrefix {
		return volumeId, nil
	} else {
		return "", fmt.Errorf("volume id %s is improperly formatted", volumeId)
	}
}

// getUpdateResizeFilesystemAdministrativeAction retrieves the AdministrativeAction associated with a file system update with the
// given target storage capacity, if one exists.
func (c *cloud) getUpdateResizeFilesystemAdministrativeAction(ctx context.Context, fileSystemId string, resizeGiB int32) (types.AdministrativeAction, error) {
	fs, err := c.getFileSystem(ctx, fileSystemId)
	if err != nil {
		return types.AdministrativeAction{}, fmt.Errorf("DescribeFileSystems failed: %v", err)
	}

	if len(fs.AdministrativeActions) == 0 {
		return types.AdministrativeAction{}, fmt.Errorf("there is no update on filesystem %s", fileSystemId)
	}

	// AdministrativeActions are sorted from newest to oldest
	for _, action := range fs.AdministrativeActions {
		if action.AdministrativeActionType == types.AdministrativeActionTypeFileSystemUpdate &&
			action.TargetFileSystemValues.StorageCapacity != nil &&
			*action.TargetFileSystemValues.StorageCapacity == resizeGiB {
			return action, nil
		}
	}

	return types.AdministrativeAction{}, fmt.Errorf("there is no update with storage capacity of %d GiB on filesystem %s", resizeGiB, fileSystemId)
}

// getUpdateResizeVolumeAdministrativeAction retrieves the AdministrativeAction associated with a volume update with the
// given target storage capacity, if one exists.
func (c *cloud) getUpdateResizeVolumeAdministrativeAction(ctx context.Context, volumeId string, resizeGiB int32) (types.AdministrativeAction, error) {
	v, err := c.getVolume(ctx, volumeId)
	if err != nil {
		return types.AdministrativeAction{}, fmt.Errorf("DescribeVolumes failed: %v", err)
	}

	if len(v.AdministrativeActions) == 0 {
		return types.AdministrativeAction{}, fmt.Errorf("there is no update on volume %s", volumeId)
	}

	// AdministrativeActions are sorted from newest to oldest
	for _, action := range v.AdministrativeActions {
		if action.AdministrativeActionType == types.AdministrativeActionTypeVolumeUpdate &&
			action.TargetVolumeValues.OpenZFSConfiguration != nil &&
			action.TargetVolumeValues.OpenZFSConfiguration.StorageCapacityQuotaGiB != nil &&
			*action.TargetVolumeValues.OpenZFSConfiguration.StorageCapacityQuotaGiB == resizeGiB &&
			action.TargetVolumeValues.OpenZFSConfiguration.StorageCapacityReservationGiB != nil &&
			*action.TargetVolumeValues.OpenZFSConfiguration.StorageCapacityReservationGiB == resizeGiB {
			return action, nil
		}
	}

	return types.AdministrativeAction{}, fmt.Errorf("there is no update with storage capacity of %d GiB on volume %s", resizeGiB, volumeId)
}

func CollapseCreateFileSystemParameters(parameters map[string]string) error {
	config := types.CreateFileSystemOpenZFSConfiguration{}
	return util.ReplaceParametersAndPopulateObject("OpenZFSConfiguration", parameters, &config)
}

func CollapseDeleteFileSystemParameters(parameters map[string]string) error {
	config := types.DeleteFileSystemOpenZFSConfiguration{}
	return util.ReplaceParametersAndPopulateObject("OpenZFSConfiguration", parameters, &config)
}

func CollapseCreateVolumeParameters(parameters map[string]string) error {
	config := types.CreateOpenZFSVolumeConfiguration{}
	return util.ReplaceParametersAndPopulateObject("OpenZFSConfiguration", parameters, &config)
}

func CollapseDeleteVolumeParameters(parameters map[string]string) error {
	config := types.DeleteVolumeOpenZFSConfiguration{}
	return util.ReplaceParametersAndPopulateObject("OpenZFSConfiguration", parameters, &config)
}

// ValidateDeleteFileSystemParameters is used in CreateVolume to remove all delete parameters from the parameters map, and ensure they are valid.
// Parameters should be unique map containing only delete parameters without the OnDeletion suffix
// This method expects there to be no remaining delete parameters and errors if there are any
// Verifies parameters are valid in accordance to the API to prevent unknown errors from occurring during DeleteVolume
func ValidateDeleteFileSystemParameters(parameters map[string]string) error {
	config := fsx.DeleteFileSystemInput{}
	err := util.StrictRemoveParametersAndPopulateObject(parameters, &config)
	if err != nil {
		return err
	}

	return validateDeleteFileSystemInput(&config)
}

// ValidateDeleteVolumeParameters is used in CreateVolume to remove all delete parameters from the parameters map, and ensure they are valid.
// Parameters should be unique map containing only delete parameters without the OnDeletion suffix
// This method expects there to be no remaining delete parameters and errors if there are any
// Verifies parameters are valid in accordance to the API to prevent unknown errors from occurring during DeleteVolume
func ValidateDeleteVolumeParameters(parameters map[string]string) error {
	config := fsx.DeleteVolumeInput{}
	err := util.StrictRemoveParametersAndPopulateObject(parameters, &config)
	if err != nil {
		return err
	}

	return validateDeleteVolumeInput(&config)
}
