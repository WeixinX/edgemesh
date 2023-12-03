#!/usr/bin/env bash

set -o nounset
set -o errexit
set -o pipefail
# set -o xtrace

touch kubeproxy.log
nohup kubectl proxy -p 10550 > kubeproxy.log 2>&1 &
