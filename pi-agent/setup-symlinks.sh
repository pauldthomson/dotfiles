#!/usr/bin/env bash
set -euo pipefail

# Idempotent setup for global pi symlinks.
# Safe to run multiple times.

PI_AGENT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
TARGET_DIR="${HOME}/.pi/agent"

for required_path in \
  "${PI_AGENT_DIR}/extensions" \
  "${PI_AGENT_DIR}/settings.json" \
  "${PI_AGENT_DIR}/themes"; do
  if [[ ! -e "${required_path}" ]]; then
    echo "Missing required path: ${required_path}" >&2
    exit 1
  fi
done

mkdir -p "${TARGET_DIR}"
ln -sfn "${PI_AGENT_DIR}/extensions" "${TARGET_DIR}/extensions"
ln -sfn "${PI_AGENT_DIR}/settings.json" "${TARGET_DIR}/settings.json"
ln -sfn "${PI_AGENT_DIR}/themes" "${TARGET_DIR}/themes"

echo "Updated pi symlinks:"
ls -ld "${TARGET_DIR}/extensions" "${TARGET_DIR}/settings.json" "${TARGET_DIR}/themes"
