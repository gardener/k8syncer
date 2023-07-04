// SPDX-FileCopyrightText: 2023 SAP SE or an SAP affiliate company and Gardener contributors.
//
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"bytes"
	"context"
	"fmt"

	"github.com/gardener/landscaper/controller-utils/pkg/logging"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/gardener/k8syncer/pkg/config"
	"github.com/gardener/k8syncer/pkg/persist"
	"github.com/gardener/k8syncer/pkg/persist/transformers"
	"github.com/gardener/k8syncer/pkg/state"
	"github.com/gardener/k8syncer/pkg/utils"
	"github.com/gardener/k8syncer/pkg/utils/constants"
)

var basicTransformer = transformers.NewBasic() // will probably be configurable somehow in the future

type Controller struct {
	Client         client.Client
	Config         *config.K8SyncerConfiguration
	SyncConfig     *config.SyncConfig
	StorageConfigs []*StorageConfiguration
	GVK            schema.GroupVersionKind
	StateDisplay   state.StateDisplay
}

// StorageConfiguration is a helper struct to bundle a storage reference with its definition.
type StorageConfiguration struct {
	*config.StorageReference
	*config.StorageDefinition
	Persister   persist.Persister
	Transformer persist.ResourceTransformer
}

func (sc *StorageConfiguration) Name() string {
	// doesn't matter whether sc.StorageReference.Name or sc.StorageDefinition.Name is returned, they should always be identical
	return sc.StorageReference.Name
}

func NewController(client client.Client, cfg *config.K8SyncerConfiguration, syncConfig *config.SyncConfig, persisters map[string]persist.Persister) (*Controller, error) {
	ctrl := &Controller{
		Client:     client,
		Config:     cfg,
		SyncConfig: syncConfig,
	}

	// set GVK
	ctrl.GVK = schema.GroupVersionKind{
		Group:   syncConfig.Resource.Group,
		Version: syncConfig.Resource.Version,
		Kind:    syncConfig.Resource.Kind,
	}

	// configure state display, if any
	if syncConfig.State != nil && syncConfig.State.Type != config.STATE_TYPE_NONE {
		sdCfg := syncConfig.State
		switch sdCfg.Type {
		case config.STATE_TYPE_ANNOTATION:
			ctrl.StateDisplay = state.NewAnnotationStateDisplay(state.StateVerbosity(sdCfg.Verbosity))
		case config.STATE_TYPE_STATUS:
			stCfg := sdCfg.StatusStateConfig
			if stCfg == nil {
				// should be prevented by validation
				return nil, fmt.Errorf("missing state configuration for state type '%s' in sync configuration with id %s", string(syncConfig.State.Type), syncConfig.ID)
			}
			ctrl.StateDisplay = state.NewStatusStateDisplay(stCfg.GenerationPath, stCfg.PhasePath, stCfg.DetailPath, state.StateVerbosity(sdCfg.Verbosity))
		default:
			// should not happen, as this check is already part of the config validation
			return nil, fmt.Errorf("unknown state type '%s' in sync configuration with id %s", string(syncConfig.State.Type), syncConfig.ID)
		}
	}

	// build storage configurations
	ctrl.StorageConfigs = make([]*StorageConfiguration, len(syncConfig.StorageRefs))
	for idx, stRef := range syncConfig.StorageRefs {
		var stCfg *StorageConfiguration
		found := false
		for _, stDef := range cfg.StorageDefinitions {
			if stDef.Name == stRef.Name {
				found = true
				stCfg = &StorageConfiguration{stRef, stDef, persisters[stDef.Name], basicTransformer}
				break
			}
		}
		if !found {
			// should not happen, as this check is already part of the config validation
			return nil, fmt.Errorf("unable to find storage definition '%s', which is referenced at index %d in sync configuration with id %s", syncConfig.StorageRefs[idx].Name, idx, syncConfig.ID)
		}
		ctrl.StorageConfigs[idx] = stCfg
	}

	return ctrl, nil
}

