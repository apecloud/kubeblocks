/*
Copyright (C) 2022-2023 ApeCloud Co., Ltd

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
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/golang/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/constant"
	"github.com/apecloud/kubeblocks/internal/controller/builder"
	testutil "github.com/apecloud/kubeblocks/internal/testutil/k8s"
)

var _ = Describe("TLSUtilsTest", func() {
	const namespace = "foo"

	Context("ComposeTLSSecret function", func() {
		It("should work well", func() {
			clusterName := "bar"
			componentName := "test"
			secret, err := ComposeTLSSecret(namespace, clusterName, componentName)
			Expect(err).Should(BeNil())
			Expect(secret).ShouldNot(BeNil())
			Expect(secret.Name).Should(Equal(fmt.Sprintf("%s-%s-tls-certs", clusterName, componentName)))
			Expect(secret.Labels).ShouldNot(BeNil())
			Expect(secret.Labels[constant.AppInstanceLabelKey]).Should(Equal(clusterName))
			Expect(secret.Labels[constant.KBManagedByKey]).Should(Equal(constant.AppName))
			Expect(secret.StringData).ShouldNot(BeNil())
			Expect(secret.StringData[builder.CAName]).ShouldNot(BeZero())
			Expect(secret.StringData[builder.CertName]).ShouldNot(BeZero())
			Expect(secret.StringData[builder.KeyName]).ShouldNot(BeZero())
		})
	})

	Context("CheckTLSSecretRef function", func() {
		It("should work well", func() {
			ctx := context.Background()
			controller, k8sMock := testutil.SetupK8sMock()
			defer controller.Finish()
			name := "bar"
			secretRef := &appsv1alpha1.TLSSecretRef{
				Name: name,
				CA:   "caName",
				Cert: "certName",
				Key:  "keyName",
			}

			By("set stringData to nil")
			k8sMock.EXPECT().
				Get(gomock.Any(), gomock.Any(), &corev1.Secret{}, gomock.Any()).
				DoAndReturn(func(_ context.Context, objKey client.ObjectKey, obj *corev1.Secret, _ ...client.GetOption) error {
					Expect(obj).ShouldNot(BeNil())
					obj.Namespace = objKey.Namespace
					obj.Name = objKey.Name
					return nil
				}).Times(1)
			err := CheckTLSSecretRef(ctx, k8sMock, namespace, secretRef)
			Expect(err).ShouldNot(BeNil())
			Expect(err.Error()).Should(ContainSubstring("tls secret's data field shouldn't be nil"))

			By("set no CA key in map stringData")
			k8sMock.EXPECT().
				Get(gomock.Any(), gomock.Any(), &corev1.Secret{}, gomock.Any()).
				DoAndReturn(func(_ context.Context, objKey client.ObjectKey, obj *corev1.Secret, _ ...client.GetOption) error {
					Expect(obj).ShouldNot(BeNil())
					obj.Namespace = objKey.Namespace
					obj.Name = objKey.Name
					obj.StringData = map[string]string{
						secretRef.Cert: "foo",
						secretRef.Key:  "bar",
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
					obj.StringData = map[string]string{
						secretRef.Cert: "foo",
						secretRef.Key:  "bar",
						secretRef.CA:   "ca",
					}
					return nil
				}).Times(1)
			Expect(CheckTLSSecretRef(ctx, k8sMock, namespace, secretRef)).Should(Succeed())
		})

		Context("GetTLSKeyWord function", func() {
			It("should work well", func() {
				suite := []struct {
					input    string
					expected string
				}{
					{input: "mysql", expected: "ssl_cert"},
					{input: "postgresql", expected: "ssl_cert_file"},
					{input: "redis", expected: "tls-cert-file"},
					{input: "others", expected: "unsupported-character-type"},
				}

				for _, s := range suite {
					Expect(GetTLSKeyWord(s.input)).Should(Equal(s.expected))
				}
			})
		})
	})
})
