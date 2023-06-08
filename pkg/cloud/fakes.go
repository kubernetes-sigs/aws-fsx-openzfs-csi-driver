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
	"math/rand"
	"reflect"
	"strconv"
	"time"
)

var random *rand.Rand

func init() {
	random = rand.New(rand.NewSource(time.Now().UnixNano()))
}

type FakeCloudProvider struct {
	m                     *metadata
	filesystemId          string
	fileSystems           map[string]*FileSystem
	fileSystemsParameters map[string]map[string]string
	volumes               map[string]*Volume
	volumesParameters     map[string]map[string]string
	snapshots             map[string]*Snapshot
	snapshotsParameters   map[string]map[string]string
}

func NewFakeCloudProvider() *FakeCloudProvider {
	filesystemId := "fs-1234"
	filesystem := FileSystem{
		DnsName:         "test.us-east-1.fsx.amazonaws.com",
		FileSystemId:    filesystemId,
		StorageCapacity: 100,
	}

	return &FakeCloudProvider{
		m:            &metadata{"instanceID", "region", "az"},
		filesystemId: filesystemId,
		fileSystems:  map[string]*FileSystem{filesystemId: &filesystem},
		volumes:      make(map[string]*Volume),
		snapshots:    make(map[string]*Snapshot),
	}
}

func (c *FakeCloudProvider) GetMetadata() MetadataService {
	return c.m
}

func (c *FakeCloudProvider) CreateFileSystem(ctx context.Context, parameters map[string]string) (*FileSystem, error) {
	exists := false
	var existingParams map[string]string
	var existingId string
	for id, params := range c.fileSystemsParameters {
		if params["ClientRequestToken"] == parameters["ClientRequestToken"] {
			exists = true
			existingParams = params
			existingId = id
			break
		}
	}

	if exists {
		if reflect.DeepEqual(existingParams, parameters) {
			return c.fileSystems[existingId], nil
		} else {
			return nil, ErrAlreadyExists
		}
	}

	storageCapacity, err := strconv.ParseInt(parameters["StorageCapacity"], 10, 64)
	if err != nil {
		return nil, err
	}
	fs := &FileSystem{
		DnsName:         "test.us-east-1.fsx.amazonaws.com",
		FileSystemId:    fmt.Sprintf("fs-%d", random.Uint64()),
		StorageCapacity: storageCapacity,
	}
	c.fileSystems[fs.FileSystemId] = fs
	c.fileSystemsParameters[fs.FileSystemId] = parameters
	return fs, nil
}

func (c *FakeCloudProvider) ResizeFileSystem(ctx context.Context, fileSystemId string, newSizeGiB int64) (*int64, error) {
	for _, fs := range c.fileSystems {
		if fs.FileSystemId == fileSystemId {
			fs.StorageCapacity = newSizeGiB
			return &newSizeGiB, nil
		}
	}
	return nil, ErrNotFound
}

func (c *FakeCloudProvider) DeleteFileSystem(ctx context.Context, parameters map[string]string) error {
	delete(c.fileSystems, parameters["FileSystemId"])
	delete(c.fileSystemsParameters, parameters["FileSystemId"])
	return nil
}

func (c *FakeCloudProvider) DescribeFileSystem(ctx context.Context, filesystemId string) (*FileSystem, error) {
	for _, fs := range c.fileSystems {
		if fs.FileSystemId == filesystemId {
			return fs, nil
		}
	}
	return nil, ErrNotFound
}

func (c *FakeCloudProvider) WaitForFileSystemAvailable(ctx context.Context, fileSystemId string) error {
	return nil
}

func (c *FakeCloudProvider) WaitForFileSystemResize(ctx context.Context, fileSystemId string, newSizeGiB int64) error {
	return nil
}

func (c *FakeCloudProvider) CreateVolume(ctx context.Context, parameters map[string]string) (*Volume, error) {
	exists := false
	var existingParams map[string]string
	var existingId string
	for id, params := range c.volumesParameters {
		if params["ClientRequestToken"] == parameters["ClientRequestToken"] {
			exists = true
			existingParams = params
			existingId = id
			break
		}
	}

	if exists {
		if reflect.DeepEqual(existingParams, parameters) {
			return c.volumes[existingId], nil
		} else {
			return nil, ErrAlreadyExists
		}
	}

	v := &Volume{
		FileSystemId: c.filesystemId,
		VolumePath:   "/",
		VolumeId:     fmt.Sprintf("fsvol-%d", random.Uint64()),
	}
	c.volumes[v.VolumeId] = v
	c.volumesParameters[v.VolumeId] = parameters
	return v, nil
}

func (c *FakeCloudProvider) DeleteVolume(ctx context.Context, parameters map[string]string) (err error) {
	delete(c.volumes, parameters["VolumeId"])
	delete(c.volumesParameters, parameters["VolumeId"])
	return nil
}

func (c *FakeCloudProvider) DescribeVolume(ctx context.Context, volumeId string) (*Volume, error) {
	for _, v := range c.volumes {
		if v.VolumeId == volumeId {
			return v, nil
		}
	}
	return nil, ErrNotFound
}

func (c *FakeCloudProvider) WaitForVolumeAvailable(ctx context.Context, volumeId string) error {
	return nil
}

func (c *FakeCloudProvider) WaitForVolumeResize(ctx context.Context, volumeId string, newSizeGiB int64) error {
	return nil
}

func (c *FakeCloudProvider) CreateSnapshot(ctx context.Context, parameters map[string]string) (snapshot *Snapshot, err error) {
	exists := false
	var existingParams map[string]string
	var existingId string
	for id, params := range c.snapshotsParameters {
		if params["ClientRequestToken"] == parameters["ClientRequestToken"] {
			exists = true
			existingParams = params
			existingId = id
			break
		}
	}

	if exists {
		if reflect.DeepEqual(existingParams, parameters) {
			return c.snapshots[existingId], nil
		} else {
			return nil, ErrAlreadyExists
		}
	}

	snapshot = &Snapshot{
		SnapshotID:     fmt.Sprintf("fsvolsnap-%d", random.Uint64()),
		SourceVolumeID: parameters["VolumeId"],
		CreationTime:   time.Now(),
	}

	c.snapshots[snapshot.SnapshotID] = snapshot
	c.snapshotsParameters[snapshot.SnapshotID] = parameters
	return snapshot, nil
}

func (c *FakeCloudProvider) DeleteSnapshot(ctx context.Context, parameters map[string]string) (err error) {
	delete(c.snapshots, parameters["SnapshotID"])
	delete(c.snapshotsParameters, parameters["SnapshotID"])
	return nil
}

func (c *FakeCloudProvider) DescribeSnapshot(ctx context.Context, snapshotId string) (*Snapshot, error) {
	for _, s := range c.snapshots {
		if s.SnapshotID == snapshotId {
			return s, nil
		}
	}
	return nil, ErrNotFound
}

func (c *FakeCloudProvider) WaitForSnapshotAvailable(ctx context.Context, snapshotId string) error {
	return nil
}

func (c *FakeCloudProvider) GetDeleteParameters(ctx context.Context, id string) (map[string]string, error) {
	return nil, nil
}

func (c *FakeCloudProvider) GetVolumeId(ctx context.Context, volumeId string) (string, error) {
	return "fsvol-123456", nil
}
