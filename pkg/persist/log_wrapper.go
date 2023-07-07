// SPDX-FileCopyrightText: 2023 SAP SE or an SAP affiliate company and Gardener contributors.
//
// SPDX-License-Identifier: Apache-2.0

package persist

import (
	"context"

	"github.com/gardener/landscaper/controller-utils/pkg/logging"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/gardener/k8syncer/pkg/utils/constants"
)

var _ Persister = &logWrappedPersister{}
var _ LoggerInjectable = &logWrappedPersister{}

var StaticDiscardLogger = logging.Discard()

// logWrappedPersister is a wrapper for a Persister which will add debug logs to all function calls.
type logWrappedPersister struct {
	Persister
	logLevel   logging.LogLevel
	injectable LoggerInjectable
}

// LoggerInjectable signals that the corresponding object allows injection of a logger.
// Before calling the internal Persister functions, the Persister's InjectLogger function will be called, if it implements the LoggerInjectable interface.
// This allows passing down the contextified logger.
// After calling the function, InjectLogger will be called again with a pointer to the discard logger, to 'uninject' the logger.
type LoggerInjectable interface {
	InjectLogger(*logging.Logger)
}

func (lwp *logWrappedPersister) InjectLogger(il *logging.Logger) {
	// pass down injected logger to wrapped persister
	if lwp.injectable != nil {
		lwp.injectable.InjectLogger(il)
	}
}

// AddDebugLoggingLayer wraps the given Persister with a logging wrapper that adds logs before and after each call to the internal Persister.
// It is strongly recommended to use only Debug log level for all non-development purposes, as everything else will likely clutter the logs.
func AddLoggingLayer(p Persister, logLevel logging.LogLevel) Persister {
	res := &logWrappedPersister{
		Persister: p,
		logLevel:  logLevel,
	}
	if li, ok := p.(LoggerInjectable); ok {
		res.injectable = li
	}
	return res
}

func (lwp *logWrappedPersister) buildLogger(ctx context.Context) logging.Logger { // TODO remove superfluous arguments
	return logging.FromContextOrDiscard(ctx)
}

func (lwp *logWrappedPersister) Exists(ctx context.Context, name, namespace string, gvk schema.GroupVersionKind, subPath string) (bool, error) {
	// create logger with context information
	curLog := lwp.buildLogger(ctx)

	// check for logger injection
	if lwp.injectable != nil {
		lwp.injectable.InjectLogger(&curLog)
	}

	// call wrapped function
	curLog.Log(lwp.logLevel, constants.Logging.CALL_EXISTS_MSG)
	res, err := lwp.Persister.Exists(ctx, name, namespace, gvk, subPath)
	errOccurred := err != nil
	if errOccurred {
		curLog = curLog.WithValues(constants.Logging.KEY_ERROR, err.Error())
	}
	curLog.Log(lwp.logLevel, constants.Logging.CALL_EXISTS_FINISHED_MSG, constants.Logging.KEY_ERROR_OCCURRED, errOccurred, constants.Logging.KEY_DATA_EXISTS, res)

	// remove injected logger again
	if lwp.injectable != nil {
		lwp.injectable.InjectLogger(&StaticDiscardLogger)
	}

	return res, err
}

func (lwp *logWrappedPersister) Get(ctx context.Context, name, namespace string, gvk schema.GroupVersionKind, subPath string) (*unstructured.Unstructured, error) {
	// create logger with context information
	curLog := lwp.buildLogger(ctx)

	// check for logger injection
	if lwp.injectable != nil {
		lwp.injectable.InjectLogger(&curLog)
	}

	// call wrapped function
	curLog.Log(lwp.logLevel, constants.Logging.CALL_GET_MSG)
	res, err := lwp.Persister.Get(ctx, name, namespace, gvk, subPath)
	errOccurred := err != nil
	if errOccurred {
		curLog = curLog.WithValues(constants.Logging.KEY_ERROR, err.Error())
	}
	curLog.Log(lwp.logLevel, constants.Logging.CALL_GET_FINISHED_MSG, constants.Logging.KEY_ERROR_OCCURRED, errOccurred, constants.Logging.KEY_DATA_EXISTS, res != nil)

	// remove injected logger again
	if lwp.injectable != nil {
		lwp.injectable.InjectLogger(&StaticDiscardLogger)
	}

	return res, err
}

func (lwp *logWrappedPersister) Persist(ctx context.Context, resource *unstructured.Unstructured, t Transformer, subPath string) (*unstructured.Unstructured, bool, error) {
	// create logger with context information
	curLog := lwp.buildLogger(ctx)

	// check for logger injection
	if lwp.injectable != nil {
		lwp.injectable.InjectLogger(&curLog)
	}

	// call wrapped function
	curLog.Log(lwp.logLevel, constants.Logging.CALL_PERSIST_MSG)
	persisted, changed, err := lwp.Persister.Persist(ctx, resource, t, subPath)
	errOccurred := err != nil
	if errOccurred {
		curLog = curLog.WithValues(constants.Logging.KEY_ERROR, err.Error())
	}
	curLog.Log(lwp.logLevel, constants.Logging.CALL_PERSIST_FINISHED_MSG, constants.Logging.KEY_ERROR_OCCURRED, errOccurred)

	// remove injected logger again
	if lwp.injectable != nil {
		lwp.injectable.InjectLogger(&StaticDiscardLogger)
	}

	return persisted, changed, err
}

func (lwp *logWrappedPersister) Delete(ctx context.Context, name, namespace string, gvk schema.GroupVersionKind, subPath string) error {
	// create logger with context information
	curLog := lwp.buildLogger(ctx)

	// check for logger injection
	if lwp.injectable != nil {
		lwp.injectable.InjectLogger(&curLog)
	}

	// call wrapped function
	curLog.Log(lwp.logLevel, constants.Logging.CALL_DELETE_MSG)
	err := lwp.Persister.Delete(ctx, name, namespace, gvk, subPath)
	errOccurred := err != nil
	if errOccurred {
		curLog = curLog.WithValues(constants.Logging.KEY_ERROR, err.Error())
	}
	curLog.Log(lwp.logLevel, constants.Logging.CALL_DELETE_FINISHED_MSG, constants.Logging.KEY_ERROR_OCCURRED, errOccurred)

	// remove injected logger again
	if lwp.injectable != nil {
		lwp.injectable.InjectLogger(&StaticDiscardLogger)
	}

	return err
}

func (lwp *logWrappedPersister) InternalPersister() Persister {
	return lwp.Persister
}
