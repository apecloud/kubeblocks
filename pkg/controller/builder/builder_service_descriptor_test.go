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

package builder

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
)

var _ = Describe("service descriptor builder", func() {
	It("should work well", func() {
		const (
			name           = "foo"
			ns             = "default"
			serviceKind    = "mock-kind"
			serviceVersion = "mock-version"
			endpointName   = "mock-endpoint"
			hostName       = "mock-host"
			secretRefName  = "foo"
		)
		endpoint := appsv1.CredentialVar{
			Value: endpointName,
		}
		host := appsv1.CredentialVar{
			Value: hostName,
		}
		port := appsv1.CredentialVar{
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{Name: secretRefName},
					Key:                  constant.ServiceDescriptorPortKey,
				},
			},
		}
		username := &appsv1.CredentialVar{
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{Name: secretRefName},
					Key:                  constant.ServiceDescriptorUsernameKey,
				},
			},
		}
		password := &appsv1.CredentialVar{
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{Name: secretRefName},
					Key:                  constant.ServiceDescriptorPasswordKey,
				},
			},
		}

		auth := appsv1.ConnectionCredentialAuth{
			Username: username,
			Password: password,
		}
		sd := NewServiceDescriptorBuilder(ns, name).
			SetServiceKind(serviceKind).
			SetServiceVersion(serviceVersion).
			SetEndpoint(endpoint).
			SetHost(host).
			SetPort(port).
			SetAuth(auth).
			GetObject()

		Expect(sd.Name).Should(Equal(name))
		Expect(sd.Namespace).Should(Equal(ns))
		Expect(sd.Spec.ServiceKind).Should(Equal(serviceKind))
		Expect(sd.Spec.ServiceVersion).Should(Equal(serviceVersion))
		Expect(sd.Spec.Endpoint.Value).Should(Equal(endpointName))
		Expect(sd.Spec.Endpoint.ValueFrom).Should(BeNil())
		Expect(sd.Spec.Host.Value).Should(Equal(hostName))
		Expect(sd.Spec.Host.ValueFrom).Should(BeNil())
		Expect(sd.Spec.Auth.Username.Value).Should(BeEmpty())
		Expect(sd.Spec.Auth.Username.ValueFrom.SecretKeyRef.Key).Should(Equal(constant.ServiceDescriptorUsernameKey))
		Expect(sd.Spec.Auth.Username.ValueFrom.SecretKeyRef.Name).Should(Equal(secretRefName))
		Expect(sd.Spec.Auth.Password.Value).Should(BeEmpty())
		Expect(sd.Spec.Auth.Password.ValueFrom.SecretKeyRef.Key).Should(Equal(constant.ServiceDescriptorPasswordKey))
		Expect(sd.Spec.Auth.Password.ValueFrom.SecretKeyRef.Name).Should(Equal(secretRefName))
		Expect(sd.Spec.Port.Value).Should(BeEmpty())
		Expect(sd.Spec.Port.ValueFrom.SecretKeyRef.Key).Should(Equal(constant.ServiceDescriptorPortKey))
		Expect(sd.Spec.Port.ValueFrom.SecretKeyRef.Name).Should(Equal(secretRefName))
	})
})
