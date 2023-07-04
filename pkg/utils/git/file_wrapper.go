// SPDX-FileCopyrightText: 2023 SAP SE or an SAP affiliate company and Gardener contributors.
//
// SPDX-License-Identifier: Apache-2.0

package git

import (
	"sync"

	"github.com/go-git/go-billy/v5"
	"github.com/mandelsoft/vfs/pkg/vfs"
)

var _ billy.File = &FWrapper{}

// FWrapper is a helper struct to map the billy.File interface to an underlying vfs.File
type FWrapper struct {
	vfs.File
	lock *sync.Mutex
}

func FWrap(fs vfs.File) billy.File {
	return &FWrapper{
		File: fs,
		lock: &sync.Mutex{},
	}
}

func (fw *FWrapper) Lock() error {
	fw.lock.Lock()
	return nil
}

func (fw *FWrapper) Unlock() error {
	fw.lock.Unlock()
	return nil
}
