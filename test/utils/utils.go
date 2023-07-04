// SPDX-FileCopyrightText: 2023 SAP SE or an SAP affiliate company and Gardener contributors.
//
// SPDX-License-Identifier: Apache-2.0

package utils

import (
	"context"

	"github.com/gardener/k8syncer/pkg/utils"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func FinalizeAll(ctx context.Context, c client.Client, gvk schema.GroupVersionKind, ns string) error {
	objList := &unstructured.UnstructuredList{}
	objList.SetGroupVersionKind(gvk)
	err := c.List(ctx, objList, client.InNamespace(ns))
	if err != nil {
		return err
	}

	errs := &utils.ErrorList{}

	for _, obj := range objList.Items {
		if utils.HasFinalizer(&obj) {
			old := obj.DeepCopy()
			changed := utils.RemoveFinalizer(&obj)
			if changed {
				err = c.Patch(ctx, &obj, client.MergeFrom(old))
				errs.Append(err)
			}
		}
	}

	return errs.Aggregate()
}

func ReconcileRequestFromObject(obj client.Object) reconcile.Request {
	return reconcile.Request{
		NamespacedName: client.ObjectKeyFromObject(obj),
	}
}
