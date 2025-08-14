/*
Copyright 2019 The Kubernetes Authors.

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
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/aws/aws-sdk-go-v2/feature/ec2/imds"
	"github.com/golang/mock/gomock"
	"github.com/kubernetes-sigs/aws-fsx-openzfs-csi-driver/pkg/cloud/mocks"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	k8s_testing "k8s.io/client-go/testing"
	"testing"
)

const (
	nodeName            = "ip-123-45-67-890.us-west-2.compute.internal"
	stdInstanceID       = "i-abcdefgh123456789"
	stdInstanceType     = "t2.medium"
	stdRegion           = "us-west-2"
	stdAvailabilityZone = "us-west-2b"
)

// Helper function to create a ReadCloser from a string
func createReadCloserFromString(s string) io.ReadCloser {
	return io.NopCloser(strings.NewReader(s))
}

func TestNewMetadataService(t *testing.T) {
	testCases := []struct {
		name                             string
		ec2metadataAvailable             bool
		clientsetReactors                func(*fake.Clientset)
		getInstanceIdentityDocumentValue imds.InstanceIdentityDocument
		getInstanceIdentityDocumentError error
		getMetadataOutput                *imds.GetMetadataOutput
		getMetadataError                 error
		invalidInstanceIdentityDocument  bool
		expectedErr                      error
		node                             v1.Node
		nodeNameEnvVar                   string
	}{
		{
			name:                 "success: normal",
			ec2metadataAvailable: true,
			getInstanceIdentityDocumentValue: imds.InstanceIdentityDocument{
				InstanceID:       stdInstanceID,
				InstanceType:     stdInstanceType,
				Region:           stdRegion,
				AvailabilityZone: stdAvailabilityZone,
			},
			getMetadataOutput: &imds.GetMetadataOutput{
				Content: createReadCloserFromString(stdInstanceID),
			},
			expectedErr: nil,
		},
		// TODO: Once topology is implemented, add test cases for kubernetes metadata
		{
			name:                 "failure: metadata not available, k8s client error",
			ec2metadataAvailable: false,
			clientsetReactors: func(clientset *fake.Clientset) {
				clientset.PrependReactor("get", "*", func(action k8s_testing.Action) (handled bool, ret runtime.Object, err error) {
					return true, nil, fmt.Errorf("client failure")
				})
			},
			getMetadataError: fmt.Errorf("IMDS not available"),
			expectedErr:      fmt.Errorf("error getting Node %s: client failure", nodeName),
			nodeNameEnvVar:   nodeName,
		},
		{
			name:                 "failure: metadata not available, node name env var not set",
			ec2metadataAvailable: false,
			getMetadataError:     fmt.Errorf("IMDS not available"),
			expectedErr:          fmt.Errorf("CSI_NODE_NAME env var not set"),
			nodeNameEnvVar:       "",
		},
		{
			name:                             "fail: GetInstanceIdentityDocument returned error",
			ec2metadataAvailable:             true,
			getInstanceIdentityDocumentError: fmt.Errorf("foo"),
			getMetadataError:                 fmt.Errorf("foo"),
			expectedErr:                      fmt.Errorf("error getting Node %s: nodes \"%s\" not found", nodeName, nodeName),
			nodeNameEnvVar:                   nodeName,
		},
		{
			name:                 "fail: GetInstanceIdentityDocument returned empty instance",
			ec2metadataAvailable: true,
			getInstanceIdentityDocumentValue: imds.InstanceIdentityDocument{
				InstanceID:       "",
				InstanceType:     stdInstanceType,
				Region:           stdRegion,
				AvailabilityZone: stdAvailabilityZone,
			},
			getMetadataOutput: &imds.GetMetadataOutput{
				Content: createReadCloserFromString(""),
			},
			invalidInstanceIdentityDocument: true,
			expectedErr:                     fmt.Errorf("could not get valid instance ID from IMDS"),
		},
		{
			name:                 "fail: GetInstanceIdentityDocument returned empty az",
			ec2metadataAvailable: true,
			getInstanceIdentityDocumentValue: imds.InstanceIdentityDocument{
				InstanceID:       stdInstanceID,
				InstanceType:     stdInstanceType,
				Region:           stdRegion,
				AvailabilityZone: "",
			},
			getMetadataOutput: &imds.GetMetadataOutput{
				Content: createReadCloserFromString(stdInstanceID),
			},
			invalidInstanceIdentityDocument: true,
			expectedErr:                     fmt.Errorf("could not get valid availability zone from IMDS"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			clientset := fake.NewSimpleClientset(&tc.node)
			clientsetInitialized := false
			if tc.clientsetReactors != nil {
				tc.clientsetReactors(clientset)
			}

			mockCtrl := gomock.NewController(t)
			mockIMDS := mocks.NewMockIMDS(mockCtrl)

			imdsClient := func() (IMDS, error) {
				return mockIMDS, nil
			}
			k8sAPIClient := func() (kubernetes.Interface, error) { clientsetInitialized = true; return clientset, nil }

			if tc.ec2metadataAvailable {
				// Mock GetMetadata for instance-id check (initial availability check)
				mockIMDS.EXPECT().GetMetadata(gomock.Any(), gomock.Eq(&imds.GetMetadataInput{Path: "instance-id"})).Return(tc.getMetadataOutput, tc.getMetadataError).AnyTimes()

				// If first call succeeds, expect GetInstanceIdentityDocument call
				if tc.getInstanceIdentityDocumentError == nil {
					docOutput := &imds.GetInstanceIdentityDocumentOutput{
						InstanceIdentityDocument: tc.getInstanceIdentityDocumentValue,
					}
					mockIMDS.EXPECT().GetInstanceIdentityDocument(gomock.Any(), gomock.Any()).Return(docOutput, nil).AnyTimes()
				}
			} else {
				// Simulate IMDS not being available by returning an error
				mockIMDS.EXPECT().GetMetadata(gomock.Any(), gomock.Any()).Return(nil, tc.getMetadataError).AnyTimes()
			}

			if tc.ec2metadataAvailable && tc.getInstanceIdentityDocumentError == nil && !tc.invalidInstanceIdentityDocument {
				if clientsetInitialized == true {
					t.Errorf("kubernetes client was unexpectedly initialized when metadata is available!")
					if len(clientset.Actions()) > 0 {
						t.Errorf("kubernetes client was unexpectedly called! %v", clientset.Actions())
					}
				}
			}

			err := os.Setenv("CSI_NODE_NAME", tc.nodeNameEnvVar)
			if err != nil {
				t.Errorf("failed to set CSI_NODE_NAME")
			}

			var m MetadataService
			m, err = NewMetadataService(imdsClient, k8sAPIClient, stdRegion)

			if err != nil {
				if tc.expectedErr == nil {
					t.Errorf("NewMetadataService() failed: got error %q, expected no error", err)
				} else if err.Error() != tc.expectedErr.Error() {
					t.Errorf("NewMetadataService() failed: got error %q, expected %q", err, tc.expectedErr)
				}
			} else {
				if m == nil {
					t.Fatalf("metadataService is unexpectedly nil!")
				}
				if m.GetInstanceID() != stdInstanceID {
					t.Errorf("GetInstanceID() failed: got wrong instance ID %v, expected %v", m.GetInstanceID(), stdInstanceID)
				}
				if m.GetInstanceType() != stdInstanceType {
					t.Errorf("GetInstanceType() failed: got wrong instance type %v, expected %v", m.GetInstanceType(), stdInstanceType)
				}
				if m.GetRegion() != stdRegion {
					t.Errorf("GetRegion() failed: got wrong region %v, expected %v", m.GetRegion(), stdRegion)
				}
				if m.GetAvailabilityZone() != stdAvailabilityZone {
					t.Errorf("GetAvailabilityZone() failed: got wrong AZ %v, expected %v", m.GetAvailabilityZone(), stdAvailabilityZone)
				}
			}
			mockCtrl.Finish()
		})
	}
}
