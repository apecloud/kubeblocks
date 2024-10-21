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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kbappsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	tracev1 "github.com/apecloud/kubeblocks/apis/trace/v1"
	"github.com/apecloud/kubeblocks/pkg/controller/builder"
	"github.com/apecloud/kubeblocks/pkg/controller/kubebuilderx"
	testutil "github.com/apecloud/kubeblocks/pkg/testutil/k8s"
	"github.com/apecloud/kubeblocks/pkg/testutil/k8s/mocks"
)

var _ = Describe("resources_loader test", func() {
	var (
		k8sMock    *mocks.MockClient
		controller *gomock.Controller
	)

	BeforeEach(func() {
		controller, k8sMock = testutil.SetupK8sMock()
	})

	AfterEach(func() {
		controller.Finish()
	})

	Context("Testing resources_loader", func() {
		It("should work well", func() {
			trace := &tracev1.ReconciliationTrace{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: namespace,
					Name:      name,
				},
			}
			target := builder.NewClusterBuilder(namespace, name).GetObject()

			k8sMock.EXPECT().
				Get(gomock.Any(), gomock.Any(), &kbappsv1.Cluster{}, gomock.Any()).
				DoAndReturn(func(_ context.Context, objKey client.ObjectKey, obj *kbappsv1.Cluster, _ ...client.GetOption) error {
					*obj = *target
					return nil
				}).Times(1)

			tree := kubebuilderx.NewObjectTree()
			tree.SetRoot(trace)

			reconciler := resourcesValidation(ctx, k8sMock)
			Expect(reconciler.PreCondition(tree)).To(Equal(kubebuilderx.ConditionSatisfied))
			res, err := reconciler.Reconcile(tree)
			Expect(err).ToNot(HaveOccurred())
			Expect(res).To(Equal(kubebuilderx.Continue))
		})
	})
})
