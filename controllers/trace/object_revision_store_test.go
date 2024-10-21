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

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/kubernetes/scheme"

	kbappsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	"github.com/apecloud/kubeblocks/pkg/controller/builder"
)

var _ = Describe("object_revision_store test", func() {
	Context("Testing object_revision_store", func() {
		It("should work well", func() {
			store := NewObjectStore(scheme.Scheme)

			By("Insert a component")
			primary := builder.NewClusterBuilder(namespace, name).SetUID(uid).SetResourceVersion(resourceVersion).GetObject()
			compName := "test"
			fullCompName := fmt.Sprintf("%s-%s", primary.Name, compName)
			secondary := builder.NewComponentBuilder(namespace, fullCompName, "").
				SetOwnerReferences(kbappsv1.APIVersion, kbappsv1.ClusterKind, primary).
				SetUID(uid).
				GetObject()
			secondary.ResourceVersion = resourceVersion
			Expect(store.Insert(secondary, primary)).Should(Succeed())
			objectRef, err := getObjectRef(secondary, scheme.Scheme)
			Expect(err).Should(BeNil())

			By("Get the component with right revision")
			revision := parseRevision(secondary.ResourceVersion)
			obj, err := store.Get(objectRef, revision)
			Expect(err).Should(BeNil())
			Expect(obj).Should(Equal(secondary))

			By("Get the component with wrong revision")
			_, err = store.Get(objectRef, revision+1)
			Expect(err).ShouldNot(BeNil())
			Expect(apierrors.IsNotFound(err)).Should(BeTrue())

			By("List all components")
			objects := store.List(&objectRef.GroupVersionKind)
			Expect(objects).Should(HaveLen(1))
			revisionMap, ok := objects[objectRef.ObjectKey]
			Expect(ok).Should(BeTrue())
			Expect(revisionMap).Should(HaveLen(1))
			obj, ok = revisionMap[revision]
			Expect(ok).Should(BeTrue())
			Expect(obj).Should(Equal(secondary))

			By("Delete the component")
			store.Delete(objectRef, primary, revision)
			_, err = store.Get(objectRef, revision)
			Expect(err).ShouldNot(BeNil())
			Expect(apierrors.IsNotFound(err)).Should(BeTrue())
		})
	})
})
