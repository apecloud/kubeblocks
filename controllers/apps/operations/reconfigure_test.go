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

package operations

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	opsutil "github.com/apecloud/kubeblocks/controllers/apps/operations/util"
	cfgcore "github.com/apecloud/kubeblocks/internal/configuration"
	"github.com/apecloud/kubeblocks/internal/constant"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
	"github.com/apecloud/kubeblocks/internal/generics"
	testapps "github.com/apecloud/kubeblocks/internal/testutil/apps"
)

var _ = Describe("Reconfigure OpsRequest", func() {

	var (
		randomStr             = testCtx.GetRandomStr()
		clusterDefinitionName = "cluster-definition-for-ops-" + randomStr
		clusterVersionName    = "clusterversion-for-ops-" + randomStr
		clusterName           = "cluster-for-ops-" + randomStr
	)

	cleanEnv := func() {
		// must wait until resources deleted and no longer exist before the testcases start,
		// otherwise if later it needs to create some new resource objects with the same name,
		// in race conditions, it will find the existence of old objects, resulting failure to
		// create the new objects.
		By("clean resources")

		// delete cluster(and all dependent sub-resources), clusterversion and clusterdef
		testapps.ClearClusterResources(&testCtx)

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
		Expect(opsutil.PatchClusterOpsAnnotations(ctx, k8sClient, opsRes.Cluster, nil)).Should(Succeed())
		opsRes.Cluster.Status.Phase = appsv1alpha1.RunningClusterPhase
	}

	assureCfgTplObj := func(tplName, cmName, ns string) (*corev1.ConfigMap, *appsv1alpha1.ConfigConstraint) {
		By("Assuring an cm obj")
		cfgCM := testapps.NewCustomizedObj("operations_config/config-template.yaml",
			&corev1.ConfigMap{}, testapps.WithNamespacedName(cmName, ns))
		cfgTpl := testapps.NewCustomizedObj("operations_config/config-constraint.yaml",
			&appsv1alpha1.ConfigConstraint{}, testapps.WithNamespacedName(tplName, ns))
		Expect(testCtx.CheckedCreateObj(ctx, cfgCM)).Should(Succeed())
		Expect(testCtx.CheckedCreateObj(ctx, cfgTpl)).Should(Succeed())

		return cfgCM, cfgTpl
	}

	assureConfigInstanceObj := func(clusterName, componentName, ns string, cdComponent *appsv1alpha1.ClusterComponentDefinition) *corev1.ConfigMap {
		if len(cdComponent.ConfigSpecs) == 0 {
			return nil
		}
		var cmObj *corev1.ConfigMap
		for _, configSpec := range cdComponent.ConfigSpecs {
			cmInsName := cfgcore.GetComponentCfgName(clusterName, componentName, configSpec.Name)
			cfgCM := testapps.NewCustomizedObj("operations_config/config-template.yaml",
				&corev1.ConfigMap{},
				testapps.WithNamespacedName(cmInsName, ns),
				testapps.WithLabels(
					constant.AppNameLabelKey, clusterName,
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
		return cmObj
	}

	assureMockReconfigureData := func(policyName string) (*OpsResource, cfgcore.ConfigEventContext) {
		By("init operations resources ")
		opsRes, clusterDef, clusterObject := initOperationsResources(clusterDefinitionName, clusterVersionName, clusterName)

		var (
			cfgObj       *corev1.ConfigMap
			stsComponent *appsv1alpha1.ClusterComponentDefinition
		)
		By("Test Reconfigure")
		{
			// mock cluster is Running to support reconfigure ops
			By("mock cluster status")
			patch := client.MergeFrom(clusterObject.DeepCopy())
			clusterObject.Status.Phase = appsv1alpha1.RunningClusterPhase
			Expect(k8sClient.Status().Patch(ctx, clusterObject, patch)).Should(Succeed())
		}

		{
			By("mock config tpl")
			cmObj, tplObj := assureCfgTplObj("mysql-tpl-test", "mysql-cm-test", testCtx.DefaultNamespace)
			By("update clusterdefinition tpl")
			patch := client.MergeFrom(clusterDef.DeepCopy())
			for i := range clusterDef.Spec.ComponentDefs {
				component := &clusterDef.Spec.ComponentDefs[i]
				if component.Name != consensusComp {
					continue
				}
				stsComponent = component
				component.ConfigSpecs = []appsv1alpha1.ComponentConfigSpec{{
					ComponentTemplateSpec: appsv1alpha1.ComponentTemplateSpec{
						Name:        "mysql-test",
						TemplateRef: cmObj.Name,
						VolumeName:  "mysql-config",
						Namespace:   testCtx.DefaultNamespace,
					},
					ConfigConstraintRef: tplObj.Name,
				}}
			}

			Expect(k8sClient.Patch(ctx, clusterDef, patch)).Should(Succeed())
			By("mock config cm object")
			cfgObj = assureConfigInstanceObj(clusterName, consensusComp, testCtx.DefaultNamespace, stsComponent)
		}

		By("mock event context")
		eventContext := cfgcore.ConfigEventContext{
			ConfigMap: cfgObj,
			Component: &clusterDef.Spec.ComponentDefs[0],
			Client:    k8sClient,
			ReqCtx: intctrlutil.RequestCtx{
				Ctx:      ctx,
				Log:      log.FromContext(ctx),
				Recorder: opsRes.Recorder,
			},
			Cluster:        clusterObject,
			ConfigSpecName: "mysql-test",
			ConfigPatch: &cfgcore.ConfigPatchInfo{
				AddConfig:    map[string]interface{}{},
				UpdateConfig: map[string][]byte{},
				DeleteConfig: map[string]interface{}{},
			},
			PolicyStatus: cfgcore.PolicyExecStatus{
				PolicyName:    policyName,
				SucceedCount:  2,
				ExpectedCount: 3,
			},
		}
		return opsRes, eventContext
	}

	Context("Test OpsRequest", func() {
		It("Test Reconfigure OpsRequest with restart", func() {
			opsRes, eventContext := assureMockReconfigureData("simple")
			reqCtx := intctrlutil.RequestCtx{Ctx: testCtx.Ctx}

			By("mock reconfigure success")
			ops := testapps.NewOpsRequestObj("reconfigure-ops-"+randomStr, testCtx.DefaultNamespace,
				clusterName, appsv1alpha1.ReconfiguringType)
			ops.Spec.Reconfigure = &appsv1alpha1.Reconfigure{
				Configurations: []appsv1alpha1.Configuration{{
					Name: "mysql-test",
					Keys: []appsv1alpha1.ParameterConfig{{
						Key: "my.cnf",
						Parameters: []appsv1alpha1.ParameterPair{
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
				ComponentOps: appsv1alpha1.ComponentOps{ComponentName: consensusComp},
			}

			By("Init Reconfiguring opsrequest")
			opsRes.OpsRequest = ops
			Expect(testCtx.CheckedCreateObj(ctx, ops)).Should(Succeed())
			initClusterForOps(opsRes)

			opsManager := GetOpsManager()
			reAction := reconfigureAction{}
			By("Reconfigure configure")
			Expect(reAction.Action(reqCtx, k8sClient, opsRes)).Should(Succeed())
			By("configuration Reconcile callback")
			Expect(reAction.Handle(eventContext, ops.Name, appsv1alpha1.OpsRunningPhase, nil)).Should(Succeed())
			Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(opsRes.OpsRequest), opsRes.OpsRequest)).Should(Succeed())
			_, _ = opsManager.Reconcile(reqCtx, k8sClient, opsRes)
			Expect(opsRes.OpsRequest.Status.Phase).Should(Equal(appsv1alpha1.OpsRunningPhase))

			By("Validate cluster status")
			_, err := opsManager.Do(reqCtx, k8sClient, opsRes)
			Expect(err).ShouldNot(HaveOccurred())
			// do Action
			_, err = opsManager.Do(reqCtx, k8sClient, opsRes)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(opsRes.Cluster.Status.Phase).Should(Equal(appsv1alpha1.SpecReconcilingClusterPhase))

			By("Reconfigure operation success")
			Expect(reAction.Handle(eventContext, ops.Name, appsv1alpha1.OpsSucceedPhase, nil)).Should(Succeed())
			Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(opsRes.OpsRequest), opsRes.OpsRequest)).Should(Succeed())
			_, _ = opsManager.Reconcile(reqCtx, k8sClient, opsRes)
			Expect(opsRes.OpsRequest.Status.Phase).Should(Equal(appsv1alpha1.OpsSucceedPhase))

		})

		It("Test Reconfigure OpsRequest with autoReload", func() {
			opsRes, eventContext := assureMockReconfigureData("autoReload")
			reqCtx := intctrlutil.RequestCtx{Ctx: testCtx.Ctx}

			By("mock reconfigure success")
			ops := testapps.NewOpsRequestObj("reconfigure-ops-"+randomStr+"-reload", testCtx.DefaultNamespace,
				clusterName, appsv1alpha1.ReconfiguringType)
			ops.Spec.Reconfigure = &appsv1alpha1.Reconfigure{
				Configurations: []appsv1alpha1.Configuration{{
					Name: "mysql-test",
					Keys: []appsv1alpha1.ParameterConfig{{
						Key: "my.cnf",
						Parameters: []appsv1alpha1.ParameterPair{
							{
								Key:   "binlog_stmt_cache_size",
								Value: func() *string { v := "4096"; return &v }(),
							}},
					}},
				}},
				ComponentOps: appsv1alpha1.ComponentOps{ComponentName: consensusComp},
			}

			By("Init Reconfiguring opsrequest")
			opsRes.OpsRequest = ops
			Expect(testCtx.CheckedCreateObj(ctx, ops)).Should(Succeed())
			initClusterForOps(opsRes)

			opsManager := GetOpsManager()
			reAction := reconfigureAction{}
			By("Reconfigure configure")
			_, err := opsManager.Do(reqCtx, k8sClient, opsRes)
			Expect(err).ShouldNot(HaveOccurred())
			Eventually(testapps.GetOpsRequestPhase(&testCtx, client.ObjectKeyFromObject(opsRes.OpsRequest))).Should(Equal(appsv1alpha1.OpsCreatingPhase))
			// do reconfigure
			_, err = opsManager.Do(reqCtx, k8sClient, opsRes)
			Expect(err).ShouldNot(HaveOccurred())
			By("configuration Reconcile callback")

			Expect(reAction.Handle(eventContext, ops.Name, appsv1alpha1.OpsSucceedPhase, nil)).Should(Succeed())
			By("Reconfigure configure")
			_, _ = opsManager.Reconcile(reqCtx, k8sClient, opsRes)
			Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(opsRes.Cluster), opsRes.Cluster)).Should(Succeed())

			By("check cluster.status.components[*].phase == Reconfiguring")
			Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(opsRes.OpsRequest), opsRes.OpsRequest)).Should(Succeed())
			Expect(opsRes.Cluster.Status.Components[consensusComp].Phase).Should(Equal(appsv1alpha1.SpecReconcilingClusterCompPhase)) // appsv1alpha1.ReconfiguringPhase
			// TODO: add status condition expect
			_, _ = opsManager.Reconcile(reqCtx, k8sClient, opsRes)

			By("check cluster.status.components[*].phase == Running")
			Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(opsRes.Cluster), opsRes.Cluster)).Should(Succeed())
			Expect(opsRes.Cluster.Status.Components[consensusComp].Phase).Should(Equal(appsv1alpha1.RunningClusterCompPhase))
		})

	})
})
