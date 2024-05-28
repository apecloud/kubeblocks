/*
Copyright (C) 2022-2024 ApeCloud Co., Ltd

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
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
					Host: &CredentialVar{
						Value: "mock-host",
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

		It("should return an error if the value and valueFrom are not empty", func() {
			for _, v := range []*CredentialVar{sd.Spec.Endpoint, sd.Spec.Host, sd.Spec.Port} {
				v.ValueFrom = &corev1.EnvVarSource{
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
			}
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
