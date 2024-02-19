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
	"time"

	"github.com/gardener/landscaper/controller-utils/pkg/logging"
	"github.com/go-git/go-git/v5"
	gitcfg "github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	gitcache "github.com/go-git/go-git/v5/plumbing/cache"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/transport"
	gitfs "github.com/go-git/go-git/v5/storage/filesystem"
	"github.com/mandelsoft/vfs/pkg/projectionfs"
	"github.com/mandelsoft/vfs/pkg/vfs"
)

const defaultRemoteName = "origin"

var ErrNotInitialized = fmt.Errorf("git repo is not initialized, call repo.Initialize first")

// GitRepo is a helper struct which abstracts from the git commands.
// Use NewRepo to instantiate this struct.
type GitRepo struct {
	// URL is the git repo URL.
	URL string
	// Branch is the branch of the repo which should be used.
	Branch string
	// LocalPath is the filesystem path where the repo should be checked out to.
	LocalPath string
	// Auth is the authentification information for the git repository.
	Auth transport.AuthMethod
	// SecondaryAuth is the secondary authentification information for the git repository. It is used if the first one failed and may be nil.
	SecondaryAuth transport.AuthMethod
	// Fs is the filesystem used for the repository.
	Fs vfs.FileSystem

	repo               *git.Repository
	hasUnpushedCommits bool
	lock               *sync.Mutex
}

