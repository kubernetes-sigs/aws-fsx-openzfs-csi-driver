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
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/fsx"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/klog/v2"
	"sigs.k8s.io/aws-fsx-openzfs-csi-driver/pkg/util"
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
	StorageCapacity int64
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
// See https://docs.aws.amazon.com/sdk-for-go/api/service/fsx/ for details
type FSx interface {
	CreateFileSystemWithContext(aws.Context, *fsx.CreateFileSystemInput, ...request.Option) (*fsx.CreateFileSystemOutput, error)
	UpdateFileSystemWithContext(aws.Context, *fsx.UpdateFileSystemInput, ...request.Option) (*fsx.UpdateFileSystemOutput, error)
	DeleteFileSystemWithContext(aws.Context, *fsx.DeleteFileSystemInput, ...request.Option) (*fsx.DeleteFileSystemOutput, error)
	DescribeFileSystemsWithContext(aws.Context, *fsx.DescribeFileSystemsInput, ...request.Option) (*fsx.DescribeFileSystemsOutput, error)
	CreateVolumeWithContext(aws.Context, *fsx.CreateVolumeInput, ...request.Option) (*fsx.CreateVolumeOutput, error)
	DeleteVolumeWithContext(aws.Context, *fsx.DeleteVolumeInput, ...request.Option) (*fsx.DeleteVolumeOutput, error)
	DescribeVolumesWithContext(aws.Context, *fsx.DescribeVolumesInput, ...request.Option) (*fsx.DescribeVolumesOutput, error)
	CreateSnapshotWithContext(aws.Context, *fsx.CreateSnapshotInput, ...request.Option) (*fsx.CreateSnapshotOutput, error)
	DeleteSnapshotWithContext(aws.Context, *fsx.DeleteSnapshotInput, ...request.Option) (*fsx.DeleteSnapshotOutput, error)
	DescribeSnapshotsWithContext(aws.Context, *fsx.DescribeSnapshotsInput, ...request.Option) (*fsx.DescribeSnapshotsOutput, error)
	ListTagsForResource(*fsx.ListTagsForResourceInput) (*fsx.ListTagsForResourceOutput, error)
}

type Cloud interface {
	CreateFileSystem(ctx context.Context, parameters map[string]string) (*FileSystem, error)
	ResizeFileSystem(ctx context.Context, fileSystemId string, newSizeGiB int64) (*int64, error)
	DeleteFileSystem(ctx context.Context, parameters map[string]string) error
	DescribeFileSystem(ctx context.Context, fileSystemId string) (*FileSystem, error)
	WaitForFileSystemAvailable(ctx context.Context, fileSystemId string) error
	WaitForFileSystemResize(ctx context.Context, fileSystemId string, resizeGiB int64) error
	CreateVolume(ctx context.Context, parameters map[string]string) (*Volume, error)
	DeleteVolume(ctx context.Context, parameters map[string]string) error
	DescribeVolume(ctx context.Context, volumeId string) (*Volume, error)
	WaitForVolumeAvailable(ctx context.Context, volumeId string) error
	WaitForVolumeResize(ctx context.Context, volumeId string, resizeGiB int64) error
	CreateSnapshot(ctx context.Context, options map[string]string) (*Snapshot, error)
	DeleteSnapshot(ctx context.Context, parameters map[string]string) error
	DescribeSnapshot(ctx context.Context, snapshotId string) (*Snapshot, error)
	WaitForSnapshotAvailable(ctx context.Context, snapshotId string) error
	GetDeleteParameters(ctx context.Context, id string) (map[string]string, error)
	GetVolumeId(ctx context.Context, volumeId string) (string, error)
}

type cloud struct {
	fsx FSx
}

// NewCloud returns a new instance of AWS cloud
// It panics if session is invalid
func NewCloud(region string) Cloud {
	awsConfig := &aws.Config{
		Region:                        aws.String(region),
		CredentialsChainVerboseErrors: aws.Bool(true),
	}

	return &cloud{
		fsx: fsx.New(session.Must(session.NewSession(awsConfig))),
	}
}

func (c *cloud) CreateFileSystem(ctx context.Context, parameters map[string]string) (*FileSystem, error) {
	input := fsx.CreateFileSystemInput{}
	err := util.StrictRemoveParametersAndPopulateObject(parameters, &input)
	if err != nil {
		return nil, err
	}

	output, err := c.fsx.CreateFileSystemWithContext(ctx, &input)
	if err != nil {
		if awsErr, ok := err.(awserr.Error); ok {
			if awsErr.Code() == fsx.ErrCodeIncompatibleParameterError {
				return nil, ErrAlreadyExists
			}
		}
		return nil, err
	}

	return &FileSystem{
		DnsName:         aws.StringValue(output.FileSystem.DNSName),
		FileSystemId:    aws.StringValue(output.FileSystem.FileSystemId),
		StorageCapacity: aws.Int64Value(output.FileSystem.StorageCapacity),
	}, nil
}

