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

.PHONY: clean
clean:
	rm -rf .*image-* bin/
