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

package configuration

import (
	"time"

	cfgcore "github.com/apecloud/kubeblocks/pkg/configuration/core"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/generics"
	"github.com/apecloud/kubeblocks/pkg/testutil/apps"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
)

var _ = Describe("ConfigConstraint Controller", func() {
	const clusterDefName = "test-clusterdef"
	const clusterVersionName = "test-clusterversion"
	const statefulCompDefName = "replicasets"
	const configSpecName = "mysql-config-tpl"
	const configVolumeName = "mysql-config"

	cleanEnv := func() {
		// must wait till resources deleted and no longer existed before the testcases start,
		// otherwise if later it needs to create some new resource objects with the same name,
		// in race conditions, it will find the existence of old objects, resulting failure to
		// create the new objects.
		By("clean resources")

		// delete cluster(and all dependent sub-resources), clusterversion and clusterdef
		apps.ClearClusterResources(&testCtx)

		// delete rest mocked objects
		inNS := client.InNamespace(testCtx.DefaultNamespace)
		ml := client.HasLabels{testCtx.TestObjLabelKey}
		// non-namespaced
		apps.ClearResources(&testCtx, intctrlutil.ConfigConstraintSignature, ml)
		// namespaced
		apps.ClearResourcesWithRemoveFinalizerOption(&testCtx, intctrlutil.ConfigMapSignature, true, inNS, ml)
	}

	BeforeEach(cleanEnv)

	AfterEach(cleanEnv)

	Context("Create config constraint with cue validate", func() {
		It("Should ready", func() {
			By("creating a configmap and a config constraint")

			configmap := apps.CreateCustomizedObj(&testCtx,
				"resources/mysql-config-template.yaml", &corev1.ConfigMap{},
				testCtx.UseDefaultNamespace())

			constraint := apps.CreateCustomizedObj(&testCtx,
				"resources/mysql-config-constraint.yaml",
				&appsv1alpha1.ConfigConstraint{})
			constraintKey := client.ObjectKeyFromObject(constraint)

			By("Create a clusterDefinition obj")
			clusterDefObj := apps.NewClusterDefFactory(clusterDefName).
				AddComponentDef(apps.StatefulMySQLComponent, statefulCompDefName).
				AddConfigTemplate(configSpecName, configmap.Name, constraint.Name, testCtx.DefaultNamespace, configVolumeName).
				AddLabels(cfgcore.GenerateTPLUniqLabelKeyWithConfig(configSpecName), configmap.Name,
					cfgcore.GenerateConstraintsUniqLabelKeyWithConfig(constraint.Name), constraint.Name).
				Create(&testCtx).GetObject()

			By("Create a clusterVersion obj")
			clusterVersionObj := apps.NewClusterVersionFactory(clusterVersionName, clusterDefObj.GetName()).
				AddComponentVersion(statefulCompDefName).
				AddLabels(cfgcore.GenerateTPLUniqLabelKeyWithConfig(configSpecName), configmap.Name,
					cfgcore.GenerateConstraintsUniqLabelKeyWithConfig(constraint.Name), constraint.Name).
				Create(&testCtx).GetObject()

			By("check ConfigConstraint(template) status and finalizer")
			Eventually(apps.CheckObj(&testCtx, constraintKey,
				func(g Gomega, tpl *appsv1alpha1.ConfigConstraint) {
					g.Expect(tpl.Status.Phase).To(BeEquivalentTo(appsv1alpha1.AvailablePhase))
					g.Expect(tpl.Finalizers).To(ContainElement(constant.ConfigurationTemplateFinalizerName))
				})).Should(Succeed())

			By("By delete ConfigConstraint")
			Expect(k8sClient.Delete(testCtx.Ctx, constraint)).Should(Succeed())

			By("check ConfigConstraint should not be deleted")
			log.Log.Info("expect that ConfigConstraint is not deleted.")
			Consistently(apps.CheckObjExists(&testCtx, constraintKey, &appsv1alpha1.ConfigConstraint{}, true)).Should(Succeed())

			By("check ConfigConstraint status should be deleting")
			Eventually(apps.CheckObj(&testCtx, constraintKey,
				func(g Gomega, tpl *appsv1alpha1.ConfigConstraint) {
					g.Expect(tpl.Status.Phase).To(BeEquivalentTo(appsv1alpha1.CCDeletingPhase))
				})).Should(Succeed())

			By("By delete referencing clusterdefinition and clusterversion")
			Expect(k8sClient.Delete(testCtx.Ctx, clusterVersionObj)).Should(Succeed())
			Expect(k8sClient.Delete(testCtx.Ctx, clusterDefObj)).Should(Succeed())

			By("check ConfigConstraint should be deleted")
			Eventually(apps.CheckObjExists(&testCtx, constraintKey, &appsv1alpha1.ConfigConstraint{}, false), time.Second*60, time.Second*1).Should(Succeed())
		})
	})

	Context("Create config constraint without cue validate", func() {
		It("Should ready", func() {
			By("creating a configmap and a config constraint")

			_ = apps.CreateCustomizedObj(&testCtx, "resources/mysql-config-template.yaml", &corev1.ConfigMap{},
				testCtx.UseDefaultNamespace())

			constraint := apps.CreateCustomizedObj(&testCtx, "resources/mysql-config-constraint-not-validate.yaml",
				&appsv1alpha1.ConfigConstraint{})

			By("check config constraint status")
			Eventually(apps.CheckObj(&testCtx, client.ObjectKeyFromObject(constraint),
				func(g Gomega, tpl *appsv1alpha1.ConfigConstraint) {
					g.Expect(tpl.Status.Phase).Should(BeEquivalentTo(appsv1alpha1.AvailablePhase))
				})).Should(Succeed())
		})
	})
})
