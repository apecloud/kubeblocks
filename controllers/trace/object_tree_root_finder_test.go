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
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/golang/mock/gomock"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	kbappsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	tracev1 "github.com/apecloud/kubeblocks/apis/trace/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/builder"
	testutil "github.com/apecloud/kubeblocks/pkg/testutil/k8s"
	"github.com/apecloud/kubeblocks/pkg/testutil/k8s/mocks"
)

var _ = Describe("object_tree_root_finder test", func() {
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

	Context("Testing object_tree_root_finder", func() {
		It("should work well", func() {
			trace := &tracev1.ReconciliationTrace{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: namespace,
					Name:      name,
				},
			}
			root := builder.NewClusterBuilder(namespace, name).SetUID(uid).SetResourceVersion(resourceVersion).GetObject()
			compName := "test"
			fullCompName := fmt.Sprintf("%s-%s", root.Name, compName)
			secondary := builder.NewComponentBuilder(namespace, fullCompName, "").
				SetOwnerReferences(kbappsv1.APIVersion, kbappsv1.ClusterKind, root).
				SetUID(uid).
				AddLabels(constant.AppManagedByLabelKey, constant.AppName).
				AddLabels(constant.AppInstanceLabelKey, root.Name).
				GetObject()

			k8sMock.EXPECT().
				List(gomock.Any(), &kbappsv1.ClusterList{}, gomock.Any()).
				DoAndReturn(func(_ context.Context, list *kbappsv1.ClusterList, opts ...client.ListOption) error {
					list.Items = []kbappsv1.Cluster{*root}
					return nil
				}).Times(1)
			k8sMock.EXPECT().
				List(gomock.Any(), &tracev1.ReconciliationTraceList{}, gomock.Any()).
				DoAndReturn(func(_ context.Context, list *tracev1.ReconciliationTraceList, opts ...client.ListOption) error {
					list.Items = []tracev1.ReconciliationTrace{*trace}
					return nil
				}).Times(1)
			k8sMock.EXPECT().Scheme().Return(scheme.Scheme).AnyTimes()

			finder := NewObjectTreeRootFinder(k8sMock).(*rootFinder)
			res := finder.findRoots(ctx, secondary)
			Expect(res).Should(HaveLen(1))
			Expect(res[0]).Should(Equal(reconcile.Request{NamespacedName: client.ObjectKeyFromObject(root)}))
		})
	})
})
