#!/bin/bash

# Copyright 2026 The Kubernetes Authors.
#
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

# Runs the e2e tests on an EKS cluster created by eksctl, where the controller uses
# EKS Pod Identity.
#
#   eksctl.sh up      creates the controller role, the cluster, and installs the driver
#   eksctl.sh test    runs the e2e suites, then the cluster tag isolation tests
#   eksctl.sh down    deletes the cluster and the controller role
#   eksctl.sh all     up, test, and down when CLEAN=true
#
# Without IMAGE_REPOSITORY, up builds the driver image with `make image` and pushes it
# to the ECR repository aws-fsx-openzfs-csi-driver in the caller's account and region.
# It installs eksctl, helm, gomplate and ginkgo into BIN_DIR when they are missing.

set -euo pipefail

BASE_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
REPO_ROOT=$(cd "${BASE_DIR}/../.." && pwd)

TEST_ID=${TEST_ID:-$RANDOM}
CLUSTER_NAME=${CLUSTER_NAME:-fsx-openzfs-e2e-${TEST_ID}}
REGION=${AWS_REGION:-us-west-2}
ZONES=${AWS_AVAILABILITY_ZONES:-${REGION}a,${REGION}b}
K8S_VERSION=${K8S_VERSION:-1.33}
NODE_COUNT=${NODE_COUNT:-2}
INSTANCE_TYPE=${INSTANCE_TYPE:-m5.large}
CONTROLLER_ROLE_NAME=${CONTROLLER_ROLE_NAME:-${CLUSTER_NAME}-controller}
IMAGE_REPOSITORY=${IMAGE_REPOSITORY:-}
IMAGE_TAG=${IMAGE_TAG:-${TEST_ID}}
IMAGE_ARCH=${IMAGE_ARCH:-amd64}
# Prow sets PULL_NUMBER.
PULL_REQUEST=${PULL_REQUEST:-${PULL_NUMBER:-local}}
GINKGO_FOCUS=${GINKGO_FOCUS-"FSx for OpenZFS"}
GINKGO_SKIP=${GINKGO_SKIP:-"\[Disruptive\]|\[Serial\]"}
GINKGO_PROCS=${GINKGO_PROCS:-4}
ISOLATION_E2E=${ISOLATION_E2E:-true}
CLEAN=${CLEAN:-true}
EKSCTL_VERSION=${EKSCTL_VERSION:-0.231.0}
HELM_VERSION=${HELM_VERSION:-v3.19.0}
GOMPLATE_VERSION=${GOMPLATE_VERSION:-v5.3.0}
BIN_DIR=${BIN_DIR:-${REPO_ROOT}/bin}
export KUBECONFIG=${KUBECONFIG:-${BIN_DIR}/${CLUSTER_NAME}.kubeconfig}
export AWS_REGION=${REGION}
# Tools installed by install_tools come first, for this script and the scripts the tests run.
export PATH=${BIN_DIR}:${PATH}

function log() { echo "### $(date -u +%H:%M:%S) $*" >&2; }

function host_arch() {
  case $(uname -m) in
  x86_64) echo amd64 ;;
  aarch64 | arm64) echo arm64 ;;
  *) uname -m ;;
  esac
}

# The kubekins-e2e image of the CI jobs has none of these tools.
function install_tools() {
  local arch tmp
  arch=$(host_arch)
  mkdir -p "${BIN_DIR}"
  if ! command -v eksctl >/dev/null; then
    log "installing eksctl ${EKSCTL_VERSION}"
    curl -sSfL "https://github.com/eksctl-io/eksctl/releases/download/v${EKSCTL_VERSION}/eksctl_Linux_${arch}.tar.gz" | tar xz -C "${BIN_DIR}" eksctl
  fi
  if ! command -v helm >/dev/null; then
    log "installing helm ${HELM_VERSION}"
    tmp=$(mktemp -d)
    curl -sSfL "https://get.helm.sh/helm-${HELM_VERSION}-linux-${arch}.tar.gz" | tar xz -C "${tmp}"
    mv "${tmp}/linux-${arch}/helm" "${BIN_DIR}/helm"
    rm -rf "${tmp}"
  fi
  if ! command -v gomplate >/dev/null; then
    log "installing gomplate ${GOMPLATE_VERSION}"
    curl -sSfL -o "${BIN_DIR}/gomplate" "https://github.com/hairyhenderson/gomplate/releases/download/${GOMPLATE_VERSION}/gomplate_linux-${arch}"
    chmod +x "${BIN_DIR}/gomplate"
  fi
  if ! command -v ginkgo >/dev/null; then
    log "installing ginkgo"
    # The version in go.mod, so the CLI matches the library.
    (cd "${REPO_ROOT}" && GOBIN=${BIN_DIR} go install "github.com/onsi/ginkgo/v2/ginkgo@$(go list -m -f '{{.Version}}' github.com/onsi/ginkgo/v2)")
  fi
}

