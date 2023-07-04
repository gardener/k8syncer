// SPDX-FileCopyrightText: 2023 SAP SE or an SAP affiliate company and Gardener contributors.
//
// SPDX-License-Identifier: Apache-2.0

package environment

import (
	"os"
	"path/filepath"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
)

type Environment struct {
	Env    *envtest.Environment
	Client client.Client
}

// New creates a new test environment with the landscaper known crds.
func New(projectRoot string) (*Environment, error) {
	projectRoot, err := filepath.Abs(projectRoot)
	if err != nil {
		return nil, err
	}
	testBinPath := filepath.Join(projectRoot, "tmp", "test", "bin")
	// if the default Landscaper test bin does not exist we default to the kubebuilder testenv default
	// that uses the KUBEBUILDER_ASSETS env var.
	if _, err := os.Stat(testBinPath); err == nil {
		if err := os.Setenv("TEST_ASSET_KUBE_APISERVER", filepath.Join(testBinPath, "kube-apiserver")); err != nil {
			return nil, err
		}
		if err := os.Setenv("TEST_ASSET_ETCD", filepath.Join(testBinPath, "etcd")); err != nil {
			return nil, err
		}
		if err := os.Setenv("TEST_ASSET_KUBECTL", filepath.Join(testBinPath, "kubectl")); err != nil {
			return nil, err
		}
	}

	return &Environment{
		Env: &envtest.Environment{
			CRDDirectoryPaths: []string{filepath.Join(projectRoot, "test", "crd")},
		},
	}, nil
}

// Start starts the fake environment and creates a client for the started kubernetes cluster.
func (e *Environment) Start() error {
	restConfig, err := e.Env.Start()
	if err != nil {
		return err
	}

	c, err := client.New(restConfig, client.Options{})
	if err != nil {
		return err
	}

	e.Client = c
	return nil
}

// Stop stops the running dev environment
func (e *Environment) Stop() error {
	return e.Env.Stop()
}
