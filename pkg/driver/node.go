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
	"context"
	"encoding/json"
	"fmt"
	"github.com/kubernetes-sigs/aws-fsx-openzfs-csi-driver/pkg/cloud"
	"github.com/kubernetes-sigs/aws-fsx-openzfs-csi-driver/pkg/driver/internal"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"os"
	"strings"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	corev1 "k8s.io/api/core/v1"
	k8stypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"
)

const (
	rootVolumePath = "fsx"
)

var (
	nodeCaps = []csi.NodeServiceCapability_RPC_Type{}
)

type nodeService struct {
	metadata      cloud.MetadataService
	mounter       Mounter
	inFlight      *internal.InFlight
	driverOptions *DriverOptions
}

// Struct for JSON patch operations
type JSONPatch struct {
	OP    string      `json:"op,omitempty"`
	Path  string      `json:"path,omitempty"`
	Value interface{} `json:"value"`
}

func newNodeService(driverOptions *DriverOptions) nodeService {
	klog.V(5).InfoS("[Debug] Retrieving node info from metadata service")
	region := os.Getenv("AWS_REGION")
	metadata, err := cloud.NewMetadataService(cloud.DefaultEC2MetadataClient, cloud.DefaultKubernetesAPIClient, region)
	if err != nil {
		panic(err)
	}
	klog.InfoS("regionFromSession Node service", "region", metadata.GetRegion())

	nodeMounter, err := newNodeMounter()
	if err != nil {
		panic(err)
	}

	// Remove taint from node to indicate driver startup success
	// This is done at the last possible moment to prevent race conditions or false positive removals
	err = removeNotReadyTaint(cloud.DefaultKubernetesAPIClient)
	if err != nil {
		klog.ErrorS(err, "Unexpected failure when attempting to remove node taint(s)")
	}

	return nodeService{
		metadata:      metadata,
		mounter:       nodeMounter,
		inFlight:      internal.NewInFlight(),
		driverOptions: driverOptions,
	}
}

func (d *nodeService) NodeStageVolume(ctx context.Context, req *csi.NodeStageVolumeRequest) (*csi.NodeStageVolumeResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

func (d *nodeService) NodeUnstageVolume(ctx context.Context, req *csi.NodeUnstageVolumeRequest) (*csi.NodeUnstageVolumeResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

// NodePublishVolume Mounts the PV at the target path.
func (d *nodeService) NodePublishVolume(ctx context.Context, req *csi.NodePublishVolumeRequest) (*csi.NodePublishVolumeResponse, error) {
	klog.V(4).Infof("NodePublishVolume: Called with args %+v", req)

	volumeID := req.GetVolumeId()
	if len(volumeID) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Volume ID missing in NodePublishVolumeRequest")
	}

	volCap := req.GetVolumeCapability()
	if volCap == nil {
		return nil, status.Error(codes.InvalidArgument, "Volume capability missing in NodePublishVolumeRequest")
	}
	if !isValidVolumeCapabilities([]*csi.VolumeCapability{volCap}) {
		return nil, status.Error(codes.InvalidArgument, "NodePublishVolume: Volume capability not supported")
	}

	if ok := d.inFlight.Insert(volumeID); !ok {
		return nil, status.Errorf(codes.Aborted, internal.VolumeOperationAlreadyExistsErrorMsg, volumeID)
	}
	defer func() {
		klog.V(4).InfoS("NodePublishVolume: volume operation finished", "volumeId", volumeID)
		d.inFlight.Delete(volumeID)
	}()

	context := req.GetVolumeContext()
	dnsName := context[volumeContextDnsName]
	volumePath := context[volumeContextVolumePath]
	resourceType := context[volumeContextResourceType]

	if len(dnsName) == 0 {
		return nil, status.Error(codes.InvalidArgument, "NodePublishVolume: dnsName is not provided")
	}

	if resourceType != fsType && resourceType != volType {
		return nil, status.Errorf(codes.InvalidArgument, "NodePublishVolume: resourceType %q is invalid", resourceType)
	}

	// If the volumePath is not provided and we are attempting to "mount" a file system, then we should mount the
	// root volume of the file system. On the other hand, if the volumePath is not provided and we are attempting
	// to mount an OpenZFS volume, throw an error.
	if len(volumePath) == 0 {
		if resourceType == fsType {
			volumePath = rootVolumePath
		} else {
			return nil, status.Error(codes.InvalidArgument, "NodePublishVolume: volumePath must be provided when mounting an OpenZFS volume")
		}
	}

	// If we are attempting to "mount" a file system, the mount path must be equal to the root volume path.
	if resourceType == fsType && volumePath != rootVolumePath {
		return nil, status.Error(codes.InvalidArgument, "NodePublishVolume: volumePath must match the root volume path when mounting an OpenZFS file system")
	}

	source := fmt.Sprintf("%s:%s", dnsName, volumePath)

	targetPath := req.GetTargetPath()
	if len(targetPath) == 0 {
		return nil, status.Error(codes.InvalidArgument, "NodePublishVolume: Target path not provided")
	}

	mountOptions := []string{}
	if req.GetReadonly() {
		mountOptions = append(mountOptions, "ro")
	}
	if m := volCap.GetMount(); m != nil {
		hasOption := func(options []string, opt string) bool {
			for _, o := range options {
				if o == opt {
					return true
				}
			}
			return false
		}
		for _, f := range m.MountFlags {
			if !hasOption(mountOptions, f) {
				mountOptions = append(mountOptions, f)
			}
		}
	}

	klog.V(5).Infof("NodePublishVolume: Creating dir %s", targetPath)
	if err := d.mounter.MakeDir(targetPath); err != nil {
		return nil, status.Errorf(codes.Internal, "NodePublishVolume: Could not create target dir %q: %v", targetPath, err)
	}

	// Check if the target directory is already mounted with a volume.
	mounted, err := d.isMounted(source, targetPath)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Could not check if %q is mounted: %v", targetPath, err)
	}

	if mounted {
		klog.V(5).Infof("NodePublishVolume: Target dir %q is already mounted with a volume. Not mounting volume source %q", targetPath, source)
	} else {
		klog.V(5).Infof("NodePublishVolume: Attempting to mount with volumeID(%v) source(%s) targetPath(%s) mountflags(%v)", volumeID, source, targetPath, mountOptions)
		err = d.mounter.Mount(source, targetPath, "nfs", mountOptions)
		if err != nil {
			if os.IsPermission(err) {
				return nil, status.Error(codes.PermissionDenied, err.Error())
			}
			if strings.Contains(err.Error(), "invalid argument") {
				return nil, status.Error(codes.InvalidArgument, err.Error())
			}
			return nil, status.Error(codes.Internal, err.Error())
		}
		klog.V(5).Infof("NodePublishVolume: Successfully mounted at target path %s", targetPath)
	}
	return &csi.NodePublishVolumeResponse{}, nil
}

