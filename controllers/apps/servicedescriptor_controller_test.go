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

package apps

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	testapps "github.com/apecloud/kubeblocks/pkg/testutil/apps"
)

var _ = Describe("test clusterVersion controller", func() {

	var (
		randomStr = testCtx.GetRandomStr()
		namespace = "default"
	)

	cleanEnv := func() {
		// must wait till resources deleted and no longer existed before the testcases start,
		// otherwise if later it needs to create some new resource objects with the same name,
		// in race conditions, it will find the existence of old objects, resulting failure to
		// create the new objects.
		By("clean resources")

		// delete cluster(and all dependent sub-resources), clusterversion and clusterdef
		testapps.ClearClusterResources(&testCtx)
	}
	BeforeEach(cleanEnv)

	AfterEach(cleanEnv)

	Context("test ServiceDescriptor controller", func() {
		It("test ServiceDescriptor controller", func() {
			By("create a ServiceDescriptor obj")

			endpoint := appsv1alpha1.CredentialVar{
				Value: "mock-endpoint",
			}
			port := appsv1alpha1.CredentialVar{
				Value: "mock-port",
			}
			auth := appsv1alpha1.ConnectionCredentialAuth{
				Username: &appsv1alpha1.CredentialVar{
					Value: "mock-username",
				},
				Password: &appsv1alpha1.CredentialVar{
					Value: "mock-password",
				},
			}
			validSDName := "service-descriptor-valid-" + randomStr
			validServiceDescriptor := testapps.NewServiceDescriptorFactory(namespace, validSDName).
				SetServiceKind("mock-kind").
				SetServiceVersion("mock-version").
				SetEndpoint(endpoint).
				SetPort(port).
				SetAuth(auth).
				Create(&testCtx).GetObject()

			By("wait for ServiceDescriptor phase is available when serviceDescriptor is valid")
			Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(validServiceDescriptor),
				func(g Gomega, tmpSCC *appsv1alpha1.ServiceDescriptor) {
					g.Expect(tmpSCC.Status.Phase).Should(Equal(appsv1alpha1.AvailablePhase))
				})).Should(Succeed())

			invalidSDName := "service-descriptor-invalid-" + randomStr
			mockSecretRefName := "mock-secret-for-scc" + randomStr
			authValueFrom := appsv1alpha1.ConnectionCredentialAuth{
				Username: &appsv1alpha1.CredentialVar{
					ValueFrom: &corev1.EnvVarSource{
						SecretKeyRef: &corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: mockSecretRefName,
							},
							Key: "mock-secret-username-key",
						},
					},
				},
				Password: &appsv1alpha1.CredentialVar{
					ValueFrom: &corev1.EnvVarSource{
						SecretKeyRef: &corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: mockSecretRefName,
							},
							Key: "mock-secret-password-key",
						},
					},
				},
			}
			invalidServiceDescriptor := testapps.NewServiceDescriptorFactory(testCtx.DefaultNamespace, invalidSDName).
				SetServiceKind("mock-kind").
				SetServiceVersion("mock-version").
				SetEndpoint(endpoint).
				SetPort(port).
				SetAuth(authValueFrom).
				Create(&testCtx).GetObject()
			By("wait for ServiceDescriptor phase is Unavailable because serviceDescriptor secretRef not found")
			Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(invalidServiceDescriptor),
				func(g Gomega, tmpSCC *appsv1alpha1.ServiceDescriptor) {
					g.Expect(tmpSCC.Status.Phase).Should(Equal(appsv1alpha1.UnavailablePhase))
				})).Should(Succeed())

			secret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      mockSecretRefName,
					Namespace: namespace,
				},
				Data: map[string][]byte{
					"mock-secret-username-key": []byte("mock-username"),
					"mock-secret-password-key": []byte("mock-password"),
				},
			}
			Expect(testCtx.CheckedCreateObj(ctx, secret)).Should(Succeed())
			By("wait for ServiceDescriptor phase is available because serviceDescriptor secretRef found")
			Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(invalidServiceDescriptor),
				func(g Gomega, tmpSCC *appsv1alpha1.ServiceDescriptor) {
					g.Expect(tmpSCC.Status.Phase).Should(Equal(appsv1alpha1.AvailablePhase))
				})).Should(Succeed())
		})
	})
})
