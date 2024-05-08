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

package kubebuilderx

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/golang/mock/gomock"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/apecloud/kubeblocks/pkg/controller/builder"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
	testutil "github.com/apecloud/kubeblocks/pkg/testutil/k8s"
)

var _ = Describe("utils test", func() {
	const (
		namespace = "foo"
		name      = "bar"
	)

	Context("ReadObjectTree", func() {
		It("should work well", func() {
			controller, k8sMock := testutil.SetupK8sMock()
			defer controller.Finish()

			root := builder.NewStatefulSetBuilder(namespace, name).GetObject()
			obj0 := builder.NewPodBuilder(namespace, name+"-0").GetObject()
			obj1 := builder.NewPodBuilder(namespace, name+"-1").GetObject()
			obj2 := builder.NewPodBuilder(namespace, name+"-2").GetObject()
			for _, pod := range []*corev1.Pod{obj0, obj1, obj2} {
				Expect(controllerutil.SetControllerReference(root, pod, model.GetScheme())).Should(Succeed())
			}

			k8sMock.EXPECT().
				Get(gomock.Any(), gomock.Any(), &appsv1.StatefulSet{}, gomock.Any()).
				DoAndReturn(func(_ context.Context, objKey client.ObjectKey, obj *appsv1.StatefulSet, _ ...client.GetOption) error {
					*obj = *root
					return nil
				}).Times(1)
			k8sMock.EXPECT().
				List(gomock.Any(), &corev1.PodList{}, gomock.Any()).
				DoAndReturn(func(_ context.Context, list *corev1.PodList, _ ...client.ListOption) error {
					Expect(list).ShouldNot(BeNil())
					list.Items = []corev1.Pod{*obj0, *obj1, *obj2}
					return nil
				}).Times(1)
			req := ctrl.Request{NamespacedName: client.ObjectKeyFromObject(root)}
			ml := client.MatchingLabels{"foo": "bar"}
			tree, err := ReadObjectTree[*appsv1.StatefulSet](context.Background(), k8sMock, req, ml, &corev1.PodList{})
			Expect(err).Should(BeNil())
			Expect(tree.GetRoot()).ShouldNot(BeNil())
			Expect(tree.GetRoot()).Should(Equal(root))
			Expect(tree.GetSecondaryObjects()).Should(HaveLen(3))
			objList := []*corev1.Pod{obj0, obj1, obj2}
			for _, pod := range objList {
				obj, err := tree.Get(pod)
				Expect(err).Should(BeNil())
				Expect(obj).Should(Equal(pod))
			}
		})
	})
})
