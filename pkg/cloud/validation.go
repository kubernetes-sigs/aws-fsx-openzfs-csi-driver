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
	_ "unsafe" // Required for go:linkname

	"github.com/aws/aws-sdk-go-v2/service/fsx"
)

// AWS SDK v2 does not expose a client-side request validation function as done in v1.
// This function links to the applicable, unexported SDK v2 function which performs client-side validation
//
//go:linkname validateDeleteFileSystemInput github.com/aws/aws-sdk-go-v2/service/fsx.validateOpDeleteFileSystemInput
func validateDeleteFileSystemInput(v *fsx.DeleteFileSystemInput) error

// AWS SDK v2 does not expose a client-side request validation function as done in v1.
// This function links to the applicable, unexported SDK v2 function which performs client-side validation
//
//go:linkname validateDeleteVolumeInput github.com/aws/aws-sdk-go-v2/service/fsx.validateOpDeleteVolumeInput
func validateDeleteVolumeInput(v *fsx.DeleteVolumeInput) error
