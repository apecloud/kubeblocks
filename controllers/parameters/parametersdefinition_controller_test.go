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

package parameters

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	parametersv1alpha1 "github.com/apecloud/kubeblocks/apis/parameters/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/generics"
	testapps "github.com/apecloud/kubeblocks/pkg/testutil/apps"
	testparameters "github.com/apecloud/kubeblocks/pkg/testutil/parameters"
)

var _ = Describe("ConfigConstraint Controller", func() {
	cleanEnv := func() {
		// must wait till resources deleted and no longer existed before the testcases start,
		// otherwise if later it needs to create some new resource objects with the same name,
		// in race conditions, it will find the existence of old objects, resulting failure to
		// create the new objects.
		By("clean resources")

		// delete cluster(and all dependent sub-resources), cluster definition
		testapps.ClearClusterResources(&testCtx)

		// delete rest mocked objects
		inNS := client.InNamespace(testCtx.DefaultNamespace)
		ml := client.HasLabels{testCtx.TestObjLabelKey}
		// non-namespaced
		testapps.ClearResources(&testCtx, intctrlutil.ComponentDefinitionSignature, ml)
		testapps.ClearResources(&testCtx, intctrlutil.ParametersDefinitionSignature, ml)
		// namespaced
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, intctrlutil.ConfigMapSignature, true, inNS, ml)
	}

	BeforeEach(cleanEnv)

	AfterEach(cleanEnv)

	Context("Create config constraint with cue validate", func() {
		It("Should ready", func() {
			By("creating a configmap and a config parametersDef")
			testparameters.NewComponentTemplateFactory(configSpecName,
				testCtx.DefaultNamespace).
				Create(&testCtx)

			parametersDef := testparameters.NewParametersDefinitionFactory("mysql-parameters-8.0").
				StaticParameters([]string{"automatic_sp_privileges"}).
				DynamicParameters([]string{"innodb_autoinc_lock_mode"}).
				Schema(`
#MysqlParameter: {
  // [OFF|ON] default ON
  automatic_sp_privileges: string & "OFF" | "ON" | *"ON"
  // [1~65535] default ON
  auto_increment_increment: int & >= 1 & <= 65535 | *1
  // [4096~16777216] default 2G
  binlog_stmt_cache_size?: int & >= 4096 & <= 16777216 | *2097152
  // [0|1|2] default: 2
  innodb_autoinc_lock_mode?: int & 0 | 1 | 2 | *2
  // other parameters
  // reference mysql parameters
  ...
}
mysqld: #MysqlParameter
// ignore client parameter validate
// mysql client: a set of name/value pairs.
client?: {
  [string]: string
} @protobuf(2,type=map<string,string>)
`).
				Create(&testCtx).
				GetObject()
			parametersDefKey := client.ObjectKeyFromObject(parametersDef)

			By("check ParametersDefinition(template) status and finalizer")
			Eventually(testapps.CheckObj(&testCtx, parametersDefKey,
				func(g Gomega, tpl *parametersv1alpha1.ParametersDefinition) {
					g.Expect(tpl.Status.Phase).To(BeEquivalentTo(parametersv1alpha1.PDAvailablePhase))
					g.Expect(tpl.Finalizers).To(ContainElement(constant.ConfigFinalizerName))
				})).Should(Succeed())

			By("By delete ParametersDefinition")
			Expect(k8sClient.Delete(testCtx.Ctx, parametersDef)).Should(Succeed())

			By("check ParametersDefinition should be deleted")
			Eventually(testapps.CheckObjExists(&testCtx, parametersDefKey, &parametersv1alpha1.ParametersDefinition{}, false), time.Second*60, time.Second*1).Should(Succeed())
		})
	})

	Context("Create config constraint without cue validate", func() {
		It("Should ready", func() {
			By("creating a configmap and a config constraint")

			testparameters.NewComponentTemplateFactory(configSpecName,
				testCtx.DefaultNamespace).
				Create(&testCtx)

			parametersDef := testparameters.NewParametersDefinitionFactory("mysql-parameters-8.0").
				StaticParameters([]string{"automatic_sp_privileges"}).
				DynamicParameters([]string{"innodb_autoinc_lock_mode"}).
				Create(&testCtx).
				GetObject()

			By("check config constraint status")
			Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(parametersDef),
				func(g Gomega, tpl *parametersv1alpha1.ParametersDefinition) {
					g.Expect(tpl.Status.Phase).Should(BeEquivalentTo(parametersv1alpha1.PDAvailablePhase))
				})).Should(Succeed())
		})
	})

	Context("Validate referenced config template", func() {
		It("Should not be available when templateName is not defined in matched ComponentDefinition configs", func() {
			configmap := testparameters.NewComponentTemplateFactory("pd-template-check-cm",
				testCtx.DefaultNamespace).
				Create(&testCtx).
				GetObject()

			compDef := testapps.NewComponentDefinitionFactory("pd-template-check-compdef").
				SetDefaultSpec().
				AddConfigTemplate(configSpecName, configmap.Name, testCtx.DefaultNamespace, configVolumeName, false).
				Create(&testCtx).
				GetObject()
			Expect(testapps.GetAndChangeObjStatus(&testCtx, client.ObjectKeyFromObject(compDef), func(obj *appsv1.ComponentDefinition) {
				obj.Status.Phase = appsv1.AvailablePhase
			})()).Should(Succeed())

			parametersDef := testparameters.NewParametersDefinitionFactory("pd-template-check-params").
				SetComponentDefinition(compDef.Name).
				SetTemplateName("missing-config").
				Create(&testCtx).
				GetObject()

			Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(parametersDef),
				func(g Gomega, tpl *parametersv1alpha1.ParametersDefinition) {
					g.Expect(tpl.Status.Phase).Should(BeEquivalentTo(parametersv1alpha1.PDUnavailablePhase))
				})).Should(Succeed())

			Expect(testapps.GetAndChangeObj(&testCtx, client.ObjectKeyFromObject(parametersDef), func(obj *parametersv1alpha1.ParametersDefinition) {
				obj.Spec.TemplateName = configSpecName
			})()).Should(Succeed())

			Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(parametersDef),
				func(g Gomega, tpl *parametersv1alpha1.ParametersDefinition) {
					g.Expect(tpl.Status.Phase).Should(BeEquivalentTo(parametersv1alpha1.PDAvailablePhase))
				})).Should(Succeed())

			Expect(testapps.GetAndChangeObj(&testCtx, client.ObjectKeyFromObject(compDef), func(obj *appsv1.ComponentDefinition) {
				obj.Spec.Configs = nil
			})()).Should(Succeed())

			Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(parametersDef),
				func(g Gomega, tpl *parametersv1alpha1.ParametersDefinition) {
					g.Expect(tpl.Status.Phase).Should(BeEquivalentTo(parametersv1alpha1.PDUnavailablePhase))
				})).Should(Succeed())
		})

		It("Should not be available when any matched ComponentDefinition misses templateName", func() {
			configmap := testparameters.NewComponentTemplateFactory("pd-template-check-cm2",
				testCtx.DefaultNamespace).
				Create(&testCtx).
				GetObject()

			validCompDef := testapps.NewComponentDefinitionFactory("pd-template-check-valid").
				SetDefaultSpec().
				AddConfigTemplate(configSpecName, configmap.Name, testCtx.DefaultNamespace, configVolumeName, false).
				Create(&testCtx).
				GetObject()
			Expect(testapps.GetAndChangeObjStatus(&testCtx, client.ObjectKeyFromObject(validCompDef), func(obj *appsv1.ComponentDefinition) {
				obj.Status.Phase = appsv1.AvailablePhase
			})()).Should(Succeed())

			missingCompDef := testapps.NewComponentDefinitionFactory("pd-template-check-missing").
				SetDefaultSpec().
				Create(&testCtx).
				GetObject()
			Expect(testapps.GetAndChangeObjStatus(&testCtx, client.ObjectKeyFromObject(missingCompDef), func(obj *appsv1.ComponentDefinition) {
				obj.Status.Phase = appsv1.AvailablePhase
			})()).Should(Succeed())

			parametersDef := testparameters.NewParametersDefinitionFactory("pd-template-check-params2").
				SetComponentDefinition("pd-template-check").
				SetTemplateName(configSpecName).
				Create(&testCtx).
				GetObject()

			Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(parametersDef),
				func(g Gomega, tpl *parametersv1alpha1.ParametersDefinition) {
					g.Expect(tpl.Status.Phase).Should(BeEquivalentTo(parametersv1alpha1.PDUnavailablePhase))
				})).Should(Succeed())
		})

		It("Should ignore ComponentDefinitions with unmatched serviceVersion", func() {
			configmap := testparameters.NewComponentTemplateFactory("pd-template-check-cm3",
				testCtx.DefaultNamespace).
				Create(&testCtx).
				GetObject()

			validCompDef := testapps.NewComponentDefinitionFactory("pd-template-check-v8").
				SetDefaultSpec().
				SetServiceVersion("8.0").
				AddConfigTemplate(configSpecName, configmap.Name, testCtx.DefaultNamespace, configVolumeName, false).
				Create(&testCtx).
				GetObject()
			Expect(testapps.GetAndChangeObjStatus(&testCtx, client.ObjectKeyFromObject(validCompDef), func(obj *appsv1.ComponentDefinition) {
				obj.Status.Phase = appsv1.AvailablePhase
			})()).Should(Succeed())

			unmatchedCompDef := testapps.NewComponentDefinitionFactory("pd-template-check-v9").
				SetDefaultSpec().
				SetServiceVersion("9.0").
				Create(&testCtx).
				GetObject()
			Expect(testapps.GetAndChangeObjStatus(&testCtx, client.ObjectKeyFromObject(unmatchedCompDef), func(obj *appsv1.ComponentDefinition) {
				obj.Status.Phase = appsv1.AvailablePhase
			})()).Should(Succeed())

			parametersDef := testparameters.NewParametersDefinitionFactory("pd-template-check-params3").
				SetComponentDefinition("pd-template-check").
				SetServiceVersion("8.0").
				SetTemplateName(configSpecName).
				Create(&testCtx).
				GetObject()

			Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(parametersDef),
				func(g Gomega, tpl *parametersv1alpha1.ParametersDefinition) {
					g.Expect(tpl.Status.Phase).Should(BeEquivalentTo(parametersv1alpha1.PDAvailablePhase))
				})).Should(Succeed())
		})
	})
})
