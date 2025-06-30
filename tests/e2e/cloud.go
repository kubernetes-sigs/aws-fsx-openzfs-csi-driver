package e2e

import (
	"context"
	"errors"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/fsx"
	fsxtypes "github.com/aws/aws-sdk-go-v2/service/fsx/types"
	"time"
)

type Cloud struct {
	FSx       *fsx.Client
	EC2client *ec2.Client
}

func NewCloud(region string) *Cloud {
	cfg, err := config.LoadDefaultConfig(context.TODO(), config.WithRegion(region))
	if err != nil {
		panic(err)
	}

	return &Cloud{
		FSx:       fsx.NewFromConfig(cfg),
		EC2client: ec2.NewFromConfig(cfg),
	}
}

func (c *Cloud) CreateFileSystem(ctx context.Context, input fsx.CreateFileSystemInput) (*fsxtypes.FileSystem, error) {
	response, err := c.FSx.CreateFileSystem(ctx, &input)
	if err != nil {
		return nil, err
	}

	return response.FileSystem, nil
}

func (c *Cloud) DeleteFileSystem(ctx context.Context, input fsx.DeleteFileSystemInput) error {
	_, err := c.FSx.DeleteFileSystem(ctx, &input)
	return err
}

func (c *Cloud) CreateVolume(ctx context.Context, input fsx.CreateVolumeInput) (*fsxtypes.Volume, error) {
	response, err := c.FSx.CreateVolume(ctx, &input)
	if err != nil {
		return nil, err
	}

	return response.Volume, nil
}

func (c *Cloud) DeleteVolume(ctx context.Context, input fsx.DeleteVolumeInput) error {
	_, err := c.FSx.DeleteVolume(ctx, &input)
	return err
}

func (c *Cloud) GetDNSName(ctx context.Context, filesystemId string) (string, error) {
	input := fsx.DescribeFileSystemsInput{
		FileSystemIds: []string{filesystemId},
	}

	response, err := c.FSx.DescribeFileSystems(ctx, &input)
	if err != nil {
		return "", err
	}

	return aws.ToString(response.FileSystems[0].DNSName), nil
}

func (c *Cloud) GetNodeInstance(ctx context.Context, clusterName string) (*types.Instance, error) {
	request := &ec2.DescribeInstancesInput{
		Filters: []types.Filter{
			{
				Name:   aws.String("tag:eks:cluster-name"),
				Values: []string{clusterName},
			},
		},
	}

	var instances []types.Instance
	response, err := c.EC2client.DescribeInstances(ctx, request)
	if err != nil {
		return nil, err
	}
	for _, reservation := range response.Reservations {
		instances = append(instances, reservation.Instances...)
	}

	if len(instances) == 0 {
		return nil, fmt.Errorf("no instances in cluster %q found", clusterName)
	}

	return &instances[0], nil
}

func (c *Cloud) GetSecurityGroupIds(node *types.Instance) []string {
	var groups []string
	for _, sg := range node.SecurityGroups {
		groups = append(groups, aws.ToString(sg.GroupId))
	}
	return groups
}

func (c *Cloud) WaitForFilesystemAvailable(ctx context.Context, filesystemId string) error {
	request := &fsx.DescribeFileSystemsInput{
		FileSystemIds: []string{filesystemId},
	}

	timeout := 15 * time.Minute
	for start := time.Now(); time.Since(start) < timeout; time.Sleep(15 * time.Second) {
		response, err := c.FSx.DescribeFileSystems(ctx, request)
		if err != nil {
			return err
		}

		if len(response.FileSystems) == 0 {
			return errors.New("no filesystem found")
		}

		if response.FileSystems[0].Lifecycle == fsxtypes.FileSystemLifecycleAvailable {
			return nil
		}
	}
	return errors.New("WaitForFilesystemAvailable timed out")
}

func (c *Cloud) WaitForVolumeAvailable(ctx context.Context, volumeId string) error {
	request := &fsx.DescribeVolumesInput{
		VolumeIds: []string{volumeId},
	}

	timeout := 15 * time.Minute
	for start := time.Now(); time.Since(start) < timeout; time.Sleep(15 * time.Second) {
		response, err := c.FSx.DescribeVolumes(ctx, request)
		if err != nil {
			return err
		}

		if len(response.Volumes) == 0 {
			return errors.New("no volume found")
		}

		if response.Volumes[0].Lifecycle == fsxtypes.VolumeLifecycleAvailable {
			return nil
		}
	}
	return errors.New("WaitForVolumeAvailable timed out")
}
