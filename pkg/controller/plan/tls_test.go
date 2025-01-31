/*
Copyright (C) 2022-2025 ApeCloud Co., Ltd

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

package plan

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/golang/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	testutil "github.com/apecloud/kubeblocks/pkg/testutil/k8s"
)

var _ = Describe("TLSUtilsTest", func() {
	const namespace = "foo"

	Context("ComposeTLSSecret function", func() {
		It("should work well", func() {
			compDef := &appsv1.ComponentDefinition{
				Spec: appsv1.ComponentDefinitionSpec{
					TLS: &appsv1.TLS{
						CAFile:   ptr.To("ca.pem"),
						CertFile: ptr.To("cert.pem"),
						KeyFile:  ptr.To("key.pem"),
					},
				},
			}
			synthesizedComp := component.SynthesizedComponent{
				Namespace:   testCtx.DefaultNamespace,
				ClusterName: "bar",
				Name:        "test",
			}
			secret, err := ComposeTLSSecret(compDef, synthesizedComp, nil)
			Expect(err).Should(BeNil())
			Expect(secret).ShouldNot(BeNil())
			Expect(secret.Name).Should(Equal(GenerateTLSSecretName(synthesizedComp.ClusterName, synthesizedComp.Name)))
			Expect(secret.Labels).ShouldNot(BeNil())
			Expect(secret.Labels[constant.AppInstanceLabelKey]).Should(Equal(synthesizedComp.ClusterName))
			Expect(secret.Labels[constant.AppManagedByLabelKey]).Should(Equal(constant.AppName))
			Expect(secret.Labels[constant.KBAppComponentLabelKey]).Should(Equal(synthesizedComp.Name))
			Expect(secret.StringData).ShouldNot(BeNil())
			Expect(secret.StringData[*compDef.Spec.TLS.CAFile]).ShouldNot(BeZero())
			Expect(secret.StringData[*compDef.Spec.TLS.CertFile]).ShouldNot(BeZero())
			Expect(secret.StringData[*compDef.Spec.TLS.KeyFile]).ShouldNot(BeZero())
		})
	})

	Context("CheckTLSSecretRef function", func() {
		It("should work well", func() {
			ctx := context.Background()
			name := "bar"
			secretRef := &appsv1.TLSSecretRef{
				Name: name,
				CA:   "caName",
				Cert: "certName",
				Key:  "keyName",
			}
			controller, k8sMock := testutil.SetupK8sMock()
			defer controller.Finish()

			By("secret not found")
			k8sMock.EXPECT().
				Get(gomock.Any(), gomock.Any(), &corev1.Secret{}, gomock.Any()).
				DoAndReturn(func(_ context.Context, objKey client.ObjectKey, obj *corev1.Secret, _ ...client.GetOption) error {
					return apierrors.NewNotFound(schema.GroupResource{}, obj.Name)
				}).Times(1)
			err := CheckTLSSecretRef(ctx, k8sMock, namespace, secretRef)
			Expect(apierrors.IsNotFound(err)).Should(BeTrue())

			By("set empty CA in map Data")
			k8sMock.EXPECT().
				Get(gomock.Any(), gomock.Any(), &corev1.Secret{}, gomock.Any()).
				DoAndReturn(func(_ context.Context, objKey client.ObjectKey, obj *corev1.Secret, _ ...client.GetOption) error {
					Expect(obj).ShouldNot(BeNil())
					obj.Namespace = objKey.Namespace
					obj.Name = objKey.Name
					obj.Data = map[string][]byte{
						secretRef.Cert: []byte("foo"),
						secretRef.Key:  []byte("bar"),
						secretRef.CA:   []byte(""),
					}
					return nil
				}).Times(1)
			err = CheckTLSSecretRef(ctx, k8sMock, namespace, secretRef)
			Expect(err).ShouldNot(BeNil())
			Expect(err.Error()).Should(ContainSubstring(secretRef.CA))

			By("set everything ok")
			k8sMock.EXPECT().
				Get(gomock.Any(), gomock.Any(), &corev1.Secret{}, gomock.Any()).
				DoAndReturn(func(_ context.Context, objKey client.ObjectKey, obj *corev1.Secret, _ ...client.GetOption) error {
					Expect(obj).ShouldNot(BeNil())
					obj.Namespace = objKey.Namespace
					obj.Name = objKey.Name
					obj.Data = map[string][]byte{
						secretRef.Cert: []byte("foo"),
						secretRef.Key:  []byte("bar"),
						secretRef.CA:   []byte("ca"),
					}
					return nil
				}).Times(1)
			Expect(CheckTLSSecretRef(ctx, k8sMock, namespace, secretRef)).Should(Succeed())
		})
	})
})
