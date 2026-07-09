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
	"encoding/json"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
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
	const targetConfigHash = "target-hash"

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

		It("consumes stable permanent reconfigure failures and withdraws only matching desired entries", func() {
			reqCtx := intctrlutil.RequestCtx{Ctx: ctx}
			opsRes, _, _ := initOperationsResources(compDefName, clusterName)
			componentParameter := createReconfigureComponentParameter(clusterName, compDefName)
			opsRes.OpsRequest = createReconfigureOpsRequest("permanent-reconfigure-"+testCtx.GetRandomStr(), clusterName, "max_connections", pointer.String("200"))

			Expect(opsutil.UpdateClusterOpsAnnotations(ctx, k8sClient, opsRes.Cluster, nil)).Should(Succeed())
			opsRes.OpsRequest.Status.Phase = opsv1alpha1.OpsPendingPhase
			_, err := GetOpsManager().Do(reqCtx, k8sClient, opsRes)
			Expect(err).ShouldNot(HaveOccurred())
			_, err = GetOpsManager().Do(reqCtx, k8sClient, opsRes)
			Expect(err).ShouldNot(HaveOccurred())

			Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(componentParameter), func(g Gomega, cp *parametersv1alpha1.ComponentParameter) {
				g.Expect(cp.Spec.Desired.Assignments).Should(HaveKeyWithValue("max_connections", pointer.String("200")))
			})).Should(Succeed())
			Expect(testCtx.Cli.Get(ctx, client.ObjectKeyFromObject(opsRes.OpsRequest), opsRes.OpsRequest)).Should(Succeed())
			setReconfigureTargetConfigHash(opsRes.Cluster, targetConfigHash)

			Expect(testapps.GetAndChangeObjStatus(&testCtx, client.ObjectKeyFromObject(componentParameter), func(cp *parametersv1alpha1.ComponentParameter) {
				cp.Status.ObservedGeneration = cp.Generation
				cp.Status.Phase = parametersv1alpha1.CFailedPhase
				cp.Status.ConfigurationItemStatus = []parametersv1alpha1.ConfigTemplateItemDetailStatus{stableReconfigureFailureStatus(
					cp, opsRes.OpsRequest, parametersv1alpha1.ReconfigureFailureClassPermanent,
					parametersv1alpha1.ReconfigureFailureReasonUnsupportedParameter,
					"password=secret token=abc SELECT * FROM t")}
			})()).Should(Succeed())
			Expect(testCtx.Cli.Get(ctx, client.ObjectKeyFromObject(componentParameter), componentParameter)).Should(Succeed())
			Expect(isConsumablePermanentReconfigureFailure(opsRes.OpsRequest, componentParameter,
				&componentParameter.Status.ConfigurationItemStatus[0], targetConfigHash)).Should(BeTrue())

			Expect(testCtx.Cli.Get(ctx, client.ObjectKeyFromObject(opsRes.OpsRequest), opsRes.OpsRequest)).Should(Succeed())
			_, err = GetOpsManager().Reconcile(reqCtx, k8sClient, opsRes)
			Expect(err).ShouldNot(HaveOccurred())

			Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(opsRes.OpsRequest), func(g Gomega, fetched *opsv1alpha1.OpsRequest) {
				g.Expect(fetched.Status.Phase).Should(Equal(opsv1alpha1.OpsFailedPhase))
				condition := meta.FindStatusCondition(fetched.Status.Conditions, opsv1alpha1.ConditionTypeFailed)
				g.Expect(condition).ShouldNot(BeNil())
				g.Expect(condition.Message).Should(ContainSubstring(parametersv1alpha1.ReconfigureFailureReasonUnsupportedParameter))
				g.Expect(condition.Message).ShouldNot(ContainSubstring("password"))
				g.Expect(condition.Message).ShouldNot(ContainSubstring("SELECT"))
				expectNoRawReconfigureLeak(g, fetched.Status)
			})).Should(Succeed())
			Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(componentParameter), func(g Gomega, cp *parametersv1alpha1.ComponentParameter) {
				g.Expect(cp.Status.ConfigurationItemStatus).ShouldNot(BeEmpty())
				g.Expect(cp.Status.ConfigurationItemStatus[0].ReconcileDetail).ShouldNot(BeNil())
				g.Expect(cp.Status.ConfigurationItemStatus[0].ReconcileDetail.ErrMessage).Should(ContainSubstring("password=secret"))
				g.Expect(cp.Status.ConfigurationItemStatus[0].ReconcileDetail.ErrMessage).Should(ContainSubstring("SELECT * FROM t"))
				expectNoRawReconfigureLeak(g, cp.ObjectMeta)
			})).Should(Succeed())
			Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(componentParameter), func(g Gomega, cp *parametersv1alpha1.ComponentParameter) {
				g.Expect(cp.Spec.Desired.Assignments).ShouldNot(HaveKey("max_connections"))
			})).Should(Succeed())
		})

		It("does not consume permanent-looking failures with invalid identity or reason matrix", func() {
			reqCtx := intctrlutil.RequestCtx{Ctx: ctx}
			opsRes, _, _ := initOperationsResources(compDefName, clusterName)
			componentParameter := createReconfigureComponentParameter(clusterName, compDefName)
			opsRes.OpsRequest = createReconfigureOpsRequest("invalid-permanent-"+testCtx.GetRandomStr(), clusterName, "max_connections", pointer.String("200"))

			Expect(opsutil.UpdateClusterOpsAnnotations(ctx, k8sClient, opsRes.Cluster, nil)).Should(Succeed())
			opsRes.OpsRequest.Status.Phase = opsv1alpha1.OpsPendingPhase
			_, err := GetOpsManager().Do(reqCtx, k8sClient, opsRes)
			Expect(err).ShouldNot(HaveOccurred())
			_, err = GetOpsManager().Do(reqCtx, k8sClient, opsRes)
			Expect(err).ShouldNot(HaveOccurred())

			cases := []struct {
				name   string
				mutate func(*parametersv1alpha1.ConfigTemplateItemDetailStatus)
			}{
				{
					name: "missing operation uid",
					mutate: func(status *parametersv1alpha1.ConfigTemplateItemDetailStatus) {
						status.ReconcileDetail.OperationUID = ""
					},
				},
				{
					name: "missing config name",
					mutate: func(status *parametersv1alpha1.ConfigTemplateItemDetailStatus) {
						status.ReconcileDetail.ConfigName = ""
					},
				},
				{
					name: "missing target config hash",
					mutate: func(status *parametersv1alpha1.ConfigTemplateItemDetailStatus) {
						status.ReconcileDetail.TargetConfigHash = ""
					},
				},
				{
					name: "missing component parameter generation",
					mutate: func(status *parametersv1alpha1.ConfigTemplateItemDetailStatus) {
						status.ReconcileDetail.ComponentParameterGeneration = 0
					},
				},
				{
					name: "stale operation uid",
					mutate: func(status *parametersv1alpha1.ConfigTemplateItemDetailStatus) {
						status.ReconcileDetail.OperationUID = "old-ops-uid"
					},
				},
				{
					name: "mismatched config name",
					mutate: func(status *parametersv1alpha1.ConfigTemplateItemDetailStatus) {
						status.ReconcileDetail.ConfigName = "other-config"
					},
				},
				{
					name: "stale target config hash",
					mutate: func(status *parametersv1alpha1.ConfigTemplateItemDetailStatus) {
						status.ReconcileDetail.TargetConfigHash = "old-target-hash"
					},
				},
				{
					name: "stale component parameter generation",
					mutate: func(status *parametersv1alpha1.ConfigTemplateItemDetailStatus) {
						status.ReconcileDetail.ComponentParameterGeneration = 1
					},
				},
				{
					name: "missing failure class",
					mutate: func(status *parametersv1alpha1.ConfigTemplateItemDetailStatus) {
						status.ReconcileDetail.FailureClass = ""
					},
				},
				{
					name: "permanent transport error",
					mutate: func(status *parametersv1alpha1.ConfigTemplateItemDetailStatus) {
						status.ReconcileDetail.Reason = parametersv1alpha1.ReconfigureFailureReasonActionTransportError
					},
				},
				{
					name: "permanent unknown",
					mutate: func(status *parametersv1alpha1.ConfigTemplateItemDetailStatus) {
						status.ReconcileDetail.Reason = parametersv1alpha1.ReconfigureFailureReasonUnknown
					},
				},
			}
			Expect(testCtx.Cli.Get(ctx, client.ObjectKeyFromObject(opsRes.OpsRequest), opsRes.OpsRequest)).Should(Succeed())
			setReconfigureTargetConfigHash(opsRes.Cluster, targetConfigHash)
			for _, tc := range cases {
				By(tc.name)
				Expect(testapps.GetAndChangeObjStatus(&testCtx, client.ObjectKeyFromObject(componentParameter), func(cp *parametersv1alpha1.ComponentParameter) {
					status := stableReconfigureFailureStatus(cp, opsRes.OpsRequest,
						parametersv1alpha1.ReconfigureFailureClassPermanent,
						parametersv1alpha1.ReconfigureFailureReasonUnsupportedParameter, "raw stderr")
					tc.mutate(&status)
					cp.Status.ObservedGeneration = cp.Generation
					cp.Status.Phase = parametersv1alpha1.CFailedPhase
					cp.Status.ConfigurationItemStatus = []parametersv1alpha1.ConfigTemplateItemDetailStatus{status}
				})()).Should(Succeed())
				Expect(testCtx.Cli.Get(ctx, client.ObjectKeyFromObject(opsRes.OpsRequest), opsRes.OpsRequest)).Should(Succeed())
				_, err = GetOpsManager().Reconcile(reqCtx, k8sClient, opsRes)
				Expect(err).ShouldNot(HaveOccurred())
				Consistently(testops.GetOpsRequestPhase(&testCtx, client.ObjectKeyFromObject(opsRes.OpsRequest))).ShouldNot(Equal(opsv1alpha1.OpsFailedPhase))
				Expect(testapps.GetAndChangeObj(&testCtx, client.ObjectKeyFromObject(opsRes.OpsRequest), func(ops *opsv1alpha1.OpsRequest) {
					ops.Status.Phase = opsv1alpha1.OpsRunningPhase
				})()).Should(Succeed())
				Expect(testapps.GetAndChangeObj(&testCtx, client.ObjectKeyFromObject(componentParameter), func(cp *parametersv1alpha1.ComponentParameter) {
					if cp.Spec.Desired == nil {
						cp.Spec.Desired = &parametersv1alpha1.ParameterInputs{}
					}
					if cp.Spec.Desired.Assignments == nil {
						cp.Spec.Desired.Assignments = map[string]*string{}
					}
					cp.Spec.Desired.Assignments["max_connections"] = pointer.String("200")
				})()).Should(Succeed())
			}
		})

		It("does not withdraw desired values changed by a later operation", func() {
			reqCtx := intctrlutil.RequestCtx{Ctx: ctx}
			opsRes, _, _ := initOperationsResources(compDefName, clusterName)
			componentParameter := createReconfigureComponentParameter(clusterName, compDefName)
			opsRes.OpsRequest = createReconfigureOpsRequest("stale-permanent-"+testCtx.GetRandomStr(), clusterName, "max_connections", pointer.String("200"))

			Expect(opsutil.UpdateClusterOpsAnnotations(ctx, k8sClient, opsRes.Cluster, nil)).Should(Succeed())
			opsRes.OpsRequest.Status.Phase = opsv1alpha1.OpsPendingPhase
			_, err := GetOpsManager().Do(reqCtx, k8sClient, opsRes)
			Expect(err).ShouldNot(HaveOccurred())
			_, err = GetOpsManager().Do(reqCtx, k8sClient, opsRes)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(testapps.GetAndChangeObj(&testCtx, client.ObjectKeyFromObject(componentParameter), func(cp *parametersv1alpha1.ComponentParameter) {
				cp.Spec.Desired.Assignments["max_connections"] = pointer.String("300")
			})()).Should(Succeed())
			Expect(testCtx.Cli.Get(ctx, client.ObjectKeyFromObject(opsRes.OpsRequest), opsRes.OpsRequest)).Should(Succeed())
			setReconfigureTargetConfigHash(opsRes.Cluster, "new-target-hash")

			Expect(testapps.GetAndChangeObjStatus(&testCtx, client.ObjectKeyFromObject(componentParameter), func(cp *parametersv1alpha1.ComponentParameter) {
				cp.Status.ObservedGeneration = cp.Generation
				cp.Status.Phase = parametersv1alpha1.CFailedPhase
				cp.Status.ConfigurationItemStatus = []parametersv1alpha1.ConfigTemplateItemDetailStatus{stableReconfigureFailureStatus(
					cp, opsRes.OpsRequest, parametersv1alpha1.ReconfigureFailureClassPermanent,
					parametersv1alpha1.ReconfigureFailureReasonUnsupportedParameter, "raw stderr")}
			})()).Should(Succeed())

			Expect(testCtx.Cli.Get(ctx, client.ObjectKeyFromObject(opsRes.OpsRequest), opsRes.OpsRequest)).Should(Succeed())
			_, err = GetOpsManager().Reconcile(reqCtx, k8sClient, opsRes)
			Expect(err).ShouldNot(HaveOccurred())
			Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(componentParameter), func(g Gomega, cp *parametersv1alpha1.ComponentParameter) {
				g.Expect(cp.Spec.Desired.Assignments).Should(HaveKeyWithValue("max_connections", pointer.String("300")))
			})).Should(Succeed())
		})

		It("does not consume permanent failures for a stale target config hash", func() {
			reqCtx := intctrlutil.RequestCtx{Ctx: ctx}
			opsRes, _, _ := initOperationsResources(compDefName, clusterName)
			componentParameter := createReconfigureComponentParameter(clusterName, compDefName)
			opsRes.OpsRequest = createReconfigureOpsRequest("stale-hash-permanent-"+testCtx.GetRandomStr(), clusterName, "max_connections", pointer.String("200"))

			Expect(opsutil.UpdateClusterOpsAnnotations(ctx, k8sClient, opsRes.Cluster, nil)).Should(Succeed())
			opsRes.OpsRequest.Status.Phase = opsv1alpha1.OpsPendingPhase
			_, err := GetOpsManager().Do(reqCtx, k8sClient, opsRes)
			Expect(err).ShouldNot(HaveOccurred())
			_, err = GetOpsManager().Do(reqCtx, k8sClient, opsRes)
			Expect(err).ShouldNot(HaveOccurred())
			setReconfigureTargetConfigHash(opsRes.Cluster, "new-target-hash")

			Expect(testapps.GetAndChangeObjStatus(&testCtx, client.ObjectKeyFromObject(componentParameter), func(cp *parametersv1alpha1.ComponentParameter) {
				cp.Status.ObservedGeneration = cp.Generation
				cp.Status.Phase = parametersv1alpha1.CFailedPhase
				cp.Status.ConfigurationItemStatus = []parametersv1alpha1.ConfigTemplateItemDetailStatus{stableReconfigureFailureStatus(
					cp, opsRes.OpsRequest, parametersv1alpha1.ReconfigureFailureClassPermanent,
					parametersv1alpha1.ReconfigureFailureReasonUnsupportedParameter, "raw stderr")}
			})()).Should(Succeed())

			Expect(testCtx.Cli.Get(ctx, client.ObjectKeyFromObject(opsRes.OpsRequest), opsRes.OpsRequest)).Should(Succeed())
			_, err = GetOpsManager().Reconcile(reqCtx, k8sClient, opsRes)
			Expect(err).ShouldNot(HaveOccurred())
			Consistently(testops.GetOpsRequestPhase(&testCtx, client.ObjectKeyFromObject(opsRes.OpsRequest))).ShouldNot(Equal(opsv1alpha1.OpsFailedPhase))
			Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(componentParameter), func(g Gomega, cp *parametersv1alpha1.ComponentParameter) {
				g.Expect(cp.Spec.Desired.Assignments).Should(HaveKeyWithValue("max_connections", pointer.String("200")))
			})).Should(Succeed())
		})

		It("uses optimistic locking when withdrawing desired entries", func() {
			reqCtx := intctrlutil.RequestCtx{Ctx: ctx}
			componentParameter := createReconfigureComponentParameter(clusterName, compDefName)
			Expect(testapps.GetAndChangeObj(&testCtx, client.ObjectKeyFromObject(componentParameter), func(cp *parametersv1alpha1.ComponentParameter) {
				cp.Spec.Desired = &parametersv1alpha1.ParameterInputs{Assignments: map[string]*string{
					"max_connections": pointer.String("200"),
				}}
			})()).Should(Succeed())
			Expect(testCtx.Cli.Get(ctx, client.ObjectKeyFromObject(componentParameter), componentParameter)).Should(Succeed())
			stale := componentParameter.DeepCopy()
			Expect(testapps.GetAndChangeObj(&testCtx, client.ObjectKeyFromObject(componentParameter), func(cp *parametersv1alpha1.ComponentParameter) {
				cp.Spec.Desired.Assignments["max_connections"] = pointer.String("300")
			})()).Should(Succeed())

			err := withdrawReconfigureDesired(reqCtx, k8sClient, stale, opsv1alpha1.Reconfigure{
				Parameters: []opsv1alpha1.ParameterPair{{
					Key:   "max_connections",
					Value: pointer.String("200"),
				}},
			})
			Expect(apierrors.IsConflict(err)).Should(BeTrue())
			Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(componentParameter), func(g Gomega, cp *parametersv1alpha1.ComponentParameter) {
				g.Expect(cp.Spec.Desired.Assignments).Should(HaveKeyWithValue("max_connections", pointer.String("300")))
			})).Should(Succeed())
		})

		It("withdraws matching desired entries only from components with matching failure identity", func() {
			reqCtx := intctrlutil.RequestCtx{Ctx: ctx}
			opsRes, _, _ := initOperationsResources(compDefName, clusterName)
			componentParameter := createReconfigureComponentParameter(clusterName, compDefName)
			otherCompName := defaultCompName + "-" + testCtx.GetRandomStr()
			otherComponentParameter := createReconfigureComponentParameterForComp(clusterName, otherCompName, compDefName)
			opsRes.OpsRequest = createReconfigureOpsRequest("multi-withdraw-"+testCtx.GetRandomStr(), clusterName, "max_connections", pointer.String("200"))
			setReconfigureTargetConfigHash(opsRes.Cluster, targetConfigHash)
			createResolvedComponentWithConfigHash(opsRes.Cluster.Name, otherCompName, compDefName, targetConfigHash)
			for _, cp := range []*parametersv1alpha1.ComponentParameter{componentParameter, otherComponentParameter} {
				Expect(testapps.GetAndChangeObj(&testCtx, client.ObjectKeyFromObject(cp), func(fetched *parametersv1alpha1.ComponentParameter) {
					fetched.Spec.Desired = &parametersv1alpha1.ParameterInputs{Assignments: map[string]*string{
						"max_connections": pointer.String("200"),
						"innodb_buffer":   pointer.String("1G"),
					}}
				})()).Should(Succeed())
				Expect(testapps.GetAndChangeObjStatus(&testCtx, client.ObjectKeyFromObject(cp), func(fetched *parametersv1alpha1.ComponentParameter) {
					fetched.Status.ObservedGeneration = fetched.Generation
					fetched.Status.Phase = parametersv1alpha1.CFailedPhase
					fetched.Status.ConfigurationItemStatus = []parametersv1alpha1.ConfigTemplateItemDetailStatus{stableReconfigureFailureStatus(
						fetched, opsRes.OpsRequest, parametersv1alpha1.ReconfigureFailureClassPermanent,
						parametersv1alpha1.ReconfigureFailureReasonUnsupportedParameter, "raw stderr")}
				})()).Should(Succeed())
			}

			err := (&reconfigureAction{}).withdrawReconfigureDesiredFromComponents(reqCtx, k8sClient, opsRes,
				[]string{defaultCompName, otherCompName}, opsv1alpha1.Reconfigure{
					Parameters: []opsv1alpha1.ParameterPair{{
						Key:   "max_connections",
						Value: pointer.String("200"),
					}},
				})
			Expect(err).ShouldNot(HaveOccurred())
			for _, cp := range []*parametersv1alpha1.ComponentParameter{componentParameter, otherComponentParameter} {
				Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(cp), func(g Gomega, fetched *parametersv1alpha1.ComponentParameter) {
					g.Expect(fetched.Spec.Desired.Assignments).ShouldNot(HaveKey("max_connections"))
					g.Expect(fetched.Spec.Desired.Assignments).Should(HaveKeyWithValue("innodb_buffer", pointer.String("1G")))
				})).Should(Succeed())
			}
		})

		It("does not withdraw matching desired entries from components without matching failure identity", func() {
			reqCtx := intctrlutil.RequestCtx{Ctx: ctx}
			opsRes, _, _ := initOperationsResources(compDefName, clusterName)
			componentParameter := createReconfigureComponentParameter(clusterName, compDefName)
			otherCompName := defaultCompName + "-" + testCtx.GetRandomStr()
			otherComponentParameter := createReconfigureComponentParameterForComp(clusterName, otherCompName, compDefName)
			opsRes.OpsRequest = createReconfigureOpsRequest("multi-withdraw-skip-"+testCtx.GetRandomStr(), clusterName, "max_connections", pointer.String("200"))
			setReconfigureTargetConfigHash(opsRes.Cluster, targetConfigHash)
			createResolvedComponentWithConfigHash(opsRes.Cluster.Name, otherCompName, compDefName, targetConfigHash)
			for _, cp := range []*parametersv1alpha1.ComponentParameter{componentParameter, otherComponentParameter} {
				Expect(testapps.GetAndChangeObj(&testCtx, client.ObjectKeyFromObject(cp), func(fetched *parametersv1alpha1.ComponentParameter) {
					fetched.Spec.Desired = &parametersv1alpha1.ParameterInputs{Assignments: map[string]*string{
						"max_connections": pointer.String("200"),
					}}
				})()).Should(Succeed())
			}
			Expect(testapps.GetAndChangeObjStatus(&testCtx, client.ObjectKeyFromObject(componentParameter), func(fetched *parametersv1alpha1.ComponentParameter) {
				fetched.Status.ObservedGeneration = fetched.Generation
				fetched.Status.Phase = parametersv1alpha1.CFailedPhase
				fetched.Status.ConfigurationItemStatus = []parametersv1alpha1.ConfigTemplateItemDetailStatus{stableReconfigureFailureStatus(
					fetched, opsRes.OpsRequest, parametersv1alpha1.ReconfigureFailureClassPermanent,
					parametersv1alpha1.ReconfigureFailureReasonUnsupportedParameter, "raw stderr")}
			})()).Should(Succeed())

			err := (&reconfigureAction{}).withdrawReconfigureDesiredFromComponents(reqCtx, k8sClient, opsRes,
				[]string{defaultCompName, otherCompName}, opsv1alpha1.Reconfigure{
					Parameters: []opsv1alpha1.ParameterPair{{
						Key:   "max_connections",
						Value: pointer.String("200"),
					}},
				})
			Expect(err).ShouldNot(HaveOccurred())
			Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(componentParameter), func(g Gomega, fetched *parametersv1alpha1.ComponentParameter) {
				g.Expect(fetched.Spec.Desired.Assignments).ShouldNot(HaveKey("max_connections"))
			})).Should(Succeed())
			Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(otherComponentParameter), func(g Gomega, fetched *parametersv1alpha1.ComponentParameter) {
				g.Expect(fetched.Spec.Desired.Assignments).Should(HaveKeyWithValue("max_connections", pointer.String("200")))
			})).Should(Succeed())
		})

		It("does not withdraw desired values when later legal state keeps the same key and value", func() {
			reqCtx := intctrlutil.RequestCtx{Ctx: ctx}
			opsRes, _, _ := initOperationsResources(compDefName, clusterName)
			componentParameter := createReconfigureComponentParameter(clusterName, compDefName)
			opsRes.OpsRequest = createReconfigureOpsRequest("same-value-later-"+testCtx.GetRandomStr(), clusterName, "max_connections", pointer.String("200"))
			setReconfigureTargetConfigHash(opsRes.Cluster, targetConfigHash)

			Expect(testapps.GetAndChangeObj(&testCtx, client.ObjectKeyFromObject(componentParameter), func(cp *parametersv1alpha1.ComponentParameter) {
				cp.Spec.Desired = &parametersv1alpha1.ParameterInputs{Assignments: map[string]*string{
					"max_connections": pointer.String("200"),
				}}
			})()).Should(Succeed())
			Expect(testCtx.Cli.Get(ctx, client.ObjectKeyFromObject(componentParameter), componentParameter)).Should(Succeed())
			oldGeneration := componentParameter.Generation
			Expect(testapps.GetAndChangeObjStatus(&testCtx, client.ObjectKeyFromObject(componentParameter), func(cp *parametersv1alpha1.ComponentParameter) {
				cp.Status.ObservedGeneration = cp.Generation
				cp.Status.Phase = parametersv1alpha1.CFailedPhase
				status := stableReconfigureFailureStatus(cp, opsRes.OpsRequest, parametersv1alpha1.ReconfigureFailureClassPermanent,
					parametersv1alpha1.ReconfigureFailureReasonUnsupportedParameter, "raw stderr")
				status.ReconcileDetail.ComponentParameterGeneration = oldGeneration
				cp.Status.ConfigurationItemStatus = []parametersv1alpha1.ConfigTemplateItemDetailStatus{status}
			})()).Should(Succeed())
			Expect(testapps.GetAndChangeObj(&testCtx, client.ObjectKeyFromObject(componentParameter), func(cp *parametersv1alpha1.ComponentParameter) {
				cp.Spec.Desired.Assignments["max_connections"] = pointer.String("200")
				cp.Spec.Desired.Assignments["later_safe_key"] = pointer.String("kept")
			})()).Should(Succeed())

			err := (&reconfigureAction{}).withdrawReconfigureDesiredFromComponents(reqCtx, k8sClient, opsRes,
				[]string{defaultCompName}, opsv1alpha1.Reconfigure{
					Parameters: []opsv1alpha1.ParameterPair{{
						Key:   "max_connections",
						Value: pointer.String("200"),
					}},
				})
			Expect(err).ShouldNot(HaveOccurred())
			Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(componentParameter), func(g Gomega, fetched *parametersv1alpha1.ComponentParameter) {
				g.Expect(fetched.Spec.Desired.Assignments).Should(HaveKeyWithValue("max_connections", pointer.String("200")))
				g.Expect(fetched.Spec.Desired.Assignments).Should(HaveKeyWithValue("later_safe_key", pointer.String("kept")))
			})).Should(Succeed())
		})

		It("resolves target config hash from concrete Component objects", func() {
			opsRes, _, _ := initOperationsResources(compDefName, clusterName)
			shardCompName := "shard-" + testCtx.GetRandomStr()
			createResolvedComponentWithConfigHash(opsRes.Cluster.Name, shardCompName, compDefName, targetConfigHash)

			got, ok, err := getCurrentConfigHash(ctx, k8sClient, testCtx.DefaultNamespace, opsRes.Cluster, shardCompName, "mysql-config")
			Expect(err).ShouldNot(HaveOccurred())
			Expect(ok).Should(BeTrue())
			Expect(got).Should(Equal(targetConfigHash))
		})

		It("returns an explicit error when a resolved Component hash source is missing", func() {
			opsRes, _, _ := initOperationsResources(compDefName, clusterName)
			shardCompName := "missing-shard-" + testCtx.GetRandomStr()

			got, ok, err := getCurrentConfigHash(ctx, k8sClient, testCtx.DefaultNamespace, opsRes.Cluster, shardCompName, "mysql-config")

			Expect(err).Should(HaveOccurred())
			Expect(apierrors.IsNotFound(err)).Should(BeTrue())
			Expect(ok).Should(BeFalse())
			Expect(got).Should(BeEmpty())
		})

		It("returns an explicit error when a resolved Component hash source is terminating", func() {
			opsRes, _, _ := initOperationsResources(compDefName, clusterName)
			shardCompName := "terminating-shard-" + testCtx.GetRandomStr()
			componentObj := createResolvedComponentWithConfigHash(opsRes.Cluster.Name, shardCompName, compDefName, targetConfigHash)
			Expect(testapps.GetAndChangeObj(&testCtx, client.ObjectKeyFromObject(componentObj), func(comp *appsv1.Component) {
				comp.Finalizers = append(comp.Finalizers, "test.kubeblocks.io/hold")
			})()).Should(Succeed())
			Expect(testCtx.Cli.Delete(ctx, componentObj)).Should(Succeed())

			got, ok, err := getCurrentConfigHash(ctx, k8sClient, testCtx.DefaultNamespace, opsRes.Cluster, shardCompName, "mysql-config")

			Expect(err).Should(HaveOccurred())
			Expect(err.Error()).Should(ContainSubstring("terminating"))
			Expect(ok).Should(BeFalse())
			Expect(got).Should(BeEmpty())
			Expect(testapps.GetAndChangeObj(&testCtx, client.ObjectKeyFromObject(componentObj), func(comp *appsv1.Component) {
				comp.Finalizers = nil
			})()).Should(Succeed())
		})

		It("does not silently convert resolved Component hash read errors into Running", func() {
			reqCtx := intctrlutil.RequestCtx{Ctx: ctx}
			opsRes, _, _ := initOperationsResources(compDefName, clusterName)
			shardingName := "sharding-" + testCtx.GetRandomStr()
			shardCompName := shardingName + "-0"
			componentObj := createResolvedComponentWithConfigHash(opsRes.Cluster.Name, shardCompName, compDefName, targetConfigHash)
			Expect(testapps.GetAndChangeObj(&testCtx, client.ObjectKeyFromObject(componentObj), func(comp *appsv1.Component) {
				comp.Labels[constant.KBAppShardingNameLabelKey] = shardingName
				comp.Finalizers = append(comp.Finalizers, "test.kubeblocks.io/hold")
			})()).Should(Succeed())
			Expect(testapps.GetAndChangeObj(&testCtx, client.ObjectKeyFromObject(opsRes.Cluster), func(cluster *appsv1.Cluster) {
				cluster.Spec.ComponentSpecs = nil
				cluster.Spec.Shardings = []appsv1.ClusterSharding{{
					Name: shardingName,
					Template: appsv1.ClusterComponentSpec{
						Name:         "ignored",
						ComponentDef: compDefName,
					},
					Shards: 1,
				}}
			})()).Should(Succeed())
			Expect(testCtx.Cli.Get(ctx, client.ObjectKeyFromObject(opsRes.Cluster), opsRes.Cluster)).Should(Succeed())
			componentParameter := createReconfigureComponentParameterForComp(clusterName, shardCompName, compDefName)
			opsRes.OpsRequest = createReconfigureOpsRequest("terminating-shard-hash-"+testCtx.GetRandomStr(), clusterName, "max_connections", pointer.String("200"))
			opsRes.OpsRequest.Spec.Reconfigures[0].ComponentName = shardingName
			Expect(testapps.GetAndChangeObjStatus(&testCtx, client.ObjectKeyFromObject(componentParameter), func(cp *parametersv1alpha1.ComponentParameter) {
				cp.Status.ObservedGeneration = cp.Generation
				cp.Status.Phase = parametersv1alpha1.CFailedPhase
				cp.Status.ConfigurationItemStatus = []parametersv1alpha1.ConfigTemplateItemDetailStatus{stableReconfigureFailureStatus(
					cp, opsRes.OpsRequest, parametersv1alpha1.ReconfigureFailureClassPermanent,
					parametersv1alpha1.ReconfigureFailureReasonUnsupportedParameter, "raw stderr")}
			})()).Should(Succeed())
			Expect(testCtx.Cli.Delete(ctx, componentObj)).Should(Succeed())

			phase, requeueAfter, err := (&reconfigureAction{}).ReconcileAction(reqCtx, k8sClient, opsRes)

			Expect(err).Should(HaveOccurred())
			Expect(err.Error()).Should(ContainSubstring("terminating"))
			Expect(phase).Should(BeEmpty())
			Expect(requeueAfter).Should(Equal(noRequeueAfter))
			Expect(testapps.GetAndChangeObj(&testCtx, client.ObjectKeyFromObject(componentObj), func(comp *appsv1.Component) {
				comp.Finalizers = nil
			})()).Should(Succeed())
		})
	})
})

