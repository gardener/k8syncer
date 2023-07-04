// SPDX-FileCopyrightText: 2023 SAP SE or an SAP affiliate company and Gardener contributors.
//
// SPDX-License-Identifier: Apache-2.0

package transformers

import (
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"

	"github.com/gardener/k8syncer/pkg/persist"
)

var _ persist.ResourceTransformer = &Basic{}

// Basic is a simple transformer.
// It removes volatile fields from the metadata and removes the status, if any.
// It serializes to YAML.
type Basic struct {
	MetadataCopyFields []string
}

// NewBasic constructs a new basic transformer.
// It is initialized with a list of fields which should be retained in the metadata.
// In theory, this list can be modified, but adding volatile fields to it is strongly discouraged.
// By default, the following fields are retained: name, generateName, namespace, uid, labels, ownerReferences
func NewBasic() *Basic {
	return &Basic{
		MetadataCopyFields: []string{
			"name",
			"generateName",
			"namespace",
			"uid",
			"labels",
			"ownerReferences",
		},
	}
}

func (b *Basic) Transform(obj *unstructured.Unstructured) (interface{}, error) {
	res := obj.DeepCopy().Object
	oldMeta := res["metadata"].(map[string]interface{})
	newMeta := map[string]interface{}{}
	for _, field := range b.MetadataCopyFields {
		if oldMeta[field] != nil {
			newMeta[field] = oldMeta[field]
		}
	}
	res["metadata"] = newMeta
	delete(res, "status")

	return res, nil
}

func (b *Basic) TransformAndSerialize(obj *unstructured.Unstructured) ([]byte, error) {
	raw, err := b.Transform(obj)
	if err != nil {
		return nil, err
	}
	data, err := yaml.Marshal(raw)
	if err != nil {
		return nil, fmt.Errorf("error while marshalling object to yaml: %w", err)
	}
	return data, nil
}
