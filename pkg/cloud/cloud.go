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
	"strconv"
	"strings"
	"time"
)

const (
	DefaultFileSystemType = "OPENZFS"
)

// Polling
const (
	// PollCheckInterval specifies the interval to check if the resource is ready;
	// this needs to be shorter than the timeout.
	PollCheckInterval = 15 * time.Second
	// PollCheckTimeout specifies the time limit for polling the describe API for a completed create/update operation.
	PollCheckTimeout = 15 * time.Minute
)

// Tags
const (
	// VolumeNameTagKey is the key value that refers to the volume's name.
	VolumeNameTagKey = "CSIVolumeName"
	// SkipFinalBackupTagKey is the key value that stores whether or not a user wants to SkipFinalBackup during delete
	SkipFinalBackupTagKey = "CSISkipFinalBackup"
	// SnapshotNameTagKey is the key value that refers to the snapshot's name.
	SnapshotNameTagKey = "CSIVolumeSnapshotName"
	// AwsFsxOpenZfsDriverTagKey is the tag to identify if a volume/snapshot is managed by the openzfs csi driver
	AwsFsxOpenZfsDriverTagKey = "fsx.openzfs.csi.aws.com/cluster"
)

// Errors
var (
	ErrInvalidParameter             = errors.New("invalid input")
	ErrAlreadyExists                = errors.New("resource already exists with the given name and different parameters")
	ErrNotFound                     = errors.New("resource could not be found using the respective ID")
	ErrMultipleFound                = errors.New("multiple resource found with the given ID")
	errParseDiskIopsConfiguration   = errors.New("error when parsing DiskIopsConfiguration")
	errParseRootVolumeConfiguration = errors.New("error when parsing RootVolumeConfiguration")
	errParseUserAndGroupQuotas      = errors.New("error when parsing UserAndGroupQuotas")
	errParseNfsExports              = errors.New("error when parsing NfsExports")
	errParseTags                    = errors.New("error when parsing Tags")
	errInvalidMap                   = errors.New("object is incorrectly formatted")
	errKeyDoesNotExist              = errors.New("inputted key does not exist")
	errValueNotAnInt                = errors.New("inputted value is not an int")
	errValueNotABool                = errors.New("inputted value is not an bool")
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

// FileSystemOptions represents parameters to create an OpenZFS filesystem
type FileSystemOptions struct {
	KmsKeyId                      *string
	AutomaticBackupRetentionDays  *int64
	CopyTagsToBackups             *bool
	CopyTagsToVolumes             *bool
	DailyAutomaticBackupStartTime *string
	DeploymentType                *string
	DiskIopsConfiguration         *string
	RootVolumeConfiguration       *string
	ThroughputCapacity            *int64
	WeeklyMaintenanceStartTime    *string
	SecurityGroupIds              []string
	StorageCapacity               *int64
	SubnetIds                     []string
	Tags                          *string
	SkipFinalBackup               *bool
}

// Volume represents an OpenZFS volume
type Volume struct {
	FileSystemId                  string
	VolumeId                      string
	StorageCapacityQuotaGiB       int64
	StorageCapacityReservationGiB int64
	VolumePath                    string
}

// VolumeOptions represents parameters to create an OpenZFS volume
type VolumeOptions struct {
	Name                          *string
	CopyTagsToSnapshots           *bool
	DataCompressionType           *string
	NfsExports                    *string
	ParentVolumeId                *string
	ReadOnly                      *bool
	RecordSizeKiB                 *int64
	StorageCapacityQuotaGiB       *int64
	StorageCapacityReservationGiB *int64
	UserAndGroupQuotas            *string
	Tags                          *string
	SnapshotARN                   *string
}

// Snapshot represents an OpenZFS volume snapshot
type Snapshot struct {
	SnapshotID     string
	SourceVolumeID string
	ResourceARN    string
	CreationTime   time.Time
}

// SnapshotOptions represents parameters to create an OpenZFS snapshot
type SnapshotOptions struct {
	SnapshotName   *string
	SourceVolumeId *string
	Tags           *string
}

// FSx abstracts FSx client to facilitate its mocking.
// See https://docs.aws.amazon.com/sdk-for-go/api/service/fsx/ for details
type FSx interface {
	CreateFileSystemWithContext(aws.Context, *fsx.CreateFileSystemInput, ...request.Option) (*fsx.CreateFileSystemOutput, error)
	UpdateFileSystemWithContext(aws.Context, *fsx.UpdateFileSystemInput, ...request.Option) (*fsx.UpdateFileSystemOutput, error)
	DeleteFileSystemWithContext(aws.Context, *fsx.DeleteFileSystemInput, ...request.Option) (*fsx.DeleteFileSystemOutput, error)
	DescribeFileSystemsWithContext(aws.Context, *fsx.DescribeFileSystemsInput, ...request.Option) (*fsx.DescribeFileSystemsOutput, error)
	CreateVolumeWithContext(aws.Context, *fsx.CreateVolumeInput, ...request.Option) (*fsx.CreateVolumeOutput, error)
	UpdateVolumeWithContext(aws.Context, *fsx.UpdateVolumeInput, ...request.Option) (*fsx.UpdateVolumeOutput, error)
	DeleteVolumeWithContext(aws.Context, *fsx.DeleteVolumeInput, ...request.Option) (*fsx.DeleteVolumeOutput, error)
	DescribeVolumesWithContext(aws.Context, *fsx.DescribeVolumesInput, ...request.Option) (*fsx.DescribeVolumesOutput, error)
	CreateSnapshotWithContext(aws.Context, *fsx.CreateSnapshotInput, ...request.Option) (*fsx.CreateSnapshotOutput, error)
	DeleteSnapshotWithContext(aws.Context, *fsx.DeleteSnapshotInput, ...request.Option) (*fsx.DeleteSnapshotOutput, error)
	DescribeSnapshotsWithContext(aws.Context, *fsx.DescribeSnapshotsInput, ...request.Option) (*fsx.DescribeSnapshotsOutput, error)
}

type Cloud interface {
	CreateFileSystem(ctx context.Context, fileSystemId string, fileSystemOptions FileSystemOptions) (*FileSystem, error)
	DeleteFileSystem(ctx context.Context, fileSystemId string) error
	DescribeFileSystem(ctx context.Context, fileSystemId string) (*FileSystem, error)
	WaitForFileSystemAvailable(ctx context.Context, fileSystemId string) error
	CreateVolume(ctx context.Context, volumeId string, volumeOptions VolumeOptions) (*Volume, error)
	DeleteVolume(ctx context.Context, volumeId string) error
	DescribeVolume(ctx context.Context, volumeId string) (*Volume, error)
	WaitForVolumeAvailable(ctx context.Context, volumeId string) error
	CreateSnapshot(ctx context.Context, snapshotOptions SnapshotOptions) (snapshot *Snapshot, err error)
	DeleteSnapshot(ctx context.Context, snapshotId string) error
	DescribeSnapshot(ctx context.Context, snapshotId string) (*Snapshot, error)
	WaitForSnapshotAvailable(ctx context.Context, snapshotId string) error
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

func (c *cloud) CreateFileSystem(ctx context.Context, fileSystemId string, fileSystemOptions FileSystemOptions) (*FileSystem, error) {
	var err error

	var diskIopsConfiguration *fsx.DiskIopsConfiguration
	if fileSystemOptions.DiskIopsConfiguration != nil {
		diskIopsConfiguration, err = c.parseDiskIopsConfiguration(*fileSystemOptions.DiskIopsConfiguration)
		if err != nil {
			return nil, fmt.Errorf("%w. %s. %s", ErrInvalidParameter, errParseDiskIopsConfiguration, err)
		}
	}

	var rootVolumeConfig *fsx.OpenZFSCreateRootVolumeConfiguration
	if fileSystemOptions.RootVolumeConfiguration != nil {
		rootVolumeConfig, err = c.parseRootVolumeConfiguration(*fileSystemOptions.RootVolumeConfiguration)
		if err != nil {
			return nil, fmt.Errorf("%w. %s. %s", ErrInvalidParameter, errParseRootVolumeConfiguration, err)
		}
	}

	openZFSConfiguration := &fsx.CreateFileSystemOpenZFSConfiguration{
		AutomaticBackupRetentionDays:  fileSystemOptions.AutomaticBackupRetentionDays,
		CopyTagsToBackups:             fileSystemOptions.CopyTagsToBackups,
		CopyTagsToVolumes:             fileSystemOptions.CopyTagsToVolumes,
		DailyAutomaticBackupStartTime: fileSystemOptions.DailyAutomaticBackupStartTime,
		DeploymentType:                fileSystemOptions.DeploymentType,
		DiskIopsConfiguration:         diskIopsConfiguration,
		RootVolumeConfiguration:       rootVolumeConfig,
		ThroughputCapacity:            fileSystemOptions.ThroughputCapacity,
		WeeklyMaintenanceStartTime:    fileSystemOptions.WeeklyMaintenanceStartTime,
	}

	tags := []*fsx.Tag{
		{
			Key:   aws.String(AwsFsxOpenZfsDriverTagKey),
			Value: aws.String("true"),
		},
		{
			Key:   aws.String(VolumeNameTagKey),
			Value: aws.String(fileSystemId),
		},
	}
	if fileSystemOptions.Tags != nil {
		userTags, err := c.parseTags(*fileSystemOptions.Tags)
		if err != nil {
			return nil, fmt.Errorf("%w. %s. %s", ErrInvalidParameter, errParseTags, err)
		}
		tags = append(tags, userTags...)
	}

	//Store SkipFinalBackup as a tag, so we can retrieve it during delete
	if fileSystemOptions.SkipFinalBackup != nil {
		tags = append(tags, &fsx.Tag{
			Key:   aws.String(SkipFinalBackupTagKey),
			Value: util.BoolToStringPointer(*fileSystemOptions.SkipFinalBackup),
		})
	}

	input := fsx.CreateFileSystemInput{
		ClientRequestToken:   aws.String(fileSystemId),
		FileSystemType:       aws.String(DefaultFileSystemType),
		KmsKeyId:             fileSystemOptions.KmsKeyId,
		OpenZFSConfiguration: openZFSConfiguration,
		SecurityGroupIds:     aws.StringSlice(fileSystemOptions.SecurityGroupIds),
		StorageCapacity:      fileSystemOptions.StorageCapacity,
		SubnetIds:            aws.StringSlice(fileSystemOptions.SubnetIds),
		Tags:                 tags,
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

func (c *cloud) DeleteFileSystem(ctx context.Context, fileSystemId string) error {
	fs, err := c.getFileSystem(ctx, fileSystemId)
	if err != nil {
		return err
	}

	var skipFinalBackup *bool
	for _, tag := range fs.Tags {
		if *tag.Key == SkipFinalBackupTagKey {
			if tag.Value != nil {
				//If the tag was manually modified and is no longer a bool, set the behavior to the CSI driver default of true
				skipFinalBackup, err = util.StringToBoolPointer(*tag.Value)
			}
			break
		}
	}

	var finalBackupTags []*fsx.Tag
	if skipFinalBackup == nil || *skipFinalBackup == false {
		finalBackupTags = []*fsx.Tag{
			{
				Key:   aws.String(AwsFsxOpenZfsDriverTagKey),
				Value: aws.String("true"),
			},
		}
	}

	openZFSConfiguration := &fsx.DeleteFileSystemOpenZFSConfiguration{
		FinalBackupTags: finalBackupTags,
		SkipFinalBackup: skipFinalBackup,
	}

	input := fsx.DeleteFileSystemInput{
		FileSystemId:         aws.String(fileSystemId),
		OpenZFSConfiguration: openZFSConfiguration,
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

func (c *cloud) CreateVolume(ctx context.Context, volumeId string, volumeOptions VolumeOptions) (*Volume, error) {
	var err error

	var nfsExports []*fsx.OpenZFSNfsExport
	if volumeOptions.NfsExports != nil {
		nfsExports, err = c.parseNfsExports(*volumeOptions.NfsExports)
		if err != nil {
			return nil, fmt.Errorf("%w. %s. %s", ErrInvalidParameter, errParseNfsExports, err)
		}
	}

	var originSnapshotConfiguration *fsx.CreateOpenZFSOriginSnapshotConfiguration
	if volumeOptions.SnapshotARN != nil {
		originSnapshotConfiguration = &fsx.CreateOpenZFSOriginSnapshotConfiguration{
			//CLONE strategy must be used since FULL_COPY creates a new snapshot.
			//It is possible to delete a snapshot but not the child volumes, therefore users must sequentially delete
			CopyStrategy: aws.String(fsx.OpenZFSCopyStrategyClone),
			SnapshotARN:  volumeOptions.SnapshotARN,
		}
	}

	var userAndGroupQuotas []*fsx.OpenZFSUserOrGroupQuota
	if volumeOptions.UserAndGroupQuotas != nil {
		userAndGroupQuotas, err = c.parseUserAndGroupQuotas(*volumeOptions.UserAndGroupQuotas)
		if err != nil {
			return nil, fmt.Errorf("%w. %s. %s", ErrInvalidParameter, errParseUserAndGroupQuotas, err)
		}
	}

	openZFSConfiguration := &fsx.CreateOpenZFSVolumeConfiguration{
		CopyTagsToSnapshots:           volumeOptions.CopyTagsToSnapshots,
		DataCompressionType:           volumeOptions.DataCompressionType,
		NfsExports:                    nfsExports,
		OriginSnapshot:                originSnapshotConfiguration,
		ParentVolumeId:                volumeOptions.ParentVolumeId,
		ReadOnly:                      volumeOptions.ReadOnly,
		RecordSizeKiB:                 volumeOptions.RecordSizeKiB,
		StorageCapacityQuotaGiB:       volumeOptions.StorageCapacityQuotaGiB,
		StorageCapacityReservationGiB: volumeOptions.StorageCapacityReservationGiB,
		UserAndGroupQuotas:            userAndGroupQuotas,
	}

	tags := []*fsx.Tag{
		{
			Key:   aws.String(AwsFsxOpenZfsDriverTagKey),
			Value: aws.String("true"),
		},
		{
			Key:   aws.String(VolumeNameTagKey),
			Value: aws.String(volumeId),
		},
	}
	if volumeOptions.Tags != nil {
		userTags, err := c.parseTags(*volumeOptions.Tags)
		if err != nil {
			return nil, fmt.Errorf("%w. %s. %s", ErrInvalidParameter, errParseTags, err)
		}
		tags = append(tags, userTags...)
	}

	input := fsx.CreateVolumeInput{
		ClientRequestToken:   aws.String(volumeId),
		Name:                 volumeOptions.Name,
		OpenZFSConfiguration: openZFSConfiguration,
		Tags:                 tags,
		VolumeType:           aws.String(DefaultFileSystemType),
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
		FileSystemId:                  aws.StringValue(output.Volume.FileSystemId),
		VolumeId:                      aws.StringValue(output.Volume.VolumeId),
		StorageCapacityQuotaGiB:       aws.Int64Value(output.Volume.OpenZFSConfiguration.StorageCapacityQuotaGiB),
		StorageCapacityReservationGiB: aws.Int64Value(output.Volume.OpenZFSConfiguration.StorageCapacityReservationGiB),
		VolumePath:                    aws.StringValue(output.Volume.OpenZFSConfiguration.VolumePath),
	}, nil
}

func (c *cloud) DeleteVolume(ctx context.Context, volumeId string) error {
	input := fsx.DeleteVolumeInput{
		VolumeId: aws.String(volumeId),
	}

	_, err := c.fsx.DeleteVolumeWithContext(ctx, &input)
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
		VolumeId:                      aws.StringValue(v.VolumeId),
		StorageCapacityQuotaGiB:       aws.Int64Value(v.OpenZFSConfiguration.StorageCapacityQuotaGiB),
		StorageCapacityReservationGiB: aws.Int64Value(v.OpenZFSConfiguration.StorageCapacityReservationGiB),
		VolumePath:                    aws.StringValue(v.OpenZFSConfiguration.VolumePath),
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

func (c *cloud) CreateSnapshot(ctx context.Context, snapshotOptions SnapshotOptions) (snapshot *Snapshot, err error) {
	snapshotName := snapshotOptions.SnapshotName
	volumeId := snapshotOptions.SourceVolumeId
	tags := snapshotOptions.Tags
	if snapshotName == nil {
		return nil, fmt.Errorf("snapshot name not provided")
	}
	if volumeId == nil {
		return nil, fmt.Errorf("volume id not provided")
	}

	// The volume id associated with the persistent volume could refer to the file system id. In that case,
	// we need to retrieve the id of the root volume associated with the file system
	volumeId, err = c.getVolumeId(ctx, *volumeId)
	if err != nil {
		return nil, err
	}

	snapshotTags := []*fsx.Tag{
		{
			Key:   aws.String(SnapshotNameTagKey),
			Value: snapshotName,
		},
		{
			Key:   aws.String(AwsFsxOpenZfsDriverTagKey),
			Value: aws.String("true"),
		},
	}

	if tags != nil {
		userTags, err := c.parseTags(*tags)
		if err != nil {
			return nil, fmt.Errorf("%w. %s. %s", ErrInvalidParameter, errParseTags, err)
		}
		snapshotTags = append(snapshotTags, userTags...)
	}

	input := &fsx.CreateSnapshotInput{
		ClientRequestToken: snapshotName,
		Name:               snapshotName,
		Tags:               snapshotTags,
		VolumeId:           volumeId,
	}
	klog.V(4).Infof("CreateSnapshotInput: ", input.GoString())
	output, err := c.fsx.CreateSnapshotWithContext(ctx, input)

	if err != nil {
		if awsErr, ok := err.(awserr.Error); ok {
			if awsErr.Code() == fsx.ErrCodeIncompatibleParameterError {
				return nil, ErrAlreadyExists
			}
		}
		return nil, fmt.Errorf("error creating snapshot of volume %s: %w", *volumeId, err)
	}
	if output == nil {
		return nil, fmt.Errorf("nil CreateSnapshotResponse")
	}
	klog.V(4).Infof("CreateSnapshotResponse: ", output.GoString())
	return &Snapshot{
		SnapshotID:     aws.StringValue(output.Snapshot.SnapshotId),
		SourceVolumeID: aws.StringValue(volumeId),
		ResourceARN:    aws.StringValue(output.Snapshot.ResourceARN),
		CreationTime:   aws.TimeValue(output.Snapshot.CreationTime),
	}, nil
}

func (c *cloud) DeleteSnapshot(ctx context.Context, snapshotId string) (err error) {
	if len(snapshotId) == 0 {
		return fmt.Errorf("snapshot id not provided")
	}

	input := &fsx.DeleteSnapshotInput{
		ClientRequestToken: aws.String(snapshotId),
		SnapshotId:         aws.String(snapshotId),
	}
	if _, err = c.fsx.DeleteSnapshotWithContext(ctx, input); err != nil {
		if awsErr, ok := err.(awserr.Error); ok {
			if awsErr.Code() == fsx.ErrCodeSnapshotNotFound {
				return fmt.Errorf("DeleteSnapshot: Unable to find snapshot %s", snapshotId)
			}
		}
		return fmt.Errorf("DeleteSnapshot: Failed to delete snapshot %s, received error %v", snapshotId, err)
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

	if len(output.Snapshots) > 1 {
		return nil, fmt.Errorf("getSnapshot: snapshot id %s returned multiple snapshots", snapshotId)
	}

	if len(output.Snapshots) == 0 {
		return nil, fmt.Errorf("getSnapshot: could not find snapshot with snapshot id %s", snapshotId)
	}

	return output.Snapshots[0], nil
}

// getVolumeId Parses the volumeId to determine if it is the id of a file system or an OpenZFS volume.
// If the volumeId references a file system, this function retrieves the file system's root volume id.
// If the volumeId references an OpenZFS volume, this function returns a pointer to the volumeId.
func (c *cloud) getVolumeId(ctx context.Context, volumeId string) (*string, error) {
	splitVolumeId := strings.Split(volumeId, "-")
	if len(splitVolumeId) != 2 {
		return nil, fmt.Errorf("volume id %s is improperly formatted", volumeId)
	}
	idPrefix := splitVolumeId[0]
	if idPrefix == FilesystemPrefix {
		filesystem, err := c.getFileSystem(ctx, volumeId)
		if err != nil {
			return nil, err
		}
		if filesystem.OpenZFSConfiguration != nil && len(*filesystem.OpenZFSConfiguration.RootVolumeId) > 0 {
			return filesystem.OpenZFSConfiguration.RootVolumeId, nil
		}
		return nil, fmt.Errorf("failed to retrieve root volume id for file system %s", volumeId)
	} else if idPrefix == VolumePrefix {
		return aws.String(volumeId), nil
	} else {
		return nil, fmt.Errorf("volume id %s is improperly formatted", volumeId)
	}
}

func (c *cloud) parseDiskIopsConfiguration(input string) (*fsx.DiskIopsConfiguration, error) {
	splitConfig := util.SplitCommasAndRemoveOuterBrackets(input)
	mappedConfig, err := util.MapValues(splitConfig)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", errInvalidMap, input)
	}

	diskIopsConfiguration := fsx.DiskIopsConfiguration{}

	for key, value := range mappedConfig {
		if value == nil {
			continue
		}

		switch key {
		case "Iops":
			iops, err := strconv.ParseInt(*value, 10, 64)
			if err != nil {
				return nil, fmt.Errorf("%w: %s=%s", errValueNotAnInt, key, *value)
			}
			diskIopsConfiguration.SetIops(iops)
		case "Mode":
			diskIopsConfiguration.SetMode(*value)
		default:
			return nil, fmt.Errorf("%w: %s", errKeyDoesNotExist, key)
		}
	}

	return &diskIopsConfiguration, nil
}

func (c *cloud) parseRootVolumeConfiguration(input string) (*fsx.OpenZFSCreateRootVolumeConfiguration, error) {
	splitConfig := util.SplitCommasAndRemoveOuterBrackets(input)
	mappedConfig, err := util.MapValues(splitConfig)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", errInvalidMap, input)
	}

	rootVolumeConfiguration := fsx.OpenZFSCreateRootVolumeConfiguration{}

	for key, value := range mappedConfig {
		if value == nil {
			continue
		}

		switch key {
		case "CopyTagsToSnapshots":
			copyTagsToSnapshots, err := strconv.ParseBool(*value)
			if err != nil {
				return nil, fmt.Errorf("%w: %s=%s", errValueNotABool, key, *value)
			}
			rootVolumeConfiguration.SetCopyTagsToSnapshots(copyTagsToSnapshots)
		case "DataCompressionType":
			rootVolumeConfiguration.SetDataCompressionType(*value)
		case "NfsExports":
			nfsExports, err := c.parseNfsExports(*value)
			if err != nil {
				return nil, fmt.Errorf("%w. %s", errParseNfsExports, err)
			}
			rootVolumeConfiguration.SetNfsExports(nfsExports)
		case "ReadOnly":
			readOnly, err := strconv.ParseBool(*value)
			if err != nil {
				return nil, fmt.Errorf("%w: %s=%s", errValueNotABool, key, *value)
			}
			rootVolumeConfiguration.SetReadOnly(readOnly)
		case "RecordSizeKiB":
			recordSize, err := strconv.ParseInt(*value, 10, 64)
			if err != nil {
				return nil, fmt.Errorf("%w: %s=%s", errValueNotAnInt, key, *value)
			}
			rootVolumeConfiguration.SetRecordSizeKiB(recordSize)
		case "UserAndGroupQuotas":
			userAndGroupQuotas, err := c.parseUserAndGroupQuotas(*value)
			if err != nil {
				return nil, fmt.Errorf("%w. %s", errParseUserAndGroupQuotas, err)
			}
			rootVolumeConfiguration.SetUserAndGroupQuotas(userAndGroupQuotas)
		default:
			return nil, fmt.Errorf("%w: %s", errKeyDoesNotExist, key)
		}
	}

	return &rootVolumeConfiguration, nil
}

func (c *cloud) parseNfsExports(input string) ([]*fsx.OpenZFSNfsExport, error) {
	splitElements := util.SplitCommasAndRemoveOuterBrackets(input)

	var nfsExports []*fsx.OpenZFSNfsExport

	for _, clientConfig := range splitElements {
		mappedClientConfiguration, err := util.MapValues([]string{clientConfig})
		if err != nil {
			return nil, fmt.Errorf("%w: %s", errInvalidMap, []string{clientConfig})
		}

		openZFSNfsExport := fsx.OpenZFSNfsExport{}

		for clientKey, clientValue := range mappedClientConfiguration {
			if clientValue == nil {
				continue
			}

			switch clientKey {
			case "ClientConfigurations":
				splitClientConfiguration := util.SplitCommasAndRemoveOuterBrackets(*clientValue)

				var clientConfigurations []*fsx.OpenZFSClientConfiguration

				for _, clientConfiguration := range splitClientConfiguration {
					splitConfig := util.SplitCommasAndRemoveOuterBrackets(clientConfiguration)
					mappedConfig, err := util.MapValues(splitConfig)
					if err != nil {
						return nil, fmt.Errorf("%w: %s", errInvalidMap, clientConfiguration)
					}

					openZFSClientConfiguration := fsx.OpenZFSClientConfiguration{}

					for key, value := range mappedConfig {
						if value == nil {
							continue
						}

						switch key {
						case "Clients":
							openZFSClientConfiguration.SetClients(*value)
						case "Options":
							options := aws.StringSlice(util.SplitCommasAndRemoveOuterBrackets(*value))
							openZFSClientConfiguration.SetOptions(options)
						default:
							return nil, fmt.Errorf("%w: %s", errKeyDoesNotExist, key)
						}
					}

					clientConfigurations = append(clientConfigurations, &openZFSClientConfiguration)
				}

				openZFSNfsExport.SetClientConfigurations(clientConfigurations)
			default:
				return nil, fmt.Errorf("%w: %s", errKeyDoesNotExist, clientKey)
			}
		}

		nfsExports = append(nfsExports, &openZFSNfsExport)
	}

	return nfsExports, nil
}

func (c *cloud) parseUserAndGroupQuotas(input string) ([]*fsx.OpenZFSUserOrGroupQuota, error) {
	splitElements := util.SplitCommasAndRemoveOuterBrackets(input)

	var userAndGroupQuotas []*fsx.OpenZFSUserOrGroupQuota

	for _, element := range splitElements {
		splitConfig := util.SplitCommasAndRemoveOuterBrackets(element)
		mappedConfig, err := util.MapValues(splitConfig)
		if err != nil {
			return nil, fmt.Errorf("%w: %s", errInvalidMap, input)
		}

		openZFSUserOrGroupQuota := fsx.OpenZFSUserOrGroupQuota{}

		for key, value := range mappedConfig {
			if value == nil {
				continue
			}

			switch key {
			case "Id":
				id, err := strconv.ParseInt(*value, 10, 64)
				if err != nil {
					return nil, fmt.Errorf("%w: %s=%s", errValueNotAnInt, key, *value)
				}
				openZFSUserOrGroupQuota.SetId(id)
			case "StorageCapacityQuotaGiB":
				storageCapacityQuotaGiB, err := strconv.ParseInt(*value, 10, 64)
				if err != nil {
					return nil, fmt.Errorf("%w: %s=%s", errValueNotAnInt, key, *value)
				}
				openZFSUserOrGroupQuota.SetStorageCapacityQuotaGiB(storageCapacityQuotaGiB)
			case "Type":
				openZFSUserOrGroupQuota.SetType(*value)
			default:
				return nil, fmt.Errorf("%w: %s", errKeyDoesNotExist, key)
			}
		}

		userAndGroupQuotas = append(userAndGroupQuotas, &openZFSUserOrGroupQuota)
	}

	return userAndGroupQuotas, nil
}

func (c *cloud) parseTags(input string) ([]*fsx.Tag, error) {
	splitConfig := util.SplitCommasAndRemoveOuterBrackets(input)
	mappedConfig, err := util.MapValues(splitConfig)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", errInvalidMap, input)
	}

	var tags []*fsx.Tag

	for key, value := range mappedConfig {
		var tag = fsx.Tag{
			Key:   aws.String(key),
			Value: value,
		}

		tags = append(tags, &tag)
	}

	return tags, nil
}