func createReconfigureComponentParameter(clusterName, compDefName string) *parametersv1alpha1.ComponentParameter {
	return createReconfigureComponentParameterForComp(clusterName, defaultCompName, compDefName)
}

func createReconfigureComponentParameterForComp(clusterName, compName, compDefName string) *parametersv1alpha1.ComponentParameter {
	componentParameter := builder.NewComponentParameterBuilder(testCtx.DefaultNamespace, parameterscore.GenerateComponentConfigurationName(clusterName, compName)).
		AddLabelsInMap(constant.GetCompLabelsWithDef(clusterName, compName, compDefName)).
		SetClusterName(clusterName).
		SetCompName(compName).
		GetObject()
	componentParameter.Spec.ConfigItemDetails = []parametersv1alpha1.ConfigTemplateItemDetail{{
		Name: "mysql-config",
		ConfigSpec: &appsv1.ComponentFileTemplate{
			Name:            "mysql-config",
			Template:        "mysql-config",
			Namespace:       testCtx.DefaultNamespace,
			VolumeName:      "mysql-config",
			ExternalManaged: pointer.Bool(true),
		},
	}}
	Expect(testCtx.CreateObj(ctx, componentParameter)).Should(Succeed())
	return componentParameter
}

func createReconfigureOpsRequest(name, clusterName, key string, value *string) *opsv1alpha1.OpsRequest {
	ops := testops.NewOpsRequestObj(name, testCtx.DefaultNamespace, clusterName, opsv1alpha1.ReconfiguringType)
	ops.Spec.Reconfigures = []opsv1alpha1.Reconfigure{{
		ComponentOps: opsv1alpha1.ComponentOps{ComponentName: defaultCompName},
		Parameters: []opsv1alpha1.ParameterPair{{
			Key:   key,
			Value: value,
		}},
	}}
	return testops.CreateOpsRequest(ctx, testCtx, ops)
}

