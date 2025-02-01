#!/bin/bash

DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"

echo "Generating CRDs"
"${DIR}/components/metallb/crds.sh"
"${DIR}/components/tailscale/crds.sh"
