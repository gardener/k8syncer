// SPDX-FileCopyrightText: 2023 SAP SE or an SAP affiliate company and Gardener contributors.
//
// SPDX-License-Identifier: Apache-2.0

package filesystem

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/gardener/landscaper/controller-utils/pkg/logging"
	"github.com/mandelsoft/vfs/pkg/memoryfs"
	"github.com/mandelsoft/vfs/pkg/vfs"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/gardener/k8syncer/pkg/config"
	"github.com/gardener/k8syncer/pkg/persist/transformers"
	"github.com/gardener/k8syncer/pkg/utils"
)

func TestConfig(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Filesystem Persister Test Suite")
}

var _ = Describe("Filesystem Persister Tests", func() {

	var (
		cfg              *config.FileSystemConfiguration
		dummy            *unstructured.Unstructured
		basicTransformer = transformers.NewBasic()
		fs               vfs.FileSystem
		ctx              context.Context
		subPath          string
	)

	BeforeEach(func() {
		cfg = &config.FileSystemConfiguration{
			NamespacePrefix:  utils.Ptr("ns_"),
			GVKNameSeparator: utils.Ptr("_"),
			FileExtension:    utils.Ptr("yaml"),
			InMemory:         utils.Ptr(true),
			RootPath:         "/tmp",
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

		fs = memoryfs.New()
		ctx = logging.NewContext(context.Background(), logging.Discard())

		subPath = ""
	})

	AfterEach(func() {
		Expect(vfs.Cleanup(fs)).To(Succeed())
	})

	It("should correctly handle persisted resources", func() {
		fsp, err := New(fs, cfg, true)
		Expect(err).ToNot(HaveOccurred())

		By("persisting a new resource")
		persisted, changed, err := fsp.Persist(ctx, dummy, basicTransformer, subPath)
		Expect(err).ToNot(HaveOccurred())
		Expect(changed).To(BeTrue())

		transformed, err := basicTransformer.Transform(dummy)
		Expect(err).ToNot(HaveOccurred())

		Expect(persisted).To(Equal(transformed))

		dummyFile, _ := fsp.GetResourceFilepath(dummy.GetName(), dummy.GetNamespace(), dummy.GroupVersionKind(), subPath, true)

		storedRaw, err := vfs.ReadFile(fs, dummyFile)
		Expect(err).ToNot(HaveOccurred())

		stored, err := ConvertFromPersistence(storedRaw)
		Expect(err).ToNot(HaveOccurred())

		Expect(stored).To(Equal(transformed))

		By("update an existing resource without any changes")
		_, changed, err = fsp.Persist(ctx, dummy, basicTransformer, subPath)
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

		persisted, changed, err = fsp.Persist(ctx, dummy, basicTransformer, subPath)
		Expect(err).ToNot(HaveOccurred())
		Expect(changed).To(BeTrue())

		Expect(persisted).To(Equal(transformed))

		Expect(persisted.GetLabels()).To(Equal(al))

		storedRaw, err = vfs.ReadFile(fs, dummyFile)
		Expect(err).ToNot(HaveOccurred())

		stored, err = ConvertFromPersistence(storedRaw)
		Expect(err).ToNot(HaveOccurred())

		Expect(stored).To(Equal(transformed))

		By("getting a resource from persistence")
		stored, err = fsp.Get(ctx, dummy.GetName(), dummy.GetNamespace(), dummy.GroupVersionKind(), subPath)
		Expect(err).ToNot(HaveOccurred())
		Expect(stored).To(Equal(transformed))

		By("checking for existence of a resource")
		exists, err := fsp.Exists(ctx, dummy.GetName(), dummy.GetNamespace(), dummy.GroupVersionKind(), subPath)
		Expect(err).ToNot(HaveOccurred())
		Expect(exists).To(BeTrue())

		By("deleting a resource from persistence")
		err = fsp.Delete(ctx, dummy.GetName(), dummy.GetNamespace(), dummy.GroupVersionKind(), subPath)
		Expect(err).ToNot(HaveOccurred())

		exists, err = vfs.Exists(fs, dummyFile)
		Expect(err).ToNot(HaveOccurred())
		Expect(exists).To(BeFalse())

		// it was the only resource in that namespace, so the namespace directory should have been removed too
		exists, err = vfs.Exists(fs, vfs.Dir(fs, dummyFile))
		Expect(err).ToNot(HaveOccurred())
		Expect(exists).To(BeFalse())

		exists, err = fsp.Exists(ctx, dummy.GetName(), dummy.GetNamespace(), dummy.GroupVersionKind(), subPath)
		Expect(err).ToNot(HaveOccurred())
		Expect(exists).To(BeFalse())
	})

	It("should correctly compute resource filepaths", func() {
		fsp, err := New(fs, cfg, true)
		Expect(err).ToNot(HaveOccurred())

		name := "my-resource"
		namespace := "my-namespace"
		gvk := dummy.GroupVersionKind()

		By("default values, namespaced resource, empty subPath")
		file, dir := fsp.GetResourceFilepath(name, namespace, gvk, subPath, true)
		Expect(dir).To(Equal(fmt.Sprintf("%s%s", *cfg.NamespacePrefix, namespace)))
		Expect(file).To(Equal(vfs.Join(fs, cfg.RootPath, subPath, dir, fmt.Sprintf("%s%s%s.%s", utils.GVKToString(gvk, true), *cfg.GVKNameSeparator, name, *cfg.FileExtension))))

		By("default values, non-namespaced, empty subPath")
		file, dir = fsp.GetResourceFilepath(name, "", gvk, subPath, true)
		Expect(dir).To(BeEmpty())
		Expect(vfs.Dir(fs, file)).To(Equal(vfs.Join(fs, cfg.RootPath, subPath)))

		By("default values, namespaced resource, non-empty subPath")
		subPath = "subPath"
		file, dir = fsp.GetResourceFilepath(name, namespace, gvk, subPath, true)
		Expect(dir).To(Equal(fmt.Sprintf("%s%s", *cfg.NamespacePrefix, namespace)))
		Expect(vfs.Dir(fs, file)).To(Equal(vfs.Join(fs, cfg.RootPath, subPath, dir)))

		By("default values, namespaced resource, non-empty subPath, without root path")
		subPath = "subPath"
		file, dir = fsp.GetResourceFilepath(name, namespace, gvk, subPath, false)
		Expect(dir).To(Equal(fmt.Sprintf("%s%s", *cfg.NamespacePrefix, namespace)))
		Expect(vfs.Dir(fs, file)).To(Equal(vfs.Join(fs, subPath, dir)))

		By("default values, non-namespaced, non-empty subPath")
		file, dir = fsp.GetResourceFilepath(name, "", gvk, subPath, true)
		Expect(dir).To(BeEmpty())
		Expect(file).To(Equal(vfs.Join(fs, cfg.RootPath, subPath, fmt.Sprintf("%s%s%s.%s", utils.GVKToString(gvk, true), *cfg.GVKNameSeparator, name, *cfg.FileExtension))))

		By("non-default values")
		fsp, err = New(fs, &config.FileSystemConfiguration{
			NamespacePrefix:  utils.Ptr("&"),
			GVKNameSeparator: utils.Ptr("#"),
			FileExtension:    utils.Ptr(".txt"),
			RootPath:         "/my/root/path",
			InMemory:         utils.Ptr(true),
		}, true)
		Expect(err).ToNot(HaveOccurred())
		file, dir = fsp.GetResourceFilepath(name, namespace, gvk, subPath, true)
		Expect(dir).To(Equal(fmt.Sprintf("&%s", namespace)))
		Expect(file).To(Equal(fmt.Sprintf("/my/root/path/%s/&%s/%s#%s.txt", subPath, namespace, utils.GVKToString(gvk, true), name)))
	})

})
