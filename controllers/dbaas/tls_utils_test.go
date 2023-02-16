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

package dbaas

import (
	"context"
	"github.com/apecloud/kubeblocks/internal/controller/plan"
	"github.com/apecloud/kubeblocks/internal/controllerutil"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
	testdbaas "github.com/apecloud/kubeblocks/internal/testutil/dbaas"
	testk8s "github.com/apecloud/kubeblocks/internal/testutil/k8s"
)

var _ = Describe("TLS self-signed cert function", func() {
	const (
		clusterDefName     = "test-clusterdef"
		clusterVersionName = "test-clusterversion"
		clusterNamePrefix  = "test-cluster"
		statefulCompType   = "replicasets"
		statefulCompName   = "mysql"
		mysqlContainerName = "mysql"
		configTplName      = "mysql-config-tpl"
		configVolumeName   = "mysql-config"
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
		testdbaas.ClearClusterResources(&testCtx)

		// delete rest configurations
		ml := client.HasLabels{testCtx.TestObjLabelKey}
		// non-namespaced
		testdbaas.ClearResources(&testCtx, controllerutil.ConfigConstraintSignature, ml)
		testdbaas.ClearResources(&testCtx, controllerutil.BackupPolicyTemplateSignature, ml)
	}

	BeforeEach(cleanEnv)

	AfterEach(cleanEnv)

	// Testcases
	// Scenarios

	Context("tls is enabled/disabled", func() {
		BeforeEach(func() {
			configMapObj := testdbaas.CheckedCreateCustomizedObj(&testCtx,
				"resources/mysql_tls_config_cm.yaml",
				&corev1.ConfigMap{},
				testCtx.UseDefaultNamespace())

			configConstraintObj := testdbaas.CheckedCreateCustomizedObj(&testCtx,
				"resources/mysql_config_template.yaml",
				&dbaasv1alpha1.ConfigConstraint{})

			By("Create a clusterDef obj")
			testdbaas.NewClusterDefFactory(clusterDefName, testdbaas.MySQLType).
				SetConnectionCredential(map[string]string{"username": "root", "password": ""}).
				AddComponent(testdbaas.ConsensusMySQLComponent, statefulCompType).
				AddConfigTemplate(configTplName, configMapObj.Name, configConstraintObj.Name, configVolumeName, nil).
				AddContainerEnv(mysqlContainerName, corev1.EnvVar{Name: "MYSQL_ALLOW_EMPTY_PASSWORD", Value: "yes"}).
				CheckedCreate(&testCtx).GetObject()

			By("Create a clusterVersion obj")
			testdbaas.NewClusterVersionFactory(clusterVersionName, clusterDefName).
				AddComponent(statefulCompType).AddContainerShort(mysqlContainerName, testdbaas.ApeCloudMySQLImage).
				CheckedCreate(&testCtx).GetObject()

		})

		Context("when issuer is SelfSigned", func() {
			var tlsIssuer *dbaasv1alpha1.Issuer

			BeforeEach(func() {
				tlsIssuer = &dbaasv1alpha1.Issuer{
					Name: dbaasv1alpha1.IssuerSelfSigned,
				}
			})

			It("should create/delete the tls cert Secret", func() {
				By("create a cluster obj")
				clusterObj := testdbaas.NewClusterFactory(testCtx.DefaultNamespace, clusterNamePrefix, clusterDefName, clusterVersionName).
					WithRandomName().
					AddComponent(statefulCompName, statefulCompType).
					SetReplicas(3).
					SetTLS(true).
					SetIssuer(tlsIssuer).
					Create(&testCtx).
					GetObject()
				ns := clusterObj.Namespace
				name := plan.GenerateTLSSecretName(clusterObj.Name, statefulCompName)
				nsName := types.NamespacedName{Namespace: ns, Name: name}
				secret := &corev1.Secret{}
				Eventually(func() error {
					err := k8sClient.Get(ctx, nsName, secret)
					return err
				}).WithPolling(time.Second).WithTimeout(10 * time.Second).Should(Succeed())
				By("Checking volume & volumeMount settings in podSpec")
				stsList := testk8s.ListAndCheckStatefulSet(&testCtx, client.ObjectKeyFromObject(clusterObj))
				sts := stsList.Items[0]
				hasTLSVolume := false
				for _, volume := range sts.Spec.Template.Spec.Volumes {
					if volume.Name == plan.VolumeName {
						hasTLSVolume = true
						break
					}
				}
				Expect(hasTLSVolume).Should(BeTrue())
				for _, container := range sts.Spec.Template.Spec.Containers {
					hasTLSVolumeMount := false
					for _, mount := range container.VolumeMounts {
						if mount.Name == plan.VolumeName {
							hasTLSVolumeMount = true
							break
						}
					}
					Expect(hasTLSVolumeMount).Should(BeTrue())
				}
			})
		})

		Context("when issuer is SelfProvided", func() {
			var selfProvidedTLSSecretObj *corev1.Secret

			BeforeEach(func() {
				// prepare self provided tls certs secret
				var err error
				selfProvidedTLSSecretObj, err = plan.ComposeTLSSecret(testCtx.DefaultNamespace, "test", "self-provided")
				Expect(err).Should(BeNil())
				Expect(k8sClient.Create(ctx, selfProvidedTLSSecretObj)).Should(Succeed())
			})
			AfterEach(func() {
				// delete self provided tls certs secret
				Expect(k8sClient.Delete(ctx, selfProvidedTLSSecretObj)).Should(Succeed())
				Eventually(func() bool {
					err := k8sClient.Get(ctx,
						client.ObjectKeyFromObject(selfProvidedTLSSecretObj),
						selfProvidedTLSSecretObj)
					return apierrors.IsNotFound(err)
				}).Should(BeTrue())
			})
			It("should create the cluster when secret referenced exist", func() {
				tlsIssuer := &dbaasv1alpha1.Issuer{
					Name: dbaasv1alpha1.IssuerSelfProvided,
					SecretRef: &dbaasv1alpha1.TLSSecretRef{
						Name: selfProvidedTLSSecretObj.Name,
						CA:   "ca.crt",
						Cert: "tls.crt",
						Key:  "tls.key",
					},
				}
				By("create cluster obj")
				clusterObj := testdbaas.NewClusterFactory(testCtx.DefaultNamespace, clusterNamePrefix, clusterDefName, clusterVersionName).
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
			It("should not create the cluster when secret referenced not exist", func() {
				tlsIssuer := &dbaasv1alpha1.Issuer{
					Name: dbaasv1alpha1.IssuerSelfProvided,
					SecretRef: &dbaasv1alpha1.TLSSecretRef{
						Name: "secret-name-not-exist",
						CA:   "ca.crt",
						Cert: "tls.crt",
						Key:  "tls.key",
					},
				}
				By("create cluster obj")
				clusterObj := testdbaas.NewClusterFactory(testCtx.DefaultNamespace, clusterNamePrefix, clusterDefName, clusterVersionName).
					WithRandomName().
					AddComponent(statefulCompName, statefulCompType).
					SetReplicas(3).
					SetTLS(true).
					SetIssuer(tlsIssuer).
					Create(&testCtx).
					GetObject()
				time.Sleep(time.Second)
				Eventually(testdbaas.GetClusterPhase(&testCtx, client.ObjectKeyFromObject(clusterObj))).
					Should(Equal(dbaasv1alpha1.CreatingPhase))
			})
		})

		Context("when switch between disabled and enabled", func() {
			It("should handle tls settings properly", func() {
				By("create cluster with tls disabled")
				clusterObj := testdbaas.NewClusterFactory(testCtx.DefaultNamespace, clusterNamePrefix, clusterDefName, clusterVersionName).
					WithRandomName().
					AddComponent(statefulCompName, statefulCompType).
					SetReplicas(3).
					SetTLS(false).
					Create(&testCtx).
					GetObject()
				stsList := testk8s.ListAndCheckStatefulSet(&testCtx, client.ObjectKeyFromObject(clusterObj))
				sts := stsList.Items[0]
				cmName := sts.Name + "-" + configVolumeName
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

				Eventually(hasTLSSettings).WithPolling(time.Second).WithTimeout(10 * time.Second).Should(BeFalse())

				By("update tls to enabled")
				Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(clusterObj), clusterObj)).Should(Succeed())
				patch := client.MergeFrom(clusterObj.DeepCopy())
				clusterObj.Spec.Components[0].TLS = true
				clusterObj.Spec.Components[0].Issuer = &dbaasv1alpha1.Issuer{Name: dbaasv1alpha1.IssuerSelfSigned}
				Expect(k8sClient.Patch(ctx, clusterObj, patch)).Should(Succeed())
				Eventually(hasTLSSettings).WithPolling(time.Second).WithTimeout(10 * time.Second).Should(BeTrue())

				By("update tls to disabled")
				patch = client.MergeFrom(clusterObj.DeepCopy())
				clusterObj.Spec.Components[0].TLS = false
				clusterObj.Spec.Components[0].Issuer = nil
				Expect(k8sClient.Patch(ctx, clusterObj, patch)).Should(Succeed())
				Eventually(hasTLSSettings).WithPolling(time.Second).WithTimeout(10 * time.Second).Should(BeFalse())
			})
		})
	})
})
