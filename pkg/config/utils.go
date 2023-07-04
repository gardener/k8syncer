// SPDX-FileCopyrightText: 2023 SAP SE or an SAP affiliate company and Gardener contributors.
//
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"fmt"
	"os"
	"strings"

	"k8s.io/apimachinery/pkg/util/yaml"

	"github.com/gardener/k8syncer/pkg/utils"
)

// Complete performs some completion tasks as setting defaults and transforming values into the expected format.
func (cfg *K8SyncerConfiguration) Complete() error {
	for _, sc := range cfg.SyncConfigs {
		// default finalizer config
		if sc.Finalize == nil {
			sc.Finalize = utils.Ptr(true)
		}
	}

	for _, sd := range cfg.StorageDefinitions {
		switch sd.Type {
		case STORAGE_TYPE_GIT:
			// transform git auth types to lowercase
			if sd.GitConfig != nil {
				// default branch
				if sd.GitConfig.Branch == "" {
					sd.GitConfig.Branch = "master"
				}
				if sd.GitConfig.Auth != nil {
					sd.GitConfig.Auth.Type = GitAuthenticationType(strings.ToLower(string(sd.GitConfig.Auth.Type)))
					// set arbitrary username for access token
					if sd.GitConfig.Auth.Type == GIT_AUTH_USERNAME_PASSWORD && sd.GitConfig.Auth.Username == "" {
						sd.GitConfig.Auth.Username = "anonymous"
					}
				}
			}
			// default filesystemconfig
			if sd.FileSystemConfig == nil {
				sd.FileSystemConfig = &FileSystemConfiguration{}
			}
			if sd.FileSystemConfig.InMemory == nil {
				sd.FileSystemConfig.InMemory = utils.Ptr(true)
			}
			if *sd.FileSystemConfig.InMemory && sd.FileSystemConfig.RootPath == "" {
				sd.FileSystemConfig.RootPath = "/data"
			}
		case STORAGE_TYPE_FILESYSTEM:
			// default filesystemconfig
			// has to be specified for this type, so only default single missing values
			if sd.FileSystemConfig != nil {
				if sd.FileSystemConfig.InMemory == nil {
					sd.FileSystemConfig.InMemory = utils.Ptr(false)
				}
				if *sd.FileSystemConfig.InMemory && sd.FileSystemConfig.RootPath == "" {
					sd.FileSystemConfig.RootPath = "/data"
				}
			}
		case STORAGE_TYPE_MOCK:
			// default mockconfig
			if sd.MockConfig == nil {
				sd.MockConfig = &MockConfiguration{}
			}
			// default filesystemconfig
			if sd.FileSystemConfig == nil {
				sd.FileSystemConfig = &FileSystemConfiguration{}
			}
			if sd.FileSystemConfig.RootPath == "" {
				sd.FileSystemConfig.RootPath = "/data"
			}
		}
	}
	return nil
}

// LoadConfig reads the configuration file from a given path and parses the data into a K8SyncerConfiguration
func LoadConfig(path string) (*K8SyncerConfiguration, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("unable to read config file: %w", err)
	}

	cfg := &K8SyncerConfiguration{}
	err = yaml.Unmarshal(data, cfg)
	if err != nil {
		return nil, fmt.Errorf("unable to parse config file: %w", err)
	}

	return cfg, nil
}

// IncludesPhase returns true if the verbosity includes the phase.
func (sv StateVerbosity) IncludesPhase() bool {
	switch sv {
	case STATE_VERBOSITY_PHASE:
	case STATE_VERBOSITY_DETAIL:
	default:
		return false
	}
	return true
}

// IncludesDetail returns true if the verbosity includes error details.
func (sv StateVerbosity) IncludesDetail() bool {
	switch sv {
	case STATE_VERBOSITY_DETAIL:
	default:
		return false
	}
	return true
}
