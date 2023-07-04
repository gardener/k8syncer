// SPDX-FileCopyrightText: 2023 SAP SE or an SAP affiliate company and Gardener contributors.
//
// SPDX-License-Identifier: Apache-2.0

package state

import (
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/k8syncer/pkg/config"
)

type StateVerbosity string

const (
	// STATE_VERBOSITY_UNDEFINED is a dummy value for undefined verbosity.
	STATE_VERBOSITY_UNDEFINED StateVerbosity = "undefined"
	// STATE_VERBOSITY_ANY can be used to avoid MissingStateErrors.
	STATE_VERBOSITY_ANY StateVerbosity = "any"
	// STATE_VERBOSITY_GENERATION means that only the last synced generation is displayed.
	STATE_VERBOSITY_GENERATION StateVerbosity = StateVerbosity(config.STATE_VERBOSITY_GENERATION)
	// STATE_VERBOSITY_PHASE means that last synced generation and phase are displayed.
	STATE_VERBOSITY_PHASE StateVerbosity = StateVerbosity(config.STATE_VERBOSITY_PHASE)
	// STATE_VERBOSITY_DETAIL means that last synced generation, phase, and details are displayed.
	STATE_VERBOSITY_DETAIL StateVerbosity = StateVerbosity(config.STATE_VERBOSITY_DETAIL)
)

// SyncState represents the current state.
type SyncState struct {
	// Verbosity defines what is contained in the state.
	Verbosity StateVerbosity
	// Phase is the current phase.
	Phase Phase
	// LastSyncedGeneration is the last generation of the resource which has successfully been synced.
	LastSyncedGeneration int64
	// Detail can contain further details (e.g. error messages)
	Detail string
}

type Phase string

const (
	// PHASE_UNDEFINED is a dummy phase used as 'invalid' return value.
	PHASE_UNDEFINED Phase = "Undefined"
	// PHASE_PROGRESSING means that k8syncer has picked up the change and is working on it.
	PHASE_PROGRESSING Phase = "Progressing"
	// PHASE_FINISHED means that the sync is finished and there is nothing to do at the moment.
	PHASE_FINISHED Phase = "Finished"
	// PHASE_ERROR means that an error occurred and the resource will be requeued.
	PHASE_ERROR Phase = "Error"
	// PHASE_DELETING means that k8syncer has picked up a deletion and is working on it.
	PHASE_DELETING Phase = "Deleting"
	// PHASE_ERROR_DELETING is like PHASE_ERROR, but is used for errors which occur during deletion.
	PHASE_ERROR_DELETING Phase = "ErrorDeleting"
)

// PhaseFromString parses a given string into the corresponding phase.
// Returns PHASE_UNDEFINED if no phase matches.
func PhaseFromString(s string) Phase {
	phase := Phase(s)
	switch phase {
	case PHASE_PROGRESSING:
	case PHASE_FINISHED:
	case PHASE_ERROR:
	case PHASE_DELETING:
	case PHASE_ERROR_DELETING:
	default:
		return PHASE_UNDEFINED
	}
	return phase
}

// New creates a new SyncState with an undefined phase and the specified verbosity.
func New(v StateVerbosity) *SyncState {
	return &SyncState{
		Phase:     PHASE_UNDEFINED,
		Verbosity: v,
	}
}

type StateDisplay interface {
	// Type returns the type of the state display.
	Type() string
	// Read reads the state from the given object.
	// Returns a MissingStateError if not enough fields are specified for the verbosity configured at the state display.
	// Returns an InvalidStateError if one of the state fields has an invalid value.
	Read(obj client.Object) (*SyncState, StateError)
	// Write writes the given SyncState into the given resource.
	// The first return value contains the top-level fields of the resource which actually changed.
	//   e.g. "metadata", "spec", "status"
	//   This is used to determine whether the resource needs to be updated after writing the state.
	//   nil or an empty set means that the resource has not changed.
	// It will write the state according to its verbosity.
	//   Not defined for STATE_VERBOSITY_ANY and STATE_VERBOSITY_UNDEFINED.
	// Only the specified fields are written, and only if they are included in the configured verbosity.
	//   Pass the ALL_STATE_FIELDS variable to write the complete state (also filtered by verbosity).
	Write(obj client.Object, state *SyncState, fields ...*StateField) (sets.Set[string], error)
	// Verbosity returns the verbosity for which this state display is configured.
	Verbosity() StateVerbosity
}

