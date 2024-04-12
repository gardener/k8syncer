// SPDX-FileCopyrightText: 2023 SAP SE or an SAP affiliate company and Gardener contributors.
//
// SPDX-License-Identifier: Apache-2.0

package app

import (
	"fmt"
	"os"
	"path"

	flag "github.com/spf13/pflag"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/gardener/landscaper/controller-utils/pkg/logging"
	ctrlrun "sigs.k8s.io/controller-runtime"

	"github.com/gardener/k8syncer/pkg/config"
)

// Options describes the options to configure the Landscaper controller.
type Options struct {
	MetricsAddr       string
	ProbeAddr         string
	ConfigPath        string
	ClusterConfigPath string

	Log           logging.Logger
	Config        *config.K8SyncerConfiguration
	ClusterConfig *rest.Config
}

func NewOptions() *Options {
	return &Options{}
}

func (o *Options) AddFlags(fs *flag.FlagSet) {
	fs.StringVar(&o.MetricsAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	fs.StringVar(&o.ProbeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	fs.StringVar(&o.ConfigPath, "config", "", "Specify the path to the configuration file.")
	fs.StringVar(&o.ClusterConfigPath, "kubeconfig", "", "Path to the kubeconfig file or directory containing either a kubeconfig or host, token, and ca file. Leave empty to use in-cluster config.")
	logging.InitFlags(fs)
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

	// load kubeconfig
	o.ClusterConfig, err = LoadKubeconfig(o.ClusterConfigPath)
	if err != nil {
		return fmt.Errorf("unable to load kubeconfig: %w", err)
	}

	return nil
}

// validates the Options
func (o *Options) validate() error {
	return config.Validate(o.Config).ToAggregate()
}

// LoadKubeconfig loads a cluster configuration from the given path.
// If the path points to a single file, this file is expected to contain a kubeconfig which is then loaded.
// If the path points to a directory which contains a file named "kubeconfig", that file is used.
// If the path points to a directory which does not contain a "kubeconfig" file, there must be "host", "token", and "ca.crt" files present,
// which are used to configure cluster access based on an OIDC trust relationship.
// If the path is empty, the in-cluster config is returned.
func LoadKubeconfig(configPath string) (*rest.Config, error) {
	if configPath == "" {
		return rest.InClusterConfig()
	}
	fi, err := os.Stat(configPath)
	if err != nil {
		return nil, err
	}
	if fi.IsDir() {
		if kfi, err := os.Stat(path.Join(configPath, "kubeconfig")); err == nil && !kfi.IsDir() {
			// there is a kubeconfig file in the specified folder
			// point configPath to the kubeconfig
			configPath = path.Join(configPath, "kubeconfig")
		} else {
			// no kubeconfig file present, load OIDC trust configuration
			host, err := os.ReadFile(path.Join(configPath, "host"))
			if err != nil {
				return nil, fmt.Errorf("error reading host file: %w", err)
			}
			return &rest.Config{
				Host:            string(host),
				BearerTokenFile: path.Join(configPath, "token"),
				TLSClientConfig: rest.TLSClientConfig{
					CAFile: path.Join(configPath, "ca.crt"),
				},
			}, nil
		}
	}
	// at this point, configPath points to a single file which is expected to contain a kubeconfig
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, err
	}
	return clientcmd.RESTConfigFromKubeConfig(data)
}
