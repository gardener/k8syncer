// SPDX-FileCopyrightText: 2023 SAP SE or an SAP affiliate company and Gardener contributors.
//
// SPDX-License-Identifier: Apache-2.0

package transformers

import (
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/gardener/k8syncer/pkg/persist"
)

var _ persist.Transformer = &Basic{}

// Basic is a simple transformer.
// It removes volatile fields from the metadata and removes the status, if any.
// It serializes to YAML.
type Basic struct {
	MetadataCopyFields []string
}

// NewBasic constructs a new basic transformer.
// If used without any arguments, it is initialized with the default set of metadata fields to persist.
// It is recommended to use it this way.
// If the argument list is not empty, it will be used as the list of metadata fields to persist instead. The default list is ignored in that case.
// By default, the following fields are retained: name, generateName, namespace, generation, uid, labels, ownerReferences
func NewBasic(metadataFields ...string) *Basic {
	return &Basic{
		MetadataCopyFields: []string{
			"name",
			"generateName",
			"namespace",
			"generation",
			"uid",
			"labels",
			"ownerReferences",
		},
	}
}

func (b *Basic) Transform(obj *unstructured.Unstructured) (*unstructured.Unstructured, error) {
	res := obj.DeepCopy()
	oldMeta, found, err := unstructured.NestedMap(obj.UnstructuredContent(), "metadata")
	if err != nil {
		return nil, fmt.Errorf("object metadata is not a map: %w", err)
	}
	if !found {
		return nil, fmt.Errorf("object does not have metadata")
	}
	newMeta := map[string]interface{}{}
	for _, field := range b.MetadataCopyFields {
		if oldMeta[field] != nil {
			newMeta[field] = oldMeta[field]
		}
	}
	err = unstructured.SetNestedMap(res.Object, newMeta, "metadata")
	if err != nil {
		return nil, fmt.Errorf("error setting new metadata: %w", err)
	}
	delete(res.Object, "status")

	return res, nil
}
