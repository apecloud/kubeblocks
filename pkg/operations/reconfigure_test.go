/*
Copyright (C) 2022-2025 ApeCloud Co., Ltd

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
	"github.com/apecloud/kubeblocks/pkg/parameters"
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
				ClusterRef(clusterName).
				Component(defaultCompName).
				AddConfigurationItem(appsv1.ComponentFileTemplate{
					Name:            "mysql-config",
					Template:        template.Name,
					Namespace:       template.Namespace,
					VolumeName:      "mysql-config",
					ExternalManaged: pointer.Bool(true),
				}).
				GetObject()
			Expect(testCtx.CreateObj(ctx, componentParameter)).Should(Succeed())
			Expect(testapps.ChangeObjStatus(&testCtx, componentParameter, func() {
				componentParameter.Status.ObservedGeneration = componentParameter.Generation
				componentParameter.Status.Phase = parametersv1alpha1.CFinishedPhase
				componentParameter.Status.ConfigurationItemStatus = []parametersv1alpha1.ConfigTemplateItemDetailStatus{{
					Name:  "mysql-config",
					Phase: parametersv1alpha1.CFinishedPhase,
				}}
			})).Should(Succeed())

			configMap := &corev1.ConfigMap{}
			configMap.Name = parameterscore.GetComponentCfgName(clusterName, defaultCompName, "mysql-config")
			configMap.Namespace = testCtx.DefaultNamespace
			configMap.Labels = constant.GetCompLabels(clusterName, defaultCompName)
			configMap.Labels[constant.CMConfigurationTemplateNameLabelKey] = "mysql-config"
			configMap.Labels[constant.CMConfigurationTypeLabelKey] = constant.ConfigInstanceType
			configMap.Labels[constant.CMConfigurationSpecProviderLabelKey] = "mysql-config"
			configMap.Data = map[string]string{testparameters.MysqlConfigFile: template.Data[testparameters.MysqlConfigFile]}
			Expect(testCtx.CreateObj(ctx, configMap)).Should(Succeed())

			customTemplate := testparameters.NewComponentTemplateFactory("custom-mysql-config", testCtx.DefaultNamespace).
				AddConfigFile(testparameters.MysqlConfigFile, "max_connections=300\ngtid_mode=ON\n").
				Create(&testCtx).
				GetObject()

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
					UserConfigTemplates: map[string]parametersv1alpha1.ConfigTemplateExtension{
						"mysql-config": {
							TemplateRef: customTemplate.Name,
							Namespace:   customTemplate.Namespace,
							Policy:      parametersv1alpha1.ReplacePolicy,
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

			Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(componentParameter), func(g Gomega, cp *parametersv1alpha1.ComponentParameter) {
				item := parameters.GetConfigTemplateItem(&cp.Spec, "mysql-config")
				g.Expect(item).ShouldNot(BeNil())
				g.Expect(item.CustomTemplates).ShouldNot(BeNil())
				g.Expect(item.CustomTemplates.TemplateRef).Should(Equal(customTemplate.Name))
				g.Expect(item.ConfigFileParams).Should(HaveKey(testparameters.MysqlConfigFile))
				g.Expect(item.ConfigFileParams[testparameters.MysqlConfigFile].Parameters).Should(HaveKeyWithValue("max_connections", pointer.String("200")))
			})).Should(Succeed())

			Expect(testapps.ChangeObjStatus(&testCtx, componentParameter, func() {
				componentParameter.Status.ObservedGeneration = componentParameter.Generation
				componentParameter.Status.Phase = parametersv1alpha1.CFinishedPhase
				componentParameter.Status.ConfigurationItemStatus = []parametersv1alpha1.ConfigTemplateItemDetailStatus{{
					Name:  "mysql-config",
					Phase: parametersv1alpha1.CFinishedPhase,
				}}
			})).Should(Succeed())

			_, err = GetOpsManager().Reconcile(reqCtx, k8sClient, opsRes)
			Eventually(testops.GetOpsRequestPhase(&testCtx, client.ObjectKeyFromObject(opsRes.OpsRequest))).Should(Equal(opsv1alpha1.OpsSucceedPhase))

			Expect(err).Should(BeNil())

		})
	})
})
