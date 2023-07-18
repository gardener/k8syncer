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

// ParseSimpleJSONPath splits a string into single fields.
// '.' is used as separator.
// To include '.' in a field, escape it with a preceding '\'.
// To have a field end on '\', escape the final '\' with an additional '\'.
// Note that escaping happens only for '\' which directly precede a '.'.
// Examples:
// a.b.c => a b c
// a\.b.c => a.b c
// a\\.b.c => a\ b c
// a\a.b.c\ => a\a b c\
func ParseSimpleJSONPath(path string) []string {
	if path == "" {
		return []string{}
	}
	rawFields := strings.Split(path, ".")
	fields := []string{}
	add := false
	for i, f := range rawFields {
		newAdd := false
		if i < len(rawFields)-1 && strings.HasSuffix(f, "\\") {
			f = strings.TrimRight(f, "\\")
			if !strings.HasSuffix(f, "\\") {
				f = f + "."
				newAdd = true
			}
		}
		if add {
			fields[len(fields)-1] = fields[len(fields)-1] + f
		} else {
			fields = append(fields, f)
		}
		add = newAdd
	}
	return fields
}
