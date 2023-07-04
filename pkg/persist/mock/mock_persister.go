// SPDX-FileCopyrightText: 2023 SAP SE or an SAP affiliate company and Gardener contributors.
//
// SPDX-License-Identifier: Apache-2.0

package mock

import (
	"context"

	"github.com/gardener/landscaper/controller-utils/pkg/logging"

	"github.com/gardener/k8syncer/pkg/config"
	"github.com/gardener/k8syncer/pkg/persist"
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
}

// New creates a new MockPersister.
func New(mockCfg *config.MockConfiguration, fsCfg *config.FileSystemConfiguration) (persist.Persister, error) {
	fsp, err := fspersist.NewForMemory(fsCfg)
	if err != nil {
		return nil, err
	}
	logLevel := logging.DEBUG
	if mockCfg.LogPersisterCallsOnInfoLevel {
		logLevel = logging.INFO
	}

	mp := &MockPersister{
		Persister:      fsp,
		injectedLogger: &persist.StaticDiscardLogger,
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
	exists, err := p.Persister.Exists(ctx, name, namespace, gvk, subPath)
	p.injectedLogger.Info("Checking if data exists", constants.Logging.KEY_ERROR_OCCURRED, err != nil, constants.Logging.KEY_DATA_EXISTS, exists)
	return exists, err
}

func (p *MockPersister) Get(ctx context.Context, name, namespace string, gvk schema.GroupVersionKind, subPath string) ([]byte, error) {
	data, err := p.Persister.Get(ctx, name, namespace, gvk, subPath)
	p.injectedLogger.Info("Getting data", constants.Logging.KEY_ERROR_OCCURRED, err != nil, constants.Logging.KEY_DATA, string(data))
	return data, err
}

func (p *MockPersister) Persist(ctx context.Context, resource *unstructured.Unstructured, gvk schema.GroupVersionKind, rt persist.ResourceTransformer, subPath string) error {
	err := p.Persister.Persist(ctx, resource, gvk, rt, subPath)
	p.injectedLogger.Info("Persisting resource", constants.Logging.KEY_ERROR_OCCURRED, err != nil)
	return err
}

func (p *MockPersister) PersistData(ctx context.Context, name, namespace string, gvk schema.GroupVersionKind, data []byte, subPath string) error {
	err := p.Persister.PersistData(ctx, name, namespace, gvk, data, subPath)
	p.injectedLogger.Info("Persisting data", constants.Logging.KEY_ERROR_OCCURRED, err != nil, constants.Logging.KEY_DATA, string(data))
	return err
}

func (p *MockPersister) Delete(ctx context.Context, name, namespace string, gvk schema.GroupVersionKind, subPath string) error {
	err := p.Persister.Delete(ctx, name, namespace, gvk, subPath)
	p.injectedLogger.Info("Deleting resource", constants.Logging.KEY_ERROR_OCCURRED, err != nil)
	return err
}

func (p *MockPersister) InternalPersister() persist.Persister {
	return p.Persister
}
