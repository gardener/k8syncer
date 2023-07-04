// SPDX-FileCopyrightText: 2023 SAP SE or an SAP affiliate company and Gardener contributors.
//
// SPDX-License-Identifier: Apache-2.0

package git

import (
	"fmt"
	"os"

	"github.com/go-git/go-billy/v5"
	"github.com/mandelsoft/vfs/pkg/projectionfs"
	"github.com/mandelsoft/vfs/pkg/vfs"
)

var _ billy.Filesystem = &FSWrapper{}

// FSWrapper is a helper struct to map the billy.Filesystem interface to an underlying vfs.FileSystem
type FSWrapper struct {
	vfs.FileSystem
}

func FSWrap(fs vfs.FileSystem) billy.Filesystem {
	return &FSWrapper{fs}
}

func (fsw *FSWrapper) Open(filename string) (billy.File, error) {
	file, err := fsw.FileSystem.Open(filename)
	return FWrap(file), wrapIsNotExistError(filename, err)
}

func (fsw *FSWrapper) OpenFile(filename string, flag int, perm os.FileMode) (billy.File, error) {
	// the OpenFile implementation of billy.Filesystem creates missing parent directories, opposed to os.OpenFile or the vfs.FileSystem implementation.
	if flag&os.O_CREATE != 0 {
		// create flag is set
		// create parent directories to match the billy.Filesystem implementation
		dirname := vfs.Dir(fsw.FileSystem, filename)
		if dirname != "" && dirname != "/" && dirname != "." && dirname != ".." {
			if err := fsw.MkdirAll(dirname, os.ModeDir|os.ModePerm); err != nil {
				return nil, fmt.Errorf("error creating parent directories (%s): %w", dirname, err)
			}
		}
	}
	file, err := fsw.FileSystem.OpenFile(filename, flag, perm)
	if err != nil {
		return nil, err
	}
	return FWrap(file), err
}

func (fsw *FSWrapper) Rename(oldpath, newpath string) error {
	// the Rename implementation of billy.Filesystem seems to create missing parent directories for the target location
	dirname := vfs.Dir(fsw.FileSystem, newpath)
	if dirname != "" && dirname != "/" && dirname != "." && dirname != ".." {
		if err := fsw.MkdirAll(dirname, os.ModeDir|os.ModePerm); err != nil {
			return fmt.Errorf("error creating parent directories (%s): %w", dirname, err)
		}
	}
	return fsw.FileSystem.Rename(oldpath, newpath)
}

func (fsw *FSWrapper) TempFile(dir string, prefix string) (billy.File, error) {
	file, err := vfs.TempFile(fsw.FileSystem, dir, prefix)
	if err != nil {
		return nil, err
	}
	return FWrap(file), err
}

func (fsw *FSWrapper) ReadDir(path string) ([]os.FileInfo, error) {
	fis, err := vfs.ReadDir(fsw.FileSystem, path)
	if err != nil {
		return nil, err
	}
	for i := range fis {
		fis[i] = wrapFileInfo(fis[i])
	}
	return fis, nil
}

// Root returns the root path of the filesystem.
func (fsw *FSWrapper) Root() string {
	return projectionfs.Root(fsw.FileSystem)
}

func (fsw *FSWrapper) Chroot(path string) (billy.Filesystem, error) {
	pfs, err := projectionfs.New(fsw.FileSystem, path)
	if err != nil {
		return nil, err
	}
	return FSWrap(pfs), nil
}

func (fsw *FSWrapper) Create(filename string) (billy.File, error) {
	file, err := fsw.FileSystem.Create(filename)
	if err != nil {
		if !vfs.IsErrExist(err) {
			return nil, err
		}
		// Create should not return an error if the file exists and instead just overwrite it
		err = fsw.FileSystem.Remove(filename)
		if err != nil {
			return nil, fmt.Errorf("error during fs.Create workaround deletion: %w", err)
		}
		file, err = fsw.FileSystem.Create(filename)
		if err != nil {
			return nil, err
		}
	}
	return FWrap(file), err
}

func (fsw *FSWrapper) Join(elem ...string) string {
	return vfs.Join(fsw.FileSystem, elem...)
}

func (fsw *FSWrapper) Stat(filename string) (os.FileInfo, error) {
	fi, err := fsw.FileSystem.Stat(filename)
	return wrapFileInfo(fi), wrapIsNotExistError(filename, err)
}

func (fsw *FSWrapper) Lstat(filename string) (os.FileInfo, error) {
	fi, err := fsw.FileSystem.Lstat(filename)
	return wrapFileInfo(fi), wrapIsNotExistError(filename, err)
}

// In some cases os.IsNotExist doesn't recognize an error correctly.
// Since this check is used excessively in the go-git implementation, we have to ensure that these errors are recognized.
// This is a somewhat ugly workaround which slightly changes the error message
// but enables using the upstream go-git implementation instead of a fork.
// Can (probably) be removed if https://github.com/go-git/go-git/pull/798 gets merged.
func wrapIsNotExistError(path string, err error) error {
	if err == nil {
		return nil
	}
	if !os.IsNotExist(err) && vfs.IsErrNotExist(err) {
		return &os.PathError{
			Op:   "stat",
			Path: path,
			Err:  os.ErrNotExist,
		}
	}
	return err
}

// When using vfs.memoryfs, the files are flagged as temporary.
// The git library can't handle this, so we just overwrite that flag to avoid troubles.
func wrapFileInfo(fi os.FileInfo) os.FileInfo {
	if fi == nil {
		return nil
	}
	return &dummyFileInfo{fi}
}

type dummyFileInfo struct {
	os.FileInfo
}

func (dfi *dummyFileInfo) Mode() os.FileMode {
	mode := dfi.FileInfo.Mode()
	if mode&os.ModeTemporary != 0 {
		mode = mode &^ os.ModeTemporary
	}
	return mode
}
