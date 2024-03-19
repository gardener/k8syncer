#!/bin/bash
#
# SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors.
#
# SPDX-License-Identifier: Apache-2.0

set -euo pipefail

PROJECT_ROOT="$(realpath $(dirname $0)/..)"
if [[ -z ${LOCALBIN:-} ]]; then
  LOCALBIN="$PROJECT_ROOT/bin"
fi
if [[ -z ${HELM:-} ]]; then
  HELM="$LOCALBIN/helm"
fi

HELM_VERSION="$1"

echo "Installing helm $HELM_VERSION ..."
arch=$(uname -m)
if [[ "$arch" == "x86_64" ]]; then
  arch="amd64"
fi
os=$(uname | tr '[:upper:]' '[:lower:]')
curl -sfL "https://get.helm.sh/helm-${HELM_VERSION}-${os}-${arch}.tar.gz" --output "$LOCALBIN/helm.tar.gz"
mkdir -p "$LOCALBIN/helm-unpacked"
tar -xzf "$LOCALBIN/helm.tar.gz" --directory "$LOCALBIN/helm-unpacked"
mv "$LOCALBIN/helm-unpacked/${os}-${arch}/helm" "$LOCALBIN/helm"
chmod +x "$LOCALBIN/helm"
rm -rf "$LOCALBIN/helm.tar.gz" "$LOCALBIN/helm-unpacked"
