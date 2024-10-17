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
			primaryV1 := builder.NewClusterBuilder(namespace, name).SetUID(uid).SetResourceVersion("2").GetObject()
			primaryV1.Generation = 1
			primaryV1.Status.Phase = kbappsv1.RunningClusterPhase
			primaryV2 := builder.NewClusterBuilder(namespace, name).SetUID(uid).SetResourceVersion("3").GetObject()
			primaryV2.Generation = 2
			ref1, err := getObjectReference(primaryV1, scheme.Scheme)
			Expect(err).ToNot(HaveOccurred())
			ref2, ok := ref1.DeepCopyObject().(*corev1.ObjectReference)
			Expect(ok).To(BeTrue())
			ref1.ResourceVersion = "1"
			ref3, err := getObjectReference(primaryV2, scheme.Scheme)
			Expect(err).ToNot(HaveOccurred())
			trace := &tracev1.ReconciliationTrace{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: namespace,
					Name:      name,
				},
				Spec: tracev1.ReconciliationTraceSpec{
					TargetObject: &tracev1.ObjectReference{
						Namespace: primaryV1.Namespace,
						Name:      primaryV1.Name,
					},
				},
				Status: tracev1.ReconciliationTraceStatus{
					CurrentState: tracev1.ReconciliationCycleState{
						Changes: []tracev1.ObjectChange{
							{
								ObjectReference: *ref1,
								ChangeType:      tracev1.ObjectCreationType,
								Revision:        parseRevision(ref1.ResourceVersion),
								Description:     "Creation",
							},
							{
								ObjectReference: *ref2,
								ChangeType:      tracev1.ObjectUpdateType,
								Revision:        parseRevision(ref2.ResourceVersion),
								Description:     "Update",
							},
							{
								ObjectReference: *ref3,
								ChangeType:      tracev1.ObjectUpdateType,
								Revision:        parseRevision(ref3.ResourceVersion),
								Description:     "Update",
							},
						},
					},
				},
			}

			store := NewObjectStore(scheme.Scheme)
			Expect(store.Insert(primaryV1, trace)).Should(Succeed())
			Expect(store.Insert(primaryV2, trace)).Should(Succeed())

			tree := kubebuilderx.NewObjectTree()
			tree.SetRoot(trace)
			k8sMock.EXPECT().
				Get(gomock.Any(), gomock.Any(), &kbappsv1.Cluster{}, gomock.Any()).
				DoAndReturn(func(_ context.Context, objKey client.ObjectKey, obj *kbappsv1.Cluster, _ ...client.GetOption) error {
					*obj = *primaryV2
					return nil
				})

			reconciler := updateDesiredState(ctx, k8sMock, scheme.Scheme, store)
			res, err := reconciler.Reconcile(tree)
			Expect(err).ToNot(HaveOccurred())
			Expect(res).Should(Equal(kubebuilderx.Continue))
			Expect(trace.Status.CurrentState.ObjectTree).ShouldNot(BeNil())
			Expect(trace.Status.CurrentState.Changes).Should(HaveLen(4))
			Expect(trace.Status.CurrentState.Summary.ObjectSummaries).Should(HaveLen(2))
		})
	})
})
