// SPDX-FileCopyrightText: 2023 SAP SE or an SAP affiliate company and Gardener contributors.
//
// SPDX-License-Identifier: Apache-2.0

package mock

import (
	"context"

	"github.com/gardener/landscaper/controller-utils/pkg/logging"
	"sigs.k8s.io/yaml"

	"github.com/gardener/k8syncer/pkg/config"
	"github.com/gardener/k8syncer/pkg/persist"
	"github.com/gardener/k8syncer/pkg/utils"
	"github.com/gardener/k8syncer/pkg/utils/constants"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	fspersist "github.com/gardener/k8syncer/pkg/persist/filesystem"
)

var _ persist.Persister = &MockPersister{}
var _ persist.LoggerInjectable = &MockPersister{}

// MockPersister uses an (initially empty) in-memory filesystem and logs operations on it.
// It does not actually persist anything.
type MockPersister struct {
	persist.Persister
	injectedLogger *logging.Logger
	expectedCalls  utils.Queue[*MockedCall]
}

// New creates a new MockPersister.
func New(mockCfg *config.MockConfiguration, fsCfg *config.FileSystemConfiguration, testMode bool) (persist.Persister, error) {
	fsp, err := fspersist.NewForMemory(fsCfg)
	if err != nil {
		return nil, err
	}
	logLevel := logging.DEBUG
	if mockCfg != nil && mockCfg.LogPersisterCallsOnInfoLevel {
		logLevel = logging.INFO
	}

	mp := &MockPersister{
		Persister:      fsp,
		injectedLogger: &persist.StaticDiscardLogger,
	}

	if testMode {
		mp.expectedCalls = utils.NewQueue[*MockedCall]()
		return mp, nil
	}

	return persist.AddLoggingLayer(mp, logLevel), nil
}

func (p *MockPersister) InjectLogger(il *logging.Logger) {
	p.injectedLogger = il
	// pass down injected logger to wrapped persister
	if li, ok := p.Persister.(persist.LoggerInjectable); ok {
		li.InjectLogger(il)
	}
}

func (p *MockPersister) Exists(ctx context.Context, name, namespace string, gvk schema.GroupVersionKind, subPath string) (bool, error) {
	if p.expectedCalls != nil {
		if err := p.compareExpectedVsActualCall(MockedExistCall(name, namespace, gvk, subPath)); err != nil {
			return false, err
		}
	}
	exists, err := p.Persister.Exists(ctx, name, namespace, gvk, subPath)
	p.injectedLogger.Info("Checking if data exists", constants.Logging.KEY_ERROR_OCCURRED, err != nil, constants.Logging.KEY_DATA_EXISTS, exists)
	return exists, err
}

func (p *MockPersister) Get(ctx context.Context, name, namespace string, gvk schema.GroupVersionKind, subPath string) (*unstructured.Unstructured, error) {
	if p.expectedCalls != nil {
		if err := p.compareExpectedVsActualCall(MockedGetCall(name, namespace, gvk, subPath)); err != nil {
			return nil, err
		}
	}
	data, err := p.Persister.Get(ctx, name, namespace, gvk, subPath)
	logFields := []interface{}{
		constants.Logging.KEY_ERROR_OCCURRED, err != nil,
	}
	if err == nil {
		rawData, err2 := yaml.Marshal(data)
		if err2 == nil {
			logFields = append(logFields, constants.Logging.KEY_DATA, string(rawData))
		}
	}
	p.injectedLogger.Info("Getting data", logFields...)
	return data, err
}

func (p *MockPersister) Persist(ctx context.Context, resource *unstructured.Unstructured, t persist.Transformer, subPath string) (*unstructured.Unstructured, bool, error) {
	if p.expectedCalls != nil {
		if err := p.compareExpectedVsActualCall(MockedPersistCall(resource, t, subPath)); err != nil {
			return nil, false, err
		}
	}
	persisted, changed, err := p.Persister.Persist(ctx, resource, t, subPath)
	p.injectedLogger.Info("Persisting resource if changed", constants.Logging.KEY_ERROR_OCCURRED, err != nil, constants.Logging.KEY_RESOURCE_IN_STORAGE_CHANGED, changed)
	return persisted, changed, err
}

func (p *MockPersister) Delete(ctx context.Context, name, namespace string, gvk schema.GroupVersionKind, subPath string) error {
	if p.expectedCalls != nil {
		if err := p.compareExpectedVsActualCall(MockedDeleteCall(name, namespace, gvk, subPath)); err != nil {
			return err
		}
	}
	err := p.Persister.Delete(ctx, name, namespace, gvk, subPath)
	p.injectedLogger.Info("Deleting resource", constants.Logging.KEY_ERROR_OCCURRED, err != nil)
	return err
}

func (p *MockPersister) InternalPersister() persist.Persister {
	return p.Persister
}
