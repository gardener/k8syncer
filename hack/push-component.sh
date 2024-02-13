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
COMPONENT_REGISTRY="$($HACK_DIR/get-registry.sh --component)"

if [[ -z ${LOCALBIN:-} ]]; then
  LOCALBIN="$PROJECT_ROOT/bin"
fi
if [[ -z ${OCM:-} ]]; then
  OCM="$LOCALBIN/ocm"
fi

overwrite=""
if [[ -n ${OVERWRITE_COMPONENTS:-} ]] && [[ ${OVERWRITE_COMPONENTS} != "false" ]]; then
  overwrite="--overwrite"
fi

echo "> Uploading Component Descriptors to $COMPONENT_REGISTRY ..."
$OCM transfer componentversions "$PROJECT_ROOT/components" "$COMPONENT_REGISTRY" $overwrite | indent 1
