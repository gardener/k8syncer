// SPDX-FileCopyrightText: 2023 SAP SE or an SAP affiliate company and Gardener contributors.
//
// SPDX-License-Identifier: Apache-2.0

package mock

import (
	"context"
	"reflect"

	"github.com/gardener/landscaper/controller-utils/pkg/logging"
	"sigs.k8s.io/yaml"

	"github.com/gardener/k8syncer/pkg/config"
	"github.com/gardener/k8syncer/pkg/persist"
	"github.com/gardener/k8syncer/pkg/utils"
	"github.com/gardener/k8syncer/pkg/utils/constants"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var _ persist.Persister = &MockPersister{}
var _ persist.LoggerInjectable = &MockPersister{}

// MockPersister stores resources in memory and logs operations on it.
// It does not actually persist anything.
type MockPersister struct {
	Storage        map[resourceIdentifier]*unstructured.Unstructured
	injectedLogger *logging.Logger
	expectedCalls  utils.Queue[*MockedCall]
}

type resourceIdentifier struct {
	name      string
	namespace string
	gvk       schema.GroupVersionKind
	subPath   string
}

// Identify generates the struct used as key for the internal storage.
func Identify(name, namespace string, gvk schema.GroupVersionKind, subPath string) resourceIdentifier {
	return resourceIdentifier{
		name:      name,
		namespace: namespace,
		gvk:       gvk,
		subPath:   subPath,
	}
}

// New creates a new MockPersister.
func New(mockCfg *config.MockConfiguration, testMode bool) (persist.Persister, error) {
	logLevel := logging.DEBUG
	if mockCfg != nil && mockCfg.LogPersisterCallsOnInfoLevel {
		logLevel = logging.INFO
	}

	mp := &MockPersister{
		Storage:        map[resourceIdentifier]*unstructured.Unstructured{},
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
}

func (p *MockPersister) Exists(ctx context.Context, name, namespace string, gvk schema.GroupVersionKind, subPath string) (bool, error) {
	var expectedReturn *MockedReturn
	if p.expectedCalls != nil {
		expectedCall, err := p.expectedCalls.Peek()
		if err == nil {
			expectedReturn = expectedCall.expectedReturn
		}
		if err := p.compareExpectedVsActualCall(MockedExistsCall(name, namespace, gvk, subPath)); err != nil {
			return false, err
		}
	}
	_, exists := p.Storage[Identify(name, namespace, gvk, subPath)]
	p.injectedLogger.Info("Checking if data exists", constants.Logging.KEY_DATA_EXISTS, exists)
	if expectedReturn != nil {
		if err := compareReturns(expectedReturn, MockedExistsReturn(exists, nil)); err != nil {
			return false, err
		}
	}
	return exists, nil
}

func (p *MockPersister) Get(ctx context.Context, name, namespace string, gvk schema.GroupVersionKind, subPath string) (*unstructured.Unstructured, error) {
	var expectedReturn *MockedReturn
	if p.expectedCalls != nil {
		expectedCall, err := p.expectedCalls.Peek()
		if err == nil {
			expectedReturn = expectedCall.expectedReturn
		}
		if err := p.compareExpectedVsActualCall(MockedGetCall(name, namespace, gvk, subPath)); err != nil {
			return nil, err
		}
	}
	data, exists := p.Storage[Identify(name, namespace, gvk, subPath)]
	logFields := []interface{}{
		constants.Logging.KEY_DATA_EXISTS, exists,
	}
	if exists {
		rawData, err := yaml.Marshal(data)
		if err == nil {
			logFields = append(logFields, constants.Logging.KEY_DATA, string(rawData))
		}
	}
	p.injectedLogger.Info("Getting data", logFields...)
	if expectedReturn != nil {
		if err := compareReturns(expectedReturn, MockedGetReturn(data, nil)); err != nil {
			return nil, err
		}
	}
	return data, nil
}

func (p *MockPersister) Persist(ctx context.Context, resource *unstructured.Unstructured, t persist.Transformer, subPath string) (*unstructured.Unstructured, bool, error) {
	var expectedReturn *MockedReturn
	if p.expectedCalls != nil {
		expectedCall, err := p.expectedCalls.Peek()
		if err == nil {
			expectedReturn = expectedCall.expectedReturn
		}
		if err := p.compareExpectedVsActualCall(MockedPersistCall(resource, t, subPath)); err != nil {
			return nil, false, err
		}
	}
	transformed, err := t.Transform(resource)
	if err != nil {
		return nil, false, err
	}
	id := Identify(resource.GetName(), resource.GetNamespace(), resource.GroupVersionKind(), subPath)
	data, exists := p.Storage[id]
	changed := true
	if exists {
		changed = !reflect.DeepEqual(transformed, data)
	}
	if changed {
		p.Storage[id] = transformed
	}
	p.injectedLogger.Info("Persisting resource if changed", constants.Logging.KEY_RESOURCE_IN_STORAGE_CHANGED, changed)
	if expectedReturn != nil {
		if err := compareReturns(expectedReturn, MockedPersistReturn(transformed, changed, nil)); err != nil {
			return transformed, changed, err
		}
	}
	return transformed, changed, nil
}

func (p *MockPersister) Delete(ctx context.Context, name, namespace string, gvk schema.GroupVersionKind, subPath string) error {
	var expectedReturn *MockedReturn
	if p.expectedCalls != nil {
		expectedCall, err := p.expectedCalls.Peek()
		if err == nil {
			expectedReturn = expectedCall.expectedReturn
		}
		if err := p.compareExpectedVsActualCall(MockedDeleteCall(name, namespace, gvk, subPath)); err != nil {
			return err
		}
	}
	delete(p.Storage, Identify(name, namespace, gvk, subPath))
	p.injectedLogger.Info("Deleting resource")
	if expectedReturn != nil {
		if err := compareReturns(expectedReturn, MockedDeleteReturn(nil)); err != nil {
			return err
		}
	}
	return nil
}

func (p *MockPersister) InternalPersister() persist.Persister {
	return nil
}
