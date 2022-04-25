#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

SCRIPT_ROOT=$(dirname ${BASH_SOURCE})/..

chmod +x vendor/k8s.io/code-generator/generate-groups.sh

vendor/k8s.io/code-generator/generate-groups.sh all \
  github.com/k-cloud-labs/pkg/client \
  github.com/k-cloud-labs/pkg/apis \
  policy:v1alpha1 \
  --go-header-file ${SCRIPT_ROOT}/hack/boilerplate.go.txt