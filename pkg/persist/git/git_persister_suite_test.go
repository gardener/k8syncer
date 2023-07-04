// SPDX-FileCopyrightText: 2023 SAP SE or an SAP affiliate company and Gardener contributors.
//
// SPDX-License-Identifier: Apache-2.0

package git

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/gardener/landscaper/controller-utils/pkg/logging"
	"github.com/mandelsoft/vfs/pkg/osfs"
	"github.com/mandelsoft/vfs/pkg/vfs"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/gardener/k8syncer/pkg/config"
	fspersist "github.com/gardener/k8syncer/pkg/persist/filesystem"
	"github.com/gardener/k8syncer/pkg/persist/transformers"
	"github.com/gardener/k8syncer/pkg/utils"
	"github.com/gardener/k8syncer/pkg/utils/git"
)

func TestConfig(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Git Persister Test Suite")
}

var staticDiscardLogger = logging.Discard()

var _ = Describe("Git Persister Tests", func() {

	var (
		stDef            *config.StorageDefinition
		dummy            *unstructured.Unstructured
		basicTransformer = transformers.NewBasic()
		ctx              context.Context
		subPath          string
		dr               *git.DummyRemote
		branch           = "master"
	)

	BeforeEach(func() {
		var err error
		dr, err = git.NewDummyRemote(osfs.OsFs, branch)
		Expect(err).ToNot(HaveOccurred())
		stDef = &config.StorageDefinition{
			Name: "myStorage",
			Type: config.STORAGE_TYPE_GIT,
			FileSystemConfig: &config.FileSystemConfiguration{
				NamespacePrefix:  utils.Ptr("ns_"),
				GVKNameSeparator: utils.Ptr("_"),
				FileExtension:    utils.Ptr("yaml"),
				InMemory:         utils.Ptr(true),
				RootPath:         "/tmp",
			},
			GitConfig: &config.GitConfiguration{
				URL:       dr.RootPath,
				Branch:    branch,
				Exclusive: true,
			},
		}

		dummy = &unstructured.Unstructured{}
		dummy.SetName("foo")
		dummy.SetNamespace("bar")
		dummy.SetGroupVersionKind(schema.GroupVersionKind{
			Group:   "k8syncer.gardener.cloud",
			Version: "v1",
			Kind:    "Dummy",
		})
		Expect(unstructured.SetNestedField(dummy.Object, fmt.Sprint(time.Now().Unix()), "spec", "value")).To(Succeed())

		ctx = logging.NewContext(context.Background(), logging.Discard())

		subPath = ""
	})

	AfterEach(func() {
		Expect(dr.Close()).To(Succeed())
	})

	It("should correctly handle persisted resources", func() {
		// workaround: go-git currently cannot delete the last file in a repository, see https://github.com/go-git/go-git/issues/723
		testRepo, err := dr.NewRepo()
		Expect(err).ToNot(HaveOccurred())
		Expect(vfs.WriteFile(testRepo.Fs, "preventEmpty", []byte{}, os.ModePerm)).To(Succeed())
		Expect(testRepo.CommitAndPush(staticDiscardLogger, false, "add dummy file so repo won't be empty"))

		gp, err := New(ctx, stDef)
		Expect(err).ToNot(HaveOccurred())

		By("persisting a new resource")
		persisted, changed, err := gp.Persist(ctx, dummy, basicTransformer, subPath)
		Expect(err).ToNot(HaveOccurred())
		Expect(changed).To(BeTrue())

		transformed, err := basicTransformer.Transform(dummy)
		Expect(err).ToNot(HaveOccurred())

		Expect(persisted).To(Equal(transformed))

		Expect(testRepo.Pull(staticDiscardLogger)).To(Succeed())

		internalFsp, ok := gp.InternalPersister().(*fspersist.FileSystemPersister)
		Expect(ok).To(BeTrue())
		dummyFile, _ := internalFsp.GetResourceFilepath(dummy.GetName(), dummy.GetNamespace(), dummy.GroupVersionKind(), subPath, false)

		storedRaw, err := vfs.ReadFile(testRepo.Fs, dummyFile)
		Expect(err).ToNot(HaveOccurred())

		stored, err := fspersist.ConvertFromPersistence(storedRaw)
		Expect(err).ToNot(HaveOccurred())

		Expect(stored).To(Equal(transformed))

		By("open an existing repository")
		gp, err = New(ctx, stDef)
		Expect(err).ToNot(HaveOccurred())

		By("update an existing resource without any changes")
		_, changed, err = gp.Persist(ctx, dummy, basicTransformer, subPath)
		Expect(err).ToNot(HaveOccurred())
		Expect(changed).To(BeFalse())

		By("update an existing resource")
		al := map[string]string{
			"foo.bar.baz": "foobar",
		}
		dummy.SetAnnotations(al)
		dummy.SetLabels(al)

		transformed, err = basicTransformer.Transform(dummy)
		Expect(err).ToNot(HaveOccurred())

		persisted, changed, err = gp.Persist(ctx, dummy, basicTransformer, subPath)
		Expect(err).ToNot(HaveOccurred())
		Expect(changed).To(BeTrue())

		Expect(persisted).To(Equal(transformed))

		Expect(persisted.GetLabels()).To(Equal(al))

		Expect(testRepo.Pull(staticDiscardLogger)).To(Succeed())
		storedRaw, err = vfs.ReadFile(testRepo.Fs, dummyFile)
		Expect(err).ToNot(HaveOccurred())

		stored, err = fspersist.ConvertFromPersistence(storedRaw)
		Expect(err).ToNot(HaveOccurred())

		Expect(stored).To(Equal(transformed))

		By("getting a resource from persistence")
		stored, err = gp.Get(ctx, dummy.GetName(), dummy.GetNamespace(), dummy.GroupVersionKind(), subPath)
		Expect(err).ToNot(HaveOccurred())
		Expect(stored).To(Equal(transformed))

		By("checking for existence of a resource")
		exists, err := gp.Exists(ctx, dummy.GetName(), dummy.GetNamespace(), dummy.GroupVersionKind(), subPath)
		Expect(err).ToNot(HaveOccurred())
		Expect(exists).To(BeTrue())

		By("deleting a resource from persistence")
		err = gp.Delete(ctx, dummy.GetName(), dummy.GetNamespace(), dummy.GroupVersionKind(), subPath)
		Expect(err).ToNot(HaveOccurred())

		Expect(testRepo.Pull(staticDiscardLogger)).To(Succeed())
		exists, err = vfs.DirExists(testRepo.Fs, dummyFile)
		Expect(err).ToNot(HaveOccurred())
		Expect(exists).To(BeFalse())

		exists, err = gp.Exists(ctx, dummy.GetName(), dummy.GetNamespace(), dummy.GroupVersionKind(), subPath)
		Expect(err).ToNot(HaveOccurred())
		Expect(exists).To(BeFalse())
	})

})
