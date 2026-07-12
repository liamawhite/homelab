#!/bin/bash
# Prints the value of the named constant in pkg/versions/versions.go, so
# CRD-generation scripts can derive their target version from the single
# source of truth instead of duplicating it.
#
# Usage: version.sh <ConstName>
set -euo pipefail

VERSIONS_FILE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/versions.go"
name="${1:?usage: version.sh <ConstName>}"

value="$(grep -E "^\s*${name}\s*=" "${VERSIONS_FILE}" | sed -E 's/.*=\s*"([^"]+)".*/\1/')"

if [ -z "${value}" ]; then
  echo "error: could not find constant ${name} in ${VERSIONS_FILE}" >&2
  exit 1
fi

echo "${value}"
