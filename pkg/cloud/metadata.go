package cloud

import (
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/ec2metadata"
	"github.com/aws/aws-sdk-go/aws/session"
)

type EC2Metadata interface {
	Available() bool
	GetInstanceIdentityDocument() (ec2metadata.EC2InstanceIdentityDocument, error)
}

// MetadataService represents AWS metadata service.
type MetadataService interface {
	GetInstanceID() string
	GetRegion() string
	GetAvailabilityZone() string
}

type metadata struct {
	instanceID       string
	region           string
	availabilityZone string
}

var _ MetadataService = &metadata{}

// GetInstanceID returns the instance identification.
func (m *metadata) GetInstanceID() string {
	return m.instanceID
}

// GetRegion returns the region Zone which the instance is in.
func (m *metadata) GetRegion() string {
	return m.region
}

// GetAvailabilityZone returns the Availability Zone which the instance is in.
func (m *metadata) GetAvailabilityZone() string {
	return m.availabilityZone
}

// NewMetadataService returns a new MetadataServiceImplementation.
func NewMetadata() (MetadataService, error) {
	sess := session.Must(session.NewSession(&aws.Config{}))
	svc := ec2metadata.New(sess)
	return NewMetadataService(svc)
}

// NewMetadataService returns a new MetadataServiceImplementation.
func NewMetadataService(svc EC2Metadata) (MetadataService, error) {
	if !svc.Available() {
		return nil, fmt.Errorf("EC2 instance metadata is not available")
	}

	doc, err := svc.GetInstanceIdentityDocument()
	if err != nil {
		return nil, fmt.Errorf("could not get EC2 instance identity metadata")
	}

	if len(doc.InstanceID) == 0 {
		return nil, fmt.Errorf("could not get valid EC2 instance ID")
	}

	if len(doc.Region) == 0 {
		return nil, fmt.Errorf("could not get valid EC2 region")
	}

	if len(doc.AvailabilityZone) == 0 {
		return nil, fmt.Errorf("could not get valid EC2 availavility zone")
	}

	return &metadata{
		instanceID:       doc.InstanceID,
		region:           doc.Region,
		availabilityZone: doc.AvailabilityZone,
	}, nil
}
