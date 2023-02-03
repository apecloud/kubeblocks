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
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	appv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
	cfgcore "github.com/apecloud/kubeblocks/internal/configuration"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
	testdbaas "github.com/apecloud/kubeblocks/internal/testutil/dbaas"
	"github.com/apecloud/kubeblocks/test/testdata"
)

var _ = Describe("Reconfigure Controller", func() {
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

	Context("When updating configmap", func() {
		It("Should rolling upgrade pod", func() {

			By("creating a cluster")
			testWrapper := CreateDBaasFromISV(testCtx, ctx, FakeTest{
				// for crd yaml file
				CfgCCYaml:       "mysql_config_template.yaml",
				CDYaml:          "mysql_cd.yaml",
				CVYaml:          "mysql_cv.yaml",
				CfgCMYaml:       "mysql_config_cm.yaml",
				StsYaml:         "mysql_sts.yaml",
				ClusterYaml:     "mysql_cluster.yaml",
				TestDataPath:    "resources",
				ComponentName:   TestComponentName,
				CDComponentType: TestCDComponentTypeName,
			})
			defer func() {
				By("clear TestWrapper created objects...")
				defer testWrapper.DeleteAllObjects()
			}()

			namer := testWrapper.namer

			By("check sts")
			Eventually(testdbaas.CheckObjExists(&testCtx, client.ObjectKeyFromObject(testWrapper.sts), &appv1.StatefulSet{}, true)).Should(Succeed())

			By("check config constraint")
			Eventually(testdbaas.CheckObj(&testCtx, client.ObjectKeyFromObject(testWrapper.cc), func(g Gomega, tpl *dbaasv1alpha1.ConfigConstraint) {
				g.Expect(tpl.Status.Phase).Should(BeEquivalentTo(dbaasv1alpha1.AvailablePhase))
			})).Should(Succeed())

			By("Check config for instance")
			var configHash string
			Eventually(func(g Gomega) {
				cm := &corev1.ConfigMap{}
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(testWrapper.cfgCM), cm)).Should(Succeed())
				configHash = cm.Labels[cfgcore.CMInsConfigurationHashLabelKey]
				g.Expect(
					cm.Labels[intctrlutil.AppInstanceLabelKey] == namer.ClusterName &&
						cm.Labels[cfgcore.CMConfigurationTplNameLabelKey] == namer.TPLName &&
						cm.Labels[cfgcore.CMConfigurationTypeLabelKey] != "" &&
						cm.Labels[cfgcore.CMInsLastReconfigureMethodLabelKey] == ReconfigureFirstConfigType &&
						configHash != "").Should(BeTrue())
			}).Should(Succeed())

			By("Update config, old version: " + configHash)
			updatedCM, err := testdata.GetResourceFromTestData[corev1.ConfigMap]("resources/mysql_ins_config_update.yaml")
			Expect(err).Should(Succeed())
			Eventually(testdbaas.GetAndChangeObj(&testCtx, client.ObjectKeyFromObject(testWrapper.cfgCM), func(cm *corev1.ConfigMap) {
				cm.Data = updatedCM.Data
			})).Should(Succeed())

			By("check config new version")
			Eventually(func(g Gomega) {
				cm := &corev1.ConfigMap{}
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(testWrapper.cfgCM), cm)).Should(Succeed())
				newHash := cm.Labels[cfgcore.CMInsConfigurationHashLabelKey]
				g.Expect(newHash != configHash &&
					cm.Labels[cfgcore.CMInsLastReconfigureMethodLabelKey] == ReconfigureAutoReloadType).Should(BeTrue())
			}).Should(Succeed())

			By("invalid Update")
			invalidUpdatedCM, err := testdata.GetResourceFromTestData[corev1.ConfigMap]("resources/mysql_ins_config_invalid_update.yaml")
			Expect(err).Should(Succeed())
			Eventually(testdbaas.GetAndChangeObj(&testCtx, client.ObjectKeyFromObject(testWrapper.cfgCM), func(cm *corev1.ConfigMap) {
				cm.Data = invalidUpdatedCM.Data
			})).Should(Succeed())

			By("check invalid update")
			Eventually(func(g Gomega) {
				cm := &corev1.ConfigMap{}
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(testWrapper.cfgCM), cm)).Should(Succeed())
				g.Expect(cm.Labels[cfgcore.CMInsLastReconfigureMethodLabelKey]).Should(BeEquivalentTo(ReconfigureNoChangeType))
			}).Should(Succeed())

			By("restart Update")
			restartUpdatedCM, err := testdata.GetResourceFromTestData[corev1.ConfigMap]("resources/mysql_ins_config_update_with_restart.yaml")
			Expect(err).Should(Succeed())
			Eventually(testdbaas.GetAndChangeObj(&testCtx, client.ObjectKeyFromObject(testWrapper.cfgCM), func(cm *corev1.ConfigMap) {
				cm.Data = restartUpdatedCM.Data
			})).Should(Succeed())

			By("check invalid update")
			Eventually(func(g Gomega) {
				cm := &corev1.ConfigMap{}
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(testWrapper.cfgCM), cm)).Should(Succeed())
				g.Expect(cm.Labels[cfgcore.CMInsLastReconfigureMethodLabelKey]).Should(BeEquivalentTo(ReconfigureSimpleType))
			}).Should(Succeed())
		})
	})

})
