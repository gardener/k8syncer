// SPDX-FileCopyrightText: 2023 SAP SE or an SAP affiliate company and Gardener contributors.
//
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"

	"github.com/gardener/k8syncer/pkg/config"
	mockpersist "github.com/gardener/k8syncer/pkg/persist/mock"
	"github.com/gardener/k8syncer/pkg/utils"
	testutils "github.com/gardener/k8syncer/test/utils"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	testGVK = schema.GroupVersionKind{
		Group:   "k8syncer.gardener.cloud",
		Version: "v1",
		Kind:    "Dummy",
	}
	testStorageRef = &config.StorageReference{
		Name:    "mockStorage",
		SubPath: "subpath",
	}
)

var _ = Describe("Controller Tests", func() {

	var (
		ctrl          *Controller
		mockPersister *mockpersist.MockPersister
		namespace     *corev1.Namespace
		ctx           context.Context
	)

	BeforeEach(func() {
		pers, err := mockpersist.New(nil, &config.FileSystemConfiguration{
			RootPath: "/data",
		}, true)
		Expect(err).ToNot(HaveOccurred())

		// mockpersist.New returns a log-wrapped persister, but we need the MockPersister-specific methods, so we have to 'unpack' it again
		for p := pers; p != nil; {
			mp, ok := p.(*mockpersist.MockPersister)
			if ok {
				mockPersister = mp
				break
			}
			p = p.InternalPersister()
		}
		Expect(mockPersister).ToNot(BeNil(), "unable to unwrap persister into MockPersister")

		ctrl = &Controller{
			Client: testenv.Client,
			GVK:    testGVK,
			SyncConfig: &config.SyncConfig{
				ID: "dummyWatcher",
				Resource: &config.ResourceSyncConfig{
					Group:   testGVK.Group,
					Version: testGVK.Version,
					Kind:    testGVK.Kind,
				},
				StorageRefs: []*config.StorageReference{testStorageRef},
				Finalize:    utils.Ptr(true),
			},
			StorageConfigs: []*StorageConfiguration{
				{
					StorageReference: testStorageRef,
					StorageDefinition: &config.StorageDefinition{
						Name: testStorageRef.Name,
						Type: config.STORAGE_TYPE_MOCK,
					},
					Persister:   pers,
					Transformer: basicTransformer,
				},
			},
		}

		ctx = context.Background()
		namespace = &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "test-",
			},
		}
		Expect(testenv.Client.Create(ctx, namespace)).To(Succeed())
	})

	AfterEach(func() {
		Expect(mockPersister.ClearExpectedCalls()).To(BeEmpty(), "did not receive one or more expected calls")
		Expect(testutils.FinalizeAll(ctx, testenv.Client, testGVK, namespace.GetName())).To(Succeed())
		Expect(testenv.Client.Delete(ctx, namespace)).To(Succeed())
	})

	It("should persist resources", func() {
		obj := &unstructured.Unstructured{}
		obj.SetGroupVersionKind(testGVK)
		obj.SetName("persist-new")
		obj.SetNamespace(namespace.GetName())

		Expect(testenv.Client.Create(ctx, obj)).To(Succeed())

		By("persisting a new resource")
		mockPersister.ExpectCall(mockpersist.MockedPersistCall(obj, basicTransformer, testStorageRef.SubPath))
		_, err := ctrl.Reconcile(ctx, testutils.ReconcileRequestFromObject(obj))
		Expect(err).ToNot(HaveOccurred())

		By("should react on label changes")
		old := obj.DeepCopy()
		obj.SetLabels(map[string]string{
			"foo.bar.baz": "foobar",
		})
		Expect(testenv.Client.Patch(ctx, obj, client.MergeFrom(old))).To(Succeed())
		mockPersister.ExpectCall(mockpersist.MockedPersistCall(obj, basicTransformer, testStorageRef.SubPath))
		_, err = ctrl.Reconcile(ctx, testutils.ReconcileRequestFromObject(obj))
		Expect(err).ToNot(HaveOccurred())
	})

})
