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
	"strconv"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
		It("persists an explicit parameter-removal intent", func() {
			componentParameter := builder.NewComponentParameterBuilder(testCtx.DefaultNamespace,
				"nil-assignment-"+randomStr).GetObject()
			componentParameter.Spec.Desired = &parametersv1alpha1.ParameterInputs{
				Updates: []parametersv1alpha1.ParameterUpdate{{
					Type: parametersv1alpha1.ParameterUpdateRemove,
					Key:  "expire_logs_days",
				}},
			}

			Expect(testCtx.CreateObj(ctx, componentParameter)).Should(Succeed())
			fetched := &parametersv1alpha1.ComponentParameter{}
			Expect(testCtx.Cli.Get(ctx, client.ObjectKeyFromObject(componentParameter), fetched)).Should(Succeed())
			Expect(fetched.Spec.Desired.Updates).Should(Equal([]parametersv1alpha1.ParameterUpdate{{
				Type: parametersv1alpha1.ParameterUpdateRemove,
				Key:  "expire_logs_days",
			}}))
		})

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
			Expect(testCtx.Cli.Get(ctx, client.ObjectKeyFromObject(opsRes.OpsRequest), opsRes.OpsRequest)).Should(Succeed())
			Expect(opsRes.OpsRequest.Status.LastConfiguration.Components[defaultCompName].Parameters).Should(Equal(
				[]opsv1alpha1.LastParameterAssignment{{Key: "max_connections", Present: false}}))
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
				g.Expect(cp.Annotations).Should(HaveKeyWithValue(constant.OpsRequestUIDAnnotationKey, string(opsRes.OpsRequest.UID)))
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

		It("persists a nil Ops value as an explicit removal and snapshots H0", func() {
			reqCtx := intctrlutil.RequestCtx{Ctx: ctx}
			opsRes, _, _ := initOperationsResources(compDefName, clusterName)
			componentParameter := builder.NewComponentParameterBuilder(testCtx.DefaultNamespace,
				parameterscore.GenerateComponentConfigurationName(clusterName, defaultCompName)).
				SetClusterName(clusterName).
				SetCompName(defaultCompName).
				GetObject()
			componentParameter.Spec.Desired = &parametersv1alpha1.ParameterInputs{
				Assignments: map[string]*string{"expire_logs_days": pointer.String("7")},
			}
			Expect(testCtx.CreateObj(ctx, componentParameter)).Should(Succeed())

			ops := testops.NewOpsRequestObj("remove-parameter-"+randomStr, testCtx.DefaultNamespace,
				clusterName, opsv1alpha1.ReconfiguringType)
			ops.Spec.Reconfigures = []opsv1alpha1.Reconfigure{{
				ComponentOps: opsv1alpha1.ComponentOps{ComponentName: defaultCompName},
				Parameters:   []opsv1alpha1.ParameterPair{{Key: "expire_logs_days"}},
			}}
			opsRes.OpsRequest = testops.CreateOpsRequest(ctx, testCtx, ops)
			Expect(opsutil.UpdateClusterOpsAnnotations(ctx, k8sClient, opsRes.Cluster, nil)).Should(Succeed())

			opsRes.OpsRequest.Status.Phase = opsv1alpha1.OpsPendingPhase
			_, err := GetOpsManager().Do(reqCtx, k8sClient, opsRes)
			Expect(err).ShouldNot(HaveOccurred())
			Eventually(testops.GetOpsRequestPhase(&testCtx, client.ObjectKeyFromObject(opsRes.OpsRequest))).
				Should(Equal(opsv1alpha1.OpsCreatingPhase))
			Expect(testCtx.Cli.Get(ctx, client.ObjectKeyFromObject(opsRes.OpsRequest), opsRes.OpsRequest)).Should(Succeed())
			Expect(opsRes.OpsRequest.Status.LastConfiguration.Components[defaultCompName].Parameters).
				Should(Equal([]opsv1alpha1.LastParameterAssignment{{
					Key: "expire_logs_days", Present: true, Value: pointer.String("7"),
				}}))

			_, err = GetOpsManager().Do(reqCtx, k8sClient, opsRes)
			Expect(err).ShouldNot(HaveOccurred())
			Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(componentParameter), func(g Gomega,
				fetched *parametersv1alpha1.ComponentParameter) {
				g.Expect(fetched.Spec.Desired.Assignments).ShouldNot(HaveKey("expire_logs_days"))
				g.Expect(fetched.Spec.Desired.Updates).Should(ContainElement(parametersv1alpha1.ParameterUpdate{
					Type: parametersv1alpha1.ParameterUpdateRemove,
					Key:  "expire_logs_days",
				}))
				value, present := componentParameterAssignments(fetched)["expire_logs_days"]
				g.Expect(present).Should(BeTrue())
				g.Expect(value).Should(BeNil())
			})).Should(Succeed())
		})

		It("preflights every target before writing H1", func() {
			reqCtx := intctrlutil.RequestCtx{Ctx: ctx}
			opsRes, _, _ := initOperationsResourcesWithTopology("test-clusterdef-"+randomStr,
				compDefName, clusterName)
			newComponentParameter := func(componentName, value string) *parametersv1alpha1.ComponentParameter {
				componentParameter := builder.NewComponentParameterBuilder(testCtx.DefaultNamespace,
					parameterscore.GenerateComponentConfigurationName(clusterName, componentName)).
					SetClusterName(clusterName).
					SetCompName(componentName).
					GetObject()
				componentParameter.Spec.Desired = &parametersv1alpha1.ParameterInputs{
					Assignments: map[string]*string{"max_connections": pointer.String(value)},
				}
				Expect(testCtx.CreateObj(ctx, componentParameter)).Should(Succeed())
				return componentParameter
			}
			first := newComponentParameter(defaultCompName, "100")
			_ = newComponentParameter(secondaryCompName, "300")

			opsRes.OpsRequest = &opsv1alpha1.OpsRequest{
				ObjectMeta: metav1.ObjectMeta{Name: "preflight-" + randomStr, UID: "ops-uid"},
				Spec: opsv1alpha1.OpsRequestSpec{SpecificOpsRequest: opsv1alpha1.SpecificOpsRequest{
					Reconfigures: []opsv1alpha1.Reconfigure{
						{ComponentOps: opsv1alpha1.ComponentOps{ComponentName: defaultCompName},
							Parameters: []opsv1alpha1.ParameterPair{{Key: "max_connections", Value: pointer.String("200")}}},
						{ComponentOps: opsv1alpha1.ComponentOps{ComponentName: secondaryCompName},
							Parameters: []opsv1alpha1.ParameterPair{{Key: "max_connections", Value: pointer.String("200")}}},
					},
				}},
				Status: opsv1alpha1.OpsRequestStatus{LastConfiguration: opsv1alpha1.LastConfiguration{
					Components: map[string]opsv1alpha1.LastComponentConfiguration{
						defaultCompName: {Parameters: []opsv1alpha1.LastParameterAssignment{{
							Key: "max_connections", Present: true, Value: pointer.String("100"),
						}}},
						secondaryCompName: {Parameters: []opsv1alpha1.LastParameterAssignment{{
							Key: "max_connections", Present: true, Value: pointer.String("100"),
						}}},
					},
				}},
			}

			err := (&reconfigureAction{}).Action(reqCtx, k8sClient, opsRes)
			Expect(intctrlutil.IsTargetError(err, intctrlutil.ErrorTypeFatal)).Should(BeTrue())
			fetched := &parametersv1alpha1.ComponentParameter{}
			Expect(testCtx.Cli.Get(ctx, client.ObjectKeyFromObject(first), fetched)).Should(Succeed())
			Expect(fetched.Spec.Desired.Assignments).Should(HaveKeyWithValue("max_connections", pointer.String("100")))
			Expect(fetched.Annotations).ShouldNot(HaveKeyWithValue(
				constant.OpsRequestUIDAnnotationKey, string(opsRes.OpsRequest.UID)))
		})

		It("fails with manual cleanup for an unclassified ComponentParameter merge failure", func() {
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
				g.Expect(fetched.Status.ReconfigureRollback).ShouldNot(BeNil())
				g.Expect(fetched.Status.ReconfigureRollback.Phase).Should(Equal(opsv1alpha1.ReconfigureManualCleanupRequired))
				condition := meta.FindStatusCondition(fetched.Status.Conditions, opsv1alpha1.ConditionTypeFailed)
				g.Expect(condition).ShouldNot(BeNil())
				g.Expect(condition.Message).Should(ContainSubstring(reconfigureManualMessage))
			})).Should(Succeed())
		})

		It("terminalizes a timed-out automatic rollback as manual cleanup instead of Aborted", func() {
			reqCtx := intctrlutil.RequestCtx{Ctx: ctx}
			opsRes, _, _ := initOperationsResources(compDefName, clusterName)

			componentParameter := builder.NewComponentParameterBuilder(testCtx.DefaultNamespace, parameterscore.GenerateComponentConfigurationName(clusterName, defaultCompName)).
				AddLabelsInMap(constant.GetCompLabelsWithDef(clusterName, defaultCompName, compDefName)).
				SetClusterName(clusterName).
				SetCompName(defaultCompName).
				GetObject()
			componentParameter.Spec.Desired = &parametersv1alpha1.ParameterInputs{
				Assignments: map[string]*string{"maxmemory-samples": pointer.String("0")},
			}
			componentParameter.Spec.ConfigItemDetails = []parametersv1alpha1.ConfigTemplateItemDetail{{Name: "mysql-config"}}
			Expect(testCtx.CreateObj(ctx, componentParameter)).Should(Succeed())

			timeoutSeconds := int32(1)
			ops := testops.NewOpsRequestObj("timed-out-rollback-"+randomStr, testCtx.DefaultNamespace,
				clusterName, opsv1alpha1.ReconfiguringType)
			ops.Spec.TimeoutSeconds = &timeoutSeconds
			ops.Spec.Reconfigures = []opsv1alpha1.Reconfigure{{
				ComponentOps: opsv1alpha1.ComponentOps{ComponentName: defaultCompName},
				Parameters: []opsv1alpha1.ParameterPair{{
					Key:   "maxmemory-samples",
					Value: pointer.String("0"),
				}},
			}}
			opsRes.OpsRequest = testops.CreateOpsRequest(ctx, testCtx, ops)

			Expect(testapps.GetAndChangeObj(&testCtx, client.ObjectKeyFromObject(componentParameter), func(cp *parametersv1alpha1.ComponentParameter) {
				if cp.Annotations == nil {
					cp.Annotations = map[string]string{}
				}
				cp.Annotations[constant.OpsRequestUIDAnnotationKey] = string(opsRes.OpsRequest.UID)
			})()).Should(Succeed())
			Expect(testapps.GetAndChangeObjStatus(&testCtx, client.ObjectKeyFromObject(componentParameter), func(cp *parametersv1alpha1.ComponentParameter) {
				revision := strconv.FormatInt(cp.Generation, 10)
				cp.Status.ObservedGeneration = cp.Generation
				cp.Status.Phase = parametersv1alpha1.CMergeFailedPhase
				cp.Status.ConfigurationItemStatus = []parametersv1alpha1.ConfigTemplateItemDetailStatus{{
					Name:           "mysql-config",
					Phase:          parametersv1alpha1.CMergeFailedPhase,
					UpdateRevision: revision,
					ReconcileDetail: &parametersv1alpha1.ReconcileDetail{
						CurrentRevision: revision,
						Code:            invalidParameterActionResultCode,
						Retryable:       pointer.Bool(false),
						SucceedCount:    0,
						ExpectedCount:   1,
					},
				}}
			})()).Should(Succeed())
			Expect(testapps.GetAndChangeObjStatus(&testCtx, client.ObjectKeyFromObject(opsRes.OpsRequest), func(fetched *opsv1alpha1.OpsRequest) {
				fetched.Status.Phase = opsv1alpha1.OpsRunningPhase
				fetched.Status.StartTimestamp = metav1.NewTime(time.Now().Add(-time.Minute))
				fetched.Status.LastConfiguration.Components = map[string]opsv1alpha1.LastComponentConfiguration{
					defaultCompName: {Parameters: []opsv1alpha1.LastParameterAssignment{{
						Key: "maxmemory-samples", Present: true, Value: pointer.String("5"),
					}}},
				}
			})()).Should(Succeed())

			Expect(testCtx.Cli.Get(ctx, client.ObjectKeyFromObject(opsRes.OpsRequest), opsRes.OpsRequest)).Should(Succeed())
			Expect(testCtx.Cli.Get(ctx, client.ObjectKeyFromObject(opsRes.Cluster), opsRes.Cluster)).Should(Succeed())
			_, err := GetOpsManager().Reconcile(reqCtx, k8sClient, opsRes)
			Expect(err).ShouldNot(HaveOccurred())

			Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(opsRes.OpsRequest), func(g Gomega, fetched *opsv1alpha1.OpsRequest) {
				g.Expect(fetched.Status.Phase).Should(Equal(opsv1alpha1.OpsFailedPhase))
				g.Expect(fetched.Status.ReconfigureRollback).ShouldNot(BeNil())
				g.Expect(fetched.Status.ReconfigureRollback.Phase).Should(Equal(opsv1alpha1.ReconfigureManualCleanupRequired))
				g.Expect(fetched.Status.ReconfigureRollback.Message).Should(ContainSubstring("timeout"))
			})).Should(Succeed())
		})

		It("does not overwrite a concurrent ComponentParameter edit during rollback", func() {
			reqCtx := intctrlutil.RequestCtx{Ctx: ctx}
			opsRes, _, _ := initOperationsResources(compDefName, clusterName)
			opsRes.OpsRequest = &opsv1alpha1.OpsRequest{
				ObjectMeta: metav1.ObjectMeta{UID: "ops-uid"},
				Status: opsv1alpha1.OpsRequestStatus{LastConfiguration: opsv1alpha1.LastConfiguration{
					Components: map[string]opsv1alpha1.LastComponentConfiguration{
						defaultCompName: {Parameters: []opsv1alpha1.LastParameterAssignment{{
							Key: "maxmemory-samples", Present: true, Value: pointer.String("5"),
						}}},
					},
				}},
			}

			componentParameter := builder.NewComponentParameterBuilder(testCtx.DefaultNamespace,
				parameterscore.GenerateComponentConfigurationName(clusterName, defaultCompName)).
				SetClusterName(clusterName).
				SetCompName(defaultCompName).
				GetObject()
			componentParameter.Annotations = map[string]string{
				constant.OpsRequestUIDAnnotationKey: string(opsRes.OpsRequest.UID),
			}
			componentParameter.Spec.Desired = &parametersv1alpha1.ParameterInputs{
				Assignments: map[string]*string{"maxmemory-samples": pointer.String("0")},
			}
			componentParameter.Spec.ConfigItemDetails = []parametersv1alpha1.ConfigTemplateItemDetail{{Name: "mysql-config"}}
			Expect(testCtx.CreateObj(ctx, componentParameter)).Should(Succeed())
			stale := componentParameter.DeepCopy()

			failureGate := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{
				Namespace: testCtx.DefaultNamespace,
				Name:      parameterscore.GetComponentCfgName(clusterName, defaultCompName, "mysql-config"),
				Annotations: map[string]string{
					constant.DisableUpgradeInsConfigurationAnnotationKey: "true",
				},
			}}
			Expect(testCtx.CreateObj(ctx, failureGate)).Should(Succeed())

			current := &parametersv1alpha1.ComponentParameter{}
			Expect(testCtx.Cli.Get(ctx, client.ObjectKeyFromObject(componentParameter), current)).Should(Succeed())
			current.Spec.Desired.Assignments["maxmemory-samples"] = pointer.String("7")
			Expect(testCtx.Cli.Update(ctx, current)).Should(Succeed())

			_, _, err := (&reconfigureAction{}).restoreParameterSnapshots(reqCtx, k8sClient, opsRes, []reconfigureTarget{{
				requestName: defaultCompName,
				component:   defaultCompName,
				reconfigure: opsv1alpha1.Reconfigure{Parameters: []opsv1alpha1.ParameterPair{{
					Key: "maxmemory-samples", Value: pointer.String("0"),
				}}},
				parameter: stale,
			}})
			Expect(apierrors.IsConflict(err)).Should(BeTrue())

			Expect(testCtx.Cli.Get(ctx, client.ObjectKeyFromObject(componentParameter), current)).Should(Succeed())
			Expect(current.Spec.Desired.Assignments).Should(HaveKeyWithValue("maxmemory-samples", pointer.String("7")))
			Expect(testCtx.Cli.Get(ctx, client.ObjectKeyFromObject(failureGate), failureGate)).Should(Succeed())
			Expect(failureGate.Annotations).Should(HaveKeyWithValue(
				constant.DisableUpgradeInsConfigurationAnnotationKey, "true"),
				"the failure gate must remain closed until every ComponentParameter is restored")
		})

		It("rolls back a normalized invalid parameter and restarts an applied component", func() {
			By("init operations resources and the previous desired parameter state")
			reqCtx := intctrlutil.RequestCtx{Ctx: ctx}
			opsRes, _, _ := initOperationsResources(compDefName, clusterName)

			componentParameter := builder.NewComponentParameterBuilder(testCtx.DefaultNamespace, parameterscore.GenerateComponentConfigurationName(clusterName, defaultCompName)).
				AddLabelsInMap(constant.GetCompLabelsWithDef(clusterName, defaultCompName, compDefName)).
				SetClusterName(clusterName).
				SetCompName(defaultCompName).
				GetObject()
			componentParameter.Spec.Desired = &parametersv1alpha1.ParameterInputs{
				Assignments: map[string]*string{"maxmemory-samples": pointer.String("5")},
			}
			componentParameter.Spec.ConfigItemDetails = []parametersv1alpha1.ConfigTemplateItemDetail{{Name: "mysql-config"}}
			Expect(testCtx.CreateObj(ctx, componentParameter)).Should(Succeed())
			Expect(testapps.GetAndChangeObjStatus(&testCtx, client.ObjectKeyFromObject(componentParameter), func(cp *parametersv1alpha1.ComponentParameter) {
				cp.Status.ObservedGeneration = cp.Generation
				cp.Status.Phase = parametersv1alpha1.CFinishedPhase
				cp.Status.ConfigurationItemStatus = []parametersv1alpha1.ConfigTemplateItemDetailStatus{{
					Name:  "mysql-config",
					Phase: parametersv1alpha1.CFinishedPhase,
				}}
			})()).Should(Succeed())

			configMap := &corev1.ConfigMap{}
			configMap.Name = parameterscore.GetComponentCfgName(clusterName, defaultCompName, "mysql-config")
			configMap.Namespace = testCtx.DefaultNamespace
			configMap.Labels = constant.GetCompLabels(clusterName, defaultCompName)
			configMap.Annotations = map[string]string{constant.DisableUpgradeInsConfigurationAnnotationKey: "true"}
			Expect(testCtx.CreateObj(ctx, configMap)).Should(Succeed())

			By("write H1 through the OpsRequest path")
			ops := testops.NewOpsRequestObj("rollback-reconfigure-"+randomStr, testCtx.DefaultNamespace,
				clusterName, opsv1alpha1.ReconfiguringType)
			ops.Spec.Reconfigures = []opsv1alpha1.Reconfigure{{
				ComponentOps: opsv1alpha1.ComponentOps{ComponentName: defaultCompName},
				Parameters: []opsv1alpha1.ParameterPair{{
					Key:   "maxmemory-samples",
					Value: pointer.String("0"),
				}},
			}}
			opsRes.OpsRequest = testops.CreateOpsRequest(ctx, testCtx, ops)
			Expect(opsutil.UpdateClusterOpsAnnotations(ctx, k8sClient, opsRes.Cluster, nil)).Should(Succeed())
			opsRes.OpsRequest.Status.Phase = opsv1alpha1.OpsPendingPhase
			_, err := GetOpsManager().Do(reqCtx, k8sClient, opsRes)
			Expect(err).ShouldNot(HaveOccurred())
			Eventually(testops.GetOpsRequestPhase(&testCtx, client.ObjectKeyFromObject(opsRes.OpsRequest))).Should(Equal(opsv1alpha1.OpsCreatingPhase))
			_, err = GetOpsManager().Do(reqCtx, k8sClient, opsRes)
			Expect(err).ShouldNot(HaveOccurred())
			Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(componentParameter), func(g Gomega, cp *parametersv1alpha1.ComponentParameter) {
				g.Expect(cp.Spec.Desired.Assignments).Should(HaveKeyWithValue("maxmemory-samples", pointer.String("0")))
				g.Expect(cp.Annotations).Should(HaveKeyWithValue(constant.OpsRequestUIDAnnotationKey, string(opsRes.OpsRequest.UID)))
			})).Should(Succeed())

			By("publish the exact non-retryable InvalidParameter result after one pod applied H1")
			Expect(testapps.GetAndChangeObjStatus(&testCtx, client.ObjectKeyFromObject(componentParameter), func(cp *parametersv1alpha1.ComponentParameter) {
				revision := strconv.FormatInt(cp.Generation, 10)
				cp.Status.ObservedGeneration = cp.Generation
				cp.Status.Phase = parametersv1alpha1.CMergeFailedPhase
				cp.Status.Message = "maxmemory-samples=0 was rejected"
				cp.Status.ConfigurationItemStatus = []parametersv1alpha1.ConfigTemplateItemDetailStatus{{
					Name:           "mysql-config",
					Phase:          parametersv1alpha1.CMergeFailedPhase,
					UpdateRevision: revision,
					ReconcileDetail: &parametersv1alpha1.ReconcileDetail{
						CurrentRevision: revision,
						Code:            invalidParameterActionResultCode,
						Retryable:       pointer.Bool(false),
						SucceedCount:    1,
						ExpectedCount:   3,
					},
				}}
			})()).Should(Succeed())
			Expect(testCtx.Cli.Get(ctx, client.ObjectKeyFromObject(opsRes.OpsRequest), opsRes.OpsRequest)).Should(Succeed())
			_, err = GetOpsManager().Reconcile(reqCtx, k8sClient, opsRes)
			Expect(err).ShouldNot(HaveOccurred())
			Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(opsRes.OpsRequest), func(g Gomega, fetched *opsv1alpha1.OpsRequest) {
				g.Expect(fetched.Status.Phase).Should(Equal(opsv1alpha1.OpsRunningPhase))
				g.Expect(fetched.Status.ReconfigureRollback).ShouldNot(BeNil())
				g.Expect(fetched.Status.ReconfigureRollback.Phase).Should(Equal(opsv1alpha1.ReconfigureRollbackPending))
				g.Expect(fetched.Status.ReconfigureRollback.RestartRequired).Should(BeTrue())
			})).Should(Succeed())

			By("restore H0 while keeping the parameter-controller failure gate closed")
			Expect(testCtx.Cli.Get(ctx, client.ObjectKeyFromObject(opsRes.OpsRequest), opsRes.OpsRequest)).Should(Succeed())
			_, err = GetOpsManager().Reconcile(reqCtx, k8sClient, opsRes)
			Expect(err).ShouldNot(HaveOccurred())
			var rollbackGeneration int64
			Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(componentParameter), func(g Gomega, cp *parametersv1alpha1.ComponentParameter) {
				g.Expect(cp.Spec.Desired.Assignments).Should(HaveKeyWithValue("maxmemory-samples", pointer.String("5")))
				rollbackGeneration = cp.Generation
			})).Should(Succeed())
			Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(configMap), func(g Gomega, fetched *corev1.ConfigMap) {
				g.Expect(fetched.Annotations).Should(HaveKeyWithValue(constant.DisableUpgradeInsConfigurationAnnotationKey, "true"))
			})).Should(Succeed())
			Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(opsRes.OpsRequest), func(g Gomega, fetched *opsv1alpha1.OpsRequest) {
				g.Expect(fetched.Status.ReconfigureRollback.Phase).Should(Equal(opsv1alpha1.ReconfigureRollingBack))
			})).Should(Succeed())

			By("keep the failure gate closed until the ConfigMap carries the H0 revision")
			Expect(testCtx.Cli.Get(ctx, client.ObjectKeyFromObject(opsRes.OpsRequest), opsRes.OpsRequest)).Should(Succeed())
			_, err = GetOpsManager().Reconcile(reqCtx, k8sClient, opsRes)
			Expect(err).ShouldNot(HaveOccurred())
			Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(configMap), func(g Gomega, fetched *corev1.ConfigMap) {
				g.Expect(fetched.Annotations).Should(HaveKeyWithValue(constant.DisableUpgradeInsConfigurationAnnotationKey, "true"))
			})).Should(Succeed())

			By("publish the H0 ConfigMap revision and only then reopen reconfigure")
			Expect(testapps.GetAndChangeObj(&testCtx, client.ObjectKeyFromObject(configMap), func(fetched *corev1.ConfigMap) {
				fetched.Annotations[constant.ConfigurationRevision] = strconv.FormatInt(rollbackGeneration, 10)
			})()).Should(Succeed())
			Expect(testCtx.Cli.Get(ctx, client.ObjectKeyFromObject(opsRes.OpsRequest), opsRes.OpsRequest)).Should(Succeed())
			_, err = GetOpsManager().Reconcile(reqCtx, k8sClient, opsRes)
			Expect(err).ShouldNot(HaveOccurred())
			Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(configMap), func(g Gomega, fetched *corev1.ConfigMap) {
				g.Expect(fetched.Annotations).ShouldNot(HaveKey(constant.DisableUpgradeInsConfigurationAnnotationKey))
			})).Should(Succeed())

			By("observe H0 convergence and persist one controlled-restart timestamp")
			Expect(testapps.GetAndChangeObjStatus(&testCtx, client.ObjectKeyFromObject(componentParameter), func(cp *parametersv1alpha1.ComponentParameter) {
				cp.Status.ObservedGeneration = cp.Generation
				cp.Status.Phase = parametersv1alpha1.CFinishedPhase
				cp.Status.ConfigurationItemStatus = []parametersv1alpha1.ConfigTemplateItemDetailStatus{{
					Name:  "mysql-config",
					Phase: parametersv1alpha1.CFinishedPhase,
				}}
			})()).Should(Succeed())
			Expect(testCtx.Cli.Get(ctx, client.ObjectKeyFromObject(opsRes.OpsRequest), opsRes.OpsRequest)).Should(Succeed())
			_, err = GetOpsManager().Reconcile(reqCtx, k8sClient, opsRes)
			Expect(err).ShouldNot(HaveOccurred())
			Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(opsRes.OpsRequest), func(g Gomega, fetched *opsv1alpha1.OpsRequest) {
				g.Expect(fetched.Status.ReconfigureRollback.Phase).Should(Equal(opsv1alpha1.ReconfigureRestartPending))
				g.Expect(fetched.Status.ReconfigureRollback.RestartAt).ShouldNot(BeNil())
			})).Should(Succeed())

			By("write the stable restart intent and wait for the Cluster to observe it")
			Expect(testCtx.Cli.Get(ctx, client.ObjectKeyFromObject(opsRes.OpsRequest), opsRes.OpsRequest)).Should(Succeed())
			restartAt := opsRes.OpsRequest.Status.ReconfigureRollback.RestartAt.Time.UTC().Format("2006-01-02T15:04:05Z07:00")
			_, err = GetOpsManager().Reconcile(reqCtx, k8sClient, opsRes)
			Expect(err).ShouldNot(HaveOccurred())
			Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(opsRes.Cluster), func(g Gomega, cluster *appsv1.Cluster) {
				g.Expect(cluster.Spec.GetComponentByName(defaultCompName).Annotations).Should(HaveKeyWithValue(constant.RestartAnnotationKey, restartAt))
			})).Should(Succeed())
			Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(opsRes.OpsRequest), func(g Gomega, fetched *opsv1alpha1.OpsRequest) {
				g.Expect(fetched.Status.ReconfigureRollback.Phase).Should(Equal(opsv1alpha1.ReconfigureRestarting))
			})).Should(Succeed())

			Expect(testapps.GetAndChangeObjStatus(&testCtx, client.ObjectKeyFromObject(opsRes.Cluster), func(cluster *appsv1.Cluster) {
				cluster.Status.ObservedGeneration = cluster.Generation
				cluster.Status.Phase = appsv1.RunningClusterPhase
				cluster.Status.Components[defaultCompName] = appsv1.ClusterComponentStatus{
					Phase:    appsv1.RunningComponentPhase,
					UpToDate: true,
				}
			})()).Should(Succeed())
			Expect(testCtx.Cli.Get(ctx, client.ObjectKeyFromObject(opsRes.Cluster), opsRes.Cluster)).Should(Succeed())
			Expect(testCtx.Cli.Get(ctx, client.ObjectKeyFromObject(opsRes.OpsRequest), opsRes.OpsRequest)).Should(Succeed())
			_, err = GetOpsManager().Reconcile(reqCtx, k8sClient, opsRes)
			Expect(err).ShouldNot(HaveOccurred())
			Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(opsRes.OpsRequest), func(g Gomega, fetched *opsv1alpha1.OpsRequest) {
				g.Expect(fetched.Status.Phase).Should(Equal(opsv1alpha1.OpsFailedPhase))
				g.Expect(fetched.Status.ReconfigureRollback.Phase).Should(Equal(opsv1alpha1.ReconfigureRolledBack))
				condition := meta.FindStatusCondition(fetched.Status.Conditions, opsv1alpha1.ConditionTypeFailed)
				g.Expect(condition).ShouldNot(BeNil())
				g.Expect(condition.Message).Should(ContainSubstring(reconfigureRolledBackMessage))
			})).Should(Succeed())
		})
	})
})
