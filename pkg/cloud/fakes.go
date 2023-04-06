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
	"time"
)

var random *rand.Rand

func init() {
	random = rand.New(rand.NewSource(time.Now().UnixNano()))
}

type FakeCloudProvider struct {
	m         *metadata
	snapshots map[string]*Snapshot
}

func NewFakeCloudProvider() *FakeCloudProvider {
	return &FakeCloudProvider{
		m:         &metadata{"instanceID", "region", "az"},
		snapshots: make(map[string]*Snapshot),
	}
}

func (c *FakeCloudProvider) GetMetadata() MetadataService {
	return c.m
}

func (c *FakeCloudProvider) CreateSnapshot(ctx context.Context, snapshotOptions SnapshotOptions) (snapshot *Snapshot, err error) {
	snapshotName := *snapshotOptions.SnapshotName
	sourceVolumeId := *snapshotOptions.SourceVolumeId
	snapshot, exists := c.snapshots[snapshotName]
	if exists {
		if snapshot.SourceVolumeID == sourceVolumeId {
			return snapshot, nil
		} else {
			return nil, fmt.Errorf("snapshot %s already exists", snapshotName)
		}
	}

	snapshot = &Snapshot{
		SnapshotID:     fmt.Sprintf("fsvolsnap-%d", random.Uint64()),
		SourceVolumeID: sourceVolumeId,
		CreationTime:   time.Now(),
	}

	c.snapshots[snapshotName] = snapshot
	return snapshot, nil
}

func (c *FakeCloudProvider) DeleteSnapshot(ctx context.Context, snapshotId string) (err error) {
	delete(c.snapshots, snapshotId)
	for name, snapshot := range c.snapshots {
		if snapshot.SnapshotID == snapshotId {
			delete(c.snapshots, name)
		}
	}
	return nil
}

func (c *FakeCloudProvider) WaitForSnapshotAvailable(ctx context.Context, snapshotId string) error {
	return nil
}
