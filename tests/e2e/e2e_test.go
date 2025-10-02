package e2e

import (
	"flag"
	"k8s.io/kubernetes/test/e2e/framework"
	"k8s.io/kubernetes/test/e2e/framework/testfiles"
	"os"
	"path/filepath"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

const (
	kubeconfigEnvVar = "KUBECONFIG"
)

func init() {
	testing.Init()

	if os.Getenv(kubeconfigEnvVar) == "" {
		kubeconfig := filepath.Join(os.Getenv("HOME"), ".kube", "config")
		os.Setenv(kubeconfigEnvVar, kubeconfig)
	}

	testfiles.AddFileSource(testfiles.RootFileSource{Root: "../.."})
	framework.AfterReadingAllFlags(&framework.TestContext)

	framework.RegisterCommonFlags(flag.CommandLine)
	framework.RegisterClusterFlags(flag.CommandLine)

	flag.StringVar(&ClusterName, "cluster-name", "fsx-openzfs-csi-cluster", "the eks cluster name")
	flag.StringVar(&Region, "region", "us-east-1", "the aws region")
	flag.StringVar(&PullRequest, "pull-request", "local", "the associated pull request number if present")

	flag.Parse()
}

func TestFSxCSI(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "AWS FSx OpenZFS CSI Driver End-to-End Tests")
}