func (c *cloud) ResizeFileSystem(ctx context.Context, fileSystemId string, newSizeGiB int64) (*int64, error) {
	input := &fsx.UpdateFileSystemInput{
		FileSystemId:    aws.String(fileSystemId),
		StorageCapacity: aws.Int64(newSizeGiB),
	}

	_, err := c.fsx.UpdateFileSystemWithContext(ctx, input)
	if err != nil {
		if awsErr, ok := err.(awserr.Error); ok {
			if awsErr.Code() == fsx.ErrCodeBadRequest &&
				awsErr.Message() == "Unable to perform the storage capacity update. There is an update already in progress." {
				// If a previous request timed out and was successful, don't error
				_, err = c.getUpdateResizeFilesystemAdministrativeAction(ctx, fileSystemId, newSizeGiB)
				if err != nil {
					return nil, err
				}
				return &newSizeGiB, nil
			}
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

	_, err = c.fsx.DeleteFileSystemWithContext(ctx, &input)
	if err != nil {
		if awsErr, ok := err.(awserr.Error); ok {
			if awsErr.Code() == fsx.ErrCodeFileSystemNotFound {
				return ErrNotFound
			}
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
		DnsName:         aws.StringValue(fs.DNSName),
		FileSystemId:    aws.StringValue(fs.FileSystemId),
		StorageCapacity: aws.Int64Value(fs.StorageCapacity),
	}, nil
}

func (c *cloud) WaitForFileSystemAvailable(ctx context.Context, fileSystemId string) error {
	err := wait.Poll(PollCheckInterval, PollCheckTimeout, func() (done bool, err error) {
		fs, err := c.getFileSystem(ctx, fileSystemId)
		if err != nil {
			return true, err
		}
		klog.V(2).Infof("WaitForFileSystemAvailable filesystem %q status is: %q", fileSystemId, *fs.Lifecycle)
		switch *fs.Lifecycle {
		case fsx.FileSystemLifecycleAvailable:
			return true, nil
		case fsx.FileSystemLifecycleCreating:
			return false, nil
		default:
			return true, fmt.Errorf("unexpected state for filesystem %s: %q", fileSystemId, *fs.Lifecycle)
		}
	})

	return err
}

func (c *cloud) WaitForFileSystemResize(ctx context.Context, fileSystemId string, resizeGiB int64) error {
	err := wait.Poll(PollCheckInterval, PollCheckTimeout, func() (done bool, err error) {
		updateAction, err := c.getUpdateResizeFilesystemAdministrativeAction(ctx, fileSystemId, resizeGiB)
		if err != nil {
			return true, err
		}
		klog.V(2).Infof("WaitForFileSystemResize filesystem %q update status is: %q", fileSystemId, *updateAction.Status)
		switch *updateAction.Status {
		case fsx.StatusPending, fsx.StatusInProgress:
			return false, nil
		case fsx.StatusUpdatedOptimizing, fsx.StatusCompleted:
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

	output, err := c.fsx.CreateVolumeWithContext(ctx, &input)
	if err != nil {
		if awsErr, ok := err.(awserr.Error); ok {
			if awsErr.Code() == fsx.ErrCodeIncompatibleParameterError {
				return nil, ErrAlreadyExists
			}
		}
		return nil, err
	}

	return &Volume{
		FileSystemId: aws.StringValue(output.Volume.FileSystemId),
		VolumeId:     aws.StringValue(output.Volume.VolumeId),
		VolumePath:   aws.StringValue(output.Volume.OpenZFSConfiguration.VolumePath),
	}, nil
}

func (c *cloud) DeleteVolume(ctx context.Context, parameters map[string]string) error {
	input := fsx.DeleteVolumeInput{}
	err := util.RemoveParametersAndPopulateObject(parameters, &input)
	if err != nil {
		return err
	}

	_, err = c.fsx.DeleteVolumeWithContext(ctx, &input)
	if err != nil {
		if awsErr, ok := err.(awserr.Error); ok {
			if awsErr.Code() == fsx.ErrCodeVolumeNotFound {
				return ErrNotFound
			}
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
		VolumeId:   aws.StringValue(v.VolumeId),
		VolumePath: aws.StringValue(v.OpenZFSConfiguration.VolumePath),
	}, nil
}

func (c *cloud) WaitForVolumeAvailable(ctx context.Context, volumeId string) error {
	err := wait.Poll(PollCheckInterval, PollCheckTimeout, func() (done bool, err error) {
		v, err := c.getVolume(ctx, volumeId)
		if err != nil {
			return true, err
		}
		klog.V(2).Infof("WaitForVolumeAvailable volume %q status is: %q", volumeId, *v.Lifecycle)
		switch *v.Lifecycle {
		case fsx.VolumeLifecycleAvailable:
			return true, nil
		case fsx.VolumeLifecyclePending, fsx.VolumeLifecycleCreating:
			return false, nil
		default:
			return true, fmt.Errorf("unexpected state for volume %s: %q", volumeId, *v.Lifecycle)
		}
	})

	return err
}

func (c *cloud) WaitForVolumeResize(ctx context.Context, volumeId string, resizeGiB int64) error {
	err := wait.Poll(PollCheckInterval, PollCheckTimeout, func() (done bool, err error) {
		updateAction, err := c.getUpdateResizeVolumeAdministrativeAction(ctx, volumeId, resizeGiB)
		if err != nil {
			return true, err
		}
		klog.V(2).Infof("WaitForVolumeResize volume %q update status is: %q", volumeId, *updateAction.Status)
		switch *updateAction.Status {
		case fsx.StatusPending, fsx.StatusInProgress:
			return false, nil
		case fsx.StatusUpdatedOptimizing, fsx.StatusCompleted:
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

	output, err := c.fsx.CreateSnapshotWithContext(ctx, &input)
	if err != nil {
		if awsErr, ok := err.(awserr.Error); ok {
			if awsErr.Code() == fsx.ErrCodeIncompatibleParameterError {
				return nil, ErrAlreadyExists
			}
		}
		return nil, fmt.Errorf("error creating snapshot of volume %s: %w", *input.VolumeId, err)
	}
	if output == nil {
		return nil, fmt.Errorf("nil CreateSnapshotResponse")
	}
	klog.V(4).Infof("CreateSnapshotResponse: ", output.GoString())
	return &Snapshot{
		SnapshotID:     aws.StringValue(output.Snapshot.SnapshotId),
		SourceVolumeID: aws.StringValue(output.Snapshot.VolumeId),
		ResourceARN:    aws.StringValue(output.Snapshot.ResourceARN),
		CreationTime:   aws.TimeValue(output.Snapshot.CreationTime),
	}, nil
}

func (c *cloud) DeleteSnapshot(ctx context.Context, parameters map[string]string) error {
	input := fsx.DeleteSnapshotInput{}
	err := util.RemoveParametersAndPopulateObject(parameters, &input)
	if err != nil {
		return err
	}

	if _, err = c.fsx.DeleteSnapshotWithContext(ctx, &input); err != nil {
		if awsErr, ok := err.(awserr.Error); ok {
			if awsErr.Code() == fsx.ErrCodeSnapshotNotFound {
				return fmt.Errorf("DeleteSnapshot: Unable to find snapshot %s", *input.SnapshotId)
			}
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
		SnapshotID:     aws.StringValue(snapshot.SnapshotId),
		SourceVolumeID: aws.StringValue(snapshot.VolumeId),
		ResourceARN:    aws.StringValue(snapshot.ResourceARN),
		CreationTime:   aws.TimeValue(snapshot.CreationTime),
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
		klog.V(2).Infof("WaitForSnapshotAvailable: Snapshot %s status is %q", snapshotId, *snapshot.Lifecycle)
		switch *snapshot.Lifecycle {
		case fsx.SnapshotLifecycleAvailable:
			return true, nil
		case fsx.SnapshotLifecyclePending, fsx.SnapshotLifecycleCreating:
			return false, nil
		default:
			return true, fmt.Errorf("WaitForSnapshotAvailable: Snapshot %s has unexpected status %q", snapshotId, *snapshot.Lifecycle)
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

	tags, err := c.getTagsForResource(resourceArn)
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
				err = util.StrictRemoveParametersAndPopulateObject(deleteMap, &fsx.DeleteFileSystemOpenZFSConfiguration{})
			}
			if splitVolumeId[0] == VolumePrefix {
				err = util.StrictRemoveParametersAndPopulateObject(deleteMap, &fsx.DeleteVolumeOpenZFSConfiguration{})
			}

			if err != nil {
				continue
			}
			parameters[deleteKey] = deleteValue
		}
	}
	return parameters, nil
}

func (c *cloud) getFileSystem(ctx context.Context, fileSystemId string) (*fsx.FileSystem, error) {
	input := &fsx.DescribeFileSystemsInput{
		FileSystemIds: []*string{aws.String(fileSystemId)},
	}

	output, err := c.fsx.DescribeFileSystemsWithContext(ctx, input)
	if err != nil {
		return nil, err
	}

	if len(output.FileSystems) == 0 {
		return nil, ErrNotFound
	}

	if len(output.FileSystems) > 1 {
		return nil, ErrMultipleFound
	}

	return output.FileSystems[0], nil
}

func (c *cloud) getVolume(ctx context.Context, volumeId string) (*fsx.Volume, error) {
	input := &fsx.DescribeVolumesInput{
		VolumeIds: []*string{aws.String(volumeId)},
	}

	output, err := c.fsx.DescribeVolumesWithContext(ctx, input)
	if err != nil {
		return nil, err
	}

	if len(output.Volumes) == 0 {
		return nil, ErrNotFound
	}

	if len(output.Volumes) > 1 {
		return nil, ErrMultipleFound
	}

	return output.Volumes[0], nil
}

func (c *cloud) getSnapshot(ctx context.Context, snapshotId string) (*fsx.Snapshot, error) {
	input := &fsx.DescribeSnapshotsInput{
		SnapshotIds: []*string{aws.String(snapshotId)},
	}

	output, err := c.fsx.DescribeSnapshotsWithContext(ctx, input)
	if err != nil {
		return nil, err
	}

	if len(output.Snapshots) == 0 {
		return nil, ErrNotFound
	}

	if len(output.Snapshots) > 1 {
		return nil, ErrMultipleFound
	}

	return output.Snapshots[0], nil
}

func (c *cloud) getTagsForResource(resourceARN string) ([]*fsx.Tag, error) {
	input := &fsx.ListTagsForResourceInput{
		ResourceARN: aws.String(resourceARN),
	}

	output, err := c.fsx.ListTagsForResource(input)
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
func (c *cloud) getUpdateResizeFilesystemAdministrativeAction(ctx context.Context, fileSystemId string, resizeGiB int64) (*fsx.AdministrativeAction, error) {
	fs, err := c.getFileSystem(ctx, fileSystemId)
	if err != nil {
		return nil, fmt.Errorf("DescribeFileSystems failed: %v", err)
	}

	if len(fs.AdministrativeActions) == 0 {
		return nil, fmt.Errorf("there is no update on filesystem %s", fileSystemId)
	}

	// AdministrativeActions are sorted from newest to oldest
	for _, action := range fs.AdministrativeActions {
		if *action.AdministrativeActionType == fsx.AdministrativeActionTypeFileSystemUpdate &&
			action.TargetFileSystemValues.StorageCapacity != nil &&
			*action.TargetFileSystemValues.StorageCapacity == resizeGiB {
			return action, nil
		}
	}

	return nil, fmt.Errorf("there is no update with storage capacity of %d GiB on filesystem %s", resizeGiB, fileSystemId)
}

// getUpdateResizeVolumeAdministrativeAction retrieves the AdministrativeAction associated with a volume update with the
// given target storage capacity, if one exists.
func (c *cloud) getUpdateResizeVolumeAdministrativeAction(ctx context.Context, volumeId string, resizeGiB int64) (*fsx.AdministrativeAction, error) {
	v, err := c.getVolume(ctx, volumeId)
	if err != nil {
		return nil, fmt.Errorf("DescribeVolumes failed: %v", err)
	}

	if len(v.AdministrativeActions) == 0 {
		return nil, fmt.Errorf("there is no update on volume %s", volumeId)
	}

	// AdministrativeActions are sorted from newest to oldest
	for _, action := range v.AdministrativeActions {
		if *action.AdministrativeActionType == fsx.AdministrativeActionTypeVolumeUpdate &&
			action.TargetVolumeValues.OpenZFSConfiguration != nil &&
			action.TargetVolumeValues.OpenZFSConfiguration.StorageCapacityQuotaGiB != nil &&
			*action.TargetVolumeValues.OpenZFSConfiguration.StorageCapacityQuotaGiB == resizeGiB &&
			action.TargetVolumeValues.OpenZFSConfiguration.StorageCapacityReservationGiB != nil &&
			*action.TargetVolumeValues.OpenZFSConfiguration.StorageCapacityReservationGiB == resizeGiB {
			return action, nil
		}
	}

	return nil, fmt.Errorf("there is no update with storage capacity of %d GiB on volume %s", resizeGiB, volumeId)
}

func CollapseCreateFileSystemParameters(parameters map[string]string) error {
	config := fsx.CreateFileSystemOpenZFSConfiguration{}
	return util.ReplaceParametersAndPopulateObject("OpenZFSConfiguration", parameters, &config)
}

func CollapseDeleteFileSystemParameters(parameters map[string]string) error {
	config := fsx.DeleteFileSystemOpenZFSConfiguration{}
	return util.ReplaceParametersAndPopulateObject("OpenZFSConfiguration", parameters, &config)
}

func CollapseCreateVolumeParameters(parameters map[string]string) error {
	config := fsx.CreateOpenZFSVolumeConfiguration{}
	return util.ReplaceParametersAndPopulateObject("OpenZFSConfiguration", parameters, &config)
}

func CollapseDeleteVolumeParameters(parameters map[string]string) error {
	config := fsx.DeleteVolumeOpenZFSConfiguration{}
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

	return config.Validate()
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

	return config.Validate()
}