// Reconcile performs a full reconciliation for the object referred to by the Request.
// The Controller will requeue the Request to be processed again if an error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (c *Controller) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	log, ctx := logging.FromContextOrNew(ctx, nil, constants.Logging.KEY_RESOURCE_NAME, req.Name, constants.Logging.KEY_RESOURCE_NAMESPACE, req.Namespace)
	log.Info("Starting reconcile")

	obj := &unstructured.Unstructured{}
	obj.SetName(req.Name)
	obj.SetNamespace(req.Namespace)
	obj.SetGroupVersionKind(c.GVK)
	err := c.Client.Get(ctx, client.ObjectKeyFromObject(obj), obj)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return reconcile.Result{}, c.handleDelete(ctx, obj)
		}
		return reconcile.Result{}, fmt.Errorf("error fetching resource from cluster: %w", err)
	}

	if del := obj.GetDeletionTimestamp(); del != nil && !del.IsZero() {
		return reconcile.Result{}, c.handleDelete(ctx, obj)
	}
	return reconcile.Result{}, c.handleCreateOrUpdate(ctx, obj)
}

func (c *Controller) handleCreateOrUpdate(ctx context.Context, obj *unstructured.Unstructured) error {
	log := logging.FromContextOrDiscard(ctx)
	log.Info("Handling creation or update")

	// add finalizer, if needed
	if c.SyncConfig.Finalize != nil && *c.SyncConfig.Finalize && !utils.HasFinalizer(obj) {
		err := c.updateWithRetry(ctx, obj, func(obj *unstructured.Unstructured) (sets.Set[string], error) {
			utils.AddFinalizer(obj)
			return sets.New[string]("metadata"), nil
		}, retryLimit)
		if err != nil {
			errMsg := "error adding finalizer"
			log.Error(err, errMsg)
			errs := utils.NewErrorList(fmt.Errorf("%s: %w", errMsg, err))
			err2 := c.updateStateOnResource(ctx, obj, state.STATE_FIELD_PHASE, state.PHASE_ERROR, state.STATE_FIELD_DETAIL, errs.Error())
			errs.Append(err2)
			return errs
		}
	}

	// if state display with phase is configured, update phase to progressing
	err := c.updateStateOnResource(ctx, obj, state.STATE_FIELD_PHASE, state.PHASE_PROGRESSING, state.STATE_FIELD_DETAIL, "")
	if err != nil {
		return err
	}

	for _, storage := range c.StorageConfigs {
		curLog := log.WithValues(constants.Logging.KEY_RESOURCE_STORAGE_ID, storage.Name())
		curCtx := logging.NewContext(ctx, curLog)
		// read existing data for resource
		oldData, err := storage.Persister.Get(curCtx, obj.GetName(), obj.GetNamespace(), c.GVK, storage.SubPath)
		if err != nil {
			errMsg := "error while reading old resource"
			curLog.Error(err, errMsg)
			errs := utils.NewErrorList(fmt.Errorf("[%s] %s: %w", storage.Name(), errMsg, err))
			err2 := c.updateStateOnResource(ctx, obj, state.STATE_FIELD_PHASE, state.PHASE_ERROR, state.STATE_FIELD_DETAIL, errs.Error())
			errs.Append(err2)
			return errs
		}
		// transform new resource
		newData, err := storage.Transformer.TransformAndSerialize(obj)
		if err != nil {
			errMsg := "error while transforming resource"
			curLog.Error(err, errMsg)
			errs := utils.NewErrorList(fmt.Errorf("[%s] %s: %w", storage.Name(), errMsg, err))
			err2 := c.updateStateOnResource(ctx, obj, state.STATE_FIELD_PHASE, state.PHASE_ERROR, state.STATE_FIELD_DETAIL, errs.Error())
			errs.Append(err2)
			return errs
		}
		updateRequired := true

		// if corresponding resource exists in storage
		if oldData != nil {
			if bytes.Equal(oldData, newData) {
				curLog.Debug("No relevant fields have changed, updating the resource is not necessary")
				updateRequired = false
			}
		}

		if updateRequired {
			// persist changes
			err := storage.Persister.PersistData(curCtx, obj.GetName(), obj.GetNamespace(), c.GVK, newData, storage.SubPath)
			if err != nil {
				errMsg := "error while persisting resource"
				curLog.Error(err, errMsg)
				errs := utils.NewErrorList(fmt.Errorf("[%s] %s: %w", storage.Name(), errMsg, err))
				err2 := c.updateStateOnResource(ctx, obj, state.STATE_FIELD_PHASE, state.PHASE_ERROR, state.STATE_FIELD_DETAIL, errs.Error())
				errs.Append(err2)
				return errs
			}
		}
	}

	err = c.updateStateOnResource(ctx, obj, state.STATE_FIELD_LAST_SYNCED_GENERATION, obj.GetGeneration(), state.STATE_FIELD_PHASE, state.PHASE_FINISHED, state.STATE_FIELD_DETAIL, "")
	if err != nil {
		return err
	}

	return nil
}

