#!/bin/bash
#
# SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors.
#
# SPDX-License-Identifier: Apache-2.0

set -euo pipefail

PROJECT_ROOT="$(dirname $(realpath $0))/.."
HACK_DIR="$PROJECT_ROOT/hack"
source "$HACK_DIR/lib.sh"

if [[ -z ${LOCALBIN:-} ]]; then
  LOCALBIN="$PROJECT_ROOT/bin"
fi
if [[ -z ${HELM:-} ]]; then
  HELM="$LOCALBIN/helm"
fi

VERSION=$("$HACK_DIR/get-version.sh")

echo "> Packaging helm chart to prepare for upload"
tmpdir="$PROJECT_ROOT/tmp"
mkdir -p "$tmpdir"
"$HELM" package "$PROJECT_ROOT/charts/k8syncer" -d "$tmpdir" --version "$VERSION" | indent 1 # file name is <chart name>-<chart version>.tgz (derived from Chart.yaml)
