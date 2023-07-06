// SPDX-FileCopyrightText: 2023 SAP SE or an SAP affiliate company and Gardener contributors.
//
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"path/filepath"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/gardener/k8syncer/test/environment"
)

func TestConfig(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Controller Test Suite")
}

var (
	testenv *environment.Environment
)

var _ = BeforeSuite(func() {
	projectRoot := filepath.Join("../../")
	var err error
	testenv, err = environment.New(projectRoot)
	Expect(err).ToNot(HaveOccurred())

	Expect(testenv.Start()).To(Succeed())
})

var _ = AfterSuite(func() {
	Expect(testenv.Stop()).ToNot(HaveOccurred())
})