# Builds the driver image and pushes it to ECR, unless IMAGE_REPOSITORY names one.
function build_and_push_image() {
  if [[ -n ${IMAGE_REPOSITORY} ]]; then
    return 0
  fi
  local account registry repo=aws-fsx-openzfs-csi-driver
  account=$(aws sts get-caller-identity --query Account --output text)
  registry=${account}.dkr.ecr.${REGION}.amazonaws.com
  IMAGE_REPOSITORY=${registry}/${repo}
  if ! aws ecr describe-repositories --repository-names "${repo}" >/dev/null 2>&1; then
    log "creating ECR repository ${repo}"
    aws ecr create-repository --repository-name "${repo}" --image-scanning-configuration scanOnPush=true >/dev/null
  fi
  log "building and pushing ${IMAGE_REPOSITORY}:${IMAGE_TAG}"
  aws ecr get-login-password | docker login --username AWS --password-stdin "${registry}"
  (cd "${REPO_ROOT}" && make image IMAGE="${IMAGE_REPOSITORY}" TAG="${IMAGE_TAG}" OS=linux ARCH="${IMAGE_ARCH}" OSVERSION=amazon)
  docker tag "${IMAGE_REPOSITORY}:${IMAGE_TAG}-linux-${IMAGE_ARCH}-amazon" "${IMAGE_REPOSITORY}:${IMAGE_TAG}"
  docker push "${IMAGE_REPOSITORY}:${IMAGE_TAG}"
}

function e2e_up() {
  install_tools
  build_and_push_image

  # The tests start from the broad policy, as an existing cluster would.
  log "creating controller role ${CONTROLLER_ROLE_NAME}"
  CONTROLLER_ROLE_ARN=$("${BASE_DIR}/pod-identity.sh" create-role "${CONTROLLER_ROLE_NAME}")
  "${BASE_DIR}/pod-identity.sh" set-policies "${CONTROLLER_ROLE_NAME}" "${REPO_ROOT}/docs/example-iam-policy.json"

  local config=${BIN_DIR}/${CLUSTER_NAME}.eksctl.yaml
  CLUSTER_NAME=${CLUSTER_NAME} REGION=${REGION} K8S_VERSION=${K8S_VERSION} ZONES=${ZONES} \
    NODE_COUNT=${NODE_COUNT} INSTANCE_TYPE=${INSTANCE_TYPE} CONTROLLER_ROLE_ARN=${CONTROLLER_ROLE_ARN} \
    gomplate -f "${BASE_DIR}/eksctl-cluster.yaml" -o "${config}"

  if eksctl get cluster --name "${CLUSTER_NAME}" --region "${REGION}" >/dev/null 2>&1; then
    log "reusing cluster ${CLUSTER_NAME}"
    eksctl utils write-kubeconfig --cluster "${CLUSTER_NAME}" --region "${REGION}" --kubeconfig "${KUBECONFIG}"
  else
    log "creating cluster ${CLUSTER_NAME}"
    eksctl create cluster -f "${config}" --kubeconfig "${KUBECONFIG}"
  fi

  log "installing the driver ${IMAGE_REPOSITORY}:${IMAGE_TAG}"
  helm upgrade --install aws-fsx-openzfs-csi-driver "${REPO_ROOT}/charts/aws-fsx-openzfs-csi-driver" \
    --namespace kube-system --wait \
    --set image.repository="${IMAGE_REPOSITORY}" \
    --set image.tag="${IMAGE_TAG}" \
    --set image.pullPolicy=Always \
    --set controller.region="${REGION}" \
    --set node.region="${REGION}"
  "${BASE_DIR}/pod-identity.sh" wait-for-credentials
}

function run_ginkgo() {
  (cd "${REPO_ROOT}/tests/e2e" && ginkgo "$@" . -- \
    --cluster-name="${CLUSTER_NAME}" --region="${REGION}" --pull-request="${PULL_REQUEST}" \
    --repo-root="${REPO_ROOT}" --controller-role-name="${CONTROLLER_ROLE_NAME}")
}

function e2e_test() {
  install_tools
  local rc=0
  if [[ -n ${GINKGO_FOCUS} ]]; then
    log "running the e2e suites"
    run_ginkgo --procs="${GINKGO_PROCS}" --timeout=5h --focus="${GINKGO_FOCUS}" --skip="${GINKGO_SKIP}" \
      --junit-report="${BIN_DIR}/${CLUSTER_NAME}-junit.xml" || rc=$?
  fi
  # The isolation tests swap the controller's IAM policies and settings, so they
  # run alone, after the other tests.
  if [[ ${ISOLATION_E2E} == true ]]; then
    log "running the cluster tag isolation tests"
    run_ginkgo --procs=1 --timeout=3h --focus='\[fsx-openzfs-isolation\]' -v \
      --junit-report="${BIN_DIR}/${CLUSTER_NAME}-isolation-junit.xml" || rc=$?
  fi
  return ${rc}
}

function e2e_down() {
  install_tools
  log "deleting cluster ${CLUSTER_NAME}"
  helm uninstall aws-fsx-openzfs-csi-driver --namespace kube-system --wait 2>/dev/null || true
  eksctl delete cluster --name "${CLUSTER_NAME}" --region "${REGION}" --wait || true
  "${BASE_DIR}/pod-identity.sh" delete-role "${CONTROLLER_ROLE_NAME}"
  bash "${BASE_DIR}/fsx.sh" fsx_delete_e2e_resources "${PULL_REQUEST}"
}

case "${1:-all}" in
up) e2e_up ;;
test) e2e_test ;;
down) e2e_down ;;
all)
  if [[ ${CLEAN} == true ]]; then trap e2e_down EXIT; fi
  e2e_up
  e2e_test
  ;;
*)
  echo "usage: $0 up|test|down|all" >&2
  exit 1
  ;;
esac
