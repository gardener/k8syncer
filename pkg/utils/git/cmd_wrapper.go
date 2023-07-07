// SPDX-FileCopyrightText: 2023 SAP SE or an SAP affiliate company and Gardener contributors.
//
// SPDX-License-Identifier: Apache-2.0

package git

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/gardener/landscaper/controller-utils/pkg/logging"
	"github.com/go-git/go-git/v5"
	gitcfg "github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	gitcache "github.com/go-git/go-git/v5/plumbing/cache"
	"github.com/go-git/go-git/v5/plumbing/transport"
	gitfs "github.com/go-git/go-git/v5/storage/filesystem"
	"github.com/mandelsoft/vfs/pkg/projectionfs"
	"github.com/mandelsoft/vfs/pkg/vfs"
)

const defaultRemoteName = "origin"

var ErrNotInitialized = fmt.Errorf("git repo is not initialized, call repo.Initialize first")

// GitRepo is a helper struct which abstracts from the git commands.
type GitRepo struct {
	// URL is the git repo URL.
	URL string
	// Branch is the branch of the repo which should be used.
	Branch string
	// LocalPath is the filesystem path where the repo should be checked out to.
	LocalPath string
	// Auth is the authentification information for the git repository.
	Auth transport.AuthMethod
	// Fs is the filesystem used for the repository.
	Fs vfs.FileSystem

	repo               *git.Repository
	hasUnpushedCommits bool
	lock               *sync.Mutex
}

func NewRepo(baseFs vfs.FileSystem, url, branch, localPath string, auth transport.AuthMethod) (*GitRepo, error) {
	fs, err := projectionfs.New(baseFs, localPath)
	if err != nil {
		return nil, fmt.Errorf("error creating projection filesystem: %w", err)
	}
	return &GitRepo{
		URL:                url,
		Branch:             branch,
		LocalPath:          localPath,
		Auth:               auth,
		Fs:                 fs,
		hasUnpushedCommits: false,
		lock:               &sync.Mutex{},
	}, nil
}

// Initialize opens the repository if it exists and clones it otherwise.
func (r *GitRepo) Initialize(log logging.Logger) error {
	r.lock.Lock()
	defer r.lock.Unlock()
	gitExists, err := vfs.DirExists(r.Fs, ".git")
	if err != nil {
		return fmt.Errorf("error trying to check for repo existence: %w", err)
	}
	if gitExists {
		if err := r.gitOpen(); err != nil {
			return err
		}
	} else {
		if err := r.gitClone(); err != nil {
			return err
		}
	}
	return nil
}

// Commit builds a commit containing the specified paths or all changes, if empty.
// It does not push.
// If the commit message is empty, a generic one is generated.
// If there are no changes staged after adding the specified paths, commit aborts early.
// The first return value determines whether a commit has actually been made (true = there is an unpushed commit).
func (r *GitRepo) Commit(log logging.Logger, msg string, paths ...string) (bool, error) {
	r.lock.Lock()
	defer r.lock.Unlock()
	if !r.IsInitialized() {
		return false, ErrNotInitialized
	}
	return r.commitWithoutLocking(msg, paths...)
}

func (r *GitRepo) commitWithoutLocking(msg string, paths ...string) (bool, error) {
	pushRequired, err := r.gitCommit(msg, paths...)
	if err != nil {
		return false, err
	}
	r.hasUnpushedCommits = pushRequired
	return pushRequired, nil
}

// Push pushes all unpushed commits to the remote repository.
// If pullBefore is true, it pulls before pushing to avoid conflicts.
// If an error occurs during the push, it tries to pull and then retries the push.
func (r *GitRepo) Push(log logging.Logger, pullBefore bool) error {
	r.lock.Lock()
	defer r.lock.Unlock()
	if !r.IsInitialized() {
		return ErrNotInitialized
	}
	return r.pushWithoutLocking(pullBefore)
}

func (r *GitRepo) pushWithoutLocking(pullBefore bool) error {
	if err := r.gitPush(pullBefore, false); err != nil {
		return err
	}
	r.hasUnpushedCommits = false
	return nil
}

// CommitAndPush is the same as Commit + Push, but it keeps the lock for both commands,
// preventing other git commands from being executed in between both commands.
// It pushes only if Commit returns (true, nil).
func (r *GitRepo) CommitAndPush(log logging.Logger, pullBefore bool, msg string, paths ...string) error {
	r.lock.Lock()
	defer r.lock.Unlock()
	if !r.IsInitialized() {
		return ErrNotInitialized
	}
	pushRequired, err := r.commitWithoutLocking(msg, paths...)
	if err != nil {
		return err
	}
	if pushRequired {
		return r.pushWithoutLocking(pullBefore)
	}
	return nil
}

