/*
Copyright 2022.

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

package dbaas

import (
	"context"
	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
	"github.com/apecloud/kubeblocks/controllers/dbaas/configuration"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"time"
)

var _ = Describe("Cluster Controller", func() {
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
				"./testdata",
				FakeTest{
					// for crd yaml file
					CfgTemplateYaml: "mysql_config_template.yaml",
					CdYaml:          "mysql_cd.yaml",
					AvYaml:          "mysql_av.yaml",
					CfgCMYaml:       "mysql_configmap.yaml",
				}, true)
			Expect(testWrapper.HasError()).Should(Succeed())

			// clean all cr after finished
			defer func() {
				Expect(testWrapper.DeleteAllCR()).Should(Succeed())
			}()

			// step2: Check configuration template status
			Eventually(func() bool {
				err, ok := ValidateISVCR(testWrapper, &dbaasv1alpha1.ConfigurationTemplate{},
					func(tpl *dbaasv1alpha1.ConfigurationTemplate) bool {
						return configuration.ValidateConfTplStatus(tpl.Status)
					})
				return err == nil && ok
			}, time.Second*30, time.Second*1).Should(BeTrue())

			// step3: Check cluster definition status
			Eventually(func() bool {
				_, ok := ValidateISVCR(testWrapper, &dbaasv1alpha1.ClusterDefinition{},
					func(cd *dbaasv1alpha1.ClusterDefinition) bool {
						return cd.Status.Phase == dbaasv1alpha1.AvailablePhase
					})
				return ok
			}, time.Second*10, time.Second*1).Should(BeTrue())

			// step4: Create Cluster
			clusterName := GenRandomClusterName()
			clusterObject := CreateCluster(testWrapper, clusterName)
			Expect(testWrapper.HasError()).Should(Succeed())

			// step5: Check cluster definition status
			Eventually(func() bool {
				err, ok := ValidateCR(testWrapper, &dbaasv1alpha1.Cluster{},
					testWrapper.WithCRName(clusterName),
					func(obj *dbaasv1alpha1.Cluster) bool {
						return obj.Status.Phase == dbaasv1alpha1.CreatingPhase
					})
				return err == nil && ok
			}, time.Second*30, time.Second*1).Should(BeTrue())

			// step5 Check config for instance
			Eventually(func() bool {
				_, ok := ValidateCR(testWrapper, &corev1.ConfigMap{},
					testWrapper.WithCRName(GetComponentCfgName(clusterName,
						"replicasets", // component name
						"mysql-config")), // volume name
					// testWrapper.testEnv.CfgTplName)),
					func(cm *corev1.ConfigMap) bool {
						return cm.Labels[appInstanceLabelKey] == clusterName &&
							cm.Labels[configuration.CMConfigurationTplNameLabelKey] == testWrapper.testEnv.CfgTplName &&
							cm.Labels[configuration.CMInsConfigurationLabelKey] == "true"
					})
				return ok
			}, time.Second*30, time.Second*1).Should(BeTrue())

			Expect(DeleteCluster(testWrapper, clusterObject)).Should(Succeed())
		})
	})

})
