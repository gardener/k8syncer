// SPDX-FileCopyrightText: 2023 SAP SE or an SAP affiliate company and Gardener contributors.
//
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"fmt"

	"github.com/gardener/landscaper/controller-utils/pkg/logging"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/k8syncer/pkg/state"
	"github.com/gardener/k8syncer/pkg/utils/constants"
)

const (
	retryLimit = 1
)

// updateStateOnResource sets given state fields on the resource and updates it, with retrying in case of a conflict.
// State fields and their values are expected as key-value-pairs, similar to how the logger does it.
//
//	Every even-numbered index has to be of type *state.StateField, and it has to be followed by a fitting value for that state field.
//	State fields which are not included in the state display's configured verbosity are ignored.
//	Misconfiguring this list does not cause an error, but a panic.
//
// If c.StateDisplay is nil, this is a no-op.
func (c *Controller) updateStateOnResource(ctx context.Context, obj *unstructured.Unstructured, fieldValuePairs ...any) error {
	if c.StateDisplay == nil {
		return nil
	}
	log := logging.FromContextOrDiscard(ctx)
	s := state.New(c.StateDisplay.Verbosity())
	fieldsToUpdate := sets.New[*state.StateField]()
	logFields := []interface{}{}

	if len(fieldValuePairs)%2 != 0 {
		panic(fmt.Errorf("fieldValuePairs should always have an even number of elements"))
	}
	for i := 0; i < len(fieldValuePairs); i += 2 {
		sf, ok := fieldValuePairs[i].(*state.StateField)
		if !ok {
			panic(fmt.Errorf("every even-numbered index of fieldValuePairs is supposed to contain a *state.StateField, but at %d it is %v", i, fieldValuePairs[i]))
		}
		if !c.StateDisplay.Verbosity().Includes(sf) {
			// field is not included in configured verbosity, ignore it
			continue
		}
		value := fieldValuePairs[i+1]
		logFields = append(logFields, sf.Name(), value)
		err := s.SetField(sf, value)
		if err != nil {
			panic(fmt.Errorf("fieldValuePair at indices %d and %d has an invalid value for the specified field %s", i, i+1, sf.Name()))
		}
		fieldsToUpdate.Insert(sf)
	}

	logFields = append(logFields, constants.Logging.KEY_STATE_DISPLAY, c.StateDisplay.Type(), constants.Logging.KEY_STATE_VERBOSITY, string(c.StateDisplay.Verbosity()))
	log.Debug("Updating resource state", logFields...)

	return c.updateWithRetry(ctx, obj, func(obj *unstructured.Unstructured) (sets.Set[string], error) {
		changedFields, err := c.StateDisplay.Write(obj, s, fieldsToUpdate.UnsortedList()...)
		if err != nil {
			return changedFields, fmt.Errorf("error writing state for object (using state type '%s'): %w", string(c.SyncConfig.State.Type), err)
		}
		return changedFields, nil
	}, retryLimit)
}

// updateWithRetry takes an idempotent(!) change function and applies it to the object.
// The change function is expected to return a list of top-level fields of the object, which it changed.
//
//	The function only differentiates between 'status' and anything else at the moment.
//	If this list is nil or empty, the resource is not updated in the cluster.
//
// Applying the update will be retried for up to maxRetries times, but only for conflict errors.
// All other errors cause the function to abort and return an error.
func (c *Controller) updateWithRetry(ctx context.Context, obj *unstructured.Unstructured, changeFunc func(obj *unstructured.Unstructured) (sets.Set[string], error), maxRetries int) error {
	success := false
	for tries := 0; !success; tries++ {
		if tries > 0 {
			// this is not the first try
			// fetch object from cluster, as client.Update does not update object in case of error
			_ = c.Client.Get(ctx, client.ObjectKeyFromObject(obj), obj)
			// ignore error, as failing to get the object will likely result in failing to update the object which will be returned if it happens too often
		}
		// try to write the state
		changedFields, err := changeFunc(obj)
		if err != nil {
			return err
		}
		if len(changedFields) == 0 {
			// nothing has changed in the resource, no need to update it
			return nil
		}
		if changedFields.Has("status") {
			// update status subresource
			err := c.Client.Status().Update(ctx, obj)
			if err != nil {
				if !apierrors.IsConflict(err) || tries >= maxRetries {
					// only retry update conflicts
					return fmt.Errorf("error updating object: %w", err)
				}
				success = false
				continue
			}
			success = true
			delete(changedFields, "status")
		}
		if len(changedFields) > 0 {
			// something else except for status has changed
			// update resource
			err := c.Client.Update(ctx, obj)
			if err != nil {
				if !apierrors.IsConflict(err) || tries >= maxRetries {
					// only retry update conflicts
					return fmt.Errorf("error updating object: %w", err)
				}
				success = false
				continue
			}
			success = true
		}
	}
	return nil

}
