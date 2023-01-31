/*
Copyright ApeCloud Inc.

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
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
	cfgcore "github.com/apecloud/kubeblocks/internal/configuration"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
	testdbaas "github.com/apecloud/kubeblocks/internal/testutil/dbaas"
	test "github.com/apecloud/kubeblocks/test/testdata"
)

var _ = Describe("ConfigConstraint Controller", func() {
	var ctx = context.Background()

	cleanEnv := func() {
		// must wait until resources deleted and no longer exist before the testcases start,
		// otherwise if later it needs to create some new resource objects with the same name,
		// in race conditions, it will find the existence of old objects, resulting failure to
		// create the new objects.
		By("clean resources")

		// delete cluster(and all dependent sub-resources), clusterversion and clusterdef
		testdbaas.ClearClusterResources(&testCtx)

		// delete rest mocked objects
		inNS := client.InNamespace(testCtx.DefaultNamespace)
		ml := client.HasLabels{testCtx.TestObjLabelKey}
		// namespaced
		testdbaas.ClearResources(&testCtx, intctrlutil.ConfigMapSignature, inNS, ml)
		// non-namespaced
		testdbaas.ClearResources(&testCtx, intctrlutil.ConfigConstraintSignature, ml)
	}

	BeforeEach(cleanEnv)

	AfterEach(cleanEnv)

	Context("Create config tpl with cue validate", func() {
		It("Should ready", func() {
			By("create resources")
			testWrapper := CreateDBaasFromISV(testCtx, ctx, k8sClient,
				test.SubTestDataPath("resources"),
				FakeTest{
					// for crd yaml file
					CfgTemplateYaml: "mysql_config_template.yaml",
					CDYaml:          "mysql_cd.yaml",
					CVYaml:          "mysql_cv.yaml",
					CfgCMYaml:       "mysql_config_cm.yaml",
				}, true)
			Expect(testWrapper.HasError()).ShouldNot(HaveOccurred())

			// should ensure clusterdef and clusterversion are in cache before going on
			// TODO fixme: it seems this is likely a bug in intctrlutil.ValidateReferenceCR,
			// TODO where it determines whether anyone are referencing the object to be deleted
			// TODO using the client.List interface, which just reads from the cache.
			// TODO this will cause a referenced object get deleted in race condition.
			By("check clusterversion and clusterdef exists")
			Eventually(testdbaas.CheckObjExists(&testCtx, client.ObjectKeyFromObject(testWrapper.cd),
				&dbaasv1alpha1.ClusterDefinition{}, true)).Should(Succeed())
			Eventually(testdbaas.CheckObjExists(&testCtx, client.ObjectKeyFromObject(testWrapper.cv),
				&dbaasv1alpha1.ClusterVersion{}, true)).Should(Succeed())

			tplKey := client.ObjectKeyFromObject(testWrapper.tpl)

			By("check ConfigConstraint(template) status and finalizer")
			Eventually(testdbaas.CheckObj(&testCtx, tplKey,
				func(g Gomega, tpl *dbaasv1alpha1.ConfigConstraint) {
					g.Expect(tpl.Status.Phase).To(Equal(dbaasv1alpha1.AvailablePhase))
					g.Expect(tpl.Finalizers).To(ContainElement(cfgcore.ConfigurationTemplateFinalizerName))
				})).Should(Succeed())

			By("By delete ConfigConstraint")
			Expect(testWrapper.DeleteTpl()).Should(Succeed())
			// Configuration template not deleted

			By("check ConfigConstraint should not be deleted")
			log.Log.Info("expect that ConfigConstraint is not deleted.")
			Eventually(testdbaas.CheckObjExists(&testCtx, tplKey, &dbaasv1alpha1.ConfigConstraint{}, true)).Should(Succeed())

			By("By delete referencing clusterdefinition and clusterversion")
			Expect(testWrapper.DeleteCV()).Should(Succeed())
			Expect(testWrapper.DeleteCD()).Should(Succeed())

			By("check ConfigConstraint should be deleted")
			Eventually(testdbaas.CheckObjExists(&testCtx, tplKey, &dbaasv1alpha1.ConfigConstraint{}, false),
				60*time.Second, time.Second).Should(Succeed())
		})
	})

	Context("Create config tpl without cue validate", func() {
		It("Should ready", func() {
			By("By creating a ISV resource")

			// step1: prepare env
			testWrapper := CreateDBaasFromISV(testCtx, ctx, k8sClient,
				test.SubTestDataPath("resources"),
				FakeTest{
					// for crd yaml file
					CfgTemplateYaml: "mysql_config_tpl_not_validate.yaml",
					CDYaml:          "mysql_cd.yaml",
					CVYaml:          "mysql_cv.yaml",
					CfgCMYaml:       "mysql_config_cm.yaml",
				}, true)
			Expect(testWrapper.HasError()).Should(Succeed())

			Eventually(func() bool {
				ok, err := ValidateISVCR(testWrapper, &dbaasv1alpha1.ConfigConstraint{},
					func(tpl *dbaasv1alpha1.ConfigConstraint) bool {
						return validateConfTplStatus(tpl.Status)
					})
				return err == nil && ok
			}, time.Second*30, time.Second*1).Should(BeTrue())
			Expect(testWrapper.DeleteAllCR()).Should(Succeed())
		})
	})
})
