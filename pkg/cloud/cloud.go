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
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/fsx"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/klog/v2"
	"strings"
	"time"
)

// Polling
const (
	// SnapshotPollCheckInterval specifies the interval to check if the resource is ready;
	// this needs to be shorter than the csi snapshotter timeout.
	SnapshotPollCheckInterval = 15 * time.Second
	// PollCheckTimeout specifies the time limit for polling the describe API for a completed create/update operation.
	PollCheckTimeout = 15 * time.Minute
)

// Tags
const (
	// SnapshotNameTagKey is the key value that refers to the snapshot's name.
	SnapshotNameTagKey = "CSIVolumeSnapshotName"
	// AwsFsxOpenZfsDriverTagKey is the tag to identify if a volume/snapshot is managed by the openzfs csi driver
	AwsFsxOpenZfsDriverTagKey = "fsx.openzfs.csi.aws.com/cluster"
)

// Prefixes used to parse the volume id.
const (
	fsPrefix     = "fs"
	volumePrefix = "fsvol"
)

// Snapshot represents an OpenZFS volume snapshot
type Snapshot struct {
	SnapshotID     string
	SourceVolumeID string
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
	CreateSnapshotWithContext(aws.Context, *fsx.CreateSnapshotInput, ...request.Option) (*fsx.CreateSnapshotOutput, error)
	DeleteSnapshotWithContext(aws.Context, *fsx.DeleteSnapshotInput, ...request.Option) (*fsx.DeleteSnapshotOutput, error)
	DescribeSnapshotsWithContext(aws.Context, *fsx.DescribeSnapshotsInput, ...request.Option) (*fsx.DescribeSnapshotsOutput, error)
	DescribeFileSystemsWithContext(aws.Context, *fsx.DescribeFileSystemsInput, ...request.Option) (*fsx.DescribeFileSystemsOutput, error)
}

type Cloud interface {
	CreateSnapshot(ctx context.Context, snapshotOptions SnapshotOptions) (snapshot *Snapshot, err error)
	WaitForSnapshotAvailable(ctx context.Context, snapshotId string) error
	DeleteSnapshot(ctx context.Context, snapshotId string) error
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
		snapshotTags = append(snapshotTags, parseTags(*tags)...)
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
		return nil, fmt.Errorf("error creating snapshot of volume %s: %w", *volumeId, err)
	}
	if output == nil {
		return nil, fmt.Errorf("nil CreateSnapshotResponse")
	}
	klog.V(4).Infof("CreateSnapshotResponse: ", output.GoString())
	return &Snapshot{
		SnapshotID:     *output.Snapshot.SnapshotId,
		SourceVolumeID: *volumeId,
		CreationTime:   *output.Snapshot.CreationTime,
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
		if isSnapshotNotFound(err) {
			return fmt.Errorf("DeleteSnapshot: Unable to find snapshot %s", snapshotId)
		}
		return fmt.Errorf("DeleteSnapshot: Failed to delete snapshot %s, received error %v", snapshotId, err)
	}
	return nil
}

func (c *cloud) WaitForSnapshotAvailable(ctx context.Context, snapshotId string) error {
	if len(snapshotId) == 0 {
		return fmt.Errorf("snapshot id not provided")
	}

	err := wait.Poll(SnapshotPollCheckInterval, PollCheckTimeout, func() (done bool, err error) {
		snapshot, err := c.getSnapshot(ctx, snapshotId)
		if err != nil {
			return true, err
		}
		klog.V(2).Infof("WaitForSnapshotAvailable: Snapshot %s status is %q", snapshotId, *snapshot.Lifecycle)
		switch *snapshot.Lifecycle {
		case fsx.SnapshotLifecycleAvailable:
			return true, nil
		case fsx.SnapshotLifecyclePending:
			return false, nil
		default:
			return true, fmt.Errorf("WaitForSnapshotAvailable: Snapshot %s has unexpected status %q", snapshotId, *snapshot.Lifecycle)
		}
	})

	return err
}

func (c *cloud) getFileSystem(ctx context.Context, fileSystemId string) (*fsx.FileSystem, error) {
	klog.V(2).Infof("getFileSystem: called for fileSystemId %s", fileSystemId)
	input := &fsx.DescribeFileSystemsInput{
		FileSystemIds: []*string{aws.String(fileSystemId)},
	}
	output, err := c.fsx.DescribeFileSystemsWithContext(ctx, input)
	if err != nil {
		klog.V(2).Infof("getFileSystem: DescribeFileSystems failed with error %q", err)
		return nil, err
	}

	if len(output.FileSystems) > 1 {
		return nil, fmt.Errorf("getFileSystem: snapshot id %s returned multiple file systems", fileSystemId)
	}

	if len(output.FileSystems) == 0 {
		return nil, fmt.Errorf("getFileSystem: could not find file system with fileSystemId %s", fileSystemId)
	}

	klog.V(2).Infof("getFileSystem: found file system %q", output.FileSystems[0])
	return output.FileSystems[0], nil
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

func (c *cloud) getSnapshotResourceARN(ctx context.Context, snapshotId string) (*string, error) {
	snapshot, err := c.getSnapshot(ctx, snapshotId)
	if err != nil {
		return nil, err
	}

	snapshotResourceARN := snapshot.ResourceARN
	if snapshotResourceARN == nil {
		return nil, fmt.Errorf("getSnapshotResourceARN: Failed to retrieve snapshotARN for snapshot %s", snapshotId)
	}

	return snapshotResourceARN, nil
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
	if idPrefix == fsPrefix {
		filesystem, err := c.getFileSystem(ctx, volumeId)
		if err != nil {
			return nil, err
		}
		if filesystem.OpenZFSConfiguration != nil && len(*filesystem.OpenZFSConfiguration.RootVolumeId) > 0 {
			return filesystem.OpenZFSConfiguration.RootVolumeId, nil
		}
		return nil, fmt.Errorf("failed to retrieve root volume id for file system %s", volumeId)
	} else if idPrefix == volumePrefix {
		return aws.String(volumeId), nil
	} else {
		return nil, fmt.Errorf("volume id %s is improperly formatted", volumeId)
	}
}

func isSnapshotNotFound(err error) bool {
	if awsErr, ok := err.(awserr.Error); ok {
		if awsErr.Code() == fsx.ErrCodeSnapshotNotFound {
			return true
		}
	}
	return false
}

// parseTags Parses the parameter tag string and returns a list of FSx tags
func parseTags(tags string) []*fsx.Tag {
	var tagMap []*fsx.Tag
	splitTags := strings.Split(tags, ",")
	for _, tag := range splitTags {
		tagSplit := strings.Split(tag, "=")
		if len(tagSplit) != 2 {
			klog.Warningf("Tag %s is not formatted properly. Not adding to list of tags", tag)
		} else {
			tagKey := tagSplit[0]
			tagValue := tagSplit[1]
			tagMap = append(tagMap, &fsx.Tag{
				Key:   aws.String(tagKey),
				Value: aws.String(tagValue),
			})
		}
	}
	return tagMap
}
