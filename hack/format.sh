#!/bin/bash
#
# SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors.
#
# SPDX-License-Identifier: Apache-2.0

set -euo pipefail

PROJECT_ROOT="$(dirname $(realpath $0))/.."

if [[ -z ${LOCALBIN:-} ]]; then
  LOCALBIN="$PROJECT_ROOT/bin"
fi
if [[ -z ${FORMATTER:-} ]]; then
  FORMATTER="$LOCALBIN/goimports"
fi

write_mode="-w"
if [[ ${1:-} == "--verify" ]]; then
  write_mode=""
fi

tmp=$("${FORMATTER}" -l $write_mode -local=github.com/gardener/k8syncer ./cmd ./pkg ./test)

if [[ -z ${write_mode} ]] && [[ ${tmp} ]]; then
  echo "unformatted files detected, please run 'make format'" 1>&2
  echo "$tmp" 1>&2
  exit 1
fi

if [[ ${tmp} ]]; then
  echo "$tmp"
fi
