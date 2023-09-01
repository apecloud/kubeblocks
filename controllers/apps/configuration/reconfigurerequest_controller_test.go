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
	"context"

	core2 "github.com/apecloud/kubeblocks/pkg/configuration/core"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/generics"
	"github.com/apecloud/kubeblocks/pkg/testutil/apps"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
)

var _ = Describe("Reconfigure Controller", func() {
	const clusterDefName = "test-clusterdef"
	const clusterVersionName = "test-clusterversion"
	const clusterName = "test-cluster"
	const statefulCompDefName = "replicasets"
	const statefulCompName = "mysql"
	const statefulSetName = "mysql-statefulset"
	const configSpecName = "mysql-config-tpl"
	const configVolumeName = "mysql-config"
	const cmName = "mysql-tree-node-template-8.0"

	var ctx = context.Background()

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

	Context("When updating configmap", func() {
		It("Should rolling upgrade pod", func() {

			By("creating a cluster")
			configmap := apps.CreateCustomizedObj(&testCtx,
				"resources/mysql-config-template.yaml", &corev1.ConfigMap{},
				testCtx.UseDefaultNamespace(),
				apps.WithLabels(
					constant.AppNameLabelKey, clusterName,
					constant.AppInstanceLabelKey, clusterName,
					constant.KBAppComponentLabelKey, statefulCompName,
					constant.CMConfigurationTemplateNameLabelKey, configSpecName,
					constant.CMConfigurationConstraintsNameLabelKey, cmName,
					constant.CMConfigurationSpecProviderLabelKey, configSpecName,
					constant.CMConfigurationTypeLabelKey, constant.ConfigInstanceType,
				),
				apps.WithAnnotations(constant.KBParameterUpdateSourceAnnotationKey,
					constant.ReconfigureManagerSource,
					constant.CMInsEnableRerenderTemplateKey, "true"))

			constraint := apps.CreateCustomizedObj(&testCtx,
				"resources/mysql-config-constraint.yaml",
				&appsv1alpha1.ConfigConstraint{})

			By("Create a clusterDefinition obj")
			clusterDefObj := apps.NewClusterDefFactory(clusterDefName).
				AddComponentDef(apps.StatefulMySQLComponent, statefulCompDefName).
				AddConfigTemplate(configSpecName, configmap.Name, constraint.Name, testCtx.DefaultNamespace, configVolumeName).
				AddLabels(core2.GenerateTPLUniqLabelKeyWithConfig(configSpecName), configmap.Name,
					core2.GenerateConstraintsUniqLabelKeyWithConfig(constraint.Name), constraint.Name).
				Create(&testCtx).GetObject()

			By("Create a clusterVersion obj")
			clusterVersionObj := apps.NewClusterVersionFactory(clusterVersionName, clusterDefObj.GetName()).
				AddComponentVersion(statefulCompDefName).
				AddLabels(core2.GenerateTPLUniqLabelKeyWithConfig(configSpecName), configmap.Name,
					core2.GenerateConstraintsUniqLabelKeyWithConfig(constraint.Name), constraint.Name).
				Create(&testCtx).GetObject()

			By("Creating a cluster")
			clusterObj := apps.NewClusterFactory(testCtx.DefaultNamespace, clusterName,
				clusterDefObj.Name, clusterVersionObj.Name).
				AddComponent(statefulCompName, statefulCompDefName).Create(&testCtx).GetObject()

			container := corev1.Container{
				Name: "mock-container",
				VolumeMounts: []corev1.VolumeMount{{
					Name:      configVolumeName,
					MountPath: "/mnt/config",
				}},
			}
			_ = apps.NewStatefulSetFactory(testCtx.DefaultNamespace, statefulSetName, clusterObj.Name, statefulCompName).
				AddConfigmapVolume(configVolumeName, configmap.Name).
				AddContainer(container).
				AddAppNameLabel(clusterName).
				AddAppInstanceLabel(clusterName).
				AddAppComponentLabel(statefulCompName).
				AddAnnotations(core2.GenerateTPLUniqLabelKeyWithConfig(configSpecName), configmap.Name).
				Create(&testCtx).GetObject()

			By("check config constraint")
			Eventually(apps.CheckObj(&testCtx, client.ObjectKeyFromObject(constraint), func(g Gomega, tpl *appsv1alpha1.ConfigConstraint) {
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
				g.Expect(cm.Labels[constant.CMInsLastReconfigurePhaseKey]).To(Equal(core2.ReconfigureCreatedPhase))
				configHash = cm.Labels[constant.CMInsConfigurationHashLabelKey]
				g.Expect(configHash).NotTo(Equal(""))
				g.Expect(core2.IsNotUserReconfigureOperation(cm)).To(BeTrue())
				// g.Expect(cm.Annotations[constant.KBParameterUpdateSourceAnnotationKey]).To(Equal(constant.ReconfigureManagerSource))
			}).Should(Succeed())

			By("manager changes will not change the phase of configmap.")
			Eventually(apps.GetAndChangeObj(&testCtx, client.ObjectKeyFromObject(configmap), func(cm *corev1.ConfigMap) {
				cm.Data["new_data"] = "###"
				core2.SetParametersUpdateSource(cm, constant.ReconfigureManagerSource)
			})).Should(Succeed())

			Eventually(func(g Gomega) {
				cm := &corev1.ConfigMap{}
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(configmap), cm)).Should(Succeed())
				newHash := cm.Labels[constant.CMInsConfigurationHashLabelKey]
				g.Expect(newHash).NotTo(Equal(configHash))
				g.Expect(core2.IsNotUserReconfigureOperation(cm)).To(BeTrue())
			}).Should(Succeed())

			By("recover normal update parameters")
			Eventually(apps.GetAndChangeObj(&testCtx, client.ObjectKeyFromObject(configmap), func(cm *corev1.ConfigMap) {
				delete(cm.Data, "new_data")
				core2.SetParametersUpdateSource(cm, constant.ReconfigureManagerSource)
			})).Should(Succeed())

			Eventually(func(g Gomega) {
				cm := &corev1.ConfigMap{}
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(configmap), cm)).Should(Succeed())
				newHash := cm.Labels[constant.CMInsConfigurationHashLabelKey]
				g.Expect(newHash).To(Equal(configHash))
				g.Expect(core2.IsNotUserReconfigureOperation(cm)).To(BeTrue())
			}).Should(Succeed())

			By("Update config, old version: " + configHash)
			updatedCM := apps.NewCustomizedObj("resources/mysql-ins-config-update.yaml", &corev1.ConfigMap{})
			Eventually(apps.GetAndChangeObj(&testCtx, client.ObjectKeyFromObject(configmap), func(cm *corev1.ConfigMap) {
				cm.Data = updatedCM.Data
				core2.SetParametersUpdateSource(cm, constant.ReconfigureUserSource)
			})).Should(Succeed())

			By("check config new version")
			Eventually(func(g Gomega) {
				cm := &corev1.ConfigMap{}
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(configmap), cm)).Should(Succeed())
				newHash := cm.Labels[constant.CMInsConfigurationHashLabelKey]
				g.Expect(newHash).NotTo(Equal(configHash))
				g.Expect(cm.Labels[constant.CMInsLastReconfigurePhaseKey]).To(Equal(core2.ReconfigureAutoReloadPhase))
				g.Expect(core2.IsNotUserReconfigureOperation(cm)).NotTo(BeTrue())
			}).Should(Succeed())

			By("invalid Update")
			invalidUpdatedCM := apps.NewCustomizedObj("resources/mysql-ins-config-invalid-update.yaml", &corev1.ConfigMap{})
			Eventually(apps.GetAndChangeObj(&testCtx, client.ObjectKeyFromObject(configmap), func(cm *corev1.ConfigMap) {
				cm.Data = invalidUpdatedCM.Data
				core2.SetParametersUpdateSource(cm, constant.ReconfigureUserSource)
			})).Should(Succeed())

			By("check invalid update")
			Eventually(func(g Gomega) {
				cm := &corev1.ConfigMap{}
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(configmap), cm)).Should(Succeed())
				g.Expect(core2.IsNotUserReconfigureOperation(cm)).NotTo(BeTrue())
				// g.Expect(cm.Labels[constant.CMInsLastReconfigurePhaseKey]).Should(BeEquivalentTo(cfgcore.ReconfigureNoChangeType))
			}).Should(Succeed())

			By("restart Update")
			restartUpdatedCM := apps.NewCustomizedObj("resources/mysql-ins-config-update-with-restart.yaml", &corev1.ConfigMap{})
			Eventually(apps.GetAndChangeObj(&testCtx, client.ObjectKeyFromObject(configmap), func(cm *corev1.ConfigMap) {
				cm.Data = restartUpdatedCM.Data
				core2.SetParametersUpdateSource(cm, constant.ReconfigureUserSource)
			})).Should(Succeed())

			By("check invalid update")
			Eventually(func(g Gomega) {
				cm := &corev1.ConfigMap{}
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(configmap), cm)).Should(Succeed())
				g.Expect(core2.IsNotUserReconfigureOperation(cm)).NotTo(BeTrue())
				g.Expect(cm.Labels[constant.CMInsLastReconfigurePhaseKey]).Should(BeEquivalentTo(core2.ReconfigureSimplePhase))
			}).Should(Succeed())
		})
	})

})
