// SPDX-FileCopyrightText: 2023 SAP SE or an SAP affiliate company and Gardener contributors.
//
// SPDX-License-Identifier: Apache-2.0

package git

import (
	"context"
	"fmt"
	"os"

	"github.com/gardener/landscaper/controller-utils/pkg/logging"
	"github.com/mandelsoft/vfs/pkg/memoryfs"
	"github.com/mandelsoft/vfs/pkg/osfs"
	"github.com/mandelsoft/vfs/pkg/vfs"

	"github.com/gardener/k8syncer/pkg/config"
	"github.com/gardener/k8syncer/pkg/persist"
	fspersist "github.com/gardener/k8syncer/pkg/persist/filesystem"
	"github.com/gardener/k8syncer/pkg/utils"
	"github.com/gardener/k8syncer/pkg/utils/git"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var _ persist.Persister = &GitPersister{}
var _ persist.LoggerInjectable = &GitPersister{}

// GitPersister persists data by pushing changes to a git repository.
type GitPersister struct {
	persist.Persister
	injectedLogger          *logging.Logger
	repo                    *git.GitRepo
	expectChangesFromRemote bool
}

// New creates a new GitPersister.
// If expectChangesFromRemote is false, the Persister assumes that it is the only one pushing to the repository and pulls only in case of an error during push.
// If it is true, every action causes it to pull first.
func New(ctx context.Context, stDef *config.StorageDefinition) (*GitPersister, error) {
	log := logging.FromContextOrDiscard(ctx)
	rootPath := stDef.FileSystemConfig.RootPath
	var fs vfs.FileSystem
	if *stDef.FileSystemConfig.InMemory {
		fs = memoryfs.New()
		err := fs.MkdirAll(rootPath, os.ModeDir|os.ModePerm)
		if err != nil {
			return nil, fmt.Errorf("error creating rootpath directories on in-memory filesystem: %w", err)
		}
	} else {
		fs = osfs.New()
	}
	gitRepoName := stDef.Name
	fsp, err := fspersist.New(fs, stDef.FileSystemConfig, false)
	if err != nil {
		return nil, err
	}
	err = prepareFilesystem(fsp.Fs, rootPath, gitRepoName)
	if err != nil {
		return nil, fmt.Errorf("error while preparing git repository: %w", err)
	}

	gitCfg := stDef.GitConfig
	gitAuth, err := git.AuthFromConfig(gitCfg.Auth)
	if err != nil {
		return nil, fmt.Errorf("error creating auth method from config: %w", err)
	}

	gitRepo, err := git.NewRepo(fsp.Fs, gitCfg.URL, gitCfg.Branch, rootPath, gitAuth)
	if err != nil {
		return nil, fmt.Errorf("error during git repo creation: %w", err)
	}
	err = gitRepo.Initialize(log)
	if err != nil {
		return nil, fmt.Errorf("error initializing git repo: %w", err)
	}

	gp := &GitPersister{
		Persister:               fsp,
		injectedLogger:          &persist.StaticDiscardLogger,
		repo:                    gitRepo,
		expectChangesFromRemote: !gitCfg.Exclusive,
	}

	return gp, nil
}

func (p *GitPersister) InjectLogger(il *logging.Logger) {
	p.injectedLogger = il
	// pass down injected logger to wrapped persister
	if li, ok := p.Persister.(persist.LoggerInjectable); ok {
		li.InjectLogger(il)
	}
}

func (p *GitPersister) Exists(ctx context.Context, name, namespace string, gvk schema.GroupVersionKind, subPath string) (bool, error) {
	if p.expectChangesFromRemote {
		err := p.repo.Pull(*p.injectedLogger)
		if err != nil {
			return false, err
		}
	}
	exists, err := p.Persister.Exists(ctx, name, namespace, gvk, subPath)
	return exists, err
}

func (p *GitPersister) Get(ctx context.Context, name, namespace string, gvk schema.GroupVersionKind, subPath string) (*unstructured.Unstructured, error) {
	if p.expectChangesFromRemote {
		err := p.repo.Pull(*p.injectedLogger)
		if err != nil {
			return nil, err
		}
	}
	data, err := p.Persister.Get(ctx, name, namespace, gvk, subPath)
	return data, err
}

func (p *GitPersister) commitAndPush(resource *unstructured.Unstructured) error {
	return p.repo.CommitAndPush(*p.injectedLogger, p.expectChangesFromRemote, fmt.Sprintf("update %s %s", utils.GVKToString(resource.GroupVersionKind(), true), getNamespacedName(resource.GetName(), resource.GetNamespace())))
}

func (p *GitPersister) Persist(ctx context.Context, resource *unstructured.Unstructured, t persist.Transformer, subPath string) (*unstructured.Unstructured, bool, error) {
	if p.expectChangesFromRemote {
		err := p.repo.Pull(*p.injectedLogger)
		if err != nil {
			return nil, false, err
		}
	}
	persisted, changed, err := p.Persister.Persist(ctx, resource, t, subPath)
	if err != nil {
		return nil, false, err
	}
	if changed {
		err = p.commitAndPush(persisted)
	}
	return persisted, changed, err
}

func (p *GitPersister) Delete(ctx context.Context, name, namespace string, gvk schema.GroupVersionKind, subPath string) error {
	err := p.Persister.Delete(ctx, name, namespace, gvk, subPath)
	if err != nil {
		return err
	}
	err = p.repo.CommitAndPush(*p.injectedLogger, p.expectChangesFromRemote, fmt.Sprintf("delete %s %s", utils.GVKToString(gvk, true), getNamespacedName(name, namespace)))
	return err
}

func prepareFilesystem(fs vfs.FileSystem, rootPath, gitRepoName string) error {
	if gitRepoName == "" {
		return fmt.Errorf("gitRepoPath must not be empty")
	}

	gitRepoPath := repoPath(fs, rootPath, gitRepoName)
	exists, err := vfs.DirExists(fs, gitRepoPath)
	if err != nil {
		return fmt.Errorf("error while checking git repo path existence: %w", err)
	}
	if !exists {
		err = fs.MkdirAll(gitRepoPath, os.ModeDir|os.ModePerm)
		if err != nil {
			return fmt.Errorf("error while trying to create repo directory '%s': %w", gitRepoPath, err)
		}
	}

	return nil
}

func repoPath(fs vfs.FileSystem, rootPath, gitRepoName string) string {
	return vfs.Join(fs, rootPath, "repos", gitRepoName)
}

func getNamespacedName(name, namespace string) string {
	if namespace == "" {
		return name
	}
	return fmt.Sprintf("%s/%s", namespace, name)
}
