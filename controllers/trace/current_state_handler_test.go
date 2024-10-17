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
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/golang/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kbappsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	tracev1 "github.com/apecloud/kubeblocks/apis/trace/v1"
	"github.com/apecloud/kubeblocks/pkg/controller/builder"
	"github.com/apecloud/kubeblocks/pkg/controller/kubebuilderx"
)

var _ = Describe("current_state_handler test", func() {
	Context("Testing current_state_handler", func() {
		It("should work well", func() {
			store := NewObjectStore(scheme.Scheme)
			reconciler := updateCurrentState(ctx, k8sMock, scheme.Scheme, store)

			primary, _ := mockObjects()
			trace := &tracev1.ReconciliationTrace{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: namespace,
					Name:      name,
				},
				Spec: tracev1.ReconciliationTraceSpec{
					TargetObject: &tracev1.ObjectReference{
						Namespace: primary.Namespace,
						Name:      primary.Name,
					},
				},
			}
			tree := kubebuilderx.NewObjectTree()
			tree.SetRoot(trace)
			k8sMock.EXPECT().
				Get(gomock.Any(), gomock.Any(), &kbappsv1.Cluster{}, gomock.Any()).
				DoAndReturn(func(_ context.Context, objKey client.ObjectKey, obj *kbappsv1.Cluster, _ ...client.GetOption) error {
					*obj = *primary
					return nil
				})
			objectRef, err := getObjectReference(primary, scheme.Scheme)
			Expect(err).ToNot(HaveOccurred())
			event := builder.NewEventBuilder(namespace, name).SetInvolvedObject(*objectRef).GetObject()
			k8sMock.EXPECT().
				List(gomock.Any(), &corev1.EventList{}, gomock.Any()).
				DoAndReturn(func(_ context.Context, list *corev1.EventList, _ ...client.ListOption) error {
					list.Items = []corev1.Event{*event}
					return nil
				})

			res, err := reconciler.Reconcile(tree)
			Expect(err).ToNot(HaveOccurred())
			Expect(res).Should(Equal(kubebuilderx.Continue))
			Expect(trace.Status.CurrentState.ObjectTree).ShouldNot(BeNil())
			Expect(trace.Status.CurrentState.Changes).Should(HaveLen(4))
			Expect(trace.Status.CurrentState.Summary.ObjectSummaries).Should(HaveLen(2))
		})
	})
})
