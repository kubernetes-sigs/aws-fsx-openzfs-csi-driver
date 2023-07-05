#!/bin/bash

set -euo pipefail

function fsx_delete_e2e_resources() {
  local PR=${1:-local}
  local QUERY="?Key=='OpenZFSCSIDriverE2E' && Value=='$PR'"

  for i in $(aws fsx describe-file-systems --query "FileSystems[?Tags[$QUERY]].FileSystemId" --output text); do
    aws fsx delete-file-system --file-system-id "$i" --open-zfs-configuration SkipFinalBackup=true,Options=DELETE_CHILD_VOLUMES_AND_SNAPSHOTS
  done
}

"$@"
