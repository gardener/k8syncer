#!/bin/bash
#
# SPDX-FileCopyrightText: 2022 SAP SE or an SAP affiliate company and Gardener contributors.
#
# SPDX-License-Identifier: Apache-2.0

set -e

CURRENT_DIR=$(dirname $0)
PROJECT_ROOT="${CURRENT_DIR}"/..

if [[ $EFFECTIVE_VERSION == "" ]]; then
  EFFECTIVE_VERSION=$(cat $PROJECT_ROOT/VERSION)
fi

echo "> Install $EFFECTIVE_VERSION"

CGO_ENABLED=0 GOOS=$(go env GOOS) GOARCH=$(go env GOARCH) GO111MODULE=on \
  go install -mod=vendor \
  -ldflags "-X github.com/gardener/k8syncerpkg/version.GitVersion=$EFFECTIVE_VERSION \
            -X github.com/gardener/k8syncerpkg/version.gitTreeState=$([ -z git status --porcelain 2>/dev/null ] && echo clean || echo dirty) \
            -X github.com/gardener/k8syncerpkg/version.gitCommit=$(git rev-parse --verify HEAD) \
            -X github.com/gardener/k8syncerpkg/version.buildDate=$(date --rfc-3339=seconds | sed 's/ /T/')" \
  ${PROJECT_ROOT}/cmd/...