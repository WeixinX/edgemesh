#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

# go mod tidy
# go mod vendor

cd cmd/edgemesh-agent
dlv debug --headless --listen=:2345 --api-version=2 --accept-multiclient

# go build -o edgemesh-agent -gcflags "all=-N -l"
# dlv --listen=:2345 --headless=true --api-version=2 --accept-multiclient exec ./edgemesh-agent
