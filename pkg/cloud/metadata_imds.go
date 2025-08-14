package cloud

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/ec2/imds"
	"k8s.io/klog/v2"
)

type IMDSClient func() (IMDS, error)

var DefaultIMDSClient = func() (IMDS, error) {
	cfg, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		return nil, err
	}
	svc := imds.NewFromConfig(cfg)
	return svc, nil
}

func IMDSInstanceInfo(svc IMDS, regionFromSession string) (*Metadata, error) {
	klog.InfoS("Retrieving instance metadata from IMDS", "regionFromSession", regionFromSession)

	doc, err := svc.GetInstanceIdentityDocument(context.Background(), &imds.GetInstanceIdentityDocumentInput{})
	if err != nil {
		return nil, fmt.Errorf("could not get instance identity document from IMDS: %w", err)
	}

	if len(doc.InstanceIdentityDocument.InstanceID) == 0 {
		return nil, fmt.Errorf("could not get valid instance ID from IMDS")
	}
	if len(doc.InstanceIdentityDocument.InstanceType) == 0 {
		return nil, fmt.Errorf("could not get valid instance type from IMDS")
	}

	if len(doc.InstanceIdentityDocument.AvailabilityZone) == 0 {
		return nil, fmt.Errorf("could not get valid availability zone from IMDS")
	}

	region := doc.InstanceIdentityDocument.Region
	if len(region) == 0 && len(regionFromSession) != 0 {
		region = regionFromSession
	}

	if len(region) == 0 {
		az := doc.InstanceIdentityDocument.AvailabilityZone
		if len(az) >= 1 {
			// Remove the last character from AZ to get the region
			region = az[:len(az)-1]
		}
	}

	if len(region) == 0 {
		return nil, fmt.Errorf("could not get valid region")
	}

	instanceInfo := Metadata{
		InstanceID:       doc.InstanceIdentityDocument.InstanceID,
		InstanceType:     doc.InstanceIdentityDocument.InstanceType,
		Region:           region,
		AvailabilityZone: doc.InstanceIdentityDocument.AvailabilityZone,
	}

	return &instanceInfo, nil
}
