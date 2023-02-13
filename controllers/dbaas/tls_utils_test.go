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
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
	testdbaas "github.com/apecloud/kubeblocks/internal/testutil/dbaas"
	testk8s "github.com/apecloud/kubeblocks/internal/testutil/k8s"
)

var _ = Describe("Tls cert creation/check function", func() {
	const clusterDefName = "test-clusterdef"
	const clusterVersionName = "test-clusterversion"
	const clusterNamePrefix = "test-cluster"

	const statefulCompType = "replicasets"
	const statefulCompName = "mysql"

	const mysqlContainerName = "mysql"

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
		testdbaas.ClearResources(&testCtx, intctrlutil.ConfigConstraintSignature, ml)
		testdbaas.ClearResources(&testCtx, intctrlutil.BackupPolicyTemplateSignature, ml)
	}

	BeforeEach(cleanEnv)

	AfterEach(cleanEnv)

	// Testcases

	var (
		clusterObj               *dbaasv1alpha1.Cluster
		selfProvidedTLSSecretObj *corev1.Secret
		tlsIssuer                *dbaasv1alpha1.Issuer
	)

	// Scenarios

	Context("with tls enabled", func() {
		BeforeEach(func() {
			By("Create a clusterDef obj")
			testdbaas.NewClusterDefFactory(&testCtx, clusterDefName, testdbaas.MySQLType).
				SetConnectionCredential(map[string]string{"username": "root", "password": ""}).
				AddComponent(testdbaas.ConsensusMySQL, statefulCompType).
				AddContainerEnv(mysqlContainerName, corev1.EnvVar{Name: "MYSQL_ALLOW_EMPTY_PASSWORD", Value: "yes"}).
				Create().GetClusterDef()

			By("Create a clusterVersion obj")
			testdbaas.NewClusterVersionFactory(&testCtx, clusterVersionName, clusterDefName).
				AddComponent(statefulCompType).AddContainerShort(mysqlContainerName, testdbaas.ApeCloudMySQLImage).
				Create().GetClusterVersion()

		})

		Context("when issuer is SelfSigned", func() {
			BeforeEach(func() {
				tlsIssuer = &dbaasv1alpha1.Issuer{
					Name: dbaasv1alpha1.IssuerSelfSigned,
				}
			})

			It("should create/delete the tls cert Secret", func() {
				By("create a cluster obj")
				clusterObj = testdbaas.NewClusterFactory(&testCtx, clusterNamePrefix, clusterDefName, clusterVersionName).
					WithRandomName().
					AddComponent(statefulCompName, statefulCompType).
					SetReplicas(3).
					SetTLS(true).
					SetIssuer(tlsIssuer).
					Create().
					GetCluster()
				ns := clusterObj.Namespace
				name := generateTLSSecretName(clusterObj.Name, statefulCompName)
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
					if volume.Name == volumeName {
						hasTLSVolume = true
						break
					}
				}
				Expect(hasTLSVolume).Should(BeTrue())
				for _, container := range sts.Spec.Template.Spec.Containers {
					hasTLSVolumeMount := false
					for _, mount := range container.VolumeMounts {
						if mount.Name == volumeName {
							hasTLSVolumeMount = true
							break
						}
					}
					Expect(hasTLSVolumeMount).Should(BeTrue())
				}
			})
		})

		Context("when issuer is SelfProvided", func() {
			BeforeEach(func() {
				// prepare self provided tls certs secret
				var err error
				selfProvidedTLSSecretObj, err = composeTLSSecret(testCtx.DefaultNamespace, "test", "self-provided")
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
				tlsIssuer = &dbaasv1alpha1.Issuer{
					Name: dbaasv1alpha1.IssuerSelfProvided,
					SecretRef: &dbaasv1alpha1.TLSSecretRef{
						Name: selfProvidedTLSSecretObj.Name,
						CA:   "ca.crt",
						Cert: "tls.crt",
						Key:  "tls.key",
					},
				}
				By("create cluster obj")
				clusterObj = testdbaas.NewClusterFactory(&testCtx, clusterNamePrefix, clusterDefName, clusterVersionName).
					WithRandomName().
					AddComponent(statefulCompName, statefulCompType).
					SetReplicas(3).
					SetTLS(true).
					SetIssuer(tlsIssuer).
					Create().
					GetCluster()
				Eventually(k8sClient.Get(ctx,
					client.ObjectKeyFromObject(clusterObj),
					clusterObj)).
					Should(Succeed())
			})
			It("should not create the cluster when secret referenced not exist", func() {
				tlsIssuer = &dbaasv1alpha1.Issuer{
					Name: dbaasv1alpha1.IssuerSelfProvided,
					SecretRef: &dbaasv1alpha1.TLSSecretRef{
						Name: "secret-name-not-exist",
						CA:   "ca.crt",
						Cert: "tls.crt",
						Key:  "tls.key",
					},
				}
				By("create cluster obj")
				clusterObj = testdbaas.NewClusterFactory(&testCtx, clusterNamePrefix, clusterDefName, clusterVersionName).
					WithRandomName().
					AddComponent(statefulCompName, statefulCompType).
					SetReplicas(3).
					SetTLS(true).
					SetIssuer(tlsIssuer).
					Create().
					GetCluster()
				time.Sleep(time.Second)
				Eventually(testdbaas.GetClusterPhase(&testCtx, client.ObjectKeyFromObject(clusterObj))).
					Should(Equal(dbaasv1alpha1.CreatingPhase))
			})
		})
	})
})
