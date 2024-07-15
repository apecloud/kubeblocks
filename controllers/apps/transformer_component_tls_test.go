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
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	appsv1beta1 "github.com/apecloud/kubeblocks/apis/apps/v1beta1"
	cfgcore "github.com/apecloud/kubeblocks/pkg/configuration/core"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	"github.com/apecloud/kubeblocks/pkg/controller/graph"
	"github.com/apecloud/kubeblocks/pkg/controller/plan"
	"github.com/apecloud/kubeblocks/pkg/generics"
	testapps "github.com/apecloud/kubeblocks/pkg/testutil/apps"
	testk8s "github.com/apecloud/kubeblocks/pkg/testutil/k8s"
)

var _ = Describe("TLS self-signed cert function", func() {
	const (
		clusterDefName      = "test-clusterdef-tls"
		clusterVersionName  = "test-clusterversion-tls"
		clusterNamePrefix   = "test-cluster"
		statefulCompDefName = "mysql"
		statefulCompName    = "mysql"
		mysqlContainerName  = "mysql"
		configSpecName      = "mysql-config-tpl"
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
			configMapObj := testapps.CheckedCreateCustomizedObj(&testCtx,
				"resources/mysql-tls-config-template.yaml",
				&corev1.ConfigMap{},
				testCtx.UseDefaultNamespace(),
				testapps.WithAnnotations(constant.CMInsEnableRerenderTemplateKey, "true"))

			configConstraintObj := testapps.CheckedCreateCustomizedObj(&testCtx,
				"resources/mysql-config-constraint.yaml",
				&appsv1beta1.ConfigConstraint{})

			By("Create a clusterDef obj")
			testapps.NewClusterDefFactory(clusterDefName).
				SetConnectionCredential(map[string]string{"username": "root", "password": ""}, nil).
				AddComponentDef(testapps.ConsensusMySQLComponent, statefulCompDefName).
				AddConfigTemplate(configSpecName, configMapObj.Name, configConstraintObj.Name, testCtx.DefaultNamespace, testapps.ConfVolumeName).
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
				By("create cluster with tls disabled")
				clusterObj := testapps.NewClusterFactory(testCtx.DefaultNamespace, clusterNamePrefix, clusterDefName, clusterVersionName).
					WithRandomName().
					AddComponent(statefulCompName, statefulCompDefName).
					SetReplicas(3).
					SetTLS(false).
					Create(&testCtx).
					GetObject()
				clusterKey := client.ObjectKeyFromObject(clusterObj)
				Eventually(k8sClient.Get(ctx, clusterKey, clusterObj)).Should(Succeed())
				Eventually(testapps.GetClusterObservedGeneration(&testCtx, clusterKey)).Should(BeEquivalentTo(1))
				Eventually(testapps.GetClusterPhase(&testCtx, clusterKey)).Should(Equal(appsv1alpha1.CreatingClusterPhase))

				itsList := testk8s.ListAndCheckInstanceSet(&testCtx, clusterKey)
				its := itsList.Items[0]
				cd := &appsv1alpha1.ClusterDefinition{}
				Expect(k8sClient.Get(ctx, types.NamespacedName{Name: clusterDefName, Namespace: testCtx.DefaultNamespace}, cd)).Should(Succeed())
				cmName := cfgcore.GetInstanceCMName(&its, &cd.Spec.ComponentDefs[0].ConfigSpecs[0].ComponentTemplateSpec)
				cmKey := client.ObjectKey{Namespace: its.Namespace, Name: cmName}
				hasTLSSettings := func() bool {
					cm := &corev1.ConfigMap{}
					Expect(k8sClient.Get(ctx, cmKey, cm)).Should(Succeed())
					tlsKeyWord := plan.GetTLSKeyWord("mysql")
					for _, cfgFile := range cm.Data {
						index := strings.Index(cfgFile, tlsKeyWord)
						if index >= 0 {
							return true
						}
					}
					return false
				}

				Eventually(hasTLSSettings).Should(BeFalse())

				By("update tls to enabled")
				Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(clusterObj), clusterObj)).Should(Succeed())
				patch := client.MergeFrom(clusterObj.DeepCopy())
				clusterObj.Spec.ComponentSpecs[0].TLS = true
				clusterObj.Spec.ComponentSpecs[0].Issuer = &appsv1alpha1.Issuer{Name: appsv1alpha1.IssuerKubeBlocks}
				Expect(k8sClient.Patch(ctx, clusterObj, patch)).Should(Succeed())
				Eventually(hasTLSSettings).Should(BeTrue())

				By("update tls to disabled")
				patch = client.MergeFrom(clusterObj.DeepCopy())
				clusterObj.Spec.ComponentSpecs[0].TLS = false
				clusterObj.Spec.ComponentSpecs[0].Issuer = nil
				Expect(k8sClient.Patch(ctx, clusterObj, patch)).Should(Succeed())
				Eventually(hasTLSSettings).Should(BeFalse())

				By("delete a cluster cleanly when tls enabled")
				Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(clusterObj), clusterObj)).Should(Succeed())
				patch = client.MergeFrom(clusterObj.DeepCopy())
				clusterObj.Spec.ComponentSpecs[0].TLS = true
				clusterObj.Spec.ComponentSpecs[0].Issuer = &appsv1alpha1.Issuer{Name: appsv1alpha1.IssuerKubeBlocks}
				Expect(k8sClient.Patch(ctx, clusterObj, patch)).Should(Succeed())
				Eventually(hasTLSSettings).Should(BeTrue())

				testapps.DeleteObject(&testCtx, clusterKey, &appsv1alpha1.Cluster{})
				Eventually(testapps.CheckObjExists(&testCtx, clusterKey, &appsv1alpha1.Cluster{}, false)).Should(Succeed())
			})
		})
		Context("when issuer is KubeBlocks check secret exists or not", func() {
			var (
				kbTLSSecretObj  *corev1.Secret
				synthesizedComp component.SynthesizedComponent
				dag             *graph.DAG
				err             error
			)
			BeforeEach(func() {
				synthesizedComp = component.SynthesizedComponent{
					Namespace:   testCtx.DefaultNamespace,
					ClusterName: "test-kb",
					Name:        "test-kb-tls",
					TLSConfig: &appsv1alpha1.TLSConfig{
						Enable: true,
						Issuer: &appsv1alpha1.Issuer{
							Name: appsv1alpha1.IssuerKubeBlocks,
						},
					},
				}
				dag = &graph.DAG{}
				kbTLSSecretObj, err = plan.ComposeTLSSecret(testCtx.DefaultNamespace, synthesizedComp.ClusterName, synthesizedComp.Name)
				Expect(err).Should(BeNil())
				Expect(k8sClient.Create(ctx, kbTLSSecretObj)).Should(Succeed())
			})
			AfterEach(func() {
				// delete self provided tls certs secret
				Expect(k8sClient.Delete(ctx, kbTLSSecretObj)).Should(Succeed())
				Eventually(func() bool {
					err := k8sClient.Get(ctx,
						client.ObjectKeyFromObject(kbTLSSecretObj),
						kbTLSSecretObj)
					return apierrors.IsNotFound(err)
				}).Should(BeTrue())
			})
			It("should skip creation if secret already exists", func() {
				err := buildTLSCert(ctx, k8sClient, synthesizedComp, dag)
				Expect(err).Should(BeNil())
				createdSecret := &corev1.Secret{}
				err = k8sClient.Get(ctx, types.NamespacedName{Namespace: testCtx.DefaultNamespace, Name: kbTLSSecretObj.Name}, createdSecret)
				Expect(err).Should(BeNil())
				Expect(createdSecret.Data).To(Equal(kbTLSSecretObj.Data))
			})
		})
	})
})
