// SPDX-FileCopyrightText: 2023 SAP SE or an SAP affiliate company and Gardener contributors.
//
// SPDX-License-Identifier: Apache-2.0

package mock

import (
	"errors"
	"fmt"
	"reflect"
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/gardener/k8syncer/pkg/persist"
	"github.com/gardener/k8syncer/pkg/utils"
)

type MockedCall struct {
	callType                 callName
	name, namespace, subPath *string
	gvk                      *schema.GroupVersionKind
	resource                 *unstructured.Unstructured
	t                        persist.Transformer
	expectedReturn           *MockedReturn
}

type MockedReturn struct {
	resource        *unstructured.Unstructured
	existsOrChanged *bool
	err             error
}

var ErrNotInTestMode = errors.New("mock persister is not in test mode")

type ErrUnexpectedCall struct {
	diffs                    map[callArgument]compareDifference
	onlyOneNil               bool
	expectedCall, actualCall callName
}

func newUnexpectedCallError(onlyOneNil bool, fields ...any) *ErrUnexpectedCall {
	res := &ErrUnexpectedCall{
		onlyOneNil: onlyOneNil,
	}
	if len(fields) == 1 {
		diffs, ok := fields[0].(map[callArgument]compareDifference)
		if ok {
			res.diffs = diffs
			return res
		}
	} else if len(fields) == 2 {
		expectedCall, ok := fields[0].(callName)
		if ok {
			actualCall, ok := fields[1].(callName)
			if ok {
				res.expectedCall = expectedCall
				res.actualCall = actualCall
				return res
			}
			panic(fmt.Errorf("invalid argument to newUnexpectedCallError: first field argument is callName, second one is expected to be of this type too"))
		}
	}
	res.diffs = map[callArgument]compareDifference{}
	for idx := 0; idx < len(fields); idx++ {
		arg, ok := fields[idx].(callArgument)
		if !ok {
			panic(fmt.Errorf("invalid argument to newUnexpectedCallError: expected field %d to be a callArgument", idx))
		}
		idx++
		if idx >= len(fields) {
			panic(fmt.Errorf("wrong amount of arguments to newUnexpectedCallError: argument name field %d must be followed by another value", idx))
		}
		diff, ok := fields[idx].(compareDifference)
		if !ok {
			idx++
			if idx >= len(fields) {
				panic(fmt.Errorf("wrong amount of arguments to newUnexpectedCallError: non-diff field %d must be followed by another value", idx))
			}
			diff = compareDifference{
				expected: fields[idx-1],
				actual:   fields[idx],
			}
		}
		res.diffs[arg] = diff
	}
	return res
}

func (e *ErrUnexpectedCall) Error() string {
	if e.onlyOneNil {
		return "only one of two compared calls is nil"
	}
	sb := strings.Builder{}
	sb.WriteString("the following arguments differ in the expected versus the actuall call:")
	for k, v := range e.diffs {
		sb.WriteString(fmt.Sprintf("\n  %s:\n    expected: %v\n    actual: %v", string(k), v.expected, v.actual))
	}
	return sb.String()
}

type ErrUnexpectedReturn struct {
	onlyOneNil, resourceDiffers, boolDiffers, errDiffers bool
	expectedResource, actualResource                     *unstructured.Unstructured
	expectedBool, actualBool                             bool
	expectedErr, actualErr                               error
}

func (e *ErrUnexpectedReturn) Error() string {
	if e.onlyOneNil {
		return "only one of two compared return values is nil"
	}
	sb := strings.Builder{}
	sb.WriteString("the following return values differ from the expected ones:")
	if e.resourceDiffers {
		sb.WriteString(fmt.Sprintf("\n  returned resource:\n    expected: %v\n    actual: %v", e.expectedResource.Object, e.actualResource.Object))
	}
	if e.boolDiffers {
		sb.WriteString(fmt.Sprintf("\n  returned changed/exist:\n    expected: %v\n    actual: %v", e.expectedBool, e.actualBool))
	}
	if e.errDiffers {
		sb.WriteString(fmt.Sprintf("\n  returned error:\n    expected: %v\n    actual: %v", e.expectedErr, e.actualErr))
	}
	return sb.String()
}

type callName string

const (
	callName_Exists  = callName("Exists")
	callName_Get     = callName("Get")
	callName_Persist = callName("Persist")
	callName_Delete  = callName("Delete")
)

type callArgument string

const (
	callArgument_name        = callArgument("name")
	callArgument_namespace   = callArgument("namespace")
	callArgument_subPath     = callArgument("subPath")
	callArgument_gvk         = callArgument("gvk")
	callArgument_resource    = callArgument("resource")
	callArgument_transformer = callArgument("transformer")
	callArgument_data        = callArgument("data")
)

type compareDifference struct {
	expected, actual interface{}
}

func (p *MockPersister) ExpectCall(expected *MockedCall, expectedReturn *MockedReturn) {
	if expectedReturn != nil {
		expected.expectedReturn = expectedReturn
	}
	p.expectedCalls.Push(expected)
}

func (p *MockPersister) IsExpectingCalls() bool {
	return p.expectedCalls.Size() > 0
}

func (p *MockPersister) ClearExpectedCalls() []*MockedCall {
	res := make([]*MockedCall, p.expectedCalls.Size())
	for i := range res {
		res[i], _ = p.expectedCalls.Poll()
	}
	return res
}

