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

# Upgrades the driver release in place, keeping the values eksctl.sh installed it
# with. The isolation e2e tests call this with --set flags to change the
# controller's cluster tag settings.

set -euo pipefail

HELM=${HELM:-helm}
RELEASE=${RELEASE:-aws-fsx-openzfs-csi-driver}
CHART_DIR=${CHART_DIR:-$(dirname "${BASH_SOURCE[0]}")/../../charts/aws-fsx-openzfs-csi-driver}

"${HELM}" upgrade "${RELEASE}" "${CHART_DIR}" \
  --namespace kube-system \
  --reuse-values \
  --wait \
  "$@"
