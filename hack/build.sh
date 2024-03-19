#!/bin/bash
#
# SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors.
#
# SPDX-License-Identifier: Apache-2.0

set -euo pipefail

PROJECT_ROOT="$(realpath $(dirname $0)/..)"
HACK_DIR="$PROJECT_ROOT/hack"
source "$HACK_DIR/lib.sh"

echo "> Building binaries ..."
(
  cd "$PROJECT_ROOT"
  for pf in ${PLATFORMS//,/ }; do
    echo "> Building binary for $pf ..." | indent 1
    os=${pf%/*}
    arch=${pf#*/}
    CGO_ENABLED=0 GOOS=$os GOARCH=$arch go build -a -o bin/k8syncer-${os}.${arch} cmd/k8syncer/main.go | indent 2
  done
)