// NewRepo creates a new GitRepo instance, which can be used to interact with a git repository.
// Note that this only initializes the struct, in order to perform any git actions on the repository, Initialize has to be called first.
// The GitRepo uses a projection filesystem projecting to the given localPath. This means that all operations on the returned GitRepo's filesystem have to treat the repository directory as root.
func NewRepo(baseFs vfs.FileSystem, url, branch, localPath string, auth, secondaryAuth transport.AuthMethod) (*GitRepo, error) {
	fs, err := projectionfs.New(baseFs, localPath)
	if err != nil {
		return nil, fmt.Errorf("error creating projection filesystem: %w", err)
	}
	return &GitRepo{
		URL:                url,
		Branch:             branch,
		LocalPath:          localPath,
		Auth:               auth,
		SecondaryAuth:      secondaryAuth,
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

	r.repo, err = git.InitWithOptions(gitfs.NewStorage(FSWrap(fsGitDir), gitcache.NewObjectLRUDefault()), FSWrap(r.Fs), git.InitOptions{
		DefaultBranch: plumbing.NewBranchReferenceName(r.Branch),
	})
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

	_, err = w.Commit(msg, &git.CommitOptions{
		Author: K8SyncerAuthor(),
	})
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

	pushOptions := &git.PushOptions{
		RemoteName: defaultRemoteName,
		Auth:       r.Auth,
		RefSpecs:   []gitcfg.RefSpec{refspecFromBranch(r.Branch)},
	}
	err := r.repo.Push(pushOptions)
	if err != nil {
		if errors.Is(err, transport.ErrAuthorizationFailed) && r.SecondaryAuth != nil {
			// try with secondary auth information
			pushOptions.Auth = r.SecondaryAuth
			err2 := r.repo.Push(pushOptions)
			if err2 == nil {
				// successful with second auth, ignore error from primary auth try
				return nil
			}
			return fmt.Errorf("error during 'git push' (secondary auth): %w", err2)
		}
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

	pullOptions := &git.PullOptions{
		RemoteName:    defaultRemoteName,
		SingleBranch:  true,
		ReferenceName: plumbing.NewBranchReferenceName(r.Branch),
		Auth:          r.Auth,
		Force:         force,
	}
	err = w.Pull(pullOptions)
	// ignore errors which come from
	// 1. the checked-out repo already being up-to-date
	// 2. the branch not being found upstream (this can happen if it was created locally)
	// 3. the repository being empty
	if err != nil && !errors.Is(err, git.NoErrAlreadyUpToDate) && !errors.Is(err, plumbing.ErrReferenceNotFound) && !errors.Is(err, git.NoMatchingRefSpecError{}) && !errors.Is(err, transport.ErrEmptyRemoteRepository) {
		if errors.Is(err, transport.ErrAuthorizationFailed) && r.SecondaryAuth != nil {
			pullOptions.Auth = r.SecondaryAuth
			err2 := w.Pull(pullOptions)
			if err2 != nil && !errors.Is(err2, git.NoErrAlreadyUpToDate) && !errors.Is(err2, plumbing.ErrReferenceNotFound) && !errors.Is(err2, git.NoMatchingRefSpecError{}) && !errors.Is(err2, transport.ErrEmptyRemoteRepository) {
				return fmt.Errorf("error during 'git pull' (secondary auth): %w", err2)
			}
		} else {
			return fmt.Errorf("error during 'git pull': %w", err)
		}
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

	branchRef := plumbing.NewBranchReferenceName(r.Branch)
	// try to fetch branch from remote
	fetchOptions := &git.FetchOptions{
		RemoteName: defaultRemoteName,
		RefSpecs:   []gitcfg.RefSpec{refspecFromBranch(r.Branch)},
		Auth:       r.Auth,
	}
	err = r.repo.Fetch(fetchOptions)
	if err != nil && !errors.Is(err, git.NoErrAlreadyUpToDate) && !errors.Is(err, git.NoMatchingRefSpecError{}) && !errors.Is(err, transport.ErrEmptyRemoteRepository) {
		if errors.Is(err, transport.ErrAuthorizationFailed) && r.SecondaryAuth != nil {
			fetchOptions.Auth = r.SecondaryAuth
			err2 := r.repo.Fetch(fetchOptions)
			if err2 != nil && !errors.Is(err2, git.NoErrAlreadyUpToDate) && !errors.Is(err2, git.NoMatchingRefSpecError{}) && !errors.Is(err2, transport.ErrEmptyRemoteRepository) {
				return fmt.Errorf("error during 'git fetch' (secondary auth): %s", err2)
			}
		} else {
			return fmt.Errorf("error during 'git fetch': %s", err)
		}
	}

	// evaluate whether branch exists
	_, err = r.repo.Storer.Reference(branchRef)
	branchExists := err == nil

	hash := plumbing.ZeroHash
	if !branchExists {
		// branch has to be created
		// check if there is a commit hash for HEAD
		_, err := r.repo.Head()
		if err != nil {
			// go-git currently cannot create new branches on 'empty' repositories (no head commit in current branch), see
			// https://github.com/go-git/go-git/issues/481
			// https://github.com/go-git/go-git/issues/587
			// this is a workaround which creates an empty dummy commit in order to have a hash to create the branch from
			hash, err = w.Commit("dummy initial commit", &git.CommitOptions{
				AllowEmptyCommits: true,
				Author:            K8SyncerAuthor(),
			})
			if err != nil {
				return fmt.Errorf("error creating dummy initial commit: %w", err)
			}

			// re-evaluate branch existence, as the commit could have created the branch
			_, err = r.repo.Storer.Reference(branchRef)
			branchExists = err == nil
			if branchExists {
				// if the 'Create' option is false, 'Branch' and 'Hash' both specify what to checkout and are mutually exclusive
				hash = plumbing.ZeroHash
			}
		}
	}

	err = w.Checkout(&git.CheckoutOptions{
		Branch: branchRef,
		Force:  true,
		Create: !branchExists,
		Hash:   hash,
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

// NewRepo returns a new GitRepo configured for the dummy remote.
// The repository uses a temporary directory on the remote's filesystem and is already initialized.
func (dr *DummyRemote) NewRepo() (*GitRepo, error) {
	tmpdir, err := vfs.TempDir(dr.Fs, "", "repo-")
	if err != nil {
		return nil, err
	}

	repo, err := NewRepo(dr.Fs, dr.RootPath, dr.Branch, tmpdir, nil, nil)
	if err != nil {
		return nil, err
	}

	err = repo.Initialize(logging.Discard())
	if err != nil {
		return nil, err
	}

	return repo, nil
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

// K8SyncerAuthor returns a dummy signature object which is used for commits.
func K8SyncerAuthor() *object.Signature {
	return &object.Signature{
		Name:  "K8Syncer",
		Email: "k8syncer@example.org",
		When:  time.Now(),
	}
}
