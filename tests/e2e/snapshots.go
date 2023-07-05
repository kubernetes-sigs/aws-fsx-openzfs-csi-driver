package e2e

import (
	"context"
	"github.com/onsi/ginkgo/v2"
	v1 "k8s.io/api/core/v1"
	"k8s.io/kubernetes/test/e2e/framework"
	e2epv "k8s.io/kubernetes/test/e2e/framework/pv"
	"k8s.io/kubernetes/test/e2e/framework/skipper"
	e2evolume "k8s.io/kubernetes/test/e2e/framework/volume"
	storageframework "k8s.io/kubernetes/test/e2e/storage/framework"
	admissionapi "k8s.io/pod-security-admission/api"
)

var APIGroup = "snapshot.storage.k8s.io"

type snapshotsTestSuite struct {
	tsInfo storageframework.TestSuiteInfo
}

// InitCustomSnapshotsTestSuite returns snapshotsTestSuite that implements TestSuite interface
// using custom test patterns
func InitCustomSnapshotsTestSuite(patterns []storageframework.TestPattern) storageframework.TestSuite {
	return &snapshotsTestSuite{
		tsInfo: storageframework.TestSuiteInfo{
			Name:         "snapshots",
			TestPatterns: patterns,
			SupportedSizeRange: e2evolume.SizeRange{
				Min: "1Mi",
			},
			FeatureTag: "[Feature:VolumeSnapshotDataSource]",
		},
	}
}

// InitSnapshotsTestSuite returns snapshotsTestSuite that implements TestSuite interface\
// using test suite default patterns
func InitSnapshotsTestSuite() storageframework.TestSuite {
	patterns := []storageframework.TestPattern{
		storageframework.DynamicSnapshotDelete,
		storageframework.DynamicSnapshotRetain,
		storageframework.PreprovisionedSnapshotDelete,
		storageframework.PreprovisionedSnapshotRetain,
	}
	return InitCustomSnapshotsTestSuite(patterns)
}

func (s *snapshotsTestSuite) GetTestSuiteInfo() storageframework.TestSuiteInfo {
	return s.tsInfo
}

func (s *snapshotsTestSuite) SkipUnsupportedTests(driver storageframework.TestDriver, pattern storageframework.TestPattern) {
	_, ok := driver.(FSxTestDriver)
	if !ok {
		skipper.Skipf("Driver %q is not an FSx for OpenZFS Driver - skipping", driver.GetDriverInfo().Name)
	}
}

func (s *snapshotsTestSuite) DefineTests(driver storageframework.TestDriver, pattern storageframework.TestPattern) {
	var err error
	f := framework.NewFrameworkWithCustomTimeouts("snapshots", storageframework.GetDriverTimeouts(driver))
	f.NamespacePodSecurityEnforceLevel = admissionapi.LevelPrivileged

	ginkgo.It("should take a snapshot, and restore it to a volume", func(ctx context.Context) {
		fDriver, _ := driver.(FSxTestDriver)
		sDriver, _ := driver.(storageframework.SnapshottableTestDriver)
		dDriver, _ := driver.(storageframework.DynamicPVTestDriver)

		config := driver.PrepareTest(ctx, f)
		testConfig := storageframework.ConvertTestConfig(config)

		//Create an SC, PVC, and PV
		volumeResource := storageframework.CreateVolumeResource(ctx, dDriver, config, pattern, s.GetTestSuiteInfo().SupportedSizeRange)
		ginkgo.DeferCleanup(volumeResource.CleanupResource)

		//Write to the PVC
		tests := []e2evolume.Test{
			{
				Volume:          *volumeResource.VolSource,
				Mode:            pattern.VolMode,
				File:            "index.html",
				ExpectedContent: "Hello World!",
			},
		}
		e2evolume.InjectContent(ctx, f, testConfig, nil, "", tests)

		//Take snapshot
		snapshotResource := storageframework.CreateSnapshotResource(ctx, sDriver, config, pattern, volumeResource.Pvc.Name, volumeResource.Pvc.Namespace, f.Timeouts, map[string]string{} /* parameter */)
		ginkgo.DeferCleanup(snapshotResource.CleanupResource, f.Timeouts)

		//Restore it
		restoredSC := fDriver.GetRestoreSnapshotStorageClass(ctx)

		restoredPVC := e2epv.MakePersistentVolumeClaim(e2epv.PersistentVolumeClaimConfig{
			ClaimSize:        volumeResource.Pvc.Spec.Resources.Requests.Storage().String(),
			StorageClassName: &restoredSC.Name,
		}, f.Namespace.Name)

		restoredPVC.Spec.DataSource = &v1.TypedLocalObjectReference{
			APIGroup: &APIGroup,
			Kind:     "VolumeSnapshot",
			Name:     snapshotResource.Vscontent.GetName(),
		}

		restoredPVC, err = e2epv.CreatePVC(ctx, f.ClientSet, restoredPVC.Namespace, restoredPVC)
		framework.ExpectNoError(err)
		ginkgo.DeferCleanup(e2epv.DeletePersistentVolumeClaim, f.ClientSet, restoredPVC.Namespace, restoredPVC.Name)

		//Verify file is present
		tests = []e2evolume.Test{
			{
				Volume:          *volumeResource.VolSource,
				Mode:            pattern.VolMode,
				File:            "index.html",
				ExpectedContent: "Hello World!",
			},
		}
		e2evolume.TestVolumeClientSlow(ctx, f, testConfig, nil, "", tests)
	})
}
