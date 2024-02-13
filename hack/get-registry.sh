#!/bin/bash
#
# SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors.
#
# SPDX-License-Identifier: Apache-2.0

BASE_REGISTRY=europe-docker.pkg.dev/sap-gcp-cp-k8s-stable-hub/cola
IMAGE_REGISTRY=$BASE_REGISTRY
CHART_REGISTRY=$BASE_REGISTRY/charts
COMPONENT_REGISTRY=$BASE_REGISTRY/components

mode="BASE_"

while [[ "$#" -gt 0 ]]; do
  case ${1:-} in
    "-i"|"--image")
      mode="IMAGE_"
      ;;
    "-h"|"--helm")
      mode="CHART_"
      ;;
    "-c"|"--component")
      mode="COMPONENT_"
      ;;
    *)
      echo "invalid argument: $1" 1>&2
      exit 1
      ;;
  esac
  shift
done

eval echo "\$${mode}REGISTRY"
