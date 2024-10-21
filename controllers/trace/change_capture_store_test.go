/*
Copyright (C) 2022-2024 ApeCloud Co., Ltd

This file is part of KubeBlocks project

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program.  If not, see <http://www.gnu.org/licenses/>.
*/

package trace

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"k8s.io/client-go/kubernetes/scheme"

	kbappsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	tracev1 "github.com/apecloud/kubeblocks/apis/trace/v1"
	"github.com/apecloud/kubeblocks/pkg/controller/builder"
)

var _ = Describe("change_capture_store test", func() {
	Context("Testing change_capture_store", func() {
		It("should work well", func() {
			i18n := builder.NewConfigMapBuilder(namespace, name).SetData(
				map[string]string{"en": "apps.kubeblocks.io/v1/Component/Creation=Component %s/%s is created."},
			).GetObject()
			store := newChangeCaptureStore(scheme.Scheme, buildDescriptionFormatter(i18n, defaultLocale, nil))

			By("Load a cluster")
			primary := builder.NewClusterBuilder(namespace, name).SetUID(uid).SetResourceVersion(resourceVersion).GetObject()
			Expect(store.Load(primary)).Should(Succeed())
			primaryRef, err := getObjectRef(primary, scheme.Scheme)
			Expect(err).Should(BeNil())
			Expect(store.Get(primaryRef)).ShouldNot(BeNil())

			By("Insert a component")
			compName := "test"
			fullCompName := fmt.Sprintf("%s-%s", primary.Name, compName)
			secondary := builder.NewComponentBuilder(namespace, fullCompName, "").
				SetOwnerReferences(kbappsv1.APIVersion, kbappsv1.ClusterKind, primary).
				SetUID(uid).
				GetObject()
			secondary.ResourceVersion = resourceVersion
			Expect(store.Insert(secondary)).Should(Succeed())
			objectRef, err := getObjectRef(secondary, scheme.Scheme)
			Expect(err).Should(BeNil())
			Expect(store.Get(objectRef)).ShouldNot(BeNil())

			By("Update the component")
			secondary.ResourceVersion = "123456"
			Expect(store.Update(secondary)).Should(Succeed())
			Expect(store.Get(objectRef)).Should(Equal(secondary))

			By("List all components")
			objects := store.List(&objectRef.GroupVersionKind)
			Expect(objects).Should(HaveLen(1))
			Expect(objects[0]).Should(Equal(secondary))

			By("GetAll components")
			objectMap := store.GetAll()
			Expect(objectMap).Should(HaveLen(2))
			v, ok := objectMap[*primaryRef]
			Expect(ok).Should(BeTrue())
			Expect(v).Should(Equal(primary))
			v, ok = objectMap[*objectRef]
			Expect(ok).Should(BeTrue())
			Expect(v).Should(Equal(secondary))

			By("Delete the component")
			Expect(store.Delete(secondary)).Should(Succeed())
			Expect(store.Get(objectRef)).Should(BeNil())

			By("GetChanges")
			changes := store.GetChanges()
			Expect(changes).Should(HaveLen(3))
			Expect(changes[0].ChangeType).Should(Equal(tracev1.ObjectCreationType))
			Expect(changes[1].ChangeType).Should(Equal(tracev1.ObjectUpdateType))
			Expect(changes[2].ChangeType).Should(Equal(tracev1.ObjectDeletionType))
		})
	})
})
