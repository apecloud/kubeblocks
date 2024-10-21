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
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	tracev1 "github.com/apecloud/kubeblocks/apis/trace/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
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

			k8sMock.EXPECT().
				Get(gomock.Any(), gomock.Any(), &tracev1.ReconciliationTrace{}, gomock.Any()).
				DoAndReturn(func(_ context.Context, objKey client.ObjectKey, obj *tracev1.ReconciliationTrace, _ ...client.GetOption) error {
					*obj = *trace
					return nil
				}).Times(1)
			k8sMock.EXPECT().
				Get(gomock.Any(), gomock.Any(), &corev1.ConfigMap{}, gomock.Any()).
				DoAndReturn(func(_ context.Context, objKey client.ObjectKey, obj *corev1.ConfigMap, _ ...client.GetOption) error {
					return apierrors.NewNotFound(corev1.Resource(constant.ConfigMapKind), objKey.Name)
				}).Times(1)

			loader := traceResources()
			logger := log.FromContext(ctx).WithName("resources_loader test")
			tree, err := loader.Load(ctx, k8sMock, ctrl.Request{NamespacedName: client.ObjectKeyFromObject(trace)}, nil, logger)
			Expect(err).ToNot(HaveOccurred())
			Expect(tree.GetRoot()).To(Equal(trace))
		})
	})
})
