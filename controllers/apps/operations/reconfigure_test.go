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

package operations

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	cfgcore "github.com/apecloud/kubeblocks/internal/configuration"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
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
		testapps.ClearResources(&testCtx, intctrlutil.OpsRequestSignature, inNS, ml)
		testapps.ClearResources(&testCtx, intctrlutil.ConfigMapSignature, inNS, ml)
		// non-namespaced
		testapps.ClearResources(&testCtx, intctrlutil.ConfigConstraintSignature, ml)
	}

	BeforeEach(cleanEnv)

	AfterEach(cleanEnv)

	assureCfgTplObj := func(tplName, cmName, ns string) (*corev1.ConfigMap, *appsv1alpha1.ConfigConstraint) {
		By("Assuring an cm obj")
		cfgCM := testapps.NewCustomizedObj("operations_config/configcm.yaml",
			&corev1.ConfigMap{}, testapps.WithNamespacedName(cmName, ns))
		cfgTpl := testapps.NewCustomizedObj("operations_config/configtpl.yaml",
			&appsv1alpha1.ConfigConstraint{}, testapps.WithNamespacedName(tplName, ns))
		Expect(testCtx.CheckedCreateObj(ctx, cfgCM)).Should(Succeed())
		Expect(testCtx.CheckedCreateObj(ctx, cfgTpl)).Should(Succeed())

		return cfgCM, cfgTpl
	}

	assureConfigInstanceObj := func(clusterName, componentName, ns string, cdComponent *appsv1alpha1.ClusterComponentDefinition) *corev1.ConfigMap {
		if cdComponent.ConfigSpec == nil {
			return nil
		}
		var cmObj *corev1.ConfigMap
		for _, tpl := range cdComponent.ConfigSpec.ConfigTemplateRefs {
			cmInsName := cfgcore.GetComponentCfgName(clusterName, componentName, tpl.VolumeName)
			cfgCM := testapps.NewCustomizedObj("operations_config/configcm.yaml",
				&corev1.ConfigMap{},
				testapps.WithNamespacedName(cmInsName, ns),
				testapps.WithLabels(
					intctrlutil.AppNameLabelKey, clusterName,
					intctrlutil.AppInstanceLabelKey, clusterName,
					intctrlutil.AppComponentLabelKey, componentName,
					cfgcore.CMConfigurationTplNameLabelKey, tpl.ConfigTplRef,
					cfgcore.CMConfigurationConstraintsNameLabelKey, tpl.ConfigConstraintRef,
					cfgcore.CMConfigurationISVTplLabelKey, tpl.Name,
					cfgcore.CMConfigurationTypeLabelKey, cfgcore.ConfigInstanceType,
				),
			)
			Expect(testCtx.CheckedCreateObj(ctx, cfgCM)).Should(Succeed())
			cmObj = cfgCM
		}
		return cmObj
	}

	Context("Test OpsRequest", func() {
		It("Test Reconfigure OpsRequest", func() {
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
				clusterObject.Status.Phase = appsv1alpha1.RunningPhase
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
					component.ConfigSpec = &appsv1alpha1.ConfigurationSpec{
						ConfigTemplateRefs: []appsv1alpha1.ConfigTemplate{
							{
								Name:                "mysql-test",
								ConfigTplRef:        cmObj.Name,
								ConfigConstraintRef: tplObj.Name,
								VolumeName:          "mysql-config",
								Namespace:           testCtx.DefaultNamespace,
							},
						},
					}
				}

				Expect(k8sClient.Patch(ctx, clusterDef, patch)).Should(Succeed())
				By("mock config cm object")
				cfgObj = assureConfigInstanceObj(clusterName, consensusComp, testCtx.DefaultNamespace, stsComponent)
			}

			By("mock event context")
			eventContext := cfgcore.ConfigEventContext{
				CfgCM:     cfgObj,
				Component: &clusterDef.Spec.ComponentDefs[0],
				Client:    k8sClient,
				ReqCtx: intctrlutil.RequestCtx{
					Ctx:      opsRes.Ctx,
					Log:      log.FromContext(opsRes.Ctx),
					Recorder: opsRes.Recorder,
				},
				Cluster: clusterObject,
				TplName: "mysql-test",
				ConfigPatch: &cfgcore.ConfigPatchInfo{
					AddConfig:    map[string]interface{}{},
					UpdateConfig: map[string][]byte{},
					DeleteConfig: map[string]interface{}{},
				},
				PolicyStatus: cfgcore.PolicyExecStatus{
					PolicyName:    "simple",
					SucceedCount:  2,
					ExpectedCount: 3,
				},
			}

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
								Key:   "x",
								Value: func() *string { v := "abcd"; return &v }(),
							},
						},
					}},
				}},
				ComponentOps: appsv1alpha1.ComponentOps{ComponentName: consensusComp},
			}
			opsRes.OpsRequest = ops
			Expect(testCtx.CheckedCreateObj(ctx, ops)).Should(Succeed())

			reAction := reconfigureAction{}
			Expect(reAction.Action(opsRes)).Should(Succeed())
			Expect(reAction.Handle(eventContext, ops.Name, appsv1alpha1.ReconfiguringPhase, nil)).Should(Succeed())
			Expect(opsRes.Client.Get(opsRes.Ctx, client.ObjectKeyFromObject(opsRes.OpsRequest), opsRes.OpsRequest)).Should(Succeed())
			_, _ = GetOpsManager().Reconcile(opsRes)
			Expect(opsRes.OpsRequest.Status.Phase).Should(BeEquivalentTo(appsv1alpha1.RunningPhase))
			Expect(reAction.Handle(eventContext, ops.Name, appsv1alpha1.SucceedPhase, nil)).Should(Succeed())
			Expect(opsRes.Client.Get(opsRes.Ctx, client.ObjectKeyFromObject(opsRes.OpsRequest), opsRes.OpsRequest)).Should(Succeed())
			_, _ = GetOpsManager().Reconcile(opsRes)
			Expect(opsRes.OpsRequest.Status.Phase).Should(BeEquivalentTo(appsv1alpha1.SucceedPhase))

			// TODO add failed ut
			By("mock reconfigure failed")
		})

	})
})
