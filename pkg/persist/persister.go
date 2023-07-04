// SPDX-FileCopyrightText: 2023 SAP SE or an SAP affiliate company and Gardener contributors.
//
// SPDX-License-Identifier: Apache-2.0

package persist

import (
	"context"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// Persister is an interface for all implementations which are able to persist the recorded changes somehow.
type Persister interface {
	// Exists returns whether data for the given resource exists in the persistency, without returning the actual data.
	// It could be implemented generically by checking if the return value of Get is (nil, nil),
	// but depending on the storage system, checking for existence could be implemented in a more efficient manner.
	Exists(ctx context.Context, name, namespace string, gvk schema.GroupVersionKind, subPath string) (bool, error)
	// Get returns the currently persisted data for the specified resource.
	// If no data for the resource exists, it is expected to return (nil, nil) and not an error.
	Get(ctx context.Context, name, namespace string, gvk schema.GroupVersionKind, subPath string) ([]byte, error)
	// Persist persists the specified resource.
	// In most cases, it simply calls PersistData with the data returned by rt.TransformAndSerialize.
	Persist(ctx context.Context, resource *unstructured.Unstructured, gvk schema.GroupVersionKind, rt ResourceTransformer, subPath string) error
	// PersistData works like persist, but for an already serialized resource.
	// Calling PersistData with nil data will delete the corresponding resource from persistency.
	PersistData(ctx context.Context, name, namespace string, gvk schema.GroupVersionKind, data []byte, subPath string) error
	// Delete removes the specified resource from persistency.
	// In most cases, it simply calls PersistData with a nil data object.
	// Calling Delete for a resource which doesn't exist is expected to not return an error.
	Delete(ctx context.Context, name, namespace string, gvk schema.GroupVersionKind, subPath string) error
	// InternalPersister returns the internal persister, if the current implementation wraps another implementation.
	// Otherwise, nil is returned.
	InternalPersister() Persister
}

// ResourceTransformer transforms a given k8s resource to prepare it for being persisted (usually by removing undesired fields).
type ResourceTransformer interface {
	// Transform prepares the resource for being persisted.
	// Usually, this means removing fields which should not be persisted, but it is also possible to reduce the resource to a value of a single field or something similar.
	Transform(*unstructured.Unstructured) (interface{}, error)
	// TransformAndSerialize is expected to first call Transform and then serialize the result into something which can be persisted.
	// For k8s resources, marshalling into JSON or YAML would be the obvious way, but it is not limited to this.
	TransformAndSerialize(*unstructured.Unstructured) ([]byte, error)
}
