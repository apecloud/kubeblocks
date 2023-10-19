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
	"context"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/controllers/apps/components"
	cfgcore "github.com/apecloud/kubeblocks/pkg/configuration/core"
	"github.com/apecloud/kubeblocks/pkg/constant"
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
				testCtx.UseDefaultNamespace(),
				testapps.WithAnnotations(constant.CMInsEnableRerenderTemplateKey, "true"))

			configConstraintObj := testapps.CheckedCreateCustomizedObj(&testCtx,
				"resources/mysql-config-constraint.yaml",
				&appsv1alpha1.ConfigConstraint{})

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

		// Context("when issuer is KubeBlocks", func() {
		// 	var tlsIssuer *appsv1alpha1.Issuer
		//
		// 	BeforeEach(func() {
		// 		tlsIssuer = &appsv1alpha1.Issuer{
		// 			Name: appsv1alpha1.IssuerKubeBlocks,
		// 		}
		// 	})
		//
		// 	It("should create/delete the tls cert Secret", func() {
		//
		// 		// REVIEW: do review this test setup
		// 		//  In [AfterEach] at: /Users/nashtsai/go/src/github.com/apecloud/kubeblocks/pkg/testutil/apps/common_util.go:323
		// 		// Assertion in callback at /Users/nashtsai/go/src/github.com/apecloud/kubeblocks/pkg/testutil/apps/common_util.go:322 failed:
		// 		// Expected
		// 		// <[]v1.StatefulSet | len:1, cap:1>:
		// 		// 	to be empty
		// 		// 	In [AfterEach] at:
		//
		// 		By("create a cluster obj")
		// 		clusterObj := testapps.NewClusterFactory(testCtx.DefaultNamespace,
		// 			clusterNamePrefix, clusterDefName, clusterVersionName).
		// 			WithRandomName().
		// 			AddComponentDef(statefulCompName, statefulCompDefName).
		// 			SetReplicas(3).
		// 			SetTLS(true).
		// 			SetIssuer(tlsIssuer).
		// 			Create(&testCtx).
		// 			GetObject()
		//
		// 		clusterKey := client.ObjectKeyFromObject(clusterObj)
		//
		// 		By("Waiting for the cluster enter creating phase")
		// 		Eventually(testapps.GetClusterObservedGeneration(&testCtx, clusterKey)).Should(BeEquivalentTo(1))
		// 		Eventually(testapps.GetClusterPhase(&testCtx, clusterKey)).Should(Equal(appsv1alpha1.CreatingClusterPhase))
		//
		// 		By("By inspect that TLS cert. secret")
		// 		ns := clusterObj.Namespace
		// 		name := plan.GenerateTLSSecretName(clusterObj.Name, statefulCompName)
		// 		nsName := types.NamespacedName{Namespace: ns, Name: name}
		// 		secret := &corev1.Secret{}
		//
		// 		// REVIEW: Caught following:
		// 		// [FAILED] Timed out after 10.000s.
		// 		// 	Expected success, but got an error:
		// 		// <*errors.StatusError | 0x14001dc46e0>: {
		// 		// ErrStatus: {
		// 		// TypeMeta: {Kind: "", APIVersion: ""},
		// 		// ListMeta: {
		// 		// SelfLink: "",
		// 		// 	ResourceVersion: "",
		// 		// 		Continue: "",
		// 		// 		RemainingItemCount: nil,
		// 		// },
		// 		// Status: "Failure",
		// 		// 	Message: "secrets \"test-clusterlmgbpe-mysql-tls-certs\" not found",
		// 		// 		Reason: "NotFound",
		// 		// 		Details: {
		// 		// 	Name: "test-clusterlmgbpe-mysql-tls-certs",
		// 		// 		Group: "",
		// 		// 			Kind: "secrets",
		// 		// 			UID: "",
		// 		// 			Causes: nil,
		// 		// 			RetryAfterSeconds: 0,
		// 		// 	},
		// 		// Code: 404,
		// 		// },
		// 		// }
		// 		// secrets "test-clusterlmgbpe-mysql-tls-certs" not found
		//
		// 		Eventually(k8sClient.Get(ctx, nsName, secret)).Should(Succeed())
		//
		// 		By("Checking volume & volumeMount settings in podSpec")
		// 		stsList := testk8s.ListAndCheckStatefulSet(&testCtx, client.ObjectKeyFromObject(clusterObj))
		// 		sts := stsList.Items[0]
		// 		hasTLSVolume := false
		// 		for _, volume := range sts.Spec.Template.Spec.Volumes {
		// 			if volume.Name == builder.VolumeName {
		// 				hasTLSVolume = true
		// 				break
		// 			}
		// 		}
		// 		Expect(hasTLSVolume).Should(BeTrue())
		// 		for _, container := range sts.Spec.Template.Spec.Containers {
		// 			hasTLSVolumeMount := false
		// 			for _, mount := range container.VolumeMounts {
		// 				if mount.Name == builder.VolumeName {
		// 					hasTLSVolumeMount = true
		// 					break
		// 				}
		// 			}
		// 			Expect(hasTLSVolumeMount).Should(BeTrue())
		// 		}
		// 	})
		// })

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
			// 		AddComponentDef(statefulCompName, statefulCompDefName).
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
					AddComponent(statefulCompName, statefulCompDefName).
					SetReplicas(3).
					SetTLS(false).
					Create(&testCtx).
					GetObject()
				clusterKey := client.ObjectKeyFromObject(clusterObj)
				Eventually(k8sClient.Get(ctx, clusterKey, clusterObj)).Should(Succeed())
				Eventually(testapps.GetClusterObservedGeneration(&testCtx, clusterKey)).Should(BeEquivalentTo(1))
				Eventually(testapps.GetClusterPhase(&testCtx, clusterKey)).Should(Equal(appsv1alpha1.CreatingClusterPhase))

				rsmList := testk8s.ListAndCheckRSM(&testCtx, clusterKey)
				sts := *components.ConvertRSMToSTS(&rsmList.Items[0])
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

				conf := &appsv1alpha1.Configuration{}
				confKey := client.ObjectKey{Namespace: sts.Namespace, Name: cfgcore.GenerateComponentConfigurationName(clusterObj.Name, clusterObj.Spec.ComponentSpecs[0].Name)}
				Expect(k8sClient.Get(ctx, confKey, conf)).Should(Succeed())
				patch2 := client.MergeFrom(conf.DeepCopy())
				conf.Spec.ConfigItemDetails[0].Version = "v1"
				Expect(k8sClient.Patch(ctx, conf, patch2)).Should(Succeed())
				Eventually(hasTLSSettings).Should(BeTrue())

				By("update tls to disabled")
				patch = client.MergeFrom(clusterObj.DeepCopy())
				clusterObj.Spec.ComponentSpecs[0].TLS = false
				clusterObj.Spec.ComponentSpecs[0].Issuer = nil
				Expect(k8sClient.Patch(ctx, clusterObj, patch)).Should(Succeed())

				Expect(k8sClient.Get(ctx, confKey, conf)).Should(Succeed())
				patch2 = client.MergeFrom(conf.DeepCopy())
				conf.Spec.ConfigItemDetails[0].Version = "v2"
				Expect(k8sClient.Patch(ctx, conf, patch2)).Should(Succeed())

				Eventually(hasTLSSettings).Should(BeFalse())
			})
		})
	})
})
