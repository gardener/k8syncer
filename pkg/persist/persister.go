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
	Get(ctx context.Context, name, namespace string, gvk schema.GroupVersionKind, subPath string) (*unstructured.Unstructured, error)
	// Persist persists the given resource.
	// Depending on the implementation, it might check for its existence in the storage first and only update it if it differs.
	// It returns the transformed version of the given resource, which should be the one that is persisted after this command.
	// The second return value is 'true' if the resource in the storage has changed (meaning the given resource differed from the one in the storage when this method was called).
	Persist(ctx context.Context, resource *unstructured.Unstructured, t Transformer, subPath string) (*unstructured.Unstructured, bool, error)
	// Delete deletes the resource from persistence.
	// If the resource does not exist, Delete will not return an error.
	Delete(ctx context.Context, name, namespace string, gvk schema.GroupVersionKind, subPath string) error
	// InternalPersister returns the internal persister, if the current implementation wraps another implementation.
	// Otherwise, nil is returned.
	InternalPersister() Persister
}

// Transformer transforms between unstructured.Unstructured and the storage.
type Transformer interface {
	// Transform prepares the resource for persistence by removing (volatile) fields which should not be persisted.
	Transform(*unstructured.Unstructured) (*unstructured.Unstructured, error)
}
