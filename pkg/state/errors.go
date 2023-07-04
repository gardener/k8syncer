// SPDX-FileCopyrightText: 2023 SAP SE or an SAP affiliate company and Gardener contributors.
//
// SPDX-License-Identifier: Apache-2.0

package state

import (
	"fmt"
	"strings"
)

const (
	ERR_REASON_MISSING_STATE = "MissingState"
	ERR_REASON_INVALID_STATE = "InvalidState"
	ERR_REASON_INTERNAL      = "InternalError"
	ERR_REASON_READ_FAILED   = "ReadFailed"
	ERR_REASON_WRITE_FAILED  = "WriteFailed"
)

// StateError is the parent error interface for state handling errors.
type StateError interface {
	error
	Reason() string

	IsMissingStateError() bool
	IsInvalidStateError() bool
	IsInternalStateError() bool
	IsReadStateError() bool
	IsWriteStateError() bool
}

var (
	_ StateError = &abstractStateError{}
	_ StateError = &MissingStateError{}
	_ StateError = &InvalidStateError{}
	_ StateError = &InternalStateError{}
	_ StateError = &ReadStateError{}
	_ StateError = &WriteStateError{}
)

type abstractStateError struct {
	error
	reason string
}

func newStateError(reason, msg string, values ...any) *abstractStateError {
	return &abstractStateError{
		error:  fmt.Errorf(msg, values...),
		reason: reason,
	}
}

func (e *abstractStateError) Reason() string {
	return e.reason
}

func (e *abstractStateError) IsMissingStateError() bool {
	return e.Reason() == ERR_REASON_MISSING_STATE
}
func (e *abstractStateError) IsInvalidStateError() bool {
	return e.Reason() == ERR_REASON_INVALID_STATE
}
func (e *abstractStateError) IsInternalStateError() bool {
	return e.Reason() == ERR_REASON_INTERNAL
}
func (e *abstractStateError) IsReadStateError() bool {
	return e.Reason() == ERR_REASON_READ_FAILED
}
func (e *abstractStateError) IsWriteStateError() bool {
	return e.Reason() == ERR_REASON_WRITE_FAILED
}

func IsMissingStateError(err error) bool {
	se, ok := err.(StateError)
	if !ok {
		return false
	}
	return se.IsMissingStateError()
}
func IsInvalidStateError(err error) bool {
	se, ok := err.(StateError)
	if !ok {
		return false
	}
	return se.IsInvalidStateError()
}
func IsInternalStateError(err error) bool {
	se, ok := err.(StateError)
	if !ok {
		return false
	}
	return se.Reason() == ERR_REASON_INTERNAL
}
func IsReadStateError(err error) bool {
	se, ok := err.(StateError)
	if !ok {
		return false
	}
	return se.IsReadStateError()
}
func IsWriteStateError(err error) bool {
	se, ok := err.(StateError)
	if !ok {
		return false
	}
	return se.IsWriteStateError()
}

// MissingStateError is returned if a part of the state, which is required by its verbosity, is missing.
type MissingStateError struct{ abstractStateError }

// InvalidStateError is returned if a state cannot be read due to an invalid format of one or more state values.
type InvalidStateError struct{ abstractStateError }

// InternalStateError is only returned if the developer did something wrong ;-)
type InternalStateError struct{ abstractStateError }

// ReadStateError is returned if reading the state from the resource failed for some other reason than it being missing or invalid.
type ReadStateError struct{ abstractStateError }

// WriteStateError is returned if writing the state to the resource failed.
type WriteStateError struct{ abstractStateError }

func NewMissingStateError(msg string, values ...any) *MissingStateError {
	return &MissingStateError{*newStateError(ERR_REASON_MISSING_STATE, msg, values...)}
}

func DefaultMissingStateError(v StateVerbosity, missingFields ...*StateField) *MissingStateError {
	sb := strings.Builder{}
	if v != STATE_VERBOSITY_ANY && v != STATE_VERBOSITY_UNDEFINED && v != StateVerbosity("") {
		sb.WriteString("state verbosity is '")
		sb.WriteString(string(v))
		sb.WriteString("', but ")
	}
	if len(missingFields) > 0 {
		missingFieldsAsString := make([]string, len(missingFields))
		for idx := range missingFields {
			missingFieldsAsString[idx] = missingFields[idx].Name()
		}
		sb.WriteString("following fields are missing in the state: ")
		sb.WriteString(strings.Join(missingFieldsAsString, ", "))
	} else {
		sb.WriteString("the state is missing")
	}
	return NewMissingStateError(sb.String())
}

func NewInvalidStateError(msg string, values ...any) *InvalidStateError {
	return &InvalidStateError{*newStateError(ERR_REASON_INVALID_STATE, msg, values...)}
}

func DefaultInvalidStateError(field *StateField, fieldValue interface{}, err error) *InvalidStateError {
	sb := strings.Builder{}
	sb.WriteString("invalid value '")
	sb.WriteString(fmt.Sprint(fieldValue))
	sb.WriteString("' for state field '")
	sb.WriteString(field.Name())
	sb.WriteString("'")
	if err != nil {
		sb.WriteString(": ")
		sb.WriteString(err.Error())
	}
	return NewInvalidStateError(sb.String())
}

func NewInternalStateError(msg string, values ...any) *InternalStateError {
	return &InternalStateError{*newStateError(ERR_REASON_INTERNAL, msg, values...)}
}

func NewReadStateError(msg string, values ...any) *ReadStateError {
	return &ReadStateError{*newStateError(ERR_REASON_READ_FAILED, msg, values...)}
}

func DefaultReadStateError(err error) *ReadStateError {
	return NewReadStateError("error reading state: %w", err)
}

func NewWriteStateError(msg string, values ...any) *WriteStateError {
	return &WriteStateError{*newStateError(ERR_REASON_WRITE_FAILED, msg, values...)}
}

func DefaultWriteStateError(err error) *WriteStateError {
	return NewWriteStateError("error writing state: %w", err)
}
