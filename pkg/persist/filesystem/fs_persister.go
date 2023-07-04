// SPDX-FileCopyrightText: 2023 SAP SE or an SAP affiliate company and Gardener contributors.
//
// SPDX-License-Identifier: Apache-2.0

package filesystem

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/gardener/landscaper/controller-utils/pkg/logging"

	"github.com/gardener/k8syncer/pkg/config"
	"github.com/gardener/k8syncer/pkg/persist"
	"github.com/gardener/k8syncer/pkg/utils"
	"github.com/gardener/k8syncer/pkg/utils/constants"

	"github.com/mandelsoft/vfs/pkg/memoryfs"
	"github.com/mandelsoft/vfs/pkg/osfs"
	"github.com/mandelsoft/vfs/pkg/vfs"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var _ persist.Persister = &FileSystemPersister{}
var _ persist.LoggerInjectable = &FileSystemPersister{}

// FileSystemPersister persists data by writing it to a given file system.
type FileSystemPersister struct {
	// Fs is the FileSystem used to persist the data.
	Fs vfs.FileSystem
	// NamespacePrefix is used to prefix the names for the namespace folders.
	NamespacePrefix string
	// GVRNameSeparator is uses as a separator in the filename between the resource's gvr and its name.
	GVRNameSeparator string
	// FileExtension is the extension used for the files.
	FileExtension string
	// RootPath is used as a root path.
	RootPath string

	injectedLogger *logging.Logger
}

func (p *FileSystemPersister) InjectLogger(il *logging.Logger) {
	p.injectedLogger = il
}

// New returns a new FileSystemPersister
func New(fs vfs.FileSystem, cfg *config.FileSystemConfiguration, createRootPath bool) (*FileSystemPersister, error) {
	// check if root path exists
	rootPathExists, err := vfs.DirExists(fs, cfg.RootPath)
	if err != nil {
		return nil, fmt.Errorf("error trying to verify root path existence: %w", err)
	}
	if !rootPathExists {
		if createRootPath {
			err := fs.MkdirAll(cfg.RootPath, os.ModeDir|os.ModePerm)
			if err != nil {
				return nil, fmt.Errorf("unable to create root path: %w", err)
			}
		} else {
			return nil, fmt.Errorf("specified root path '%s' does not exist or is not a directory", cfg.RootPath)
		}
	}

	fsp := &FileSystemPersister{
		Fs:               fs,
		NamespacePrefix:  "ns_",
		GVRNameSeparator: "_",
		FileExtension:    "yaml",
		RootPath:         cfg.RootPath,
	}

	if cfg.NamespacePrefix != nil {
		fsp.NamespacePrefix = *cfg.NamespacePrefix
	}
	if cfg.GVKNameSeparator != nil {
		fsp.GVRNameSeparator = *cfg.GVKNameSeparator
	}
	if cfg.FileExtension != nil {
		fsp.FileExtension = *cfg.FileExtension
	}

	fsp.injectedLogger = &persist.StaticDiscardLogger

	return fsp, nil
}

// NewForOS returns a new FileSystemPersister using the operating system's filesystem.
func NewForOS(cfg *config.FileSystemConfiguration) (*FileSystemPersister, error) {
	return New(osfs.New(), cfg, false)
}

// NewForMemory returns a new FileSystemPersister using an in-memory filesystem.
func NewForMemory(cfg *config.FileSystemConfiguration) (*FileSystemPersister, error) {
	return New(memoryfs.New(), cfg, true)
}

func (p *FileSystemPersister) Exists(ctx context.Context, name, namespace string, gvk schema.GroupVersionKind, subPath string) (bool, error) {
	filePath, _ := p.GetResourceFilepath(name, namespace, gvk, subPath)
	return vfs.FileExists(p.Fs, filePath)
}

func (p *FileSystemPersister) Get(ctx context.Context, name, namespace string, gvk schema.GroupVersionKind, subPath string) ([]byte, error) {
	filePath, _ := p.GetResourceFilepath(name, namespace, gvk, subPath)
	exists, err := vfs.FileExists(p.Fs, filePath)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, nil
	}
	return vfs.ReadFile(p.Fs, filePath)
}