// NodeUnpublishVolume Unmounts the volume from the target path
func (d *nodeService) NodeUnpublishVolume(ctx context.Context, req *csi.NodeUnpublishVolumeRequest) (*csi.NodeUnpublishVolumeResponse, error) {
	klog.V(4).Infof("NodeUnpublishVolume: Called with args %+v", req)

	volumeID := req.GetVolumeId()
	if len(volumeID) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Volume ID missing in NodeUnpublishVolumeRequest")
	}

	targetPath := req.GetTargetPath()
	if len(targetPath) == 0 {
		return nil, status.Error(codes.InvalidArgument, "NodeUnpublishVolume: Target path not provided")
	}

	if ok := d.inFlight.Insert(volumeID); !ok {
		return nil, status.Errorf(codes.Aborted, internal.VolumeOperationAlreadyExistsErrorMsg, volumeID)
	}
	defer func() {
		klog.V(4).InfoS("NodeUnpublishVolume: volume operation finished", "volumeId", volumeID)
		d.inFlight.Delete(volumeID)
	}()

	// Check if the target is mounted before unmounting
	notMnt, _ := d.mounter.IsLikelyNotMountPoint(targetPath)
	if notMnt {
		klog.V(5).Infof("NodeUnpublishVolume: Target path %s not mounted, skipping unmount", targetPath)
		return &csi.NodeUnpublishVolumeResponse{}, nil
	}

	klog.V(5).Infof("NodeUnpublishVolume: Unmounting %s", targetPath)
	err := d.mounter.Unmount(targetPath)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "NodeUnpublishVolume: Could not unmount %q: %v", targetPath, err)
	}

	err = os.RemoveAll(targetPath)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "NodeUnpublishVolume: Could not delete %q: %v", targetPath, err)
	}

	return &csi.NodeUnpublishVolumeResponse{}, nil
}

