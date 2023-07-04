// SPDX-FileCopyrightText: 2023 SAP SE or an SAP affiliate company and Gardener contributors.
//
// SPDX-License-Identifier: Apache-2.0

package git

import (
	"fmt"

	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/go-git/go-git/v5/plumbing/transport/ssh"

	"github.com/gardener/k8syncer/pkg/config"
)

func AuthFromConfig(authCfg *config.GitRepoAuth) (transport.AuthMethod, error) {
	if authCfg == nil {
		return nil, nil
	}
	switch authCfg.Type {
	case config.GIT_AUTH_USERNAME_PASSWORD:
		return &http.BasicAuth{
			Username: authCfg.Username,
			Password: authCfg.Password,
		}, nil
	case config.GIT_AUTH_SSH:
		var publicKeys *ssh.PublicKeys
		var err error
		if authCfg.PrivateKey != "" {
			publicKeys, err = ssh.NewPublicKeys("git", []byte(authCfg.PrivateKey), authCfg.Password)
		} else if authCfg.PrivateKeyFile != "" {
			publicKeys, err = ssh.NewPublicKeysFromFile("git", authCfg.PrivateKeyFile, authCfg.Password)
		} else {
			// should not happen as already part of the config validation
			return nil, fmt.Errorf("neither privateKey nor privateKeyFile is specified for git ssh config")
		}
		if err != nil {
			return nil, fmt.Errorf("unable to create public key: %w", err)
		}
		return publicKeys, nil
	default:
		// should not happen as already part of the config validation
		return nil, fmt.Errorf("unknown git auth type '%s'", string(authCfg.Type))
	}
}

func AuthViaUsernamePassword(username, password string) transport.AuthMethod {
	return &http.BasicAuth{
		Username: username,
		Password: password,
	}
}
