# Copyright 2023 The Kubernetes Authors.
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

VERSION=0.1.0

PKG=github.com/kubernetes-sigs/aws-fsx-openzfs-csi-driver
GIT_COMMIT?=$(shell git rev-parse HEAD)
BUILD_DATE?=$(shell date -u +"%Y-%m-%dT%H:%M:%SZ")

LDFLAGS?="-X ${PKG}/pkg/driver.driverVersion=${VERSION} -X ${PKG}/pkg/driver.gitCommit=${GIT_COMMIT} -X ${PKG}/pkg/driver.buildDate=${BUILD_DATE}"

GO111MODULE=on
GOPROXY=direct
GOPATH=$(shell go env GOPATH)
GOOS=$(shell go env GOOS)
GOBIN=$(shell pwd)/bin

IMAGE?=633339324534.dkr.ecr.us-east-1.amazonaws.com/aws-fsx-openzfs-csi-driver
TAG?=$(GIT_COMMIT)

OUTPUT_TYPE?=docker

ARCH=amd64
OS=linux
OSVERSION=amazon

ALL_OS?=linux
ALL_ARCH_linux?=amd64 arm64
ALL_OSVERSION_linux?=amazon
ALL_OS_ARCH_OSVERSION_linux=$(foreach arch, $(ALL_ARCH_linux), $(foreach osversion, ${ALL_OSVERSION_linux}, linux-$(arch)-${osversion}))

ALL_OS_ARCH_OSVERSION=$(foreach os, $(ALL_OS), ${ALL_OS_ARCH_OSVERSION_${os}})

PLATFORM?=linux/amd64,linux/arm64

# split words on hyphen, access by 1-index
word-hyphen = $(word $2,$(subst -, ,$1))

.EXPORT_ALL_VARIABLES:

.PHONY: build-aws-openzfs-csi-driver
build-aws-openzfs-csi-driver:
	mkdir -p bin
	CGO_ENABLED=0 GOOS=linux go build -ldflags ${LDFLAGS} -o bin/aws-fsx-openzfs-csi-driver ./cmd/

.PHONY: all
all: all-image-docker

.PHONY: all-push
all-push:
	docker buildx build \
		--no-cache-filter=linux-amazon \
		--platform=$(PLATFORM) \
		--progress=plain \
		--target=$(OS)-$(OSVERSION) \
		--output=type=registry \
		-t=$(IMAGE):$(TAG) \
		.
	touch $@

.PHONY: all-image-docker
all-image-docker: $(addprefix sub-image-docker-,$(ALL_OS_ARCH_OSVERSION_linux))

sub-image-%:
	$(MAKE) OUTPUT_TYPE=$(call word-hyphen,$*,1) OS=$(call word-hyphen,$*,2) ARCH=$(call word-hyphen,$*,3) OSVERSION=$(call word-hyphen,$*,4) image

.PHONY: test
test:
	go test -v -race ./pkg/...
	go test -v ./tests/sanity/...

.PHONY: clean
clean:
	rm -rf .*image-* bin/

bin /tmp/helm /tmp/kubeval:
	@mkdir -p $@

bin/helm: | /tmp/helm bin
	@curl -o /tmp/helm/helm.tar.gz -sSL https://get.helm.sh/helm-v3.11.2-${GOOS}-amd64.tar.gz
	@tar -zxf /tmp/helm/helm.tar.gz -C bin --strip-components=1
	@rm -rf /tmp/helm/*

.PHONY: verify-kustomize
verify: verify-kustomize
verify-kustomize:
	@ echo; echo "### $@:"
	@ ./hack/verify-kustomize

.PHONY: generate-kustomize
generate-kustomize: bin/helm
	cd charts/aws-fsx-openzfs-csi-driver && ../../bin/helm template kustomize . -s templates/controller-deployment.yaml --api-versions 'snapshot.storage.k8s.io/v1' | sed -e "/namespace: /d" > ../../deploy/kubernetes/base/controller-deployment.yaml
	cd charts/aws-fsx-openzfs-csi-driver && ../../bin/helm template kustomize . -s templates/csidriver.yaml > ../../deploy/kubernetes/base/csidriver.yaml
	cd charts/aws-fsx-openzfs-csi-driver && ../../bin/helm template kustomize . -s templates/node-daemonset.yaml | sed -e "/namespace: /d" > ../../deploy/kubernetes/base/node-daemonset.yaml
	cd charts/aws-fsx-openzfs-csi-driver && ../../bin/helm template kustomize . -s templates/poddisruptionbudget-controller.yaml --api-versions 'policy/v1/PodDisruptionBudget' | sed -e "/namespace: /d" > ../../deploy/kubernetes/base/poddisruptionbudget-controller.yaml
	cd charts/aws-fsx-openzfs-csi-driver && ../../bin/helm template kustomize . -s templates/controller-serviceaccount.yaml | sed -e "/namespace: /d" > ../../deploy/kubernetes/base/controller-serviceaccount.yaml
	cd charts/aws-fsx-openzfs-csi-driver && ../../bin/helm template kustomize . -s templates/node-serviceaccount.yaml | sed -e "/namespace: /d" > ../../deploy/kubernetes/base/node-serviceaccount.yaml
