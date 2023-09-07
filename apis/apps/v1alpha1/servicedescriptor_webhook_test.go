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

package v1alpha1

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("ServiceDescriptor Webhook", func() {
	Context("spec validation", func() {
		var (
			name      = "test-service-descriptor"
			namespace = "default"
		)
		var sd *ServiceDescriptor
		BeforeEach(func() {
			sd = &ServiceDescriptor{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: namespace,
				},
				Spec: ServiceDescriptorSpec{
					ServiceKind:    "mock-kind",
					ServiceVersion: "mock-version",
					Endpoint: &CredentialVar{
						Value: "mock-endpoint",
					},
					Port: &CredentialVar{
						Value: "mock-port",
					},
					Auth: &ConnectionCredentialAuth{
						Username: &CredentialVar{
							Value: "mock-username",
						},
						Password: &CredentialVar{
							Value: "mock-password",
						},
					},
				},
			}
		})

		It("should succeed if spec is well defined", func() {
			Expect(k8sClient.Create(ctx, sd)).Should(Succeed())
			Expect(k8sClient.Delete(ctx, sd)).Should(Succeed())
		})

		It("should return an error if endpoint value and valueFrom are not empty", func() {
			sd.Spec.Endpoint.ValueFrom = &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: "mock-secret",
					},
					Key: "mock-key",
				},
			}
			err := k8sClient.Create(ctx, sd)
			Expect(err).ShouldNot(BeNil())
			Expect(err.Error()).Should(ContainSubstring("value and valueFrom cannot be specified at the same time"))
		})

		It("should return an error if port value and valueFrom are not empty", func() {
			sd.Spec.Port.ValueFrom = &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: "mock-secret",
					},
					Key: "mock-key",
				},
			}
			err := k8sClient.Create(ctx, sd)
			Expect(err).ShouldNot(BeNil())
			Expect(err.Error()).Should(ContainSubstring("value and valueFrom cannot be specified at the same time"))
		})

		It("should return an error if auth username value and valueFrom are not empty", func() {
			sd.Spec.Auth.Username.ValueFrom = &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: "mock-secret",
					},
					Key: "mock-key",
				},
			}
			sd.Spec.Auth.Password.ValueFrom = &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: "mock-secret",
					},
					Key: "mock-key",
				},
			}
			err := k8sClient.Create(ctx, sd)
			Expect(err).ShouldNot(BeNil())
			Expect(err.Error()).Should(ContainSubstring("value and valueFrom cannot be specified at the same time"))
		})
	})
})
