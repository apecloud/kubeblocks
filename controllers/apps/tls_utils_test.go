/*
Copyright ApeCloud, Inc.

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
	cfgcore "github.com/apecloud/kubeblocks/internal/configuration"
	"github.com/apecloud/kubeblocks/internal/controller/builder"
	"github.com/apecloud/kubeblocks/internal/controller/plan"
	"github.com/apecloud/kubeblocks/internal/generics"
	testapps "github.com/apecloud/kubeblocks/internal/testutil/apps"
	testk8s "github.com/apecloud/kubeblocks/internal/testutil/k8s"
)

var _ = Describe("TLS self-signed cert function", func() {
	const (
		clusterDefName     = "test-clusterdef-tls"
		clusterVersionName = "test-clusterversion-tls"
		clusterNamePrefix  = "test-cluster"
		statefulCompType   = "replicasets"
		statefulCompName   = "mysql"
		mysqlContainerName = "mysql"
		configSpecName     = "mysql-config-tpl"
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
		testapps.ClearClusterResources(&testCtx)

		// delete rest configurations
		ml := client.HasLabels{testCtx.TestObjLabelKey}
		// non-namespaced
		testapps.ClearResources(&testCtx, generics.ConfigConstraintSignature, ml)
		testapps.ClearResources(&testCtx, generics.BackupPolicyTemplateSignature, ml)
	}

	BeforeEach(cleanEnv)

	AfterEach(cleanEnv)

	// Testcases
	// Scenarios

	Context("tls is enabled/disabled", func() {
		BeforeEach(func() {
			configMapObj := testapps.CheckedCreateCustomizedObj(&testCtx,
				"resources/mysql-tls-config-template.yaml",
				&corev1.ConfigMap{},
				testCtx.UseDefaultNamespace())

			configConstraintObj := testapps.CheckedCreateCustomizedObj(&testCtx,
				"resources/mysql-config-constraint.yaml",
				&appsv1alpha1.ConfigConstraint{})

			By("Create a clusterDef obj")
			testapps.NewClusterDefFactory(clusterDefName).
				SetConnectionCredential(map[string]string{"username": "root", "password": ""}, nil).
				AddComponent(testapps.ConsensusMySQLComponent, statefulCompType).
				AddConfigTemplate(configSpecName, configMapObj.Name, configConstraintObj.Name, testCtx.DefaultNamespace, testapps.ConfVolumeName).
				AddContainerEnv(mysqlContainerName, corev1.EnvVar{Name: "MYSQL_ALLOW_EMPTY_PASSWORD", Value: "yes"}).
				CheckedCreate(&testCtx).GetObject()

			By("Create a clusterVersion obj")
			testapps.NewClusterVersionFactory(clusterVersionName, clusterDefName).
				AddComponent(statefulCompType).AddContainerShort(mysqlContainerName, testapps.ApeCloudMySQLImage).
				CheckedCreate(&testCtx).GetObject()

		})

		Context("when issuer is KubeBlocks", func() {
			var tlsIssuer *appsv1alpha1.Issuer

			BeforeEach(func() {
				tlsIssuer = &appsv1alpha1.Issuer{
					Name: appsv1alpha1.IssuerKubeBlocks,
				}
			})

			It("should create/delete the tls cert Secret", func() {
				By("create a cluster obj")
				clusterObj := testapps.NewClusterFactory(testCtx.DefaultNamespace,
					clusterNamePrefix, clusterDefName, clusterVersionName).
					WithRandomName().
					AddComponent(statefulCompName, statefulCompType).
					SetReplicas(3).
					SetTLS(true).
					SetIssuer(tlsIssuer).
					Create(&testCtx).
					GetObject()

				clusterKey := client.ObjectKeyFromObject(clusterObj)

				By("Waiting for the cluster enter creating phase")
				Eventually(testapps.GetClusterObservedGeneration(&testCtx, clusterKey)).Should(BeEquivalentTo(1))
				Eventually(testapps.GetClusterPhase(&testCtx, clusterKey)).Should(Equal(appsv1alpha1.StartingClusterPhase))

				By("By inspect that TLS cert. secret")
				ns := clusterObj.Namespace
				name := plan.GenerateTLSSecretName(clusterObj.Name, statefulCompName)
				nsName := types.NamespacedName{Namespace: ns, Name: name}
				secret := &corev1.Secret{}
				Eventually(k8sClient.Get(ctx, nsName, secret)).Should(Succeed())

				By("Checking volume & volumeMount settings in podSpec")
				stsList := testk8s.ListAndCheckStatefulSet(&testCtx, client.ObjectKeyFromObject(clusterObj))
				sts := stsList.Items[0]
				hasTLSVolume := false
				for _, volume := range sts.Spec.Template.Spec.Volumes {
					if volume.Name == builder.VolumeName {
						hasTLSVolume = true
						break
					}
				}
				Expect(hasTLSVolume).Should(BeTrue())
				for _, container := range sts.Spec.Template.Spec.Containers {
					hasTLSVolumeMount := false
					for _, mount := range container.VolumeMounts {
						if mount.Name == builder.VolumeName {
							hasTLSVolumeMount = true
							break
						}
					}
					Expect(hasTLSVolumeMount).Should(BeTrue())
				}
			})
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
					AddComponent(statefulCompName, statefulCompType).
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

			// REVIEW/TODO: following test setup needs to be revised, the setup looks like
			//   hacking test result, it's expected that cluster.status.observerGeneration=1
			//   with error conditions
			// It("should not create the cluster when secret referenced not exist", func() {
			// 	tlsIssuer := &appsv1alpha1.Issuer{
			// 		Name: appsv1alpha1.IssuerUserProvided,
			// 		SecretRef: &appsv1alpha1.TLSSecretRef{
			// 			Name: "secret-name-not-exist",
			// 			CA:   "ca.crt",
			// 			Cert: "tls.crt",
			// 			Key:  "tls.key",
			// 		},
			// 	}
			// 	By("create cluster obj")
			// 	clusterObj := testapps.NewClusterFactory(testCtx.DefaultNamespace, clusterNamePrefix, clusterDefName, clusterVersionName).
			// 		WithRandomName().
			// 		AddComponent(statefulCompName, statefulCompType).
			// 		SetReplicas(3).
			// 		SetTLS(true).
			// 		SetIssuer(tlsIssuer).
			// 		Create(&testCtx).
			// 		GetObject()

			// 	clusterKey := client.ObjectKeyFromObject(clusterObj)
			// By("Waiting for the cluster enter creating phase")
			// Eventually(testapps.GetClusterObservedGeneration(&testCtx, clusterKey)).Should(BeEquivalentTo(1))
			// Eventually(testapps.GetClusterPhase(&testCtx, clusterKey)).Should(BeEquivalentTo(appsv1alpha1.CreatingPhase))

			// 	By("By check cluster status.phase=ConditionsError")
			// 	Eventually(testapps.GetClusterPhase(&testCtx, client.ObjectKeyFromObject(clusterObj))).
			// 		Should(Equal(appsv1alpha1.ConditionsErrorPhase))
			// })
		})

		Context("when switch between disabled and enabled", func() {
			It("should handle tls settings properly", func() {
				By("create cluster with tls disabled")
				clusterObj := testapps.NewClusterFactory(testCtx.DefaultNamespace, clusterNamePrefix, clusterDefName, clusterVersionName).
					WithRandomName().
					AddComponent(statefulCompName, statefulCompType).
					SetReplicas(3).
					SetTLS(false).
					Create(&testCtx).
					GetObject()
				stsList := testk8s.ListAndCheckStatefulSet(&testCtx, client.ObjectKeyFromObject(clusterObj))
				sts := stsList.Items[0]
				cd := &appsv1alpha1.ClusterDefinition{}
				Expect(k8sClient.Get(ctx, types.NamespacedName{Name: clusterDefName, Namespace: testCtx.DefaultNamespace}, cd)).Should(Succeed())
				cmName := cfgcore.GetInstanceCMName(&sts, &cd.Spec.ComponentDefs[0].ConfigSpecs[0].ComponentTemplateSpec)
				cmKey := client.ObjectKey{Namespace: sts.Namespace, Name: cmName}
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
			})
		})
	})
})
