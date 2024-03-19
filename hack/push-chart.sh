#!/bin/bash
#
# SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors.
#
# SPDX-License-Identifier: Apache-2.0

set -euo pipefail

PROJECT_ROOT="$(realpath $(dirname $0)/..)"
HACK_DIR="$PROJECT_ROOT/hack"
source "$HACK_DIR/lib.sh"

if [[ -z ${LOCALBIN:-} ]]; then
  LOCALBIN="$PROJECT_ROOT/bin"
fi
if [[ -z ${HELM:-} ]]; then
  HELM="$LOCALBIN/helm"
fi

VERSION=$("$HACK_DIR/get-version.sh")
HELM_REGISTRY=$("$HACK_DIR/get-registry.sh" --helm)

echo "> Uploading helm chart to $HELM_REGISTRY ..."
tmpdir="$PROJECT_ROOT/tmp"
"$HELM" push "$tmpdir/k8syncer-$VERSION.tgz" "oci://$HELM_REGISTRY" | indent 1