func stableReconfigureFailureStatus(cp *parametersv1alpha1.ComponentParameter, ops *opsv1alpha1.OpsRequest, failureClass, reason, rawMessage string) parametersv1alpha1.ConfigTemplateItemDetailStatus {
	safeMessage := renderSafeReconfigureFailureMessage("mysql-config", reason)
	storedMessage := safeMessage
	if rawMessage != "" {
		storedMessage = rawMessage
	}
	return parametersv1alpha1.ConfigTemplateItemDetailStatus{
		Name:    "mysql-config",
		Phase:   parametersv1alpha1.CFailedPhase,
		Message: pointer.String(storedMessage),
		ReconcileDetail: &parametersv1alpha1.ReconcileDetail{
			ExecResult:                   "Failed",
			ErrMessage:                   storedMessage,
			FailureClass:                 failureClass,
			Reason:                       reason,
			OperationUID:                 string(ops.UID),
			ConfigName:                   "mysql-config",
			TargetConfigHash:             "target-hash",
			ComponentParameterGeneration: cp.Generation,
			AffectedPodCount:             1,
		},
	}
}

func expectNoRawReconfigureLeak(g Gomega, obj any) {
	b, err := json.Marshal(obj)
	g.Expect(err).ShouldNot(HaveOccurred())
	rendered := string(b)
	g.Expect(rendered).ShouldNot(ContainSubstring("password"))
	g.Expect(rendered).ShouldNot(ContainSubstring("token=abc"))
	g.Expect(rendered).ShouldNot(ContainSubstring("SELECT"))
}

func setReconfigureTargetConfigHash(cluster *appsv1.Cluster, targetConfigHash string) {
	component := cluster.Spec.GetComponentByName(defaultCompName)
	Expect(component).ShouldNot(BeNil())
	component.Configs = []appsv1.ClusterComponentConfig{{
		Name:       pointer.String("mysql-config"),
		ConfigHash: pointer.String(targetConfigHash),
	}}
}

func createResolvedComponentWithConfigHash(clusterName, compName, compDefName, targetConfigHash string) *appsv1.Component {
	return testapps.NewComponentFactory(testCtx.DefaultNamespace, clusterName+"-"+compName, compDefName).
		AddLabelsInMap(constant.GetClusterLabels(clusterName)).
		SetConfigs([]appsv1.ClusterComponentConfig{{
			Name:       pointer.String("mysql-config"),
			ConfigHash: pointer.String(targetConfigHash),
		}}).
		Create(&testCtx).
		GetObject()
}