// Pull pulls from the remote repository.
func (r *GitRepo) Pull(log logging.Logger) error {
	r.lock.Lock()
	defer r.lock.Unlock()
	if !r.IsInitialized() {
		return ErrNotInitialized
	}
	if err := r.gitPull(false); err != nil {
		return err
	}
	return nil
}

func (r *GitRepo) gitInit() error {
	// folder is required to create a projectionfs
	gitDirPath := vfs.Join(r.Fs, ".git")
	err := r.Fs.MkdirAll(gitDirPath, os.ModeDir|os.ModePerm)
	if err != nil {
		return fmt.Errorf("error creating .git folder: %w", err)
	}
	fsGitDir, err := projectionfs.New(r.Fs, gitDirPath)
	if err != nil {
		return fmt.Errorf("error creating projection filesystem: %w", err)
	}

	r.repo, err = git.Init(gitfs.NewStorage(FSWrap(fsGitDir), gitcache.NewObjectLRUDefault()), FSWrap(r.Fs))
	if err != nil {
		return fmt.Errorf("error during 'git init': %w", err)
	}

	_, err = r.repo.CreateRemote(&gitcfg.RemoteConfig{
		Name:  defaultRemoteName,
		URLs:  []string{r.URL},
		Fetch: []gitcfg.RefSpec{refspecFromBranch(r.Branch)},
	})
	if err != nil {
		return fmt.Errorf("error during 'git remote add': %w", err)
	}

	return nil
}

func (r *GitRepo) gitClone() error {
	err := r.gitInit()
	if err != nil {
		return err
	}

	return r.gitOpen()

	// repo, err := git.Clone(gitfs.NewStorage(FSWrap(fsGitDir), gitcache.NewObjectLRUDefault()), FSWrap(r.Fs), &git.CloneOptions{
	// 	URL:           r.URL,
	// 	Auth:          r.Auth,
	// 	SingleBranch:  true,
	// 	ReferenceName: plumbing.NewBranchReferenceName(r.Branch),
	// })
	// if err != nil {
	// 	return fmt.Errorf("error during 'git clone': %w", err)
	// }

	// err = r.gitPull(true)
	// if err != nil {
	// 	r.repo = nil
	// 	return err
	// }

	// return nil
}

func (r *GitRepo) gitCommit(msg string, paths ...string) (bool, error) {
	w, err := r.repo.Worktree()
	if err != nil {
		return false, fmt.Errorf("error getting worktree: %w", err)
	}

	if len(paths) > 0 {
		for _, path := range paths {
			_, err = w.Add(path)
			if err != nil {
				return false, fmt.Errorf("error during 'git add': %w", err)
			}
		}
	} else {
		err = w.AddWithOptions(&git.AddOptions{
			All: true,
		})
		if err != nil {
			return false, fmt.Errorf("error during 'git add': %w", err)
		}
	}

	if msg == "" {
		sb := strings.Builder{}
		sb.WriteString("updated file")
		if len(paths) == 0 || len(paths) > 1 {
			sb.WriteString("s")

			if len(paths) > 1 && len(paths) <= 10 {
				// if there were 10 or less files updated, list their paths
				sb.WriteString("\n\n\n")
				for _, p := range paths {
					sb.WriteString(fmt.Sprintf("%s\n", p))
				}
			}
		} else {
			sb.WriteString(fmt.Sprintf(" %s", paths[0]))
		}
		msg = sb.String()
	}

	_, err = w.Commit(msg, &git.CommitOptions{})
	if err != nil {
		return false, fmt.Errorf("error during 'git commit': %w", err)
	}

	return true, nil
}

func (r *GitRepo) gitPush(pullBefore, isRetry bool) error {
	if pullBefore {
		// pull first to avoid conflicts
		err := r.gitPull(false)
		if err != nil {
			return err
		}
	}

	err := r.repo.Push(&git.PushOptions{
		RemoteName: defaultRemoteName,
		Auth:       r.Auth,
		RefSpecs:   []gitcfg.RefSpec{refspecFromBranch(r.Branch)},
	})
	if err != nil {
		if isRetry {
			return fmt.Errorf("error during 'git push': %w", err)
		}
		return r.gitPush(true, true)
	}

	return nil
}

