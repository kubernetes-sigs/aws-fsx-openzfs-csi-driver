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

# See
# https://docs.docker.com/engine/reference/builder/#automatic-platform-args-in-the-global-scope
# for info on BUILDPLATFORM, TARGETOS, TARGETARCH, etc.
FROM --platform=$BUILDPLATFORM golang:1.20.4-bullseye AS go-builder
WORKDIR /go/src/github.com/kubernetes-sigs/aws-fsx-openzfs-csi-driver
COPY . .
ARG TARGETOS
ARG TARGETARCH
RUN OS=$TARGETOS ARCH=$TARGETARCH make $TARGETOS/$TARGETARCH

# https://github.com/aws/eks-distro-build-tooling/blob/main/eks-distro-base/Dockerfile.minimal-base-csi-ebs#L36
FROM public.ecr.aws/eks-distro-build-tooling/eks-distro-minimal-base-csi-ebs-builder:latest-al2 as rpm-installer

RUN set -x && \
      install_binary /sbin/mount.nfs /sbin/umount.nfs && \
                     cleanup "fsx-openzfs-csi"

FROM public.ecr.aws/eks-distro-build-tooling/eks-distro-minimal-base-csi-ebs:latest-al2 AS linux-amazon
COPY --from=rpm-installer /newroot /
COPY --from=go-builder /go/src/github.com/kubernetes-sigs/aws-fsx-openzfs-csi-driver/bin/aws-fsx-openzfs-csi-driver /bin/aws-fsx-openzfs-csi-driver
ENTRYPOINT ["/bin/aws-fsx-openzfs-csi-driver"]
