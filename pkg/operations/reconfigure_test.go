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

package operations

import (
	"encoding/json"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/spf13/cast"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	appsv1beta1 "github.com/apecloud/kubeblocks/apis/apps/v1beta1"
	opsv1alpha1 "github.com/apecloud/kubeblocks/apis/operations/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/configuration/core"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/builder"
	configutil "github.com/apecloud/kubeblocks/pkg/controller/configuration"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	"github.com/apecloud/kubeblocks/pkg/generics"
	opsutil "github.com/apecloud/kubeblocks/pkg/operations/util"
	testapps "github.com/apecloud/kubeblocks/pkg/testutil/apps"
	testops "github.com/apecloud/kubeblocks/pkg/testutil/operations"
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
		// non-namespaced
		testapps.ClearResources(&testCtx, generics.ConfigConstraintSignature, ml)
	}

	BeforeEach(cleanEnv)

	AfterEach(cleanEnv)

	initClusterForOps := func(opsRes *OpsResource) {
		Expect(opsutil.UpdateClusterOpsAnnotations(ctx, k8sClient, opsRes.Cluster, nil)).Should(Succeed())
		opsRes.Cluster.Status.Phase = appsv1.RunningClusterPhase
	}

	assureCfgTplObj := func(tplName, cmName, ns string) (*corev1.ConfigMap, *appsv1beta1.ConfigConstraint) {
		By("Assuring an cm obj")
		cfgCM := testapps.NewCustomizedObj("operations_config/config-template.yaml",
			&corev1.ConfigMap{}, testapps.WithNamespacedName(cmName, ns))
		cfgTpl := testapps.NewCustomizedObj("operations_config/config-constraint.yaml",
			&appsv1beta1.ConfigConstraint{}, testapps.WithNamespacedName(tplName, ns))
		Expect(testCtx.CheckedCreateObj(ctx, cfgCM)).Should(Succeed())
		Expect(testCtx.CheckedCreateObj(ctx, cfgTpl)).Should(Succeed())

		return cfgCM, cfgTpl
	}

	assureConfigInstanceObj := func(clusterName, componentName, ns string, compDef *appsv1.ComponentDefinition) (*appsv1alpha1.Configuration, *corev1.ConfigMap) {
		if len(compDef.Spec.Configs) == 0 {
			return nil, nil
		}

		By("create configuration cr")
		configuration := builder.NewConfigurationBuilder(testCtx.DefaultNamespace, core.GenerateComponentParameterName(clusterName, componentName)).
			ClusterRef(clusterName).
			Component(componentName)
		for _, configSpec := range compDef.Spec.Configs {
			configuration.AddConfigurationItem(configSpec)
		}
		Expect(testCtx.CheckedCreateObj(ctx, configuration.GetObject())).Should(Succeed())

		// update status
		By("update configuration status")
		revision := "1"
		Eventually(testapps.GetAndChangeObjStatus(&testCtx, client.ObjectKeyFromObject(configuration.GetObject()),
			func(config *appsv1alpha1.Configuration) {
				revision = cast.ToString(config.GetGeneration())
				for _, item := range config.Spec.ConfigItemDetails {
					configutil.CheckAndUpdateItemStatus(config, item, revision)
				}
			})).Should(Succeed())

		By("create configmap for configSpecs")
		var cmObj *corev1.ConfigMap
		for _, configSpec := range compDef.Spec.Configs {
			cmInsName := core.GetComponentCfgName(clusterName, componentName, configSpec.Name)
			By("create configmap: " + cmInsName)
			cfgCM := testapps.NewCustomizedObj("operations_config/config-template.yaml",
				&corev1.ConfigMap{},
				testapps.WithNamespacedName(cmInsName, ns),
				testapps.WithLabels(
					constant.AppNameLabelKey, clusterName,
					constant.ConfigurationRevision, revision,
					constant.AppInstanceLabelKey, clusterName,
					constant.KBAppComponentLabelKey, componentName,
					constant.CMConfigurationTemplateNameLabelKey, configSpec.TemplateRef,
					constant.CMConfigurationConstraintsNameLabelKey, configSpec.ConfigConstraintRef,
					constant.CMConfigurationSpecProviderLabelKey, configSpec.Name,
					constant.CMConfigurationTypeLabelKey, constant.ConfigInstanceType,
				),
			)
			Expect(testCtx.CheckedCreateObj(ctx, cfgCM)).Should(Succeed())
			cmObj = cfgCM
		}
		return configuration.GetObject(), cmObj
	}

	assureMockReconfigureData := func(policyName string) (*OpsResource, *appsv1alpha1.Configuration, *corev1.ConfigMap) {
		By("init operations resources ")
		opsRes, compDef, clusterObject := initOperationsResources(compDefName, clusterName)

		By("Test Reconfigure")
		{
			// mock cluster is Running to support reconfiguring ops
			By("mock cluster status")
			patch := client.MergeFrom(clusterObject.DeepCopy())
			clusterObject.Status.Phase = appsv1.RunningClusterPhase
			Expect(k8sClient.Status().Patch(ctx, clusterObject, patch)).Should(Succeed())
		}

		By("mock config tpl")
		cmObj, tplObj := assureCfgTplObj("mysql-tpl-test", "mysql-cm-test", testCtx.DefaultNamespace)

		By("update clusterdefinition tpl")
		patch := client.MergeFrom(compDef.DeepCopy())
		compDef.Spec.Configs = []appsv1.ComponentConfigSpec{{
			ComponentTemplateSpec: appsv1.ComponentTemplateSpec{
				Name:        "mysql-test",
				TemplateRef: cmObj.Name,
				VolumeName:  "mysql-config",
				Namespace:   testCtx.DefaultNamespace,
			},
			ConfigConstraintRef: tplObj.Name,
		}}
		Expect(k8sClient.Patch(ctx, compDef, patch)).Should(Succeed())

		By("mock config cm object")
		config, cfgObj := assureConfigInstanceObj(clusterName, defaultCompName, testCtx.DefaultNamespace, compDef)

		return opsRes, config, cfgObj
	}

	Context("Test Reconfigure", func() {
		It("Test Reconfigure OpsRequest with restart", func() {
			opsRes, configuration, _ := assureMockReconfigureData("simple")
			reqCtx := intctrlutil.RequestCtx{
				Ctx:      testCtx.Ctx,
				Log:      log.FromContext(ctx).WithName("Reconfigure"),
				Recorder: opsRes.Recorder,
			}

			By("mock reconfigure success")
			ops := testops.NewOpsRequestObj("reconfigure-ops-"+randomStr, testCtx.DefaultNamespace,
				clusterName, opsv1alpha1.ReconfiguringType)
			ops.Spec.Reconfigures = []opsv1alpha1.Reconfigure{
				{
					Configurations: []opsv1alpha1.ConfigurationItem{{
						Name: "mysql-test",
						Keys: []opsv1alpha1.ParameterConfig{{
							Key: "my.cnf",
							Parameters: []opsv1alpha1.ParameterPair{
								{
									Key:   "binlog_stmt_cache_size",
									Value: func() *string { v := "4096"; return &v }(),
								},
								{
									Key:   "key",
									Value: func() *string { v := "abcd"; return &v }(),
								},
							},
						}},
					}},
					ComponentOps: opsv1alpha1.ComponentOps{ComponentName: defaultCompName},
				},
			}

			By("Init Reconfiguring opsrequest")
			opsRes.OpsRequest = ops
			Expect(testCtx.CheckedCreateObj(ctx, ops)).Should(Succeed())
			initClusterForOps(opsRes)

			opsManager := GetOpsManager()
			By("init ops phase")
			opsRes.OpsRequest.Status.Phase = opsv1alpha1.OpsPendingPhase
			_, err := opsManager.Do(reqCtx, k8sClient, opsRes)
			Expect(err).ShouldNot(HaveOccurred())

			By("Reconfigure configure")
			_, err = opsManager.Do(reqCtx, k8sClient, opsRes)
			Expect(err).ShouldNot(HaveOccurred())

			Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(opsRes.OpsRequest), opsRes.OpsRequest)).Should(Succeed())
			_, _ = opsManager.Reconcile(reqCtx, k8sClient, opsRes)
			Expect(opsRes.OpsRequest.Status.Phase).Should(Equal(opsv1alpha1.OpsRunningPhase))

			By("mock configuration.status.phase to Finished")
			var item *appsv1alpha1.ConfigurationItemDetail
			Eventually(testapps.GetAndChangeObjStatus(&testCtx, client.ObjectKeyFromObject(configuration),
				func(config *appsv1alpha1.Configuration) {
					item = config.Spec.GetConfigurationItem("mysql-test")
					for i := 0; i < len(config.Status.ConfigurationItemStatus); i++ {
						config.Status.ConfigurationItemStatus[i].Phase = appsv1alpha1.CFinishedPhase
						if config.Status.ConfigurationItemStatus[i].Name == item.Name {
							config.Status.ConfigurationItemStatus[i].ReconcileDetail = &appsv1alpha1.ReconcileDetail{
								Policy:          "simple",
								CurrentRevision: config.Status.ConfigurationItemStatus[i].UpdateRevision,
								SucceedCount:    2,
								ExpectedCount:   2,
							}
						}
					}
				})).Should(Succeed())

			By("mock configmap controller to updated")
			Eventually(testapps.GetAndChangeObj(&testCtx, client.ObjectKey{
				Name:      core.GetComponentCfgName(clusterName, defaultCompName, "mysql-test"),
				Namespace: testCtx.DefaultNamespace},
				func(cm *corev1.ConfigMap) {
					b, err := json.Marshal(item)
					Expect(err).ShouldNot(HaveOccurred())
					if cm.Annotations == nil {
						cm.Annotations = make(map[string]string)
					}
					cm.Annotations[constant.ConfigAppliedVersionAnnotationKey] = string(b)
					b, err = json.Marshal(intctrlutil.Result{
						Phase:      appsv1alpha1.CFinishedPhase,
						Policy:     "simple",
						ExecResult: "none",
					})
					Expect(err).ShouldNot(HaveOccurred())
					cm.Annotations[core.GenerateRevisionPhaseKey("1")] = string(b)
				})).Should(Succeed())

			By("Reconfigure operation success")
			// Expect(reAction.Handle(eventContext, ops.Name, opsv1alpha1.OpsSucceedPhase, nil)).Should(Succeed())
			Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(opsRes.OpsRequest), opsRes.OpsRequest)).Should(Succeed())
			_, _ = opsManager.Reconcile(reqCtx, k8sClient, opsRes)
			Expect(opsRes.OpsRequest.Status.Phase).Should(Equal(opsv1alpha1.OpsSucceedPhase))
		})

		It("Test Reconfigure OpsRequest with autoReload", func() {
			opsRes, _, _ := assureMockReconfigureData("autoReload")
			reqCtx := intctrlutil.RequestCtx{
				Ctx:      testCtx.Ctx,
				Log:      log.FromContext(ctx).WithName("Reconfigure"),
				Recorder: opsRes.Recorder,
			}

			By("mock reconfigure success")
			ops := testops.NewOpsRequestObj("reconfigure-ops-"+randomStr+"-reload", testCtx.DefaultNamespace,
				clusterName, opsv1alpha1.ReconfiguringType)
			ops.Spec.Reconfigures = []opsv1alpha1.Reconfigure{
				{
					Configurations: []opsv1alpha1.ConfigurationItem{{
						Name: "mysql-test",
						Keys: []opsv1alpha1.ParameterConfig{{
							Key: "my.cnf",
							Parameters: []opsv1alpha1.ParameterPair{
								{
									Key:   "binlog_stmt_cache_size",
									Value: func() *string { v := "4096"; return &v }(),
								}},
						}},
					}},
					ComponentOps: opsv1alpha1.ComponentOps{ComponentName: defaultCompName},
				},
			}

			By("Init Reconfiguring opsrequest")
			opsRes.OpsRequest = ops
			Expect(testCtx.CheckedCreateObj(ctx, ops)).Should(Succeed())
			initClusterForOps(opsRes)

			opsManager := GetOpsManager()
			// reAction := reconfigureAction{}
			By("Reconfigure configure")
			opsRes.OpsRequest.Status.Phase = opsv1alpha1.OpsPendingPhase
			_, err := opsManager.Do(reqCtx, k8sClient, opsRes)
			Expect(err).ShouldNot(HaveOccurred())
			Eventually(testops.GetOpsRequestPhase(&testCtx, client.ObjectKeyFromObject(opsRes.OpsRequest))).Should(Equal(opsv1alpha1.OpsCreatingPhase))
			// do reconfigure
			_, err = opsManager.Do(reqCtx, k8sClient, opsRes)
			Expect(err).ShouldNot(HaveOccurred())
			By("configuration Reconcile callback")

			// Expect(reAction.Handle(eventContext, ops.Name, opsv1alpha1.OpsSucceedPhase, nil)).Should(Succeed())
			By("Reconfigure configure")
			_, _ = opsManager.Reconcile(reqCtx, k8sClient, opsRes)
			// mock cluster.status.component.phase to Updating
			mockClusterCompPhase := func(clusterObj *appsv1.Cluster, phase appsv1.ComponentPhase) {
				clusterObject := clusterObj.DeepCopy()
				patch := client.MergeFrom(clusterObject.DeepCopy())
				compStatus := clusterObject.Status.Components[defaultCompName]
				compStatus.Phase = phase
				clusterObject.Status.Components[defaultCompName] = compStatus
				Expect(k8sClient.Status().Patch(ctx, clusterObject, patch)).Should(Succeed())
			}
			mockClusterCompPhase(opsRes.Cluster, appsv1.UpdatingComponentPhase)
			Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(opsRes.Cluster), opsRes.Cluster)).Should(Succeed())

			By("check cluster.status.components[*].phase == Reconfiguring")
			Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(opsRes.OpsRequest), opsRes.OpsRequest)).Should(Succeed())
			Expect(opsRes.Cluster.Status.Components[defaultCompName].Phase).Should(Equal(appsv1.UpdatingComponentPhase)) // appsv1.ReconfiguringPhase
			// TODO: add status condition expect
			_, _ = opsManager.Reconcile(reqCtx, k8sClient, opsRes)
			// mock cluster.status.component.phase to Running
			mockClusterCompPhase(opsRes.Cluster, appsv1.RunningComponentPhase)

			By("check cluster.status.components[*].phase == Running")
			Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(opsRes.Cluster), opsRes.Cluster)).Should(Succeed())
			Expect(opsRes.Cluster.Status.Components[defaultCompName].Phase).Should(Equal(appsv1.RunningComponentPhase))
		})
	})
})