func (r *GitRepo) gitPull(force bool) error {
	w, err := r.repo.Worktree()
	if err != nil {
		return fmt.Errorf("error getting worktree: %w", err)
	}

	err = w.Pull(&git.PullOptions{
		RemoteName:    defaultRemoteName,
		SingleBranch:  true,
		ReferenceName: plumbing.NewBranchReferenceName(r.Branch),
		Auth:          r.Auth,
		Force:         force,
	})
	// ignore errors which come from
	// 1. the checked-out repo already being up-to-date
	// 2. the branch not being found upstream (this can happen if it was created locally)
	if err != nil && !errors.Is(err, git.NoErrAlreadyUpToDate) && !errors.Is(err, plumbing.ErrReferenceNotFound) {
		return fmt.Errorf("error during 'git pull': %w", err)
	}

	return nil
}

func (r *GitRepo) gitOpen() error {
	if r.repo == nil {
		gitDirPath := vfs.Join(r.Fs, ".git")
		fsGitDir, err := projectionfs.New(r.Fs, gitDirPath)
		if err != nil {
			return fmt.Errorf("error creating projection filesystem: %w", err)
		}
		r.repo, err = git.Open(gitfs.NewStorage(FSWrap(fsGitDir), gitcache.NewObjectLRUDefault()), FSWrap(r.Fs))
		if err != nil {
			return fmt.Errorf("error opening existing git repository: %w", err)
		}
	}

	err := r.gitCheckout()
	if err != nil {
		r.repo = nil
		return err
	}

	err = r.gitPull(true)
	if err != nil {
		r.repo = nil
		return err
	}

	return nil
}

func (r *GitRepo) gitCheckout() error {
	w, err := r.repo.Worktree()
	if err != nil {
		return fmt.Errorf("error getting worktree: %w", err)
	}

	createBranch := false
	// try to fetch branch from remote
	err = r.repo.Fetch(&git.FetchOptions{
		RemoteName: defaultRemoteName,
		RefSpecs:   []gitcfg.RefSpec{refspecFromBranch(r.Branch)},
		Auth:       r.Auth,
	})
	if err != nil && !errors.Is(err, git.NoErrAlreadyUpToDate) {
		if !errors.Is(err, git.NoMatchingRefSpecError{}) {
			return fmt.Errorf("error during 'git fetch': %s", err)
		}
		// create branch if it doesn't exist upstream
		createBranch = true
	}

	err = w.Checkout(&git.CheckoutOptions{
		Branch: plumbing.NewBranchReferenceName(r.Branch),
		Force:  true,
		Create: createBranch,
	})
	if err != nil {
		return fmt.Errorf("error during 'git checkout': %s", err)
	}

	return nil
}

func (r *GitRepo) IsInitialized() bool {
	return r.repo != nil
}

func refspecFromBranch(branch string) gitcfg.RefSpec {
	return gitcfg.RefSpec(fmt.Sprintf("refs/heads/%s:refs/heads/%s", branch, branch))
}

// DummyRemote is a helper struct to spin up a local git repository which can be used as remote for integration testing.
type DummyRemote struct {
	RootPath string
	Branch   string
	Fs       vfs.FileSystem
	GitFs    vfs.FileSystem
	Repo     *git.Repository
	isClosed bool
}

func NewDummyRemote(fs vfs.FileSystem, branch string) (*DummyRemote, error) {
	res := &DummyRemote{
		Fs:       fs,
		Branch:   branch,
		isClosed: false,
	}
	var err error
	res.RootPath, err = vfs.TempDir(res.Fs, "", "remote-")
	if err != nil {
		return nil, fmt.Errorf("unable to create temporary directory for git remote: %w", err)
	}

	res.GitFs, err = projectionfs.New(res.Fs, res.RootPath)
	if err != nil {
		return nil, fmt.Errorf("error creating projection filesystem: %w", err)
	}

	res.Repo, err = git.InitWithOptions(gitfs.NewStorage(FSWrap(res.GitFs), gitcache.NewObjectLRUDefault()), nil, git.InitOptions{
		DefaultBranch: plumbing.NewBranchReferenceName(res.Branch),
	})
	if err != nil {
		return nil, fmt.Errorf("error during 'git init': %w", err)
	}

	return res, nil
}

// Close deletes the directory containing the remote.
func (dr *DummyRemote) Close() error {
	if dr.isClosed {
		return fmt.Errorf("remote is already closed")
	}
	if err := dr.Fs.RemoveAll(dr.RootPath); err != nil {
		return err
	}
	dr.Repo = nil
	if err := vfs.Cleanup(dr.GitFs); err != nil {
		return err
	}
	dr.GitFs = nil
	dr.isClosed = true
	return nil
}