func (p *MockPersister) compareExpectedVsActualCall(actual *MockedCall) error {
	if p.expectedCalls == nil {
		return ErrNotInTestMode
	}
	expected, err := p.expectedCalls.Poll()
	if err != nil {
		if err == utils.ErrQueueEmpty {
			return fmt.Errorf("got call %v, but didn't expect any call", actual)
		}
	}
	return compareCalls(expected, actual)
}

// compareCalls compares two calls and returns an error if they differ
func compareCalls(expected, actual *MockedCall) error {
	if expected == nil {
		if actual == nil {
			return nil
		}
		return newUnexpectedCallError(true)
	}
	if actual == nil {
		return newUnexpectedCallError(true)
	}

	if expected.callType != actual.callType {
		return newUnexpectedCallError(false, expected.callType, actual.callType)
	}

	diffs := map[callArgument]compareDifference{}

	if !reflect.DeepEqual(expected.name, actual.name) {
		diffs[callArgument_name] = compareDifference{expected: expected.name, actual: actual.name}
	}
	if !reflect.DeepEqual(expected.namespace, actual.namespace) {
		diffs[callArgument_namespace] = compareDifference{expected: expected.namespace, actual: actual.namespace}
	}
	if !reflect.DeepEqual(expected.subPath, actual.subPath) {
		diffs[callArgument_subPath] = compareDifference{expected: expected.subPath, actual: actual.subPath}
	}
	if !reflect.DeepEqual(expected.gvk, actual.gvk) {
		diffs[callArgument_gvk] = compareDifference{expected: expected.gvk, actual: actual.gvk}
	}
	if expected.resource != nil && actual.resource != nil {
		expectedResourceVersion, expFound, expErr := unstructured.NestedString(expected.resource.UnstructuredContent(), "metadata", "resourceVersion")
		actualResourceVersion, actFound, actErr := unstructured.NestedString(actual.resource.UnstructuredContent(), "metadata", "resourceVersion")
		if expErr == nil && actErr == nil && expFound && actFound && expectedResourceVersion != actualResourceVersion {
			// set expected's resource version to actual's, as it doesn't matter for the comparison, but might cause problems (setting the finalizer increases the resource version)
			unstructured.SetNestedField(expected.resource.Object, actualResourceVersion, "metadata", "resourceVersion") //nolint:errcheck
		}
	}
	if !reflect.DeepEqual(expected.resource, actual.resource) {
		diffs[callArgument_resource] = compareDifference{expected: expected.resource, actual: actual.resource}
	}
	if len(diffs) > 0 {
		return newUnexpectedCallError(false, diffs)
	}

	return nil
}

// compareReturns compares the expected to the actual return value of a call and returns an error if they differ
func compareReturns(expected, actual *MockedReturn) error {
	err := &ErrUnexpectedReturn{}
	errOccurred := false
	if expected == nil {
		if actual == nil {
			return nil
		}
		err.onlyOneNil = true
		return err
	}
	if actual == nil {
		err.onlyOneNil = true
		return err
	}

	if !reflect.DeepEqual(expected.resource, actual.resource) {
		errOccurred = true
		err.resourceDiffers = true
		err.expectedResource = expected.resource
		err.actualResource = actual.resource
	}
	if !reflect.DeepEqual(expected.existsOrChanged, actual.existsOrChanged) {
		errOccurred = true
		err.boolDiffers = true
		err.expectedBool = *expected.existsOrChanged
		err.actualBool = *actual.existsOrChanged
	}
	if expected.err != actual.err {
		errOccurred = true
		err.errDiffers = true
		err.expectedErr = expected.err
		err.actualErr = actual.err
	}
	if errOccurred {
		return err
	}
	return nil
}

func MockedExistsCall(name, namespace string, gvk schema.GroupVersionKind, subPath string) *MockedCall {
	return &MockedCall{
		callType:  callName_Exists,
		name:      &name,
		namespace: &namespace,
		gvk:       &gvk,
		subPath:   &subPath,
	}
}

func MockedExistsReturn(exists bool, err error) *MockedReturn {
	return &MockedReturn{
		existsOrChanged: &exists,
		err:             err,
	}
}

func MockedGetCall(name, namespace string, gvk schema.GroupVersionKind, subPath string) *MockedCall {
	return &MockedCall{
		callType:  callName_Get,
		name:      &name,
		namespace: &namespace,
		gvk:       &gvk,
		subPath:   &subPath,
	}
}

func MockedGetReturn(resource *unstructured.Unstructured, err error) *MockedReturn {
	return &MockedReturn{
		resource: resource,
		err:      err,
	}
}

func MockedPersistCall(resource *unstructured.Unstructured, t persist.Transformer, subPath string) *MockedCall {
	return &MockedCall{
		callType: callName_Persist,
		resource: resource,
		t:        t,
		subPath:  &subPath,
	}
}

func MockedPersistReturn(resource *unstructured.Unstructured, changed bool, err error) *MockedReturn {
	return &MockedReturn{
		resource:        resource,
		existsOrChanged: &changed,
		err:             err,
	}
}

func MockedDeleteCall(name, namespace string, gvk schema.GroupVersionKind, subPath string) *MockedCall {
	return &MockedCall{
		callType:  callName_Persist,
		name:      &name,
		namespace: &namespace,
		gvk:       &gvk,
		subPath:   &subPath,
	}
}

func MockedDeleteReturn(err error) *MockedReturn {
	return &MockedReturn{
		err: err,
	}
}
