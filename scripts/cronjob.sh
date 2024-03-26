#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" &> /dev/null && pwd)
TARGET="/var/www/virtual/floppnet/parkrun.flopp.net/"

rm -rf "${SCRIPT_DIR}/.output"

"${SCRIPT_DIR}/generate-linux"  \
    -data     "${SCRIPT_DIR}/repo/data" \
    -download "${SCRIPT_DIR}/.download" \
    -output   "${SCRIPT_DIR}/.output"

mkdir -p "${TARGET}"
cp -a "${SCRIPT_DIR}/.output/." "${TARGET}"
chmod -R a+rx "${TARGET}"
