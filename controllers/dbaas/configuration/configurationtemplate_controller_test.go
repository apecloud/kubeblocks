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

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
	cfgcore "github.com/apecloud/kubeblocks/internal/configuration"
)

var _ = Describe("ConfigurationTemplate Controller", func() {
	var ctx = context.Background()

	BeforeEach(func() {
		// Add any steup steps that needs to be executed before each test
	})

	AfterEach(func() {
		// Add any teardown steps that needs to be executed after each test
	})

	validateFinalizerFlag := func(crObj client.Object) bool {
		return controllerutil.ContainsFinalizer(crObj, cfgcore.ConfigurationTemplateFinalizerName)
	}

	Context("Create config tpl with cue validate", func() {
		It("Should ready", func() {
			By("By creating a ISV resource")
			// step1: prepare env
			testWrapper := CreateDBaasFromISV(testCtx, ctx, k8sClient,
				"./testdata",
				FakeTest{
					// for crd yaml file
					CfgTemplateYaml: "mysql_config_template.yaml",
					CdYaml:          "mysql_cd.yaml",
					AvYaml:          "mysql_av.yaml",
					CfgCMYaml:       "mysql_config_cm.yaml",
				}, true)
			Expect(testWrapper.HasError()).Should(Succeed())

			// step2: check configuration template cr status and finalizer
			By("By check configurationtemplate status")
			Eventually(func() bool {
				ok, err := ValidateISVCR(testWrapper, &dbaasv1alpha1.ConfigurationTemplate{},
					func(tpl *dbaasv1alpha1.ConfigurationTemplate) bool {
						return validateConfTplStatus(tpl.Status) &&
							validateFinalizerFlag(tpl)
					})
				return err == nil && ok
			}, time.Second*30, time.Second*1).Should(BeTrue())

			By("By delete configuration template cr")
			Expect(testWrapper.DeleteTpl()).Should(Succeed())
			// Configuration template not deleted
			By("check whether the configurationtemplate has been deleted")
			log.Log.Info("expect that configurationtemplate is not deleted.")
			Eventually(func() error {
				_, err := ValidateISVCR(testWrapper, &dbaasv1alpha1.ConfigurationTemplate{},
					func(tpl *dbaasv1alpha1.ConfigurationTemplate) error { return nil })
				return err
			}, time.Second*10, time.Second*1).Should(Succeed())

			By("By delete clusterdefinition and appversion")
			// step3: delete clusterdefinition and appversion
			Expect(testWrapper.DeleteAV()).Should(Succeed())
			Expect(testWrapper.DeleteCD()).Should(Succeed())

			Eventually(func() error {
				_, err := ValidateISVCR(testWrapper, &dbaasv1alpha1.ConfigurationTemplate{},
					func(tpl *dbaasv1alpha1.ConfigurationTemplate) error { return nil })
				return err
			}, time.Second*100, time.Second*1).ShouldNot(Succeed())
			Expect(testWrapper.DeleteAllCR()).Should(Succeed())
		})
	})

	Context("Create config tpl without cue validate", func() {
		It("Should ready", func() {
			By("By creating a ISV resource")

			// step1: prepare env
			testWrapper := CreateDBaasFromISV(testCtx, ctx, k8sClient,
				"./testdata",
				FakeTest{
					// for crd yaml file
					CfgTemplateYaml: "mysql_config_tpl_not_validate.yaml",
					CdYaml:          "mysql_cd.yaml",
					AvYaml:          "mysql_av.yaml",
					CfgCMYaml:       "mysql_config_cm.yaml",
				}, true)
			Expect(testWrapper.HasError()).Should(Succeed())

			Eventually(func() bool {
				ok, err := ValidateISVCR(testWrapper, &dbaasv1alpha1.ConfigurationTemplate{},
					func(tpl *dbaasv1alpha1.ConfigurationTemplate) bool {
						return validateConfTplStatus(tpl.Status)
					})
				return err == nil && ok
			}, time.Second*30, time.Second*1).Should(BeTrue())
			Expect(testWrapper.DeleteAllCR()).Should(Succeed())
		})
	})
})
