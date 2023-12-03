#!/usr/bin/env bash

set -o nounset
set -o errexit
set -o pipefail
# set -o xtrace

touch cadvisor.log
nohup cadvisor > cadvisor.log 2>&1 &
echo "cAdvisor server is running on 0.0.0.0:8080 ..."