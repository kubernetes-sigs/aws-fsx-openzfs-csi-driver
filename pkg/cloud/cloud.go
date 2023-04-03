package cloud

import (
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/ec2metadata"
	"github.com/aws/aws-sdk-go/aws/session"
)

type Cloud interface {
	GetMetadata() MetadataService
}

type cloud struct {
	metadata MetadataService
}

// NewCloud returns a new instance of AWS cloud
// It panics if session is invalid
func NewCloud() (Cloud, error) {
	sess := session.Must(session.NewSession(&aws.Config{}))
	svc := ec2metadata.New(sess)

	metadata, err := NewMetadataService(svc)
	if err != nil {
		return nil, fmt.Errorf("could not get metadata from AWS: %v", err)
	}

	return &cloud{
		metadata: metadata,
	}, nil
}

func (c *cloud) GetMetadata() MetadataService {
	return c.metadata
}
