package e2e

import (
	"context"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/fsx"
	"github.com/kubernetes-sigs/aws-fsx-openzfs-csi-driver/pkg/driver"
	v1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apiserver/pkg/storage/names"
	"k8s.io/kubernetes/test/e2e/framework"
	"k8s.io/kubernetes/test/e2e/framework/skipper"
	e2evolume "k8s.io/kubernetes/test/e2e/framework/volume"
	storageframework "k8s.io/kubernetes/test/e2e/storage/framework"
	"k8s.io/kubernetes/test/e2e/storage/utils"
	"time"
)

const (
	RESOURCETYPE            = "ResourceType"
	RESOURCETYPE_FILESYSTEM = "filesystem"
	RESOURCETYPE_VOLUME     = "volume"
)

type FSxTestDriver interface {
	storageframework.TestDriver
	GetRestoreSnapshotStorageClass(ctx context.Context) *storagev1.StorageClass
}

type fsxDriver struct {
	driverInfo                storageframework.DriverInfo
	parameters                map[string]string
	restoreSnapshotParameters map[string]string
	snapshotParameters        map[string]string
	createInput               any
	deleteInput               any
}

var _ FSxTestDriver = &fsxDriver{}
var _ storageframework.TestDriver = &fsxDriver{}
var _ storageframework.PreprovisionedVolumeTestDriver = &fsxDriver{}
var _ storageframework.PreprovisionedPVTestDriver = &fsxDriver{}
var _ storageframework.DynamicPVTestDriver = &fsxDriver{}
var _ storageframework.CustomTimeoutsTestDriver = &fsxDriver{}

type FSxResource struct {
	deleteInput any
}

type FSxFilesystem struct {
	fsxResource  FSxResource
	filesystemId string
	dnsName      string
	RootVolumeId string
}

type FSxVolume struct {
	fsxResource FSxResource
	volumeId    string
	dnsName     string
	volumePath  string
}

func InitFSxCSIDriver(parameters map[string]string, restoreSnapshotParameters map[string]string, snapshotParameters map[string]string, createInput any, deleteInput any) storageframework.TestDriver {
	var min string
	var max string
	if parameters[RESOURCETYPE] == RESOURCETYPE_FILESYSTEM {
		min = "64Gi"
		max = "512Ti"
	} else if parameters[RESOURCETYPE] == RESOURCETYPE_VOLUME {
		min = "1Gi"
		max = "1Gi"
	}

	return &fsxDriver{
		driverInfo: storageframework.DriverInfo{
			Name:            driver.DriverName,
			SupportedFsType: sets.NewString(""),
			SupportedSizeRange: e2evolume.SizeRange{
				Min: min,
				Max: max,
			},
			Capabilities: map[storageframework.Capability]bool{
				storageframework.CapPersistence: true,
				storageframework.CapExec:        true,
				storageframework.CapMultiPODs:   true,
				storageframework.CapRWX:         true,
				//storageframework.CapSnapshotDataSource:  true, //Base TestSuites doesn't allow for different storage class parameters during restore, therefore skipping in favor of custom suite
				storageframework.CapControllerExpansion: true,
			},
		},
		parameters:                parameters,
		restoreSnapshotParameters: restoreSnapshotParameters,
		snapshotParameters:        snapshotParameters,
		createInput:               createInput,
		deleteInput:               deleteInput,
	}
}

func (d *fsxDriver) GetDriverInfo() *storageframework.DriverInfo {
	return &d.driverInfo
}

func (d *fsxDriver) SkipUnsupportedTest(pattern storageframework.TestPattern) {
	if d.parameters[RESOURCETYPE] == RESOURCETYPE_VOLUME {
		if pattern.AllowExpansion != false {
			skipper.Skipf("FSx for OpenZFS Volumes does not support volume expansion -- skipping")
		}
	}
}

func (d *fsxDriver) PrepareTest(ctx context.Context, f *framework.Framework) *storageframework.PerTestConfig {
	return &storageframework.PerTestConfig{
		Driver:    d,
		Prefix:    "fsx-openzfs",
		Framework: f,
	}
}

