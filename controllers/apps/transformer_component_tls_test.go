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

package apps

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/controller/plan"
	"github.com/apecloud/kubeblocks/pkg/generics"
	testapps "github.com/apecloud/kubeblocks/pkg/testutil/apps"
)

var _ = Describe("TLS self-signed cert function", func() {
	const (
		clusterDefName      = "test-clusterdef-tls"
		clusterVersionName  = "test-clusterversion-tls"
		clusterNamePrefix   = "test-cluster"
		statefulCompDefName = "mysql"
		statefulCompName    = "mysql"
		mysqlContainerName  = "mysql"
	)

	ctx := context.Background()

	// Cleanups

	cleanEnv := func() {
		// must wait until resources deleted and no longer exist before the testcases start,
		// otherwise if later it needs to create some new resource objects with the same name,
		// in race conditions, it will find the existence of old objects, resulting failure to
		// create the new objects.
		By("clean resources")

		// delete cluster(and all dependent sub-resources), clusterversion and clusterdef
		testapps.ClearClusterResourcesWithRemoveFinalizerOption(&testCtx)

		// delete rest configurations
		ml := client.HasLabels{testCtx.TestObjLabelKey}
		// non-namespaced
		testapps.ClearResources(&testCtx, generics.ConfigConstraintSignature, ml)
		testapps.ClearResources(&testCtx, generics.BackupPolicyTemplateSignature, ml)
	}

	BeforeEach(cleanEnv)

	AfterEach(cleanEnv)

	Context("tls is enabled/disabled", func() {
		BeforeEach(func() {
			By("Create a clusterDef obj")
			testapps.NewClusterDefFactory(clusterDefName).
				SetConnectionCredential(map[string]string{"username": "root", "password": ""}, nil).
				AddComponentDef(testapps.ConsensusMySQLComponent, statefulCompDefName).
				AddContainerEnv(mysqlContainerName, corev1.EnvVar{Name: "MYSQL_ALLOW_EMPTY_PASSWORD", Value: "yes"}).
				CheckedCreate(&testCtx).GetObject()

			By("Create a clusterVersion obj")
			testapps.NewClusterVersionFactory(clusterVersionName, clusterDefName).
				AddComponentVersion(statefulCompDefName).AddContainerShort(mysqlContainerName, testapps.ApeCloudMySQLImage).
				CheckedCreate(&testCtx).GetObject()

		})

		Context("when issuer is UserProvided", func() {
			var userProvidedTLSSecretObj *corev1.Secret

			BeforeEach(func() {
				// prepare self provided tls certs secret
				var err error
				userProvidedTLSSecretObj, err = plan.ComposeTLSSecret(testCtx.DefaultNamespace, "test", "self-provided")
				Expect(err).Should(BeNil())
				Expect(k8sClient.Create(ctx, userProvidedTLSSecretObj)).Should(Succeed())
			})
			AfterEach(func() {
				// delete self provided tls certs secret
				Expect(k8sClient.Delete(ctx, userProvidedTLSSecretObj)).Should(Succeed())
				Eventually(func() bool {
					err := k8sClient.Get(ctx,
						client.ObjectKeyFromObject(userProvidedTLSSecretObj),
						userProvidedTLSSecretObj)
					return apierrors.IsNotFound(err)
				}).Should(BeTrue())
			})
			It("should create the cluster when secret referenced exist", func() {
				tlsIssuer := &appsv1alpha1.Issuer{
					Name: appsv1alpha1.IssuerUserProvided,
					SecretRef: &appsv1alpha1.TLSSecretRef{
						Name: userProvidedTLSSecretObj.Name,
						CA:   "ca.crt",
						Cert: "tls.crt",
						Key:  "tls.key",
					},
				}
				By("create cluster obj")
				clusterObj := testapps.NewClusterFactory(testCtx.DefaultNamespace, clusterNamePrefix, clusterDefName, clusterVersionName).
					WithRandomName().
					AddComponent(statefulCompName, statefulCompDefName).
					SetReplicas(3).
					SetTLS(true).
					SetIssuer(tlsIssuer).
					Create(&testCtx).
					GetObject()
				Eventually(k8sClient.Get(ctx,
					client.ObjectKeyFromObject(clusterObj),
					clusterObj)).
					Should(Succeed())
			})
		})

		Context("when switch between disabled and enabled", func() {
			It("should handle tls settings properly", func() {
				// TODO(v1.0): config template
			})
		})
	})
})