func (p *FileSystemPersister) Persist(ctx context.Context, resource *unstructured.Unstructured, gvk schema.GroupVersionKind, rt persist.ResourceTransformer, subPath string) error {
	data, err := rt.TransformAndSerialize(resource)
	if err != nil {
		return fmt.Errorf("unable to transform and serialize resource: %w", err)
	}
	return p.PersistData(ctx, resource.GetName(), resource.GetNamespace(), gvk, data, subPath)
}

func (p *FileSystemPersister) PersistData(ctx context.Context, name, namespace string, gvk schema.GroupVersionKind, data []byte, subPath string) error {
	filepath, nsdir := p.GetResourceFilepath(name, namespace, gvk, subPath)
	dirpath := vfs.Dir(p.Fs, filepath)
	parentDirExists, err := vfs.DirExists(p.Fs, dirpath)
	if err != nil {
		return err
	}

	// handle deletion
	if data == nil {
		parentDirIsNamespaceDir := nsdir != "" && nsdir == vfs.Base(p.Fs, dirpath)
		fileExists, err := vfs.FileExists(p.Fs, filepath)
		if err != nil {
			return err
		}

		if fileExists {
			err := p.Fs.Remove(filepath)
			if err != nil {
				return err
			}
		}
		if parentDirExists && parentDirIsNamespaceDir {
			// check if namespace dir is now empty
			contents, err := vfs.ReadDir(p.Fs, dirpath)
			if err != nil {
				return err
			}
			if len(contents) == 0 {
				// namespace dir is empty, remove it
				err := p.Fs.RemoveAll(dirpath)
				if err != nil {
					return err
				}
			}
		}
		return nil
	}

	// handle creation/update
	if !parentDirExists {
		// create directory if it doesn't exist
		err := p.Fs.MkdirAll(dirpath, os.ModeDir|os.ModePerm)
		if err != nil {
			return err
		}
	}
	return vfs.WriteFile(p.Fs, filepath, data, os.ModePerm)
}

func (p *FileSystemPersister) Delete(ctx context.Context, name, namespace string, gvk schema.GroupVersionKind, subPath string) error {
	return p.PersistData(ctx, name, namespace, gvk, nil, subPath)
}

func (p *FileSystemPersister) InternalPersister() persist.Persister {
	return nil
}

// GetResourceFilepath returns the filepath under which the specified resource is stored and the namespace dir, if any.
// The returned namespace dir is already part of the path returned as first argument.
func (p *FileSystemPersister) GetResourceFilepath(name, namespace string, gvk schema.GroupVersionKind, subPath string) (string, string) {
	prefixedNamespace := ""
	if namespace != "" {
		prefixedNamespace = fmt.Sprintf("%s%s", p.NamespacePrefix, namespace)
	}
	prefixedFileExtension := ""
	if p.FileExtension != "" && !strings.HasPrefix(p.FileExtension, ".") {
		prefixedFileExtension = fmt.Sprintf(".%s", p.FileExtension)
	}
	gvkString := utils.GVKToString(gvk, true)
	filename := fmt.Sprintf("%s%s%s%s", gvkString, p.GVRNameSeparator, name, prefixedFileExtension)
	filepath := vfs.Join(p.Fs, p.RootPath, subPath, prefixedNamespace, filename)
	p.injectedLogger.Debug("Computed resource filepath", constants.Logging.KEY_PATH, filepath)
	return filepath, prefixedNamespace
}

// TryGetInternalFileSystemPersister tries to get the internal FileSystemPersister of the given Persister.
// The function traverses the internal Persisters until it reaches a Persister p_final which doesn't have an internal one.
// Then, p_final.(*FileSystemPersister) is returned.
func TryGetInternalFileSystemPersister(p persist.Persister) (*FileSystemPersister, bool) {
	var curP, newCurP persist.Persister
	newCurP = p
	for newCurP != nil {
		curP = newCurP
		newCurP = newCurP.InternalPersister()
	}
	fsp, ok := curP.(*FileSystemPersister)
	return fsp, ok
}
