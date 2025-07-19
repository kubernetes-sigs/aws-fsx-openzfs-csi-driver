package cloud

import (
	"context"
	"fmt"
	"io"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/ec2/imds"
	"k8s.io/klog/v2"
)

type EC2MetadataClient func() (EC2Metadata, error)

var DefaultEC2MetadataClient = func() (EC2Metadata, error) {
	cfg, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		return nil, err
	}
	svc := imds.NewFromConfig(cfg)
	return svc, nil
}

func EC2MetadataInstanceInfo(svc EC2Metadata, regionFromSession string) (*Metadata, error) {
	klog.InfoS("Retrieving EC2 instance metadata", "regionFromSession", regionFromSession)

	// Get instance ID
	instanceIDOutput, err := svc.GetMetadata(context.Background(), &imds.GetMetadataInput{Path: "instance-id"})
	if err != nil {
		return nil, fmt.Errorf("could not get EC2 instance ID: %w", err)
	}

	// Read content from ReadCloser
	instanceIDBytes, err := io.ReadAll(instanceIDOutput.Content)
	if err != nil {
		return nil, fmt.Errorf("could not read EC2 instance ID: %w", err)
	}
	defer instanceIDOutput.Content.Close()

	instanceID := string(instanceIDBytes)
	if len(instanceID) == 0 {
		return nil, fmt.Errorf("could not get valid EC2 instance ID")
	}

	// Get instance type
	instanceTypeOutput, err := svc.GetMetadata(context.Background(), &imds.GetMetadataInput{Path: "instance-type"})
	if err != nil {
		return nil, fmt.Errorf("could not get EC2 instance type: %w", err)
	}

	// Read content from ReadCloser
	instanceTypeBytes, err := io.ReadAll(instanceTypeOutput.Content)
	if err != nil {
		return nil, fmt.Errorf("could not read EC2 instance type: %w", err)
	}
	defer instanceTypeOutput.Content.Close()

	instanceType := string(instanceTypeBytes)
	if len(instanceType) == 0 {
		return nil, fmt.Errorf("could not get valid EC2 instance type")
	}

	// Get availability zone
	azOutput, err := svc.GetMetadata(context.Background(), &imds.GetMetadataInput{Path: "placement/availability-zone"})
	if err != nil {
		return nil, fmt.Errorf("could not get EC2 availability zone: %w", err)
	}

	// Read content from ReadCloser
	azBytes, err := io.ReadAll(azOutput.Content)
	if err != nil {
		return nil, fmt.Errorf("could not read EC2 availability zone: %w", err)
	}
	defer azOutput.Content.Close()

	az := string(azBytes)
	if len(az) == 0 {
		return nil, fmt.Errorf("could not get valid EC2 availability zone")
	}

	// Get region from AZ or use the provided region
	region := ""
	if len(az) >= 1 {
		// Remove the last character from AZ to get the region
		region = az[:len(az)-1]
	}

	if len(region) == 0 {
		if len(regionFromSession) != 0 {
			region = regionFromSession
		} else {
			return nil, fmt.Errorf("could not get valid EC2 Region")
		}
	}

	instanceInfo := Metadata{
		InstanceID:       instanceID,
		InstanceType:     instanceType,
		Region:           region,
		AvailabilityZone: az,
	}

	return &instanceInfo, nil
}
