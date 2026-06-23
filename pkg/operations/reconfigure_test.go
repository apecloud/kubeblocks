/*
Copyright (C) 2022-2026 ApeCloud Co., Ltd

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

package operations

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	opsv1alpha1 "github.com/apecloud/kubeblocks/apis/operations/v1alpha1"
	parametersv1alpha1 "github.com/apecloud/kubeblocks/apis/parameters/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/builder"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	"github.com/apecloud/kubeblocks/pkg/generics"
	opsutil "github.com/apecloud/kubeblocks/pkg/operations/util"
	parameterscore "github.com/apecloud/kubeblocks/pkg/parameters/core"
	testapps "github.com/apecloud/kubeblocks/pkg/testutil/apps"
	testops "github.com/apecloud/kubeblocks/pkg/testutil/operations"
	testparameters "github.com/apecloud/kubeblocks/pkg/testutil/parameters"
)

var _ = Describe("Reconfigure OpsRequest", func() {
	var (
		randomStr   = testCtx.GetRandomStr()
		compDefName = "test-compdef-" + randomStr
		clusterName = "test-cluster-" + randomStr
	)

	cleanEnv := func() {
		// must wait till resources deleted and no longer existed before the testcases start,
		// otherwise if later it needs to create some new resource objects with the same name,
		// in race conditions, it will find the existence of old objects, resulting failure to
		// create the new objects.
		By("clean resources")

		// delete cluster(and all dependent sub-resources), cluster definition
		testapps.ClearClusterResourcesWithRemoveFinalizerOption(&testCtx)

		// delete rest resources
		inNS := client.InNamespace(testCtx.DefaultNamespace)
		ml := client.HasLabels{testCtx.TestObjLabelKey}
		// namespaced
		testapps.ClearResources(&testCtx, generics.OpsRequestSignature, inNS, ml)
		testapps.ClearResources(&testCtx, generics.ConfigMapSignature, inNS, ml)
		testapps.ClearResources(&testCtx, generics.ParametersDefinitionSignature, ml)
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.InstanceSetSignature, true, inNS, ml)
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.ComponentParameterSignature, true, inNS)
	}

	BeforeEach(cleanEnv)

	AfterEach(cleanEnv)

	Context("Test Reconfigure", func() {
		It("Test Reconfigure OpsRequest", func() {
			By("init operations resources ")
			reqCtx := intctrlutil.RequestCtx{Ctx: ctx}
			opsRes, _, _ := initOperationsResources(compDefName, clusterName)
			testapps.MockInstanceSetComponent(&testCtx, clusterName, defaultCompName)

			By("prepare configuration metadata and component parameter")
			template := testparameters.NewComponentTemplateFactory("mysql-config", testCtx.DefaultNamespace).
				Create(&testCtx).
				GetObject()
			paramsDef := testparameters.NewParametersDefinitionFactory("mysql-params-" + randomStr).
				SetComponentDefinition(compDefName).
				SetTemplateName("mysql-config").
				Schema(`
parameter: {
  max_connections?: string
  gtid_mode?: string
}`).
				Create(&testCtx).
				GetObject()
			Expect(testapps.ChangeObjStatus(&testCtx, paramsDef, func() {
				paramsDef.Status.Phase = parametersv1alpha1.PDAvailablePhase
			})).Should(Succeed())
			Expect(testapps.GetAndChangeObj(&testCtx, client.ObjectKey{Name: compDefName}, func(compDef *appsv1.ComponentDefinition) {
				compDef.Spec.ServiceVersion = "8.0.30"
				compDef.Spec.Configs = []appsv1.ComponentFileTemplate{
					{
						Name:            "mysql-config",
						Template:        template.Name,
						Namespace:       template.Namespace,
						VolumeName:      "mysql-config",
						ExternalManaged: pointer.Bool(true),
					},
				}
			})()).Should(Succeed())

			componentParameter := builder.NewComponentParameterBuilder(testCtx.DefaultNamespace, parameterscore.GenerateComponentConfigurationName(clusterName, defaultCompName)).
				AddLabelsInMap(constant.GetCompLabelsWithDef(clusterName, defaultCompName, compDefName)).
				SetClusterName(clusterName).
				SetCompName(defaultCompName).
				GetObject()
			// The ops suite does not run the ComponentParameter controller, so it uses a
			// prepared normalized config item skeleton as the starting point.
			componentParameter.Spec.ConfigItemDetails = []parametersv1alpha1.ConfigTemplateItemDetail{{
				Name: "mysql-config",
				ConfigSpec: &appsv1.ComponentFileTemplate{
					Name:            "mysql-config",
					Template:        template.Name,
					Namespace:       template.Namespace,
					VolumeName:      "mysql-config",
					ExternalManaged: pointer.Bool(true),
				},
			}}
			Expect(testCtx.CreateObj(ctx, componentParameter)).Should(Succeed())

			Expect(testapps.GetAndChangeObjStatus(&testCtx, client.ObjectKeyFromObject(componentParameter), func(cp *parametersv1alpha1.ComponentParameter) {
				cp.Status.ObservedGeneration = cp.Generation
				cp.Status.Phase = parametersv1alpha1.CFinishedPhase
				cp.Status.ConfigurationItemStatus = []parametersv1alpha1.ConfigTemplateItemDetailStatus{{
					Name:  cp.Spec.ConfigItemDetails[0].Name,
					Phase: parametersv1alpha1.CFinishedPhase,
				}}
			})()).Should(Succeed())

			configMap := &corev1.ConfigMap{}
			configMap.Name = parameterscore.GetComponentCfgName(clusterName, defaultCompName, "mysql-config")
			configMap.Namespace = testCtx.DefaultNamespace
			configMap.Labels = constant.GetCompLabels(clusterName, defaultCompName)
			configMap.Labels[constant.CMConfigurationTemplateNameLabelKey] = "mysql-config"
			configMap.Labels[constant.CMConfigurationTypeLabelKey] = constant.ConfigInstanceType
			configMap.Labels[constant.CMConfigurationSpecProviderLabelKey] = "mysql-config"
			configMap.Data = map[string]string{testparameters.MysqlConfigFile: template.Data[testparameters.MysqlConfigFile]}
			Expect(testCtx.CreateObj(ctx, configMap)).Should(Succeed())

			By("create Start opsRequest")
			ops := testops.NewOpsRequestObj("start-ops-"+randomStr, testCtx.DefaultNamespace,
				clusterName, opsv1alpha1.ReconfiguringType)
			ops.Spec.Reconfigures = []opsv1alpha1.Reconfigure{
				{
					ComponentOps: opsv1alpha1.ComponentOps{ComponentName: defaultCompName},
					Parameters: []opsv1alpha1.ParameterPair{
						{
							Key:   "max_connections",
							Value: pointer.String("200"),
						},
					},
				},
			}
			opsRes.OpsRequest = testops.CreateOpsRequest(ctx, testCtx, ops)

			By("test start action and reconcile function")
			Expect(opsutil.UpdateClusterOpsAnnotations(ctx, k8sClient, opsRes.Cluster, nil)).Should(Succeed())

			opsRes.OpsRequest.Status.Phase = opsv1alpha1.OpsPendingPhase
			_, err := GetOpsManager().Do(reqCtx, k8sClient, opsRes)
			Expect(err).ShouldNot(HaveOccurred())
			Eventually(testops.GetOpsRequestPhase(&testCtx, client.ObjectKeyFromObject(opsRes.OpsRequest))).Should(Equal(opsv1alpha1.OpsCreatingPhase))
			// do start action
			_, err = GetOpsManager().Do(reqCtx, k8sClient, opsRes)
			Expect(err).ShouldNot(HaveOccurred())
			_, err = GetOpsManager().Reconcile(reqCtx, k8sClient, opsRes)
			Expect(err).ShouldNot(HaveOccurred())

			Eventually(testops.GetOpsRequestPhase(&testCtx, client.ObjectKeyFromObject(opsRes.OpsRequest))).Should(Equal(opsv1alpha1.OpsRunningPhase))
			Expect(testCtx.Cli.Get(ctx, client.ObjectKeyFromObject(opsRes.OpsRequest), opsRes.OpsRequest)).Should(Succeed())

			Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(componentParameter), func(g Gomega, cp *parametersv1alpha1.ComponentParameter) {
				g.Expect(cp.Spec.Desired).ShouldNot(BeNil())
				g.Expect(cp.Spec.Desired.Assignments).Should(HaveKeyWithValue("max_connections", pointer.String("200")))
			})).Should(Succeed())

			Expect(testapps.GetAndChangeObjStatus(&testCtx, client.ObjectKeyFromObject(componentParameter), func(cp *parametersv1alpha1.ComponentParameter) {
				cp.Status.ObservedGeneration = cp.Generation
				cp.Status.Phase = parametersv1alpha1.CFinishedPhase
				cp.Status.ConfigurationItemStatus = []parametersv1alpha1.ConfigTemplateItemDetailStatus{{
					Name:  cp.Spec.ConfigItemDetails[0].Name,
					Phase: parametersv1alpha1.CFinishedPhase,
				}}
			})()).Should(Succeed())
			Expect(testCtx.Cli.Get(ctx, client.ObjectKeyFromObject(opsRes.OpsRequest), opsRes.OpsRequest)).Should(Succeed())

			_, err = GetOpsManager().Reconcile(reqCtx, k8sClient, opsRes)
			Eventually(testops.GetOpsRequestPhase(&testCtx, client.ObjectKeyFromObject(opsRes.OpsRequest))).Should(Equal(opsv1alpha1.OpsSucceedPhase))

			Expect(err).Should(BeNil())
		})

		It("rejects unknown parameter before patching CP desired", func() {
			By("init operations resources ")
			reqCtx := intctrlutil.RequestCtx{Ctx: ctx}
			opsRes, _, _ := initOperationsResources(compDefName, clusterName)
			testapps.MockInstanceSetComponent(&testCtx, clusterName, defaultCompName)

			By("prepare configuration metadata and component parameter")
			template := testparameters.NewComponentTemplateFactory("mysql-config", testCtx.DefaultNamespace).
				Create(&testCtx).
				GetObject()
			paramsDef := testparameters.NewParametersDefinitionFactory("mysql-params-prevalidate-" + randomStr).
				SetComponentDefinition(compDefName).
				SetTemplateName("mysql-config").
				Schema(`
parameter: {
  max_connections?: string
  gtid_mode?: string
}`).
				Create(&testCtx).
				GetObject()
			Expect(testapps.ChangeObjStatus(&testCtx, paramsDef, func() {
				paramsDef.Status.Phase = parametersv1alpha1.PDAvailablePhase
			})).Should(Succeed())
			Expect(testapps.GetAndChangeObj(&testCtx, client.ObjectKey{Name: compDefName}, func(compDef *appsv1.ComponentDefinition) {
				compDef.Spec.ServiceVersion = "8.0.30"
				compDef.Spec.Configs = []appsv1.ComponentFileTemplate{
					{
						Name:            "mysql-config",
						Template:        template.Name,
						Namespace:       template.Namespace,
						VolumeName:      "mysql-config",
						ExternalManaged: pointer.Bool(true),
					},
				}
			})()).Should(Succeed())

			componentParameter := builder.NewComponentParameterBuilder(testCtx.DefaultNamespace, parameterscore.GenerateComponentConfigurationName(clusterName, defaultCompName)).
				AddLabelsInMap(constant.GetCompLabelsWithDef(clusterName, defaultCompName, compDefName)).
				SetClusterName(clusterName).
				SetCompName(defaultCompName).
				GetObject()
			componentParameter.Spec.ConfigItemDetails = []parametersv1alpha1.ConfigTemplateItemDetail{{
				Name: "mysql-config",
				ConfigSpec: &appsv1.ComponentFileTemplate{
					Name:            "mysql-config",
					Template:        template.Name,
					Namespace:       template.Namespace,
					VolumeName:      "mysql-config",
					ExternalManaged: pointer.Bool(true),
				},
			}}
			Expect(testCtx.CreateObj(ctx, componentParameter)).Should(Succeed())

			Expect(testapps.GetAndChangeObjStatus(&testCtx, client.ObjectKeyFromObject(componentParameter), func(cp *parametersv1alpha1.ComponentParameter) {
				cp.Status.ObservedGeneration = cp.Generation
				cp.Status.Phase = parametersv1alpha1.CFinishedPhase
				cp.Status.ConfigurationItemStatus = []parametersv1alpha1.ConfigTemplateItemDetailStatus{{
					Name:  cp.Spec.ConfigItemDetails[0].Name,
					Phase: parametersv1alpha1.CFinishedPhase,
				}}
			})()).Should(Succeed())

			By("create reconfigure with unknown parameter http_port")
			ops := testops.NewOpsRequestObj("reject-unknown-"+randomStr, testCtx.DefaultNamespace,
				clusterName, opsv1alpha1.ReconfiguringType)
			ops.Spec.Reconfigures = []opsv1alpha1.Reconfigure{
				{
					ComponentOps: opsv1alpha1.ComponentOps{ComponentName: defaultCompName},
					Parameters: []opsv1alpha1.ParameterPair{
						{
							Key:   "http_port",
							Value: pointer.String("9999"),
						},
					},
				},
			}
			opsRes.OpsRequest = testops.CreateOpsRequest(ctx, testCtx, ops)
			Expect(opsutil.UpdateClusterOpsAnnotations(ctx, k8sClient, opsRes.Cluster, nil)).Should(Succeed())

			opsRes.OpsRequest.Status.Phase = opsv1alpha1.OpsPendingPhase
			_, err := GetOpsManager().Do(reqCtx, k8sClient, opsRes)
			Expect(err).ShouldNot(HaveOccurred())
			Eventually(testops.GetOpsRequestPhase(&testCtx, client.ObjectKeyFromObject(opsRes.OpsRequest))).Should(Equal(opsv1alpha1.OpsCreatingPhase))

			By("Action should reject the unknown parameter with a fatal error")
			_, err = GetOpsManager().Do(reqCtx, k8sClient, opsRes)
			Expect(err).ShouldNot(HaveOccurred())

			Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(opsRes.OpsRequest), func(g Gomega, fetched *opsv1alpha1.OpsRequest) {
				g.Expect(fetched.Status.Phase).Should(Equal(opsv1alpha1.OpsFailedPhase))
				condition := meta.FindStatusCondition(fetched.Status.Conditions, opsv1alpha1.ConditionTypeFailed)
				g.Expect(condition).ShouldNot(BeNil())
				g.Expect(condition.Message).Should(ContainSubstring("http_port"))
			})).Should(Succeed())

			By("CP desired must NOT contain http_port")
			Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(componentParameter), func(g Gomega, cp *parametersv1alpha1.ComponentParameter) {
				if cp.Spec.Desired != nil && cp.Spec.Desired.Assignments != nil {
					g.Expect(cp.Spec.Desired.Assignments).ShouldNot(HaveKey("http_port"))
				}
			})).Should(Succeed())
		})

		It("propagates ComponentParameter merge failure", func() {
			By("init operations resources ")
			reqCtx := intctrlutil.RequestCtx{Ctx: ctx}
			opsRes, _, _ := initOperationsResources(compDefName, clusterName)

			componentParameter := builder.NewComponentParameterBuilder(testCtx.DefaultNamespace, parameterscore.GenerateComponentConfigurationName(clusterName, defaultCompName)).
				AddLabelsInMap(constant.GetCompLabelsWithDef(clusterName, defaultCompName, compDefName)).
				SetClusterName(clusterName).
				SetCompName(defaultCompName).
				GetObject()
			Expect(testCtx.CreateObj(ctx, componentParameter)).Should(Succeed())

			By("create a reconfigure opsRequest")
			ops := testops.NewOpsRequestObj("failed-reconfigure-"+randomStr, testCtx.DefaultNamespace,
				clusterName, opsv1alpha1.ReconfiguringType)
			ops.Spec.Reconfigures = []opsv1alpha1.Reconfigure{
				{
					ComponentOps: opsv1alpha1.ComponentOps{ComponentName: defaultCompName},
					Parameters: []opsv1alpha1.ParameterPair{
						{
							Key:   "maxmemory-samples",
							Value: pointer.String("0"),
						},
					},
				},
			}
			opsRes.OpsRequest = testops.CreateOpsRequest(ctx, testCtx, ops)

			By("write desired parameters through the ops path")
			Expect(opsutil.UpdateClusterOpsAnnotations(ctx, k8sClient, opsRes.Cluster, nil)).Should(Succeed())

			opsRes.OpsRequest.Status.Phase = opsv1alpha1.OpsPendingPhase
			_, err := GetOpsManager().Do(reqCtx, k8sClient, opsRes)
			Expect(err).ShouldNot(HaveOccurred())
			Eventually(testops.GetOpsRequestPhase(&testCtx, client.ObjectKeyFromObject(opsRes.OpsRequest))).Should(Equal(opsv1alpha1.OpsCreatingPhase))
			_, err = GetOpsManager().Do(reqCtx, k8sClient, opsRes)
			Expect(err).ShouldNot(HaveOccurred())
			Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(componentParameter), func(g Gomega, cp *parametersv1alpha1.ComponentParameter) {
				g.Expect(cp.Spec.Desired).ShouldNot(BeNil())
				g.Expect(cp.Spec.Desired.Assignments).Should(HaveKeyWithValue("maxmemory-samples", pointer.String("0")))
			})).Should(Succeed())

			By("surface the ComponentParameter failure back to the opsRequest")
			Expect(testapps.GetAndChangeObjStatus(&testCtx, client.ObjectKeyFromObject(componentParameter), func(cp *parametersv1alpha1.ComponentParameter) {
				cp.Status.ObservedGeneration = cp.Generation
				cp.Status.Phase = parametersv1alpha1.CMergeFailedPhase
				cp.Status.Message = "parameter maxmemory-samples value \"0\" is invalid"
				cp.Status.ConfigurationItemStatus = []parametersv1alpha1.ConfigTemplateItemDetailStatus{{
					Name:  "mysql-config",
					Phase: parametersv1alpha1.CMergeFailedPhase,
				}}
			})()).Should(Succeed())
			Expect(testCtx.Cli.Get(ctx, client.ObjectKeyFromObject(opsRes.OpsRequest), opsRes.OpsRequest)).Should(Succeed())
			_, err = GetOpsManager().Reconcile(reqCtx, k8sClient, opsRes)
			Expect(err).ShouldNot(HaveOccurred())
			Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(opsRes.OpsRequest), func(g Gomega, fetched *opsv1alpha1.OpsRequest) {
				g.Expect(fetched.Status.Phase).Should(Equal(opsv1alpha1.OpsFailedPhase))
				condition := meta.FindStatusCondition(fetched.Status.Conditions, opsv1alpha1.ConditionTypeFailed)
				g.Expect(condition).ShouldNot(BeNil())
				g.Expect(condition.Message).Should(ContainSubstring("maxmemory-samples"))
			})).Should(Succeed())
		})
	})
})
