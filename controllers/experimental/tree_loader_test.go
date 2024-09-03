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

package experimental

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/golang/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	experimental "github.com/apecloud/kubeblocks/apis/experimental/v1alpha1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/builder"
	testutil "github.com/apecloud/kubeblocks/pkg/testutil/k8s"
)

var _ = Describe("tree loader test", func() {
	Context("Read", func() {
		It("should work well", func() {
			ctx := context.Background()
			logger := logf.FromContext(ctx).WithValues("tree-loader-test", "foo")
			controller, k8sMock := testutil.SetupK8sMock()
			defer controller.Finish()

			clusterName := "foo"
			componentNames := []string{"bar-0", "bar-1"}
			root := builder.NewNodeCountScalerBuilder(namespace, name).SetTargetClusterName(clusterName).SetTargetComponentNames(componentNames).GetObject()
			cluster := builder.NewClusterBuilder(namespace, clusterName).GetObject()
			its0 := builder.NewInstanceSetBuilder(namespace, constant.GenerateClusterComponentName(clusterName, componentNames[0])).GetObject()
			its1 := builder.NewInstanceSetBuilder(namespace, constant.GenerateClusterComponentName(clusterName, componentNames[1])).GetObject()
			node0 := &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: namespace,
					Name:      "node-0",
				},
			}
			node1 := &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: namespace,
					Name:      "node-1",
				},
			}

			k8sMock.EXPECT().
				Get(gomock.Any(), gomock.Any(), &experimental.NodeCountScaler{}, gomock.Any()).
				DoAndReturn(func(_ context.Context, objKey client.ObjectKey, obj *experimental.NodeCountScaler, _ ...client.GetOption) error {
					*obj = *root
					return nil
				}).Times(1)
			k8sMock.EXPECT().
				Get(gomock.Any(), gomock.Any(), &appsv1.Cluster{}, gomock.Any()).
				DoAndReturn(func(_ context.Context, objKey client.ObjectKey, obj *appsv1.Cluster, _ ...client.GetOption) error {
					*obj = *cluster
					return nil
				}).Times(1)
			k8sMock.EXPECT().
				Get(gomock.Any(), gomock.Any(), &workloads.InstanceSet{}, gomock.Any()).
				DoAndReturn(func(_ context.Context, objKey client.ObjectKey, obj *workloads.InstanceSet, _ ...client.GetOption) error {
					if objKey.Name == its0.Name {
						*obj = *its0
					} else {
						*obj = *its1
					}
					return nil
				}).Times(2)
			k8sMock.EXPECT().
				List(gomock.Any(), &corev1.NodeList{}, gomock.Any()).
				DoAndReturn(func(_ context.Context, list *corev1.NodeList, _ ...client.ListOption) error {
					list.Items = []corev1.Node{*node0, *node1}
					return nil
				}).Times(1)
			req := ctrl.Request{NamespacedName: client.ObjectKeyFromObject(root)}
			loader := objectTree()
			tree, err := loader.Load(ctx, k8sMock, req, nil, logger)
			Expect(err).Should(BeNil())
			Expect(tree.GetRoot()).ShouldNot(BeNil())
			Expect(tree.GetRoot()).Should(Equal(root))
			Expect(tree.GetSecondaryObjects()).Should(HaveLen(5))
			objectList := []client.Object{cluster, its0, its1, node0, node1}
			for _, object := range objectList {
				obj, err := tree.Get(object)
				Expect(err).Should(BeNil())
				Expect(obj).Should(Equal(object))
			}
		})
	})
})
