#!/bin/bash
#
# SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors.
#
# SPDX-License-Identifier: Apache-2.0

set -e

echo "> Format"

goimports -l -w -local=github.com/gardener/k8syncer $@