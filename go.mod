module sigs.k8s.io/aws-fsx-openzfs-csi-driver

go 1.20

require (
	github.com/aws/aws-sdk-go v1.44.264
	github.com/container-storage-interface/spec v1.7.0
	github.com/golang/mock v1.6.0
	github.com/kubernetes-csi/csi-test/v5 v5.0.0
	github.com/onsi/ginkgo/v2 v2.11.0
	github.com/onsi/gomega v1.27.8
	google.golang.org/grpc v1.48.0
	google.golang.org/protobuf v1.28.1
	k8s.io/apimachinery v0.27.1
	k8s.io/klog/v2 v2.80.1
	k8s.io/mount-utils v0.26.0
)

require (
	github.com/go-logr/logr v1.2.4 // indirect
	github.com/go-task/slim-sprig v0.0.0-20230315185526-52ccab3ef572 // indirect
	github.com/golang/protobuf v1.5.3 // indirect
	github.com/google/go-cmp v0.5.9 // indirect
	github.com/google/pprof v0.0.0-20210407192527-94a9f03dee38 // indirect
	github.com/google/uuid v1.3.0 // indirect
	github.com/jmespath/go-jmespath v0.4.0 // indirect
	golang.org/x/net v0.10.0 // indirect
	golang.org/x/sys v0.9.0 // indirect
	golang.org/x/text v0.9.0 // indirect
	golang.org/x/tools v0.9.3 // indirect
	google.golang.org/genproto v0.0.0-20201209185603-f92720507ed4 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
	k8s.io/utils v0.0.0-20221107191617-1a15be271d1d // indirect
)

replace k8s.io/api => k8s.io/api v0.24.0

replace k8s.io/apiextensions-apiserver => k8s.io/apiextensions-apiserver v0.24.0

replace k8s.io/apimachinery => k8s.io/apimachinery v0.24.5-rc.0

replace k8s.io/apiserver => k8s.io/apiserver v0.24.0

replace k8s.io/cli-runtime => k8s.io/cli-runtime v0.24.0

replace k8s.io/client-go => k8s.io/client-go v0.24.0

replace k8s.io/cloud-provider => k8s.io/cloud-provider v0.24.0

replace k8s.io/cluster-bootstrap => k8s.io/cluster-bootstrap v0.24.0

replace k8s.io/code-generator => k8s.io/code-generator v0.24.7-rc.0

replace k8s.io/component-base => k8s.io/component-base v0.24.0

replace k8s.io/component-helpers => k8s.io/component-helpers v0.24.0

replace k8s.io/controller-manager => k8s.io/controller-manager v0.24.0

replace k8s.io/cri-api => k8s.io/cri-api v0.25.0-alpha.0

replace k8s.io/csi-translation-lib => k8s.io/csi-translation-lib v0.24.0

replace k8s.io/kube-aggregator => k8s.io/kube-aggregator v0.24.0

replace k8s.io/kube-controller-manager => k8s.io/kube-controller-manager v0.24.0

replace k8s.io/kube-proxy => k8s.io/kube-proxy v0.24.0

replace k8s.io/kube-scheduler => k8s.io/kube-scheduler v0.24.0

replace k8s.io/kubectl => k8s.io/kubectl v0.24.0

replace k8s.io/kubelet => k8s.io/kubelet v0.24.0

replace k8s.io/legacy-cloud-providers => k8s.io/legacy-cloud-providers v0.24.0

replace k8s.io/metrics => k8s.io/metrics v0.24.0

replace k8s.io/mount-utils => k8s.io/mount-utils v0.24.7-rc.0

replace k8s.io/sample-apiserver => k8s.io/sample-apiserver v0.24.0

replace k8s.io/sample-cli-plugin => k8s.io/sample-cli-plugin v0.24.0

replace k8s.io/sample-controller => k8s.io/sample-controller v0.24.0

replace k8s.io/pod-security-admission => k8s.io/pod-security-admission v0.24.0

replace k8s.io/dynamic-resource-allocation => k8s.io/dynamic-resource-allocation v0.26.0

replace k8s.io/kms => k8s.io/kms v0.26.2-rc.0
