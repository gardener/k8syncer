// SPDX-FileCopyrightText: 2023 SAP SE or an SAP affiliate company and Gardener contributors.
//
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"fmt"
	"reflect"
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/gardener/landscaper/controller-utils/pkg/logging"
	"github.com/go-logr/logr"

	"github.com/gardener/k8syncer/pkg/config"
	"github.com/gardener/k8syncer/pkg/persist"
	"github.com/gardener/k8syncer/pkg/utils/constants"
)

// AddControllerToManager register the installation Controller in a manager.
func AddControllerToManager(baseLogger logging.Logger, mgr manager.Manager, cfg *config.K8SyncerConfiguration, syncConfig *config.SyncConfig, persisters map[string]persist.Persister) error {
	log := baseLogger.WithName(syncConfig.ID).WithValues(constants.Logging.KEY_ID, syncConfig.ID, constants.Logging.KEY_RESOURCE_GROUP, syncConfig.Resource.Group, constants.Logging.KEY_RESOURCE_VERSION, syncConfig.Resource.Version, constants.Logging.KEY_RESOURCE_KIND, syncConfig.Resource.Kind)
	c, err := NewController(mgr.GetClient(), cfg, syncConfig, persisters)
	if err != nil {
		return err
	}
	logFields := []interface{}{}
	if c.SyncConfig.Resource.Namespace != "" {
		logFields = append(logFields, constants.Logging.KEY_WATCHED_NAMESPACE, c.SyncConfig.Resource.Namespace)
	}
	if c.StateDisplay != nil {
		logFields = append(logFields, constants.Logging.KEY_STATE_DISPLAY, c.StateDisplay.Type(), constants.Logging.KEY_STATE_VERBOSITY, c.StateDisplay.Verbosity())
	}
	scNames := []string{}
	for _, sc := range c.StorageConfigs {
		scNames = append(scNames, fmt.Sprintf("%s<%s>", sc.Name(), string(sc.Type)))
	}
	logFields = append(logFields, constants.Logging.KEY_CONFIGURED_STORAGES, fmt.Sprintf("[%s]", strings.Join(scNames, ", ")))
	log.Info("sync configured", logFields...)

	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   syncConfig.Resource.Group,
		Version: syncConfig.Resource.Version,
		Kind:    syncConfig.Resource.Kind,
	})

	// only react if generation, labels, or ownerReferences changed
	preds := predicate.Or(
		predicate.GenerationChangedPredicate{},
		predicate.LabelChangedPredicate{},
		OwnerReferencesChangedPredicate{},
	)
	if syncConfig.Finalize != nil && *syncConfig.Finalize {
		// to remove finalizers, we have to get notified for deletion timestamps
		preds = predicate.Or(preds, DeletionTimestampChangedPredicate{})
	}
	if syncConfig.Resource.Namespace != "" {
		preds = predicate.And(preds, predicate.NewPredicateFuncs(func(obj client.Object) bool {
			return obj.GetNamespace() != "" && obj.GetNamespace() == syncConfig.Resource.Namespace
		}))
	}

	return builder.ControllerManagedBy(mgr).
		For(u).
		Named(strings.ToLower(syncConfig.ID)).
		WithEventFilter(preds).
		WithLogConstructor(func(r *reconcile.Request) logr.Logger { return log.Logr() }).
		Complete(c)
}

// OwnerReferencesChangedPredicate reacts to changes of the owner references.
type OwnerReferencesChangedPredicate struct {
	predicate.Funcs
}

func (OwnerReferencesChangedPredicate) Update(e event.UpdateEvent) bool {
	if e.ObjectOld == nil {
		return false
	}
	if e.ObjectNew == nil {
		return false
	}
	oldOwners := e.ObjectOld.GetOwnerReferences()
	newOwners := e.ObjectNew.GetOwnerReferences()
	return !(len(newOwners) == len(oldOwners) && reflect.DeepEqual(newOwners, oldOwners))
}

// DeletionTimestampChangedPredicate reacts to changes of the deletion timestamp.
type DeletionTimestampChangedPredicate struct {
	predicate.Funcs
}

func (DeletionTimestampChangedPredicate) Update(e event.UpdateEvent) bool {
	if e.ObjectOld == nil {
		return false
	}
	if e.ObjectNew == nil {
		return false
	}
	oldDel := e.ObjectOld.GetDeletionTimestamp()
	newDel := e.ObjectNew.GetDeletionTimestamp()
	return !reflect.DeepEqual(newDel, oldDel)
}
