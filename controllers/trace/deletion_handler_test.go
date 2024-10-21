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
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"

	tracev1 "github.com/apecloud/kubeblocks/apis/trace/v1"
	"github.com/apecloud/kubeblocks/pkg/controller/kubebuilderx"
)

var _ = Describe("deletion_handler test", func() {
	Context("Testing deletion_handler", func() {
		It("should work well", func() {
			store := NewObjectStore(scheme.Scheme)
			reconciler := handleDeletion(store)

			trace := &tracev1.ReconciliationTrace{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: namespace,
					Name:      name,
				},
			}
			tree := kubebuilderx.NewObjectTree()
			tree.SetRoot(trace)

			Expect(reconciler.PreCondition(tree)).To(Equal(kubebuilderx.ConditionUnsatisfied))
			trace.DeletionTimestamp = &metav1.Time{Time: time.Now()}
			Expect(reconciler.PreCondition(tree)).To(Equal(kubebuilderx.ConditionSatisfied))

			res, err := reconciler.Reconcile(tree)
			Expect(err).ToNot(HaveOccurred())
			Expect(res).Should(Equal(kubebuilderx.Commit))
			Expect(tree.GetRoot()).Should(BeNil())
		})
	})
})
