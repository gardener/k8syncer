#!/bin/bash -eu
#
# SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors.
#
# SPDX-License-Identifier: Apache-2.0


# pipe some text into 'indent X' to indent each line by X levels (one 'level' being two spaces)
function indent() {
  local level=${1:-""}
  if [[ -z "$level" ]]; then
    level=1
  fi
  local spaces=$(($level * 2))
  local iv=$(printf %${spaces}s)
  sed "s/^/$iv/"
}