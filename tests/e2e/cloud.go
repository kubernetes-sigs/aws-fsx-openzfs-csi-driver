package e2e

import (
	"context"
	"errors"
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/fsx"
	"time"
)

type Cloud struct {
	FSx       fsx.FSx
	EC2client ec2.EC2
}

func NewCloud(region string) *Cloud {
	sess := session.Must(session.NewSession(&aws.Config{Region: aws.String(region)}))

	return &Cloud{
		FSx:       *fsx.New(sess),
		EC2client: *ec2.New(sess),
	}
}

func (c *Cloud) CreateFileSystem(ctx context.Context, input fsx.CreateFileSystemInput) (*fsx.FileSystem, error) {
	response, err := c.FSx.CreateFileSystemWithContext(ctx, &input)
	if err != nil {
		return nil, err
	}

	return response.FileSystem, nil
}

func (c *Cloud) DeleteFileSystem(ctx context.Context, input fsx.DeleteFileSystemInput) error {
	_, err := c.FSx.DeleteFileSystemWithContext(ctx, &input)
	return err
}

func (c *Cloud) CreateVolume(ctx context.Context, input fsx.CreateVolumeInput) (*fsx.Volume, error) {
	response, err := c.FSx.CreateVolumeWithContext(ctx, &input)
	if err != nil {
		return nil, err
	}

	return response.Volume, nil
}

func (c *Cloud) DeleteVolume(ctx context.Context, input fsx.DeleteVolumeInput) error {
	_, err := c.FSx.DeleteVolumeWithContext(ctx, &input)
	return err
}

func (c *Cloud) GetDNSName(ctx context.Context, filesystemId string) (string, error) {
	input := fsx.DescribeFileSystemsInput{
		FileSystemIds: []*string{&filesystemId},
	}

	response, err := c.FSx.DescribeFileSystemsWithContext(ctx, &input)
	if err != nil {
		return "", err
	}

	return *response.FileSystems[0].DNSName, nil
}

func (c *Cloud) GetNodeInstance(ctx context.Context, clusterName string) (*ec2.Instance, error) {
	request := &ec2.DescribeInstancesInput{
		Filters: []*ec2.Filter{
			{
				Name:   aws.String("tag:eks:cluster-name"),
				Values: []*string{aws.String(clusterName)},
			},
		},
	}

	var instances []*ec2.Instance
	response, err := c.EC2client.DescribeInstancesWithContext(ctx, request)
	if err != nil {
		return nil, err
	}
	for _, reservation := range response.Reservations {
		instances = append(instances, reservation.Instances...)
	}

	if len(instances) == 0 {
		return nil, fmt.Errorf("no instances in cluster %q found", clusterName)
	}

	return instances[0], nil
}

func (c *Cloud) GetSecurityGroupIds(node *ec2.Instance) []string {
	var groups []string
	for _, sg := range node.SecurityGroups {
		groups = append(groups, *sg.GroupId)
	}
	return groups
}

func (c *Cloud) WaitForFilesystemAvailable(ctx context.Context, filesystemId string) error {
	request := &fsx.DescribeFileSystemsInput{
		FileSystemIds: []*string{aws.String(filesystemId)},
	}

	timeout := 15 * time.Minute
	for start := time.Now(); time.Since(start) < timeout; time.Sleep(15 * time.Second) {
		response, err := c.FSx.DescribeFileSystemsWithContext(ctx, request)
		if err != nil {
			return err
		}

		if len(response.FileSystems) == 0 {
			return errors.New("no filesystem found")
		}

		if *response.FileSystems[0].Lifecycle == fsx.FileSystemLifecycleAvailable {
			return nil
		}
	}
	return errors.New("WaitForFilesystemAvailable timed out")
}

func (c *Cloud) WaitForVolumeAvailable(ctx context.Context, volumeId string) error {
	request := &fsx.DescribeVolumesInput{
		VolumeIds: []*string{aws.String(volumeId)},
	}

	timeout := 15 * time.Minute
	for start := time.Now(); time.Since(start) < timeout; time.Sleep(15 * time.Second) {
		response, err := c.FSx.DescribeVolumesWithContext(ctx, request)
		if err != nil {
			return err
		}

		if len(response.Volumes) == 0 {
			return errors.New("no volume found")
		}

		if *response.Volumes[0].Lifecycle == fsx.VolumeLifecycleAvailable {
			return nil
		}
	}
	return errors.New("WaitForVolumeAvailable timed out")
}
