// SPDX-FileCopyrightText: 2023 SAP SE or an SAP affiliate company and Gardener contributors.
//
// SPDX-License-Identifier: Apache-2.0

package state

import (
	"fmt"
	"reflect"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/k8syncer/pkg/utils"
)

var _ StateDisplay = &StatusStateDisplay{}

type StatusStateDisplay struct {
	fieldStatusPaths map[string][]string
	verbosity        StateVerbosity
}

func NewStatusStateDisplay(lastSyncedGenerationPath, phasePath, detailPath string, v StateVerbosity) *StatusStateDisplay {
	return &StatusStateDisplay{
		fieldStatusPaths: map[string][]string{
			STATE_FIELD_LAST_SYNCED_GENERATION.name: utils.ParseSimpleJSONPath(lastSyncedGenerationPath),
			STATE_FIELD_PHASE.name:                  utils.ParseSimpleJSONPath(phasePath),
			STATE_FIELD_DETAIL.name:                 utils.ParseSimpleJSONPath(detailPath),
		},
		verbosity: v,
	}
}

func (*StatusStateDisplay) Type() string {
	return "status"
}

func (ssd *StatusStateDisplay) Read(rawObj client.Object) (*SyncState, StateError) {
	if ssd.verbosity == STATE_VERBOSITY_UNDEFINED || ssd.verbosity == StateVerbosity("") {
		return nil, NewInternalStateError("invalid desired verbosity: %s", string(ssd.verbosity))
	}
	if rawObj == nil {
		return nil, NewInternalStateError("object must not be nil")
	}
	obj, serr := ObjectToUnstructured(rawObj)
	if serr != nil {
		return nil, serr
	}
	status, found, err := unstructured.NestedMap(obj.UnstructuredContent(), "status")
	if err != nil {
		return nil, NewMissingStateError("error getting status: %w", err)
	}
	if !found || status == nil {
		return nil, DefaultMissingStateError(ssd.verbosity)
	}
	state := &SyncState{}
	for _, field := range ALL_STATE_FIELDS {
		if !ssd.verbosity.Includes(field) {
			continue
		}
		value, exists, err := unstructured.NestedFieldCopy(status, ssd.fieldStatusPaths[field.Name()]...)
		if err != nil {
			return nil, DefaultReadStateError(err)
		}
		if !exists {
			return nil, DefaultMissingStateError(ssd.verbosity, field)
		}
		err2 := state.SetField(field, value)
		if err2 != nil {
			return nil, err2
		}
	}
	return state, nil
}

func (ssd *StatusStateDisplay) Write(rawObj client.Object, state *SyncState, fields ...*StateField) (sets.Set[string], error) {
	if state == nil || state.Verbosity == STATE_VERBOSITY_UNDEFINED || state.Verbosity == STATE_VERBOSITY_ANY || state.Verbosity == StateVerbosity("") {
		return nil, NewInternalStateError("invalid SyncState object, either nil or with invalid verbosity")
	}
	if rawObj == nil {
		return nil, NewInternalStateError("object must not be nil")
	}
	obj, serr := ObjectToUnstructured(rawObj)
	if serr != nil {
		return nil, serr
	}
	status, found, err := unstructured.NestedMap(obj.UnstructuredContent(), "status")
	if err != nil {
		return nil, NewMissingStateError("error getting status: %w", err)
	}
	if !found || status == nil {
		status = map[string]interface{}{}
	}
	changed := false
	for _, field := range fields {
		if !state.Verbosity.Includes(field) {
			continue
		}
		oldValue, found, err := unstructured.NestedFieldCopy(status, ssd.fieldStatusPaths[field.Name()]...)
		if err != nil {
			return ssd.changeList(changed), DefaultReadStateError(fmt.Errorf("error reading field '%s' from resource before writing state: %w", field.Name(), err))
		}
		newValue := field.serialize(state.GetField(field))
		if found {
			// the object already has a value for this state field
			if reflect.DeepEqual(newValue, oldValue) {
				// value currently stored in the object is the same as we would write, no need to try it
				continue
			}
		}
		// there is either none or a different value in the object, we will change it
		changed = true
		err = unstructured.SetNestedField(status, newValue, ssd.fieldStatusPaths[field.Name()]...)
		if err != nil {
			return ssd.changeList(changed), DefaultWriteStateError(err)
		}
	}
	err = unstructured.SetNestedField(obj.UnstructuredContent(), status, "status")
	if err != nil {
		return ssd.changeList(changed), DefaultWriteStateError(err)
	}
	obj.UnstructuredContent()["status"] = status
	return ssd.changeList(changed), nil
}

func (ssd *StatusStateDisplay) Verbosity() StateVerbosity {
	return ssd.verbosity
}

func (ssd *StatusStateDisplay) changeList(changed bool) sets.Set[string] {
	if changed {
		return sets.New[string]("status")
	}
	return nil
}