func (d *nodeService) NodeGetVolumeStats(ctx context.Context, req *csi.NodeGetVolumeStatsRequest) (*csi.NodeGetVolumeStatsResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

func (d *nodeService) NodeExpandVolume(ctx context.Context, req *csi.NodeExpandVolumeRequest) (*csi.NodeExpandVolumeResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

// NodeGetCapabilities Returns the capabilities of the Node plugin
func (d *nodeService) NodeGetCapabilities(ctx context.Context, req *csi.NodeGetCapabilitiesRequest) (*csi.NodeGetCapabilitiesResponse, error) {
	klog.V(4).Infof("NodeGetCapabilities: Called with args %+v", req)
	var caps []*csi.NodeServiceCapability
	for _, cap := range nodeCaps {
		c := &csi.NodeServiceCapability{
			Type: &csi.NodeServiceCapability_Rpc{
				Rpc: &csi.NodeServiceCapability_RPC{
					Type: cap,
				},
			},
		}
		caps = append(caps, c)
	}
	return &csi.NodeGetCapabilitiesResponse{Capabilities: caps}, nil
}

// NodeGetInfo Returns the id of the node on which the plugin is running
func (d *nodeService) NodeGetInfo(ctx context.Context, req *csi.NodeGetInfoRequest) (*csi.NodeGetInfoResponse, error) {
	klog.V(4).Infof("NodeGetInfo: called with args %+v", req)
	return &csi.NodeGetInfoResponse{
		NodeId: d.metadata.GetInstanceID(),
	}, nil
}

// isMounted Checks if volume is mounted. It does NOT return an error if the targetPath does not exist.
func (d *nodeService) isMounted(source string, targetPath string) (bool, error) {
	/*
		Checks if the targetPath a mount point using IsLikelyNotMountPoint.
		This function has three different return values:
		1. true, err when the directory does not exist or corrupted.
		2. false, nil when the path is already mounted with a device.
		3. true, nil when the path is not mounted with any device.
	*/
	klog.V(4).Infoln(targetPath)
	notMnt, err := d.mounter.IsLikelyNotMountPoint(targetPath)
	if err != nil && !os.IsNotExist(err) {
		// Checks if the path exists.
		// If the error is related to a corrupted mount, we can unmount then re-mount the volume.
		_, pathErr := d.mounter.PathExists(targetPath)
		if pathErr != nil && d.mounter.IsCorruptedMnt(pathErr) {
			klog.V(4).Infof("NodePublishVolume: Target path %q is a corrupted mount. Trying to unmount.", targetPath)
			if mntErr := d.mounter.Unmount(targetPath); mntErr != nil {
				return false, status.Errorf(codes.Internal, "NodePublishVolume: Unable to unmount the target %q : %v", targetPath, mntErr)
			}
			// After successful unmount, the device is ready to be mounted.
			return false, nil
		}
		return false, status.Errorf(codes.Internal, "NodePublishVolume: Could not check if %q is a mount point: %v, %v", targetPath, err, pathErr)
	}

	// Do not return os.IsNotExist error. Other errors were handled above. The
	// Existence of the target should be checked by the caller explicitly and
	// independently because sometimes prior to mount it is expected not to exist
	// (in Windows, the target must NOT exist before a symlink is created at it)
	// and in others it is an error (in Linux, the target mount directory must
	// exist before mount is called on it)
	if err != nil && os.IsNotExist(err) {
		klog.V(5).Infof("[Debug] NodePublishVolume: Target path %q does not exist", targetPath)
		return false, nil
	}

	if !notMnt {
		klog.V(4).Infof("NodePublishVolume: Target path %q is already mounted", targetPath)
	}

	return !notMnt, nil
}

// removeNotReadyTaint removes the taint fsx.openzfs.csi.aws.com/agent-not-ready from the local node
// This taint can be optionally applied by users to prevent startup race conditions such as
// https://github.com/kubernetes/kubernetes/issues/95911
func removeNotReadyTaint(k8sClient cloud.KubernetesAPIClient) error {
	nodeName := os.Getenv("CSI_NODE_NAME")
	if nodeName == "" {
		klog.V(4).InfoS("CSI_NODE_NAME missing, skipping taint removal")
		return nil
	}

	clientset, err := k8sClient()
	if err != nil {
		klog.V(4).InfoS("Failed to communicate with k8s API, skipping taint removal")
		return nil
	}

	node, err := clientset.CoreV1().Nodes().Get(context.Background(), nodeName, metav1.GetOptions{})
	if err != nil {
		return err
	}

	var taintsToKeep []corev1.Taint
	for _, taint := range node.Spec.Taints {
		if taint.Key != AgentNotReadyNodeTaintKey {
			taintsToKeep = append(taintsToKeep, taint)
		} else {
			klog.V(4).InfoS("Queued taint for removal", "key", taint.Key, "effect", taint.Effect)
		}
	}

	if len(taintsToKeep) == len(node.Spec.Taints) {
		klog.V(4).InfoS("No taints to remove on node, skipping taint removal")
		return nil
	}

	patchRemoveTaints := []JSONPatch{
		{
			OP:    "test",
			Path:  "/spec/taints",
			Value: node.Spec.Taints,
		},
		{
			OP:    "replace",
			Path:  "/spec/taints",
			Value: taintsToKeep,
		},
	}

	patch, err := json.Marshal(patchRemoveTaints)
	if err != nil {
		return err
	}

	_, err = clientset.CoreV1().Nodes().Patch(context.Background(), nodeName, k8stypes.JSONPatchType, patch, metav1.PatchOptions{})
	if err != nil {
		return err
	}
	klog.InfoS("Removed taint(s) from local node", "node", nodeName)
	return nil
}
