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

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
	cfgcore "github.com/apecloud/kubeblocks/internal/configuration"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
	test "github.com/apecloud/kubeblocks/test/testdata"
)

var _ = Describe("Reconfigure Controller", func() {
	var ctx = context.Background()

	BeforeEach(func() {
		// Add any steup steps that needs to be executed before each test
	})

	AfterEach(func() {
		// Add any teardown steps that needs to be executed after each test
	})

	Context("When updating configmap", func() {
		It("Should rolling upgrade pod", func() {
			By("By creating a cluster")

			// step1: prepare env
			testWrapper := CreateDBaasFromISV(testCtx, ctx, k8sClient,
				test.SubTestDataPath("resources"),
				FakeTest{
					// for crd yaml file
					CfgTemplateYaml: "mysql_config_template.yaml",
					CDYaml:          "mysql_cd.yaml",
					CVYaml:          "mysql_cv.yaml",
					CfgCMYaml:       "mysql_config_cm.yaml",
					StsYaml:         "mysql_sts.yaml",
					MockSts:         true,
				}, true)
			Expect(testWrapper.HasError()).Should(Succeed())

			// clean all cr after finished
			defer func() {
				Expect(testWrapper.DeleteAllCR()).Should(Succeed())
			}()

			// step2: Check configuration template status
			Eventually(func() bool {
				ok, err := ValidateISVCR(testWrapper, &dbaasv1alpha1.ConfigConstraint{},
					func(tpl *dbaasv1alpha1.ConfigConstraint) bool {
						return validateConfTplStatus(tpl.Status)
					})
				return err == nil && ok
			}, time.Second*30, time.Second*1).Should(BeTrue())

			// step3: Create Cluster
			clusterName := GenRandomClusterName()
			clusterObject := CreateCluster(testWrapper, clusterName)
			Expect(testWrapper.HasError()).Should(Succeed())

			cfgObj, err := testWrapper.CreateCfgOnCluster("mysql_config_cm.yaml", clusterObject, "replicasets")
			Expect(err).Should(Succeed())
			insCfgCMName := cfgObj.Name

			// step5 Check config for instance
			var configHash string
			Eventually(func() bool {
				ok, _ := ValidateCR(testWrapper, &corev1.ConfigMap{},
					testWrapper.WithCRName(insCfgCMName),
					func(cm *corev1.ConfigMap) bool {
						configHash = cm.Labels[cfgcore.CMInsConfigurationHashLabelKey]
						return cm.Labels[intctrlutil.AppInstanceLabelKey] == clusterName &&
							cm.Labels[cfgcore.CMConfigurationTplNameLabelKey] == testWrapper.CMName() &&
							cm.Labels[cfgcore.CMConfigurationTypeLabelKey] != "" &&
							cm.Labels[cfgcore.CMInsLastReconfigureMethodLabelKey] == ReconfigureFirstConfigType &&
							configHash != ""
					})
				return ok
			}, time.Second*30, time.Second*1).Should(BeTrue())

			// step6: update configmap
			Expect(UpdateCR[corev1.ConfigMap](testWrapper, &corev1.ConfigMap{},
				testWrapper.WithCRName(insCfgCMName),
				"mysql_ins_config_update.yaml",
				func(cm *corev1.ConfigMap, newCm *corev1.ConfigMap) (client.Patch, error) {
					patch := client.MergeFrom(cm.DeepCopy())
					cm.Data = newCm.Data
					return patch, nil
				})).Should(Succeed())

			// check update configmap
			Eventually(func() bool {
				ok, _ := ValidateCR(testWrapper, &corev1.ConfigMap{},
					testWrapper.WithCRName(insCfgCMName),
					func(cm *corev1.ConfigMap) bool {
						newHash := cm.Labels[cfgcore.CMInsConfigurationHashLabelKey]
						return newHash != configHash &&
							cm.Labels[cfgcore.CMInsLastReconfigureMethodLabelKey] == ReconfigureAutoReloadType
					})
				return ok
			}, time.Second*30, time.Second*1).Should(BeTrue())

			// step7: update invalid update
			Expect(UpdateCR[corev1.ConfigMap](testWrapper, &corev1.ConfigMap{},
				testWrapper.WithCRName(insCfgCMName),
				"mysql_ins_config_invalid_update.yaml",
				func(cm *corev1.ConfigMap, newCm *corev1.ConfigMap) (client.Patch, error) {
					patch := client.MergeFrom(cm.DeepCopy())
					cm.Data = newCm.Data
					return patch, nil
				})).Should(Succeed())

			// Check update configmap
			Eventually(func() bool {
				ok, _ := ValidateCR(testWrapper, &corev1.ConfigMap{},
					testWrapper.WithCRName(insCfgCMName),
					func(cm *corev1.ConfigMap) bool {
						return cm.Labels[cfgcore.CMInsLastReconfigureMethodLabelKey] == ReconfigureNoChangeType
					})
				return ok
			}, time.Second*30, time.Second*1).Should(BeTrue())

			// step8: need restart update parameter
			Expect(UpdateCR[corev1.ConfigMap](testWrapper, &corev1.ConfigMap{},
				testWrapper.WithCRName(insCfgCMName),
				"mysql_ins_config_update_with_restart.yaml",
				func(cm *corev1.ConfigMap, newCm *corev1.ConfigMap) (client.Patch, error) {
					patch := client.MergeFrom(cm.DeepCopy())
					cm.Data = newCm.Data
					return patch, nil
				})).Should(Succeed())

			// Check update configmap
			Eventually(func() bool {
				ok, _ := ValidateCR(testWrapper, &corev1.ConfigMap{},
					testWrapper.WithCRName(insCfgCMName),
					func(cm *corev1.ConfigMap) bool {
						return cm.Labels[cfgcore.CMInsLastReconfigureMethodLabelKey] == ReconfigureSimpleType
					})
				return ok
			}, time.Second*70, time.Second*1).Should(BeTrue())

			Expect(DeleteCluster(testWrapper, clusterObject)).Should(Succeed())
		})
	})

})