func (c *Controller) handleDelete(ctx context.Context, obj *unstructured.Unstructured) error {
	log := logging.FromContextOrDiscard(ctx)
	log.Info("Handling deletion")

	hasFinalizer := utils.HasFinalizer(obj)

	if hasFinalizer {
		// only update state if there is a finalizer on the resource, otherwise it could be gone before the state can be written
		err := c.updateStateOnResource(ctx, obj, state.STATE_FIELD_PHASE, state.PHASE_DELETING, state.STATE_FIELD_DETAIL, "")
		if err != nil {
			return err
		}
	}

	for _, storage := range c.StorageConfigs {
		curLog := log.WithValues(constants.Logging.KEY_RESOURCE_STORAGE_ID, storage.Name())
		curCtx := logging.NewContext(ctx, curLog)
		exists, err := storage.Persister.Exists(curCtx, obj.GetName(), obj.GetNamespace(), c.GVK, storage.SubPath)
		if err != nil {
			errMsg := "error while checking for data existence"
			curLog.Error(err, errMsg)
			errs := utils.NewErrorList(fmt.Errorf("%s: %w", errMsg, err))
			if hasFinalizer {
				err2 := c.updateStateOnResource(ctx, obj, state.STATE_FIELD_PHASE, state.PHASE_ERROR_DELETING, state.STATE_FIELD_DETAIL, errs.Error())
				errs.Append(err2)
			}
			return errs
		}
		if !exists {
			curLog.Debug("No data found for current resource, skipping deletion")
			return nil
		}
		err = storage.Persister.Delete(curCtx, obj.GetName(), obj.GetNamespace(), c.GVK, storage.SubPath)
		if err != nil {
			errMsg := "error while deleting data"
			curLog.Error(err, errMsg)
			errs := utils.NewErrorList(fmt.Errorf("%s: %w", errMsg, err))
			if hasFinalizer {
				err2 := c.updateStateOnResource(ctx, obj, state.STATE_FIELD_PHASE, state.PHASE_ERROR_DELETING, state.STATE_FIELD_DETAIL, errs.Error())
				errs.Append(err2)
			}
			return errs
		}
	}

	// remove finalizer if any
	if hasFinalizer {
		err := c.updateWithRetry(ctx, obj, func(obj *unstructured.Unstructured) (sets.Set[string], error) {
			utils.RemoveFinalizer(obj)
			return sets.New[string]("metadata"), nil
		}, retryLimit)
		if err != nil {
			errMsg := "error removing finalizer"
			log.Error(err, errMsg)
			errs := utils.NewErrorList(fmt.Errorf("%s: %w", errMsg, err))
			err2 := c.updateStateOnResource(ctx, obj, state.STATE_FIELD_PHASE, state.PHASE_ERROR, state.STATE_FIELD_DETAIL, errs.Error())
			errs.Append(err2)
			return errs
		}
	}

	return nil
}
