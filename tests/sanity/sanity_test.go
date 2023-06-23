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

package sanity

import (
	"github.com/kubernetes-sigs/aws-fsx-openzfs-csi-driver/pkg/util"
	"os"
	"strings"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/kubernetes-csi/csi-test/v5/pkg/sanity"

	"github.com/kubernetes-sigs/aws-fsx-openzfs-csi-driver/pkg/driver"
)

var (
	socket   = os.TempDir() + "/csi-socket"
	endpoint = "unix://" + socket
)

var fsxOpenZfsDriver *driver.Driver

func TestSanity(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Sanity Tests Suite")
}

var _ = BeforeSuite(func() {
	fsxOpenZfsDriver = driver.NewFakeDriver(endpoint)
	go func() {
		Expect(fsxOpenZfsDriver.Run()).NotTo(HaveOccurred())
	}()
})

var _ = AfterSuite(func() {
	fsxOpenZfsDriver.Stop()
	Expect(os.RemoveAll(socket)).NotTo(HaveOccurred())
})

var _ = Describe("AWS FSx for OpenZFS CSI Driver", func() {
	Context("Filesystem", Ordered, func() {
		config := sanity.NewTestConfig()
		config.Address = endpoint
		config.TestVolumeParameters = map[string]string{
			"ResourceType":              "filesystem",
			"SkipFinalBackupOnDeletion": "true",
		}

		BeforeAll(func() {
			fsxOpenZfsDriver.ResetCloud()
		})

		Describe("CSI sanity", func() {
			BeforeEach(func() {
				switch {
				case strings.Contains(CurrentSpecReport().FullText(), "should create volume from an existing source snapshot"), //FileSystems don't support CreateVolume from snapshot
					strings.Contains(CurrentSpecReport().FullText(), "should fail when the volume source snapshot is not found"): //FileSystems don't support CreateVolume from snapshot
					Skip("This test produces a false negative")
				}
			})
			sanity.GinkgoTest(&config)
		})
	})

	Context("Volume", Ordered, func() {
		config := sanity.NewTestConfig()
		config.Address = endpoint
		config.TestVolumeParameters = map[string]string{
			"ResourceType": "volume",
		}
		config.TestVolumeSize = util.GiBToBytes(1)

		BeforeAll(func() {
			fsxOpenZfsDriver.ResetCloud()
		})

		Describe("CSI sanity", func() {
			BeforeEach(func() {
				switch {
				case strings.Contains(CurrentSpecReport().FullText(), "ExpandVolume"), //Volumes don't support ExpandVolume
					strings.Contains(CurrentSpecReport().FullText(), "should fail when requesting to create a volume with already existing name and different capacity"): //Volumes don't support ExpandVolume
					Skip("This test produces a false negative")
				}
			})
			sanity.GinkgoTest(&config)
		})
	})
})
