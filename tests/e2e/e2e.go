package e2e

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/fsx"
	. "github.com/onsi/ginkgo/v2"
	_ "github.com/onsi/gomega"
	"k8s.io/kubernetes/test/e2e/framework"
	storageframework "k8s.io/kubernetes/test/e2e/storage/framework"
	"k8s.io/kubernetes/test/e2e/storage/testsuites"
	"strconv"
	"strings"
)

const (
	CSIDriverE2ETagKey = "OpenZFSCSIDriverE2E"
)

var CustomSuites = []func() storageframework.TestSuite{
	InitSnapshotsTestSuite,
}

var (
	defaultSuites = append(testsuites.BaseSuites, append(testsuites.CSISuites, CustomSuites...)...)
)

var (
	ClusterName      string
	Region           string
	PullRequest      string
	subnetIds        []string
	securityGroupIds []string
	filesystem       *fsx.FileSystem
)

var _ = BeforeSuite(func() {
	c := NewCloud(Region)

	input := fsx.DeleteFileSystemInput{
		FileSystemId: filesystem.FileSystemId,
		OpenZFSConfiguration: &fsx.DeleteFileSystemOpenZFSConfiguration{
			Options:         []*string{aws.String(fsx.DeleteFileSystemOpenZFSOptionDeleteChildVolumesAndSnapshots)},
			SkipFinalBackup: aws.Bool(true)},
	}
	DeferCleanup(c.DeleteFileSystem, context.Background(), input)

	err := c.WaitForFilesystemAvailable(context.Background(), *filesystem.FileSystemId)
	framework.ExpectNoError(err)
})

var _ = Describe("FSx for OpenZFS Filesystem", func() {
	Context("Default Suites", func() {
		BeforeEach(func() {
			switch {
			case strings.Contains(CurrentSpecReport().FullText(), "should provision storage with any volume data source [Serial]"):
				Skip("This test produces a false negative")
			}
		})

		subnetIdsJson, _ := json.Marshal(subnetIds)
		securityGroupIdsJson, _ := json.Marshal(securityGroupIds)

		dynamicParameters := getDefaultFilesystemParameters(string(subnetIdsJson), string(securityGroupIdsJson))
		staticCreateInput := getDefaultCreateFilesystemInput(subnetIds, securityGroupIds)
		driver := InitFSxCSIDriver(dynamicParameters, getDefaultRestoreSnapshotParameters(), getDefaultSnapshotParameters(), staticCreateInput, getDefaultDeleteFilesystemInput())

		storageframework.DefineTestSuites(driver, defaultSuites)
	})
})

var _ = Describe("FSx for OpenZFS Volumes", func() {
	Context("Default Suites", func() {
		BeforeEach(func() {
			switch {
			case strings.Contains(CurrentSpecReport().FullText(), "should provision storage with any volume data source [Serial]"):
				Skip("This test produces a false negative")
			}
		})

		dynamicParameters := getDefaultVolumeParameters(strconv.Quote(*filesystem.OpenZFSConfiguration.RootVolumeId))
		staticCreateInput := getDefaultCreateVolumeInput(*filesystem.OpenZFSConfiguration.RootVolumeId)
		driver := InitFSxCSIDriver(dynamicParameters, getDefaultRestoreSnapshotParameters(), getDefaultSnapshotParameters(), staticCreateInput, getDefaultDeleteVolumeInput())

		storageframework.DefineTestSuites(driver, defaultSuites)
	})
})

func getDefaultFilesystemParameters(subnetIds string, securityGroupIds string) map[string]string {
	return map[string]string{
		RESOURCETYPE:                RESOURCETYPE_FILESYSTEM,
		"DeploymentType":            `"SINGLE_AZ_1"`,
		"ThroughputCapacity":        `64`,
		"SubnetIds":                 subnetIds,
		"SkipFinalBackupOnDeletion": `true`,
		"SecurityGroupIds":          securityGroupIds,
		"Tags":                      fmt.Sprintf(`[{"Key": "%s", "Value": "%s"}]`, CSIDriverE2ETagKey, PullRequest),
	}
}

func getDefaultCreateFilesystemInput(subnetIds []string, securityGroupIds []string) fsx.CreateFileSystemInput {
	return fsx.CreateFileSystemInput{
		FileSystemType: aws.String("OPENZFS"),
		OpenZFSConfiguration: &fsx.CreateFileSystemOpenZFSConfiguration{
			DeploymentType:     aws.String("SINGLE_AZ_1"),
			ThroughputCapacity: aws.Int64(64),
		},
		SecurityGroupIds: aws.StringSlice(securityGroupIds),
		StorageCapacity:  aws.Int64(64),
		SubnetIds:        aws.StringSlice(subnetIds),
		Tags: []*fsx.Tag{
			{
				Key:   aws.String(CSIDriverE2ETagKey),
				Value: aws.String(PullRequest),
			},
		},
	}
}

func getDefaultDeleteFilesystemInput() fsx.DeleteFileSystemInput {
	return fsx.DeleteFileSystemInput{
		//FileSystemId is required but is unknown, therefore it is defined in driver.go
		OpenZFSConfiguration: &fsx.DeleteFileSystemOpenZFSConfiguration{
			SkipFinalBackup: aws.Bool(true),
		},
	}
}

func getDefaultVolumeParameters(parentVolumeId string) map[string]string {
	return map[string]string{
		RESOURCETYPE:     RESOURCETYPE_VOLUME,
		"ParentVolumeId": parentVolumeId,
		"Tags":           fmt.Sprintf(`[{"Key": "%s", "Value": "%s"}]`, CSIDriverE2ETagKey, PullRequest),
	}
}

func getDefaultCreateVolumeInput(parentVolumeId string) fsx.CreateVolumeInput {
	return fsx.CreateVolumeInput{
		//Name is required and must be unique, therefore it is defined in driver.go
		OpenZFSConfiguration: &fsx.CreateOpenZFSVolumeConfiguration{
			ParentVolumeId: &parentVolumeId,
		},
		VolumeType: aws.String("OPENZFS"),
		Tags: []*fsx.Tag{
			{
				Key:   aws.String(CSIDriverE2ETagKey),
				Value: aws.String(PullRequest),
			},
		},
	}
}

func getDefaultDeleteVolumeInput() fsx.DeleteVolumeInput {
	return fsx.DeleteVolumeInput{
		//VolumeId is required but is unknown, therefore it is defined in driver.go
	}
}

func getDefaultSnapshotParameters() map[string]string {
	return map[string]string{
		"Tags": fmt.Sprintf(`[{"Key": "%s", "Value": "%s"}]`, CSIDriverE2ETagKey, PullRequest),
	}
}

func getDefaultRestoreSnapshotParameters() map[string]string {
	return map[string]string{
		"ResourceType":   "volume",
		"OriginSnapshot": `{"CopyStrategy": "CLONE"}`,
		"ParentVolumeId": strconv.Quote(*filesystem.OpenZFSConfiguration.RootVolumeId),
		"Tags":           fmt.Sprintf(`[{"Key": "%s", "Value": "%s"}]`, CSIDriverE2ETagKey, PullRequest),
	}
}
