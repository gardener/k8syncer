// SPDX-FileCopyrightText: 2023 SAP SE or an SAP affiliate company and Gardener contributors.
//
// SPDX-License-Identifier: Apache-2.0

package transformers

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/uuid"
	"sigs.k8s.io/yaml"
)

var _ = Describe("Basic Transformer", func() {

	var basic *Basic
	rawMetadata := metav1.ObjectMeta{
		Name:              "foo",
		Namespace:         "bar",
		GenerateName:      "foo-",
		UID:               uuid.NewUUID(),
		ResourceVersion:   "asdf",
		Generation:        3,
		CreationTimestamp: metav1.Now(),
		Labels: map[string]string{
			"foo.bar.baz": "foobar",
		},
		Annotations: map[string]string{
			"foo.bar.baz": "foobar",
		},
		OwnerReferences: []metav1.OwnerReference{
			{
				APIVersion: "v1",
				Kind:       "Dummy",
				Name:       "dummy",
				UID:        uuid.NewUUID(),
			},
		},
	}
	byteMeta, err := yaml.Marshal(rawMetadata)
	Expect(err).ToNot(HaveOccurred())
	originalMetadata := map[string]interface{}{}
	Expect(yaml.Unmarshal(byteMeta, &originalMetadata)).To(Succeed())
	metadataFieldCount := len(originalMetadata)

	BeforeEach(func() {
		basic = NewBasic()
	})

	Context("Transform", func() {

		var defaultSpec = map[string]interface{}{
			"foo": map[string]interface{}{
				"bar": "baz",
			},
		}

		It("should only keep configured metadata fields", func() {
			original := &unstructured.Unstructured{
				Object: map[string]interface{}{},
			}
			Expect(unstructured.SetNestedMap(original.Object, originalMetadata, "metadata")).To(Succeed())
			Expect(unstructured.SetNestedMap(original.Object, defaultSpec, "spec")).To(Succeed())

			transformed, err := basic.Transform(original)
			Expect(err).ToNot(HaveOccurred())

			transformedMetadata, found, err := unstructured.NestedMap(transformed.UnstructuredContent(), "metadata")
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			transformedSpec, found, err := unstructured.NestedMap(transformed.UnstructuredContent(), "spec")
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			Expect(transformedMetadata).To(HaveLen(len(basic.MetadataCopyFields)))
			Expect(originalMetadata).To(HaveLen(metadataFieldCount), "original metadata should not have changed")
			for _, field := range basic.MetadataCopyFields {
				Expect(transformedMetadata).To(HaveKeyWithValue(field, originalMetadata[field]))
			}
			Expect(transformedSpec).To(Equal(defaultSpec))
		})

		It("should remove the status", func() {
			original := &unstructured.Unstructured{
				Object: map[string]interface{}{},
			}
			Expect(unstructured.SetNestedMap(original.Object, originalMetadata, "metadata")).To(Succeed())
			Expect(unstructured.SetNestedMap(original.Object, defaultSpec, "status")).To(Succeed())
			Expect(unstructured.SetNestedMap(original.Object, defaultSpec, "spec")).To(Succeed())

			transformed, err := basic.Transform(original)
			Expect(err).ToNot(HaveOccurred())

			transformedSpec, found, err := unstructured.NestedMap(transformed.UnstructuredContent(), "spec")
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			_, found, err = unstructured.NestedMap(transformed.UnstructuredContent(), "status")
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeFalse())

			Expect(transformedSpec).To(Equal(defaultSpec))
		})

	})

})
