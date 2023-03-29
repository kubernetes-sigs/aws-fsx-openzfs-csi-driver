FROM --platform=$BUILDPLATFORM golang:1.20.2-bullseye as builder
WORKDIR /go/src/github.com/kubernetes-sigs/aws-fsx-openzfs-csi-driver
ADD . .
RUN make

FROM amazonlinux:2 AS linux-amazon
RUN yum update -y
RUN yum install util-linux libyaml -y
RUN yum install nfs-utils -y

COPY --from=builder /go/src/github.com/kubernetes-sigs/aws-fsx-openzfs-csi-driver/bin/aws-fsx-openzfs-csi-driver /bin/aws-fsx-openzfs-csi-driver

ENTRYPOINT ["/bin/aws-fsx-openzfs-csi-driver"]
