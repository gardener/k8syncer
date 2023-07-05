// SPDX-FileCopyrightText: 2023 SAP SE or an SAP affiliate company and Gardener contributors.
//
// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"fmt"
	"os"

	"github.com/gardener/landscaper/controller-utils/pkg/logging"
	"github.com/spf13/cobra"
	ctrlrun "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/gardener/k8syncer/pkg/config"
	"github.com/gardener/k8syncer/pkg/controller"
	"github.com/gardener/k8syncer/pkg/persist"
	fspersist "github.com/gardener/k8syncer/pkg/persist/filesystem"
	gitpersist "github.com/gardener/k8syncer/pkg/persist/git"
	mockpersist "github.com/gardener/k8syncer/pkg/persist/mock"
)

// NewK8SyncerCommand creates a new k8syncer command that runs the git sync controller.
func NewK8SyncerCommand(ctx context.Context) *cobra.Command {
	options := NewOptions()

	cmd := &cobra.Command{
		Use:   "k8syncer",
		Short: "k8syncer syncs k8s resources from the cluster into git",

		Run: func(cmd *cobra.Command, args []string) {
			if err := options.Complete(); err != nil {
				fmt.Print(err)
				os.Exit(1)
			}
			ctx = logging.NewContext(ctx, options.Log)
			if err := options.run(ctx); err != nil {
				options.Log.Error(err, "unable to run k8syncer controller")
				os.Exit(1)
			}
		},
	}

	options.AddFlags(cmd.Flags())

	return cmd
}

func (o *Options) run(ctx context.Context) error {
	logger := o.Log.WithName("k8syncer")
	ctx = logging.NewContext(ctx, logger)

	// build manager
	mOpts := manager.Options{
		LeaderElection:     false,
		MetricsBindAddress: "0",
	}
	mgr, err := ctrlrun.NewManager(ctrlrun.GetConfigOrDie(), mOpts)
	if err != nil {
		return fmt.Errorf("unable to setup manager: %w", err)
	}

	// initialize persisters for all defined storage definitions
	persisters := map[string]persist.Persister{}
	for _, stDef := range o.Config.StorageDefinitions {
		p, err := initializePersister(ctx, stDef)
		if err != nil {
			return fmt.Errorf("error initializing persister for storage definition '%s': %w", stDef.Name, err)
		}
		persisters[stDef.Name] = p
	}

	// add one Controller per sync config to the manager
	for _, syncConfig := range o.Config.SyncConfigs {
		if err := controller.AddControllerToManager(logger, mgr, o.Config, syncConfig, persisters); err != nil {
			return fmt.Errorf("error adding new controller to manager: %w", err)
		}
	}

	logger.Info("Starting controllers")
	return mgr.Start(ctx)
}

// initializePersister should be called once per storage definition
func initializePersister(ctx context.Context, stDef *config.StorageDefinition) (persist.Persister, error) {
	if stDef == nil {
		return nil, fmt.Errorf("storage definition must not be nil")
	}
	var p persist.Persister
	var err error
	switch stDef.Type {
	case config.STORAGE_TYPE_FILESYSTEM:
		var fsp *fspersist.FileSystemPersister
		var err error
		if *stDef.FileSystemConfig.InMemory {
			fsp, err = fspersist.NewForMemory(stDef.FileSystemConfig)
		} else {
			fsp, err = fspersist.NewForOS(stDef.FileSystemConfig)
		}
		if err != nil {
			return nil, fmt.Errorf("error creating FileSystemPersister: %w", err)
		}
		p = persist.AddLoggingLayer(fsp, logging.DEBUG)
	case config.STORAGE_TYPE_GIT:
		gp, err := gitpersist.New(ctx, stDef)
		if err != nil {
			return nil, fmt.Errorf("error creating GitPersister: %w", err)
		}
		p = persist.AddLoggingLayer(gp, logging.DEBUG)
	case config.STORAGE_TYPE_MOCK:
		p, err = mockpersist.New(stDef.MockConfig, stDef.FileSystemConfig, false)
		if err != nil {
			return nil, fmt.Errorf("error creating FileSystemPersister: %w", err)
		}
	default:
		// should not happen, as this check is already part of the config validation
		return nil, fmt.Errorf("unknown storage type '%s'", stDef.Type)
	}
	return p, nil
}