func (d *fsxDriver) CreateVolume(ctx context.Context, config *storageframework.PerTestConfig, volType storageframework.TestVolType) storageframework.TestVolume {
	c := NewCloud(Region)
	if d.parameters[RESOURCETYPE] == RESOURCETYPE_FILESYSTEM {
		response, err := c.CreateFileSystem(ctx, d.createInput.(fsx.CreateFileSystemInput))
		framework.ExpectNoError(err, "creating filesystem")
		err = c.WaitForFilesystemAvailable(ctx, aws.ToString(response.FileSystemId))
		framework.ExpectNoError(err, "creating filesystem")
		return &FSxFilesystem{
			filesystemId: aws.ToString(response.FileSystemId),
			dnsName:      aws.ToString(response.DNSName),
			fsxResource: FSxResource{
				deleteInput: d.deleteInput,
			},
		}
	} else if d.parameters[RESOURCETYPE] == RESOURCETYPE_VOLUME {
		input := d.createInput.(fsx.CreateVolumeInput)
		input.Name = aws.String(names.SimpleNameGenerator.GenerateName("volume"))
		response, err := c.CreateVolume(ctx, input)
		framework.ExpectNoError(err, "creating volume")
		err = c.WaitForVolumeAvailable(ctx, aws.ToString(response.VolumeId))
		framework.ExpectNoError(err, "creating volume")
		dnsName, err := c.GetDNSName(ctx, aws.ToString(response.FileSystemId))
		framework.ExpectNoError(err, "getting dnsName")
		return &FSxVolume{
			volumeId:   aws.ToString(response.VolumeId),
			dnsName:    dnsName,
			volumePath: aws.ToString(response.OpenZFSConfiguration.VolumePath),
			fsxResource: FSxResource{
				deleteInput: d.deleteInput,
			},
		}
	}
	return nil
}

func (d *fsxDriver) GetPersistentVolumeSource(readOnly bool, fsType string, volume storageframework.TestVolume) (*v1.PersistentVolumeSource, *v1.VolumeNodeAffinity) {
	if d.parameters[RESOURCETYPE] == RESOURCETYPE_FILESYSTEM {
		nv, _ := volume.(*FSxFilesystem)
		return &v1.PersistentVolumeSource{
			CSI: &v1.CSIPersistentVolumeSource{
				Driver:       d.driverInfo.Name,
				VolumeHandle: nv.filesystemId,
				ReadOnly:     readOnly,
				FSType:       fsType,
				VolumeAttributes: map[string]string{
					"DNSName":      nv.dnsName,
					"ResourceType": RESOURCETYPE_FILESYSTEM,
				},
			},
		}, nil
	} else if d.parameters[RESOURCETYPE] == RESOURCETYPE_VOLUME {
		nv, _ := volume.(*FSxVolume)
		return &v1.PersistentVolumeSource{
			CSI: &v1.CSIPersistentVolumeSource{
				Driver:       d.driverInfo.Name,
				VolumeHandle: nv.volumeId,
				ReadOnly:     readOnly,
				FSType:       fsType,
				VolumeAttributes: map[string]string{
					"DNSName":      nv.dnsName,
					"VolumePath":   nv.volumePath,
					"ResourceType": RESOURCETYPE_VOLUME,
				},
			},
		}, nil
	}
	return nil, nil
}

func (d *fsxDriver) GetDynamicProvisionStorageClass(ctx context.Context, config *storageframework.PerTestConfig, fsType string) *storagev1.StorageClass {
	defaultBindingMode := storagev1.VolumeBindingImmediate
	return &storagev1.StorageClass{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: fmt.Sprintf("fsx-openzfs-csi-dynamic-sc-test1234-"),
		},
		Provisioner:       driver.DriverName,
		Parameters:        d.parameters,
		VolumeBindingMode: &defaultBindingMode,
	}
}

func (d *fsxDriver) GetSnapshotClass(ctx context.Context, config *storageframework.PerTestConfig, parameters map[string]string) *unstructured.Unstructured {
	//Add test defined parameters
	for key, value := range d.snapshotParameters {
		parameters[key] = value
	}
	return utils.GenerateSnapshotClassSpec(driver.DriverName, parameters, config.Framework.Namespace.Name)
}

func (d *fsxDriver) GetTimeouts() *framework.TimeoutContext {
	timeoutContext := framework.NewTimeoutContext()
	timeoutContext.ClaimProvision = time.Minute * 15
	timeoutContext.PodStart = time.Minute * 15
	return timeoutContext
}

func (f *FSxFilesystem) DeleteVolume(ctx context.Context) {
	c := NewCloud(Region)
	input := f.fsxResource.deleteInput.(fsx.DeleteFileSystemInput)
	input.FileSystemId = aws.String(f.filesystemId)
	err := c.DeleteFileSystem(ctx, input)
	framework.ExpectNoError(err, "deleting filesystem")
}

func (v *FSxVolume) DeleteVolume(ctx context.Context) {
	c := NewCloud(Region)
	input := v.fsxResource.deleteInput.(fsx.DeleteVolumeInput)
	input.VolumeId = aws.String(v.volumeId)
	err := c.DeleteVolume(ctx, input)
	framework.ExpectNoError(err, "deleting volume")
}

func (d *fsxDriver) GetRestoreSnapshotStorageClass(ctx context.Context) *storagev1.StorageClass {
	return &storagev1.StorageClass{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: fmt.Sprintf("fsx-openzfs-csi-restore-snapshot-sc-test1234-"),
		},
		Provisioner: driver.DriverName,
		Parameters:  d.restoreSnapshotParameters,
	}
}
