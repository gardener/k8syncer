// SPDX-FileCopyrightText: 2023 SAP SE or an SAP affiliate company and Gardener contributors.
//
// SPDX-License-Identifier: Apache-2.0

package filesystem

import (
	"bytes"
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
	"sigs.k8s.io/yaml"
)

var _ persist.Persister = &FileSystemPersister{}
var _ persist.LoggerInjectable = &FileSystemPersister{}

// FileSystemPersister persists data by writing it to a given file system.
type FileSystemPersister struct {
	// Fs is the FileSystem used to persist the data.
	Fs vfs.FileSystem
	// NamespacePrefix is used to prefix the names for the namespace folders.
	NamespacePrefix string
	// GVKNameSeparator is uses as a separator in the filename between the resource's gvk and its name.
	GVKNameSeparator string
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
		GVKNameSeparator: "_",
		FileExtension:    "yaml",
		RootPath:         cfg.RootPath,
	}

	if cfg.NamespacePrefix != nil {
		fsp.NamespacePrefix = *cfg.NamespacePrefix
	}
	if cfg.GVKNameSeparator != nil {
		fsp.GVKNameSeparator = *cfg.GVKNameSeparator
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
	filepath, _ := p.GetResourceFilepath(name, namespace, gvk, subPath)
	return vfs.FileExists(p.Fs, filepath)
}

func (p *FileSystemPersister) getRaw(ctx context.Context, filepath string) ([]byte, error) {
	exists, err := vfs.FileExists(p.Fs, filepath)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, nil
	}
	return vfs.ReadFile(p.Fs, filepath)
}

func (p *FileSystemPersister) Get(ctx context.Context, name, namespace string, gvk schema.GroupVersionKind, subPath string) (*unstructured.Unstructured, error) {
	filepath, _ := p.GetResourceFilepath(name, namespace, gvk, subPath)
	data, err := p.getRaw(ctx, filepath)
	if err != nil {
		return nil, err
	}
	return ConvertFromPersistence(data)
}

func (p *FileSystemPersister) persistRaw(ctx context.Context, data []byte, filepath string) error {
	dirpath := vfs.Dir(p.Fs, filepath)
	parentDirExists, err := vfs.DirExists(p.Fs, dirpath)
	if err != nil {
		return err
	}

	if !parentDirExists {
		// create directory if it doesn't exist
		err := p.Fs.MkdirAll(dirpath, os.ModeDir|os.ModePerm)
		if err != nil {
			return err
		}
	}

	return vfs.WriteFile(p.Fs, filepath, data, os.ModePerm)
}

func (p *FileSystemPersister) Persist(ctx context.Context, resource *unstructured.Unstructured, t persist.Transformer, subPath string) (*unstructured.Unstructured, bool, error) {
	filepath, _ := p.GetResourceFilepath(resource.GetName(), resource.GetNamespace(), resource.GroupVersionKind(), subPath)
	existingData, err := p.getRaw(ctx, filepath)
	if err != nil {
		return nil, false, err
	}
	transformed, err := t.Transform(resource)
	if err != nil {
		return nil, false, err
	}
	newData, err := ConvertToPersistence(transformed, nil)
	if err != nil {
		return nil, false, err
	}
	if bytes.Equal(newData, existingData) {
		return transformed, false, nil
	}
	err = p.persistRaw(ctx, newData, filepath)
	return transformed, true, err
}

func (p *FileSystemPersister) Delete(ctx context.Context, name, namespace string, gvk schema.GroupVersionKind, subPath string) error {
	filepath, nsdir := p.GetResourceFilepath(name, namespace, gvk, subPath)
	dirpath := vfs.Dir(p.Fs, filepath)
	parentDirExists, err := vfs.DirExists(p.Fs, dirpath)
	if err != nil {
		return err
	}

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
	prefixedFileExtension := p.FileExtension
	if p.FileExtension != "" && !strings.HasPrefix(p.FileExtension, ".") {
		prefixedFileExtension = fmt.Sprintf(".%s", p.FileExtension)
	}
	gvkString := utils.GVKToString(gvk, true)
	filename := fmt.Sprintf("%s%s%s%s", gvkString, p.GVKNameSeparator, name, prefixedFileExtension)
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

// ConvertToPersistence serializes the given resource into a byte array which can be stored in a filesystem persistence.
// If the given Transformer is not nil, its 'Transform' method is called on the resource before, otherwise it is converted as-is.
// This implementation basically calls yaml.Marshal on the object.
func ConvertToPersistence(obj *unstructured.Unstructured, t persist.Transformer) ([]byte, error) {
	if t != nil {
		var err error
		obj, err = t.Transform(obj)
		if err != nil {
			return nil, err
		}

	}
	data, err := yaml.Marshal(obj)
	if err != nil {
		return nil, fmt.Errorf("error while marshalling object to yaml: %w", err)
	}
	return data, nil
}

// ConvertFromPersistence is the counterpart of ConvertToPersistence and converts a byte array back to a resource.
// It basically calls yaml.Unmarshal on the given data.
func ConvertFromPersistence(data []byte) (*unstructured.Unstructured, error) {
	res := &unstructured.Unstructured{}
	err := yaml.Unmarshal(data, res)
	if err != nil {
		return nil, fmt.Errorf("error while unmarshalling object from yaml: %w", err)
	}
	return res, nil
}
