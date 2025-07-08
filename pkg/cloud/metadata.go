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

	"github.com/aws/aws-sdk-go-v2/feature/ec2/imds"
	"k8s.io/klog/v2"
)

// MetadataService represents AWS metadata service.
type MetadataService interface {
	GetInstanceID() string
	GetInstanceType() string
	GetRegion() string
	GetAvailabilityZone() string
}

type EC2Metadata interface {
	GetInstanceIdentityDocument(context.Context, *imds.GetInstanceIdentityDocumentInput, ...func(*imds.Options)) (*imds.GetInstanceIdentityDocumentOutput, error)
}

type Metadata struct {
	InstanceID       string
	InstanceType     string
	Region           string
	AvailabilityZone string
}

var _ MetadataService = &Metadata{}

// GetInstanceID returns the instance identification.
func (m *Metadata) GetInstanceID() string {
	return m.InstanceID
}

// GetInstanceType returns the instance type.
func (m *Metadata) GetInstanceType() string {
	return m.InstanceType
}

// GetRegion returns the Region Zone which the instance is in.
func (m *Metadata) GetRegion() string {
	return m.Region
}

// GetAvailabilityZone returns the Availability Zone which the instance is in.
func (m *Metadata) GetAvailabilityZone() string {
	return m.AvailabilityZone
}

// NewMetadataService returns a new MetadataServiceImplementation.
func NewMetadataService(ec2MetadataClient EC2MetadataClient, k8sAPIClient KubernetesAPIClient, region string) (MetadataService, error) {
	klog.InfoS("retrieving instance data from ec2 metadata")
	svc, err := ec2MetadataClient()
	if err != nil {
		klog.InfoS("error creating ec2 metadata client", "err", err)
	} else {
		// Check if EC2 metadata is available by attempting to get instance identity
		_, err := svc.GetInstanceIdentityDocument(context.Background(), &imds.GetInstanceIdentityDocumentInput{})
		if err != nil {
			klog.InfoS("ec2 metadata is not available", "err", err)
		} else {
			klog.InfoS("ec2 metadata is available")
			return EC2MetadataInstanceInfo(svc, region)
		}
	}

	klog.InfoS("retrieving instance data from kubernetes api")
	clientset, err := k8sAPIClient()
	if err != nil {
		klog.InfoS("error creating kubernetes api client", "err", err)
	} else {
		klog.InfoS("kubernetes api is available")
		return KubernetesAPIInstanceInfo(clientset)
	}

	return nil, fmt.Errorf("error getting instance data from ec2 metadata or kubernetes api")
}
