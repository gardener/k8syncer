// SPDX-FileCopyrightText: 2023 SAP SE or an SAP affiliate company and Gardener contributors.
//
// SPDX-License-Identifier: Apache-2.0

package state

import (
	"fmt"

	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/k8syncer/pkg/utils/constants"
)

var _ StateDisplay = &AnnotationStateDisplay{}

type AnnotationStateDisplay struct {
	fieldAnnotations map[string]string

	verbosity StateVerbosity
}

func NewAnnotationStateDisplay(v StateVerbosity) *AnnotationStateDisplay {
	return &AnnotationStateDisplay{
		fieldAnnotations: map[string]string{
			STATE_FIELD_LAST_SYNCED_GENERATION.name: constants.ANNOTATION_LAST_SYNCED_GENERATION,
			STATE_FIELD_PHASE.name:                  constants.ANNOTATION_PHASE,
			STATE_FIELD_DETAIL.name:                 constants.ANNOTATION_DETAIL,
		},
		verbosity: v,
	}
}

func (*AnnotationStateDisplay) Type() string {
	return "annotation"
}

func (asd *AnnotationStateDisplay) Read(obj client.Object) (*SyncState, StateError) {
	if asd.verbosity == STATE_VERBOSITY_UNDEFINED || asd.verbosity == StateVerbosity("") {
		return nil, NewInternalStateError("invalid desired verbosity: %s", string(asd.verbosity))
	}
	if obj == nil {
		return nil, NewInternalStateError("object must not be nil")
	}
	ann := obj.GetAnnotations()
	if ann == nil {
		return nil, DefaultMissingStateError(asd.verbosity)
	}
	state := &SyncState{}
	for _, field := range ALL_STATE_FIELDS {
		if !asd.verbosity.Includes(field) {
			continue
		}
		value, exists := ann[asd.fieldAnnotations[field.Name()]]
		if !exists {
			return nil, DefaultMissingStateError(asd.verbosity, field)
		}
		err := state.SetField(field, value)
		if err != nil {
			return nil, err
		}
	}
	return state, nil
}

func (asd *AnnotationStateDisplay) Write(obj client.Object, state *SyncState, fields ...*StateField) (sets.Set[string], error) {
	if state == nil || state.Verbosity == STATE_VERBOSITY_UNDEFINED || state.Verbosity == STATE_VERBOSITY_ANY || state.Verbosity == StateVerbosity("") {
		return nil, NewInternalStateError("invalid SyncState object, either nil or with invalid verbosity")
	}
	if obj == nil {
		return nil, NewInternalStateError("object must not be nil")
	}
	ann := obj.GetAnnotations()
	if ann == nil {
		ann = map[string]string{}
	}
	changed := false
	for _, field := range fields {
		if !state.Verbosity.Includes(field) {
			continue
		}
		oldValue, found := ann[asd.fieldAnnotations[field.Name()]]
		newValue := fmt.Sprint(field.serialize(state.GetField(field)))
		if found {
			// the object already has a value for this state field
			if newValue == oldValue {
				// value currently stored in the object is the same as we would write, no need to try it
				continue
			}
			// there is a different value in the object, we will change it
			changed = true
		}
		ann[asd.fieldAnnotations[field.Name()]] = newValue
	}
	obj.SetAnnotations(ann)
	return asd.changeList(changed), nil
}

func (asd *AnnotationStateDisplay) Verbosity() StateVerbosity {
	return asd.verbosity
}

func (asd *AnnotationStateDisplay) changeList(changed bool) sets.Set[string] {
	if changed {
		return sets.New[string]("metadata")
	}
	return nil
}