// IsSynced returns whether the last version of the object has been synced.
// If the object's state has already been read, it can be passed as argument. Otherwise (if the given SyncState is nil),
// the given StateDisplay's Read method will be used to read the state from the object.
// It then returns state.LastSyncedGeneration == obj.Generation().
//
// Either the StateDisplay or the SyncState must be non-nil.
func IsSynced(obj client.Object, sd StateDisplay, state *SyncState) (bool, error) {
	if obj == nil {
		return false, fmt.Errorf("object must not be nil")
	}
	if state == nil {
		var err error
		state, err = sd.Read(obj)
		if err != nil {
			return false, err
		}
	}
	synced := state.LastSyncedGeneration == obj.GetGeneration()
	return synced, nil
}

// IsFinal returns true if the state is final.
func (s *SyncState) IsFinal() bool {
	return s.Phase == PHASE_FINISHED
}

func (s *SyncState) SetField(field *StateField, value any) StateError {
	if !field.hasCorrectType(value) {
		return DefaultInvalidStateError(field, value, nil)
	}
	field.set(s, value)
	return nil
}

func (s *SyncState) GetField(field *StateField) any {
	return field.get(s)
}

// IncludesPhase returns true if the verbosity includes the phase.
func (sv StateVerbosity) Includes(field *StateField) bool {
	return field.includedIn.Has(sv)
}

type StateField struct {
	// set sets the corresponding state field
	set func(state *SyncState, value any)
	// get returns the value of the corresponding state field
	get func(state *SyncState) any
	// serialize returns the value transformed into one of [bool, int64, float64, string]
	serialize func(value any) any
	// hasCorrectType returns true if the given value's type matches the field's type
	hasCorrectType func(value any) bool
	// name returns the name of the field as string
	// mainly used for logging
	name string
	// includedIn is a set of verbosities which include this field
	includedIn sets.Set[StateVerbosity]
}

func (sf *StateField) Name() string {
	return sf.name
}

var (
	STATE_FIELD_LAST_SYNCED_GENERATION = &StateField{
		set: func(state *SyncState, value any) {
			state.LastSyncedGeneration = value.(int64)
		},
		get: func(state *SyncState) any {
			return state.LastSyncedGeneration
		},
		serialize: func(value any) any {
			return value
		},
		hasCorrectType: func(value any) bool {
			_, ok := value.(int64)
			return ok
		},
		name:       "lastSyncedGeneration",
		includedIn: sets.New[StateVerbosity](STATE_VERBOSITY_GENERATION, STATE_VERBOSITY_PHASE, STATE_VERBOSITY_DETAIL),
	}

	STATE_FIELD_PHASE = &StateField{
		set: func(state *SyncState, value any) {
			state.Phase = value.(Phase)
		},
		get: func(state *SyncState) any {
			return state.Phase
		},
		serialize: func(value any) any {
			return string(value.(Phase))
		},
		hasCorrectType: func(value any) bool {
			_, ok := value.(Phase)
			return ok
		},
		name:       "phase",
		includedIn: sets.New[StateVerbosity](STATE_VERBOSITY_PHASE, STATE_VERBOSITY_DETAIL),
	}

	STATE_FIELD_DETAIL = &StateField{
		set: func(state *SyncState, value any) {
			state.Detail = value.(string)
		},
		get: func(state *SyncState) any {
			return state.Detail
		},
		serialize: func(value any) any {
			return value
		},
		hasCorrectType: func(value any) bool {
			_, ok := value.(string)
			return ok
		},
		name:       "detail",
		includedIn: sets.New[StateVerbosity](STATE_VERBOSITY_DETAIL),
	}

	ALL_STATE_FIELDS = []*StateField{STATE_FIELD_LAST_SYNCED_GENERATION, STATE_FIELD_PHASE, STATE_FIELD_DETAIL}
)

// ObjectToUnstructured converts a client.Object into an *unstructured.Unstructured.
func ObjectToUnstructured(obj client.Object) (*unstructured.Unstructured, StateError) {
	if converted, ok := obj.(*unstructured.Unstructured); ok {
		return converted, nil
	}
	data, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
	if err != nil {
		return nil, NewInternalStateError("error converting object to unstructured.Unstructured: %w", err)
	}
	return &unstructured.Unstructured{
		Object: data,
	}, nil
}
