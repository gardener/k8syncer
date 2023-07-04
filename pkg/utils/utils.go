// SPDX-FileCopyrightText: 2023 SAP SE or an SAP affiliate company and Gardener contributors.
//
// SPDX-License-Identifier: Apache-2.0

package utils

import (
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/gardener/k8syncer/pkg/utils/constants"
)

// GVKToString reverts a GroupVersionKind back to a string in the format <resource>.<version>.<group>
// If suppressDotSuffix is set to true, the trailing dot will be cut off if the group is empty.
func GVKToString(gvk schema.GroupVersionKind, suppressDotSuffix bool) string {
	res := fmt.Sprintf("%s.%s.%s", strings.ToLower(gvk.Kind), gvk.Version, gvk.Group)
	if suppressDotSuffix {
		res = strings.TrimSuffix(res, ".")
	}
	return res
}

// Ptr returns a pointer to the given object.
func Ptr[T any](value T) *T {
	return &value
}

type ErrorList struct {
	Errs []error
}

func NewErrorList(errs ...error) *ErrorList {
	res := &ErrorList{
		Errs: []error{},
	}
	return res.Append(errs...)
}

func (el *ErrorList) Error() string {
	if len(el.Errs) == 0 {
		return ""
	} else if len(el.Errs) == 1 {
		return el.Errs[0].Error()
	}
	sb := strings.Builder{}
	sb.WriteString("multiple errors occurred:")
	for _, e := range el.Errs {
		sb.WriteString("\n")
		sb.WriteString(e.Error())
	}
	return sb.String()
}

// Append appends all given errors to the ErrorList.
// This modifies the receiver object.
// nil pointers in the arguments are ignored.
// Returns the receiver for chaining.
func (el *ErrorList) Append(errs ...error) *ErrorList {
	for _, e := range errs {
		if e != nil {
			el.Errs = append(el.Errs, e)
		}
	}
	return el
}

// AddFinalizer adds a k8syncer finalizer to the object, if it doesn't already have one.
// Returns true if the finalizers changed.
func AddFinalizer(obj client.Object) bool {
	return controllerutil.AddFinalizer(obj, constants.K8SYNCER_FINALIZER)
}

// HasFinalizer returns true if the given object has a k8syncer finalizer.
func HasFinalizer(obj client.Object) bool {
	return controllerutil.ContainsFinalizer(obj, constants.K8SYNCER_FINALIZER)
}

// RemoveFinalizer removes all k8syncer finalizers from the object.
// Returns true if the finalizers changed.
func RemoveFinalizer(obj client.Object) bool {
	return controllerutil.RemoveFinalizer(obj, constants.K8SYNCER_FINALIZER)
}
