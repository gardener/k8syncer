#!/bin/bash
#
# Copyright (c) 2018 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
#
# SPDX-License-Identifier: Apache-2.0

CURRENT_DIR=$(dirname $0)
PROJECT_ROOT="${CURRENT_DIR}"/..

# On MacOS there is a strange race condition
# between port allocation of envtest suites when go test
# runs all the tests in parallel without any limits (spins up around 10+ environments).
#
# To avoid flakes, set we're setting the go-test parallel flag
# to limit the number of parallel executions.
#
# TODO: check the controller-runtime for root-cause and real mitigation
# https://github.com/kubernetes-sigs/controller-runtime/pull/1567
if [[ "${OSTYPE}" == "darwin"* ]]; then
  P_FLAG="-p=1"
fi

go test -mod=vendor -race -coverprofile=${PROJECT_ROOT}/coverage.main.out -covermode=atomic ${P_FLAG} \
                    ${PROJECT_ROOT}/cmd/... \
                    ${PROJECT_ROOT}/pkg/...
EXIT_STATUS_MAIN_TEST=$?
go tool cover -html=${PROJECT_ROOT}/coverage.main.out -o ${PROJECT_ROOT}/coverage.main.html

! (( EXIT_STATUS_MAIN_TEST ))
