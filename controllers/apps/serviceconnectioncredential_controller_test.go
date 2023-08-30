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
	testapps "github.com/apecloud/kubeblocks/internal/testutil/apps"
)

var _ = Describe("test clusterVersion controller", func() {

	var (
		randomStr = testCtx.GetRandomStr()
	)

	const statefulCompDefName = "stateful"

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

	Context("test serviceConnectionCredential controller", func() {
		It("test serviceConnectionCredential controller", func() {
			By("create a serviceConnectionCredential obj")

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
			validSCCName := "service-connection-credential-valid-" + randomStr
			validServiceConnectionCredential := testapps.NewServiceConnectionCredentialFactory(testCtx.DefaultNamespace, validSCCName).
				SetEndpoint(endpoint).
				SetPort(port).
				SetAuth(auth).
				SetExtra(map[string]string{"extra": "mock-extra"}).
				Create(&testCtx).GetObject()

			By("wait for serviceConnectionCredential phase is available when scc is valid")
			Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(validServiceConnectionCredential),
				func(g Gomega, tmpSCC *appsv1alpha1.ServiceConnectionCredential) {
					g.Expect(tmpSCC.Status.Phase).Should(Equal(appsv1alpha1.AvailablePhase))
				})).Should(Succeed())

			invalidSCCName := "service-connection-credential-invalid-" + randomStr
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
			invalidServiceConnectionCredential := testapps.NewServiceConnectionCredentialFactory(testCtx.DefaultNamespace, invalidSCCName).
				SetEndpoint(endpoint).
				SetPort(port).
				SetAuth(authValueFrom).
				SetExtra(map[string]string{"extra": "mock-extra"}).
				Create(&testCtx).GetObject()
			By("wait for serviceConnectionCredential phase is Unavailable because scc secretRef not found")
			Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(invalidServiceConnectionCredential),
				func(g Gomega, tmpSCC *appsv1alpha1.ServiceConnectionCredential) {
					g.Expect(tmpSCC.Status.Phase).Should(Equal(appsv1alpha1.UnavailablePhase))
				})).Should(Succeed())

			secret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name: mockSecretRefName,
				},
				Data: map[string][]byte{
					"mock-secret-username-key": []byte("mock-username"),
					"mock-secret-password-key": []byte("mock-password"),
				},
			}
			Expect(testCtx.CheckedCreateObj(ctx, secret)).Should(HaveOccurred())
			By("wait for serviceConnectionCredential phase is available because scc secretRef found")
			Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(invalidServiceConnectionCredential),
				func(g Gomega, tmpSCC *appsv1alpha1.ServiceConnectionCredential) {
					g.Expect(tmpSCC.Status.Phase).Should(Equal(appsv1alpha1.AvailablePhase))
				})).Should(Succeed())
		})
	})
})
