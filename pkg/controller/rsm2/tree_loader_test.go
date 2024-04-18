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

package rsm2

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/golang/mock/gomock"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/controller/builder"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
	testutil "github.com/apecloud/kubeblocks/pkg/testutil/k8s"
)

var _ = Describe("tree loader test", func() {
	Context("Read", func() {
		It("should work well", func() {
			ctx := context.Background()
			logger := logf.FromContext(ctx).WithValues("tree-loader-test", "foo")
			controller, k8sMock := testutil.SetupK8sMock()
			defer controller.Finish()

			templateObj, annotation, err := mockCompressedInstanceTemplates(namespace, name)
			Expect(err).Should(BeNil())
			root := builder.NewReplicatedStateMachineBuilder(namespace, name).AddAnnotations(templateRefAnnotationKey, annotation).GetObject()
			obj0 := builder.NewPodBuilder(namespace, name+"-0").GetObject()
			obj1 := builder.NewPodBuilder(namespace, name+"-1").GetObject()
			obj2 := builder.NewPodBuilder(namespace, name+"-2").GetObject()
			for _, pod := range []*corev1.Pod{obj0, obj1, obj2} {
				Expect(controllerutil.SetControllerReference(root, pod, model.GetScheme())).Should(Succeed())
			}
			k8sMock.EXPECT().
				Get(gomock.Any(), gomock.Any(), &workloads.InstanceSet{}, gomock.Any()).
				DoAndReturn(func(_ context.Context, objKey client.ObjectKey, obj *workloads.InstanceSet, _ ...client.GetOption) error {
					*obj = *root
					return nil
				}).Times(1)
			k8sMock.EXPECT().
				List(gomock.Any(), &corev1.ServiceList{}, gomock.Any()).
				DoAndReturn(func(_ context.Context, list *corev1.ServiceList, _ ...client.ListOption) error {
					return nil
				}).Times(1)
			k8sMock.EXPECT().
				List(gomock.Any(), &corev1.ConfigMapList{}, gomock.Any()).
				DoAndReturn(func(_ context.Context, list *corev1.ConfigMapList, _ ...client.ListOption) error {
					return nil
				}).Times(1)
			k8sMock.EXPECT().
				List(gomock.Any(), &corev1.PodList{}, gomock.Any()).
				DoAndReturn(func(_ context.Context, list *corev1.PodList, _ ...client.ListOption) error {
					Expect(list).ShouldNot(BeNil())
					list.Items = []corev1.Pod{*obj0, *obj1, *obj2}
					return nil
				}).Times(1)
			k8sMock.EXPECT().
				List(gomock.Any(), &corev1.PersistentVolumeClaimList{}, gomock.Any()).
				DoAndReturn(func(_ context.Context, list *corev1.PersistentVolumeClaimList, _ ...client.ListOption) error {
					return nil
				}).Times(1)
			k8sMock.EXPECT().
				List(gomock.Any(), &batchv1.JobList{}, gomock.Any()).
				DoAndReturn(func(_ context.Context, list *batchv1.JobList, _ ...client.ListOption) error {
					return nil
				}).Times(1)
			k8sMock.EXPECT().
				Get(gomock.Any(), gomock.Any(), &corev1.ConfigMap{}, gomock.Any()).
				DoAndReturn(func(_ context.Context, objKey client.ObjectKey, obj *corev1.ConfigMap, _ ...client.GetOption) error {
					*obj = *templateObj
					return nil
				}).Times(1)
			req := ctrl.Request{NamespacedName: client.ObjectKeyFromObject(root)}
			loader := NewTreeLoader()
			tree, err := loader.Load(ctx, k8sMock, req, nil, logger)
			Expect(err).Should(BeNil())
			Expect(tree.GetRoot()).ShouldNot(BeNil())
			Expect(tree.GetRoot()).Should(Equal(root))
			Expect(tree.GetSecondaryObjects()).Should(HaveLen(4))
			objList := []*corev1.Pod{obj0, obj1, obj2}
			for _, pod := range objList {
				obj, err := tree.Get(pod)
				Expect(err).Should(BeNil())
				Expect(obj).Should(Equal(pod))
			}
			obj, err := tree.Get(templateObj)
			Expect(err).Should(BeNil())
			Expect(obj).Should(Equal(templateObj))
		})
	})
})
