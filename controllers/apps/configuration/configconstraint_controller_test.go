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

package configuration

import (
	"time"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/log"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	cfgcore "github.com/apecloud/kubeblocks/internal/configuration"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
	testapps "github.com/apecloud/kubeblocks/internal/testutil/apps"
)

var _ = Describe("ConfigConstraint Controller", func() {
	const clusterDefName = "test-clusterdef"
	const clusterVersionName = "test-clusterversion"

	const statefulCompType = "replicasets"

	const configTplName = "mysql-config-tpl"

	const configVolumeName = "mysql-config"

	cleanEnv := func() {
		// must wait until resources deleted and no longer exist before the testcases start,
		// otherwise if later it needs to create some new resource objects with the same name,
		// in race conditions, it will find the existence of old objects, resulting failure to
		// create the new objects.
		By("clean resources")

		// delete cluster(and all dependent sub-resources), clusterversion and clusterdef
		testapps.ClearClusterResources(&testCtx)

		// delete rest mocked objects
		inNS := client.InNamespace(testCtx.DefaultNamespace)
		ml := client.HasLabels{testCtx.TestObjLabelKey}
		// namespaced
		testapps.ClearResources(&testCtx, intctrlutil.ConfigMapSignature, inNS, ml)
		// non-namespaced
		testapps.ClearResources(&testCtx, intctrlutil.ConfigConstraintSignature, ml)
	}

	BeforeEach(cleanEnv)

	AfterEach(cleanEnv)

	Context("Create config constraint with cue validate", func() {
		It("Should ready", func() {
			By("creating a configmap and a config constraint")

			configmap := testapps.CreateCustomizedObj(&testCtx,
				"resources/mysql_config_cm.yaml", &corev1.ConfigMap{},
				testCtx.UseDefaultNamespace())

			constraint := testapps.CreateCustomizedObj(&testCtx,
				"resources/mysql_config_template.yaml",
				&appsv1alpha1.ConfigConstraint{})
			constraintKey := client.ObjectKeyFromObject(constraint)

			By("Create a clusterDefinition obj")
			clusterDefObj := testapps.NewClusterDefFactory(clusterDefName).
				AddComponent(testapps.StatefulMySQLComponent, statefulCompType).
				AddConfigTemplate(configTplName, configmap.Name, constraint.Name, testCtx.DefaultNamespace, configVolumeName, nil).
				AddLabels(cfgcore.GenerateTPLUniqLabelKeyWithConfig(configTplName), configmap.Name,
					cfgcore.GenerateConstraintsUniqLabelKeyWithConfig(constraint.Name), constraint.Name).
				Create(&testCtx).GetObject()

			By("Create a clusterVersion obj")
			clusterVersionObj := testapps.NewClusterVersionFactory(clusterVersionName, clusterDefObj.GetName()).
				AddComponent(statefulCompType).
				AddLabels(cfgcore.GenerateTPLUniqLabelKeyWithConfig(configTplName), configmap.Name,
					cfgcore.GenerateConstraintsUniqLabelKeyWithConfig(constraint.Name), constraint.Name).
				Create(&testCtx).GetObject()

			By("check ConfigConstraint(template) status and finalizer")
			Eventually(testapps.CheckObj(&testCtx, constraintKey,
				func(g Gomega, tpl *appsv1alpha1.ConfigConstraint) {
					g.Expect(tpl.Status.Phase).To(BeEquivalentTo(appsv1alpha1.AvailablePhase))
					g.Expect(tpl.Finalizers).To(ContainElement(cfgcore.ConfigurationTemplateFinalizerName))
				})).Should(Succeed())

			By("By delete ConfigConstraint")
			Expect(k8sClient.Delete(testCtx.Ctx, constraint)).Should(Succeed())

			By("check ConfigConstraint should not be deleted")
			log.Log.Info("expect that ConfigConstraint is not deleted.")
			Consistently(testapps.CheckObjExists(&testCtx, constraintKey, &appsv1alpha1.ConfigConstraint{}, true)).Should(Succeed())

			By("By delete referencing clusterdefinition and clusterversion")
			Expect(k8sClient.Delete(testCtx.Ctx, clusterVersionObj)).Should(Succeed())
			Expect(k8sClient.Delete(testCtx.Ctx, clusterDefObj)).Should(Succeed())

			By("check ConfigConstraint should be deleted")
			Eventually(testapps.CheckObjExists(&testCtx, constraintKey, &appsv1alpha1.ConfigConstraint{}, false), time.Second*60, time.Second*1).Should(Succeed())
		})
	})

	Context("Create config constraint without cue validate", func() {
		It("Should ready", func() {
			By("creating a configmap and a config constraint")

			_ = testapps.CreateCustomizedObj(&testCtx, "resources/mysql_config_cm.yaml", &corev1.ConfigMap{},
				testCtx.UseDefaultNamespace())

			constraint := testapps.CreateCustomizedObj(&testCtx, "resources/mysql_config_tpl_not_validate.yaml",
				&appsv1alpha1.ConfigConstraint{})

			By("check config constraint status")
			Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(constraint),
				func(g Gomega, tpl *appsv1alpha1.ConfigConstraint) {
					g.Expect(tpl.Status.Phase).Should(BeEquivalentTo(appsv1alpha1.AvailablePhase))
				})).Should(Succeed())
		})
	})
})
