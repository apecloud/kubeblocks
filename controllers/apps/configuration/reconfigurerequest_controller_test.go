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

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	cfgcore "github.com/apecloud/kubeblocks/internal/configuration"
	"github.com/apecloud/kubeblocks/internal/constant"
	intctrlutil "github.com/apecloud/kubeblocks/internal/generics"
	testapps "github.com/apecloud/kubeblocks/internal/testutil/apps"
)

var _ = Describe("Reconfigure Controller", func() {
	const clusterDefName = "test-clusterdef"
	const clusterVersionName = "test-clusterversion"
	const clusterName = "test-cluster"

	const statefulCompType = "replicasets"
	const statefulCompName = "mysql"

	const statefulSetName = "mysql-statefulset"

	const configSpecName = "mysql-config-tpl"

	const configVolumeName = "mysql-config"

	const cmName = "mysql-tree-node-template-8.0"

	var ctx = context.Background()

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
		// non-namespaced
		testapps.ClearResources(&testCtx, intctrlutil.ConfigConstraintSignature, ml)
		// namespaced
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, intctrlutil.ConfigMapSignature, true, inNS, ml)
	}

	BeforeEach(cleanEnv)

	AfterEach(cleanEnv)

	Context("When updating configmap", func() {
		It("Should rolling upgrade pod", func() {

			By("creating a cluster")
			configmap := testapps.CreateCustomizedObj(&testCtx,
				"resources/mysql-config-template.yaml", &corev1.ConfigMap{},
				testCtx.UseDefaultNamespace(),
				testapps.WithLabels(
					constant.AppNameLabelKey, clusterName,
					constant.AppInstanceLabelKey, clusterName,
					constant.KBAppComponentLabelKey, statefulCompName,
					constant.CMConfigurationTemplateNameLabelKey, configSpecName,
					constant.CMConfigurationConstraintsNameLabelKey, cmName,
					constant.CMConfigurationSpecProviderLabelKey, configSpecName,
					constant.CMConfigurationTypeLabelKey, constant.ConfigInstanceType,
				),
				testapps.WithAnnotations(constant.KBParameterUpdateSourceAnnotationKey,
					constant.ReconfigureManagerSource))

			constraint := testapps.CreateCustomizedObj(&testCtx,
				"resources/mysql-config-constraint.yaml",
				&appsv1alpha1.ConfigConstraint{})

			By("Create a clusterDefinition obj")
			clusterDefObj := testapps.NewClusterDefFactory(clusterDefName).
				AddComponent(testapps.StatefulMySQLComponent, statefulCompType).
				AddConfigTemplate(configSpecName, configmap.Name, constraint.Name, testCtx.DefaultNamespace, configVolumeName).
				AddLabels(cfgcore.GenerateTPLUniqLabelKeyWithConfig(configSpecName), configmap.Name,
					cfgcore.GenerateConstraintsUniqLabelKeyWithConfig(constraint.Name), constraint.Name).
				Create(&testCtx).GetObject()

			By("Create a clusterVersion obj")
			clusterVersionObj := testapps.NewClusterVersionFactory(clusterVersionName, clusterDefObj.GetName()).
				AddComponent(statefulCompType).
				AddLabels(cfgcore.GenerateTPLUniqLabelKeyWithConfig(configSpecName), configmap.Name,
					cfgcore.GenerateConstraintsUniqLabelKeyWithConfig(constraint.Name), constraint.Name).
				Create(&testCtx).GetObject()

			By("Creating a cluster")
			clusterObj := testapps.NewClusterFactory(testCtx.DefaultNamespace, clusterName,
				clusterDefObj.Name, clusterVersionObj.Name).
				AddComponent(statefulCompName, statefulCompType).Create(&testCtx).GetObject()

			container := corev1.Container{
				Name: "mock-container",
				VolumeMounts: []corev1.VolumeMount{{
					Name:      configVolumeName,
					MountPath: "/mnt/config",
				}},
			}
			_ = testapps.NewStatefulSetFactory(testCtx.DefaultNamespace, statefulSetName, clusterObj.Name, statefulCompName).
				AddConfigmapVolume(configVolumeName, configmap.Name).
				AddContainer(container).
				AddAppNameLabel(clusterName).
				AddAppInstanceLabel(clusterName).
				AddAppComponentLabel(statefulCompName).
				AddAnnotations(cfgcore.GenerateTPLUniqLabelKeyWithConfig(configSpecName), configmap.Name).
				Create(&testCtx).GetObject()

			By("check config constraint")
			Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(constraint), func(g Gomega, tpl *appsv1alpha1.ConfigConstraint) {
				g.Expect(tpl.Status.Phase).Should(BeEquivalentTo(appsv1alpha1.AvailablePhase))
			})).Should(Succeed())

			By("Check config for instance")
			var configHash string
			Eventually(func(g Gomega) {
				cm := &corev1.ConfigMap{}
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(configmap), cm)).Should(Succeed())
				g.Expect(cm.Labels[constant.AppInstanceLabelKey]).To(Equal(clusterObj.Name))
				g.Expect(cm.Labels[constant.CMConfigurationTemplateNameLabelKey]).To(Equal(configSpecName))
				g.Expect(cm.Labels[constant.CMConfigurationTypeLabelKey]).NotTo(Equal(""))
				g.Expect(cm.Labels[constant.CMInsLastReconfigurePhaseKey]).To(Equal(cfgcore.ReconfigureCreatedPhase))
				configHash = cm.Labels[constant.CMInsConfigurationHashLabelKey]
				g.Expect(configHash).NotTo(Equal(""))
				g.Expect(cfgcore.IsNotUserReconfigureOperation(cm)).To(BeTrue())
				// g.Expect(cm.Annotations[constant.KBParameterUpdateSourceAnnotationKey]).To(Equal(constant.ReconfigureManagerSource))
			}).Should(Succeed())

			By("manager changes will not change the phase of configmap.")
			Eventually(testapps.GetAndChangeObj(&testCtx, client.ObjectKeyFromObject(configmap), func(cm *corev1.ConfigMap) {
				cm.Data["new_data"] = "###"
				cfgcore.SetParametersUpdateSource(cm, constant.ReconfigureManagerSource)
			})).Should(Succeed())

			Eventually(func(g Gomega) {
				cm := &corev1.ConfigMap{}
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(configmap), cm)).Should(Succeed())
				newHash := cm.Labels[constant.CMInsConfigurationHashLabelKey]
				g.Expect(newHash).NotTo(Equal(configHash))
				g.Expect(cfgcore.IsNotUserReconfigureOperation(cm)).To(BeTrue())
			}).Should(Succeed())

			By("recover normal update parameters")
			Eventually(testapps.GetAndChangeObj(&testCtx, client.ObjectKeyFromObject(configmap), func(cm *corev1.ConfigMap) {
				delete(cm.Data, "new_data")
				cfgcore.SetParametersUpdateSource(cm, constant.ReconfigureManagerSource)
			})).Should(Succeed())

			Eventually(func(g Gomega) {
				cm := &corev1.ConfigMap{}
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(configmap), cm)).Should(Succeed())
				newHash := cm.Labels[constant.CMInsConfigurationHashLabelKey]
				g.Expect(newHash).To(Equal(configHash))
				g.Expect(cfgcore.IsNotUserReconfigureOperation(cm)).To(BeTrue())
			}).Should(Succeed())

			By("Update config, old version: " + configHash)
			updatedCM := testapps.NewCustomizedObj("resources/mysql-ins-config-update.yaml", &corev1.ConfigMap{})
			Eventually(testapps.GetAndChangeObj(&testCtx, client.ObjectKeyFromObject(configmap), func(cm *corev1.ConfigMap) {
				cm.Data = updatedCM.Data
				cfgcore.SetParametersUpdateSource(cm, constant.ReconfigureUserSource)
			})).Should(Succeed())

			By("check config new version")
			Eventually(func(g Gomega) {
				cm := &corev1.ConfigMap{}
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(configmap), cm)).Should(Succeed())
				newHash := cm.Labels[constant.CMInsConfigurationHashLabelKey]
				g.Expect(newHash).NotTo(Equal(configHash))
				g.Expect(cm.Labels[constant.CMInsLastReconfigurePhaseKey]).To(Equal(cfgcore.ReconfigureAutoReloadPhase))
				g.Expect(cfgcore.IsNotUserReconfigureOperation(cm)).NotTo(BeTrue())
			}).Should(Succeed())

			By("invalid Update")
			invalidUpdatedCM := testapps.NewCustomizedObj("resources/mysql-ins-config-invalid-update.yaml", &corev1.ConfigMap{})
			Eventually(testapps.GetAndChangeObj(&testCtx, client.ObjectKeyFromObject(configmap), func(cm *corev1.ConfigMap) {
				cm.Data = invalidUpdatedCM.Data
				cfgcore.SetParametersUpdateSource(cm, constant.ReconfigureUserSource)
			})).Should(Succeed())

			By("check invalid update")
			Eventually(func(g Gomega) {
				cm := &corev1.ConfigMap{}
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(configmap), cm)).Should(Succeed())
				g.Expect(cfgcore.IsNotUserReconfigureOperation(cm)).NotTo(BeTrue())
				// g.Expect(cm.Labels[constant.CMInsLastReconfigurePhaseKey]).Should(BeEquivalentTo(cfgcore.ReconfigureNoChangeType))
			}).Should(Succeed())

			By("restart Update")
			restartUpdatedCM := testapps.NewCustomizedObj("resources/mysql-ins-config-update-with-restart.yaml", &corev1.ConfigMap{})
			Eventually(testapps.GetAndChangeObj(&testCtx, client.ObjectKeyFromObject(configmap), func(cm *corev1.ConfigMap) {
				cm.Data = restartUpdatedCM.Data
				cfgcore.SetParametersUpdateSource(cm, constant.ReconfigureUserSource)
			})).Should(Succeed())

			By("check invalid update")
			Eventually(func(g Gomega) {
				cm := &corev1.ConfigMap{}
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(configmap), cm)).Should(Succeed())
				g.Expect(cfgcore.IsNotUserReconfigureOperation(cm)).NotTo(BeTrue())
				g.Expect(cm.Labels[constant.CMInsLastReconfigurePhaseKey]).Should(BeEquivalentTo(cfgcore.ReconfigureSimplePhase))
			}).Should(Succeed())
		})
	})

})
