// SPDX-FileCopyrightText: 2023 SAP SE or an SAP affiliate company and Gardener contributors.
//
// SPDX-License-Identifier: Apache-2.0

package app

import (
	goflag "flag"

	flag "github.com/spf13/pflag"

	"github.com/gardener/landscaper/controller-utils/pkg/logging"
	ctrlrun "sigs.k8s.io/controller-runtime"

	"github.com/gardener/k8syncer/pkg/config"
)

// Options describes the options to configure the Landscaper controller.
type Options struct {
	MetricsAddr string
	ProbeAddr   string
	ConfigPath  string

	Log    logging.Logger
	Config *config.K8SyncerConfiguration
}

func NewOptions() *Options {
	return &Options{}
}

func (o *Options) AddFlags(fs *flag.FlagSet) {
	fs.StringVar(&o.MetricsAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	fs.StringVar(&o.ProbeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	fs.StringVar(&o.ConfigPath, "config", "", "specify the path to the configuration file")
	logging.InitFlags(fs)

	flag.CommandLine.AddGoFlagSet(goflag.CommandLine)
}

// Complete parses all Options and flags and initializes the basic functions
func (o *Options) Complete() error {
	// build logger
	log, err := logging.GetLogger()
	if err != nil {
		return err
	}
	o.Log = log
	ctrlrun.SetLogger(o.Log.Logr())

	// build k8syncer config
	o.Config, err = config.LoadConfig(o.ConfigPath)
	if err != nil {
		return err
	}

	err = o.Config.Complete()
	if err != nil {
		return err
	}

	err = o.validate() // validate Options
	if err != nil {
		return err
	}

	return nil
}

// validates the Options
func (o *Options) validate() error {
	return config.Validate(o.Config).ToAggregate()
}
