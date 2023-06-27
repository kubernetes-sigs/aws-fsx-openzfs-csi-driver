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
	"github.com/kubernetes-sigs/aws-fsx-openzfs-csi-driver/pkg/cloud"
	"github.com/kubernetes-sigs/aws-fsx-openzfs-csi-driver/pkg/driver/internal"
	"k8s.io/mount-utils"
)

func NewFakeMounter() Mounter {
	return &NodeMounter{
		Interface: &mount.FakeMounter{
			MountPoints: []mount.MountPoint{},
		},
	}
}

// NewFakeDriver creates a new mock driver used for testing
func NewFakeDriver(endpoint string) *Driver {
	driverOptions := DriverOptions{
		endpoint: endpoint,
		mode:     AllMode,
	}

	driver := &Driver{
		options: &driverOptions,
		controllerService: controllerService{
			cloud:         cloud.NewFakeCloudProvider(),
			inFlight:      internal.NewInFlight(),
			driverOptions: &driverOptions,
		},
		nodeService: nodeService{
			metadata: &cloud.Metadata{
				InstanceID:       "instanceID",
				Region:           "region",
				AvailabilityZone: "az",
			},
			mounter:       NewFakeMounter(),
			inFlight:      internal.NewInFlight(),
			driverOptions: &DriverOptions{},
		},
	}
	return driver
}

func (d *Driver) ResetCloud() {
	d.controllerService.cloud = cloud.NewFakeCloudProvider()
}
