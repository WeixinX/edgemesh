#!/usr/bin/env bash

set -o nounset
set -o errexit
set -o pipefail
# set -o xtrace

go mod tidy
go mod vendor
go run cmd/edgemesh-agent/agent.go
