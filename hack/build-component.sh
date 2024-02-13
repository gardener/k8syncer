#!/bin/bash
#
# SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors.
#
# SPDX-License-Identifier: Apache-2.0

set -euo pipefail

PROJECT_ROOT="$(dirname $(realpath $0))/.."
HACK_DIR="$PROJECT_ROOT/hack"
source "$HACK_DIR/lib.sh"

VERSION=$("$HACK_DIR/get-version.sh")
CHART_REGISTRY=$("$HACK_DIR/get-registry.sh" --helm)
IMG_REGISTRY=$("$HACK_DIR/get-registry.sh" --image)

if [[ -z ${LOCALBIN:-} ]]; then
  LOCALBIN="$PROJECT_ROOT/bin"
fi
if [[ -z ${OCM:-} ]]; then
  OCM="$LOCALBIN/ocm"
fi

if [[ -z ${CDVERSION:-} ]]; then
  CDVERSION=$VERSION
fi
if [[ -z ${CHART_VERSION:-} ]]; then
  CHART_VERSION=$VERSION
fi
if [[ -z ${IMG_VERSION:-} ]]; then
  IMG_VERSION=$VERSION
fi

echo "> Building component in version $CDVERSION (image version $IMG_VERSION, chart version $CHART_VERSION)"
$OCM add componentversions --file "$PROJECT_ROOT/components" --version "$CDVERSION" --create --force "$HACK_DIR/components.yaml" -- \
  CHART_REGISTRY="$CHART_REGISTRY" \
  IMG_REGISTRY="$IMG_REGISTRY" \
  CDVERSION="$CDVERSION" \
  CHART_VERSION="$CHART_VERSION" \
  IMG_VERSION="$IMG_VERSION" \
  | indent 1
