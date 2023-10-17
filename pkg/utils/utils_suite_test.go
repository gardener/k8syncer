// SPDX-FileCopyrightText: 2023 SAP SE or an SAP affiliate company and Gardener contributors.
//
// SPDX-License-Identifier: Apache-2.0

package utils

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestConfig(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Utils Test Suite")
}

var _ = Describe("Utils Tests", func() {

	Context("ParseSimpleJSONPath", func() {

		It("should correctly parse a path without any escapes", func() {
			Expect(ParseSimpleJSONPath("a.bc.d")).To(Equal([]string{"a", "bc", "d"}))
			Expect(ParseSimpleJSONPath("a")).To(Equal([]string{"a"}))
			Expect(ParseSimpleJSONPath("")).To(Equal([]string{}))
		})

		It("should correctly parse a path with one or more escapes", func() {
			Expect(ParseSimpleJSONPath("a.b\\.c.d")).To(Equal([]string{"a", "b.c", "d"}))
			Expect(ParseSimpleJSONPath("a\\")).To(Equal([]string{"a\\"}))
			Expect(ParseSimpleJSONPath("a.b\\.c\\.d.e")).To(Equal([]string{"a", "b.c.d", "e"}))
			Expect(ParseSimpleJSONPath("a.b\\.c\\.d.e.f\\.g")).To(Equal([]string{"a", "b.c.d", "e", "f.g"}))
		})

	})

})
