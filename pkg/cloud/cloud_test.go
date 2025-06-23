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

package cloud

import (
	"context"
	"errors"

	"strconv"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/fsx"
	"github.com/aws/aws-sdk-go-v2/service/fsx/types"
	"github.com/golang/mock/gomock"
	"github.com/kubernetes-sigs/aws-fsx-openzfs-csi-driver/pkg/cloud/mocks"
)

func TestCreateFileSystem(t *testing.T) {
	var (
		fileSystemId    = aws.String("fs-1234")
		dnsName         = aws.String("https://aws.com")
		storageCapacity = aws.Int64(64)
		parameters      map[string]string
	)
	testCases := []struct {
		name     string
		testFunc func(t *testing.T)
	}{
		{
			name: "success: all variables",
			testFunc: func(t *testing.T) {
				mockCtl := gomock.NewController(t)
				mockFSx := mocks.NewMockFSx(mockCtl)
				c := &cloud{
					fsx: mockFSx,
				}

				output := &fsx.CreateFileSystemOutput{
					FileSystem: &types.FileSystem{
						DNSName:         dnsName,
						FileSystemId:    fileSystemId,
						StorageCapacity: aws.Int32(64),
					},
				}

				ctx := context.Background()
				mockFSx.EXPECT().CreateFileSystem(gomock.Eq(ctx), gomock.Any()).Return(output, nil)
				resp, err := c.CreateFileSystem(ctx, parameters)
				if err != nil {
					t.Fatalf("CreateFileSystem is failed: %v", err)
				}

				if resp == nil {
					t.Fatal("resp is nil")
				}

				if resp.DnsName != aws.ToString(dnsName) {
					t.Fatalf("DnsName mismatches. actual: %v expected: %v", resp.DnsName, dnsName)
				}

				if resp.FileSystemId != aws.ToString(fileSystemId) {
					t.Fatalf("FileSystemId mismatches. actual: %v expected: %v", resp.FileSystemId, fileSystemId)
				}

				if resp.StorageCapacity != 64 {
					t.Fatalf("StorageCapacity mismatches. actual: %v expected: %v", resp.StorageCapacity, 64)
				}

				mockCtl.Finish()
			},
		},
		{
			name: "fail: CreateFileSystem return IncompatibleParameterError",
			testFunc: func(t *testing.T) {
				mockCtl := gomock.NewController(t)
				mockFSx := mocks.NewMockFSx(mockCtl)
				c := &cloud{
					fsx: mockFSx,
				}

				ctx := context.Background()
				mockFSx.EXPECT().CreateFileSystem(gomock.Eq(ctx), gomock.Any()).Return(nil, &types.IncompatibleParameterError{})
				_, err := c.CreateFileSystem(ctx, parameters)
				if !errors.Is(err, ErrAlreadyExists) {
					t.Fatal("CreateVolume is not ErrAlreadyExists")
				}

				mockCtl.Finish()
			},
		},
	}

	for _, tc := range testCases {
		parameters = map[string]string{
			"ClientRequestToken": `"Test"`,
			"FileSystemType":     `"OPENZFS"`,
			"StorageCapacity":    strconv.FormatInt(*storageCapacity, 10),
			"SubnetIds":          `["test","test2"]`,
		}

		t.Run(tc.name, tc.testFunc)
	}
}
