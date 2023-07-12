// SPDX-FileCopyrightText: 2023 SAP SE or an SAP affiliate company and Gardener contributors.
//
// SPDX-License-Identifier: Apache-2.0

package git

import (
	"os"
	"testing"

	"github.com/gardener/landscaper/controller-utils/pkg/logging"
	"github.com/mandelsoft/vfs/pkg/osfs"
	"github.com/mandelsoft/vfs/pkg/vfs"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestConfig(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Git Wrapper Test Suite")
}

var staticDiscardLogger = logging.Discard()

var _ = Describe("Git Wrapper Tests", func() {

	var dr *DummyRemote

	BeforeEach(func() {
		var err error
		dr, err = NewDummyRemote(osfs.OsFs, "foo")
		Expect(err).ToNot(HaveOccurred())

	})

	AfterEach(func() {
		Expect(dr.Close()).To(Succeed())
	})

	// basic idea:
	// create a local remote
	// create two repositories using that remote
	// commit a file in one repo an pull from the other
	It("should perform basic git actions", func() {
		srcRepo, err := dr.NewRepo()
		Expect(err).ToNot(HaveOccurred())

		dstRepo, err := dr.NewRepo()
		Expect(err).ToNot(HaveOccurred())

		filename := "foofile"
		srcData := []byte("testvalue")
		Expect(vfs.WriteFile(srcRepo.Fs, filename, srcData, os.ModePerm)).To(Succeed())

		Expect(srcRepo.CommitAndPush(staticDiscardLogger, false, "")).To(Succeed())

		Expect(dstRepo.Pull(staticDiscardLogger)).To(Succeed())

		dstData, err := vfs.ReadFile(dstRepo.Fs, filename)
		Expect(err).ToNot(HaveOccurred())

		Expect(dstData).To(Equal(srcData))
	})

	It("should be able to create and switch between branches on new and existing repositores", func() {
		tempdir, err := vfs.TempDir(osfs.OsFs, "", "repo-")
		Expect(err).ToNot(HaveOccurred())

		// new repo with default branch 'bar'
		branch1 := "bar"
		repo1, err := NewRepo(osfs.OsFs, dr.RootPath, branch1, tempdir, nil)
		Expect(err).ToNot(HaveOccurred())
		Expect(repo1.Initialize(staticDiscardLogger)).To(Succeed())

		branch1file := "barfile"
		Expect(vfs.WriteFile(repo1.Fs, branch1file, []byte("test"), os.ModePerm)).To(Succeed())
		Expect(repo1.CommitAndPush(staticDiscardLogger, false, "")).To(Succeed())

		tempdir, err = vfs.TempDir(osfs.OsFs, "", "repo-")
		Expect(err).ToNot(HaveOccurred())

		// new repo with default branch 'foobar'
		branch2 := "foobar"
		repo2, err := NewRepo(osfs.OsFs, dr.RootPath, branch2, tempdir, nil)
		Expect(err).ToNot(HaveOccurred())
		Expect(repo2.Initialize(staticDiscardLogger)).To(Succeed())

		branch2file := "foobarfile"
		Expect(vfs.WriteFile(repo2.Fs, branch2file, []byte("test"), os.ModePerm)).To(Succeed())
		Expect(repo2.CommitAndPush(staticDiscardLogger, false, "")).To(Succeed())

		// new repo with same default branch as the dummy remote
		repo3, err := dr.NewRepo()
		Expect(err).ToNot(HaveOccurred())
		// should be on branch "foo", so no files should exist
		exists, err := vfs.FileExists(repo3.Fs, branch1file)
		Expect(err).ToNot(HaveOccurred())
		Expect(exists).To(BeFalse(), "file '%s' should not be present on branch %s", branch1file, repo3.Branch)
		exists, err = vfs.FileExists(repo3.Fs, branch2file)
		Expect(err).ToNot(HaveOccurred())
		Expect(exists).To(BeFalse(), "file '%s' should not be present on branch %s", branch2file, repo3.Branch)

		repo3.Branch = branch1
		Expect(repo3.gitCheckout()).To(Succeed())
		// should be on branch "bar", so one file should exist
		exists, err = vfs.FileExists(repo3.Fs, branch1file)
		Expect(err).ToNot(HaveOccurred())
		Expect(exists).To(BeTrue(), "file '%s' should be present on branch %s", branch1file, branch1)
		exists, err = vfs.FileExists(repo3.Fs, branch2file)
		Expect(err).ToNot(HaveOccurred())
		Expect(exists).To(BeFalse(), "file '%s' should not be present on branch %s", branch2file, branch1)

		repo3.Branch = branch2
		Expect(repo3.gitCheckout()).To(Succeed())
		// should be on branch "foobar", so one file should exist
		exists, err = vfs.FileExists(repo3.Fs, branch1file)
		Expect(err).ToNot(HaveOccurred())
		Expect(exists).To(BeFalse(), "file '%s' should not be present on branch %s", branch1file, branch2)
		exists, err = vfs.FileExists(repo3.Fs, branch2file)
		Expect(err).ToNot(HaveOccurred())
		Expect(exists).To(BeTrue(), "file '%s' should not be present on branch %s", branch2file, branch2)

		// opening the existing repo from repo3 with its currently checked-out branch
		repo4, err := NewRepo(osfs.OsFs, dr.RootPath, repo3.Branch, repo3.LocalPath, nil)
		Expect(err).ToNot(HaveOccurred())
		Expect(repo4.Initialize(staticDiscardLogger)).To(Succeed())
		// should be on branch "foobar", so one file should exist
		exists, err = vfs.FileExists(repo3.Fs, branch1file)
		Expect(err).ToNot(HaveOccurred())
		Expect(exists).To(BeFalse(), "file '%s' should not be present on branch %s", branch1file, branch2)
		exists, err = vfs.FileExists(repo3.Fs, branch2file)
		Expect(err).ToNot(HaveOccurred())
		Expect(exists).To(BeTrue(), "file '%s' should not be present on branch %s", branch2file, branch2)

		repo4.Branch = branch1
		Expect(repo4.gitCheckout()).To(Succeed())
		// should be on branch "bar", so one file should exist
		exists, err = vfs.FileExists(repo4.Fs, branch1file)
		Expect(err).ToNot(HaveOccurred())
		Expect(exists).To(BeTrue(), "file '%s' should be present on branch %s", branch1file, branch1)
		exists, err = vfs.FileExists(repo4.Fs, branch2file)
		Expect(err).ToNot(HaveOccurred())
		Expect(exists).To(BeFalse(), "file '%s' should not be present on branch %s", branch2file, branch1)

		// opening the existing repo from repo4 with a new branch
		branch5 := "xyz"
		repo5, err := NewRepo(osfs.OsFs, dr.RootPath, branch5, repo4.LocalPath, nil)
		Expect(err).ToNot(HaveOccurred())
		Expect(repo5.Initialize(staticDiscardLogger)).To(Succeed())
		// should be on branch "xyz" which is based on "bar", so one file should exist
		exists, err = vfs.FileExists(repo5.Fs, branch1file)
		Expect(err).ToNot(HaveOccurred())
		Expect(exists).To(BeTrue(), "file '%s' should be present on branch %s", branch1file, branch5)
		exists, err = vfs.FileExists(repo5.Fs, branch2file)
		Expect(err).ToNot(HaveOccurred())
		Expect(exists).To(BeFalse(), "file '%s' should not be present on branch %s", branch2file, branch5)
	})

})
