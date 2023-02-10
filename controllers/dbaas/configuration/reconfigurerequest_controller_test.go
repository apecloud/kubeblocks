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

	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
	cfgcore "github.com/apecloud/kubeblocks/internal/configuration"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
	testdbaas "github.com/apecloud/kubeblocks/internal/testutil/dbaas"
)

var _ = Describe("Reconfigure Controller", func() {
	const clusterDefName = "test-clusterdef"
	const clusterVersionName = "test-clusterversion"
	const clusterName = "test-cluster"

	const statefulCompType = "replicasets"
	const statefulCompName = "mysql"

	const statefulSetName = "mysql-statefulset"

	const configTplName = "mysql-config-tpl"

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
			configmap := testdbaas.CreateCustomizedObj(&testCtx,
				"resources/mysql_config_cm.yaml", &corev1.ConfigMap{},
				testCtx.UseDefaultNamespace(),
				testdbaas.WithLabels(
					intctrlutil.AppNameLabelKey, clusterName,
					intctrlutil.AppInstanceLabelKey, clusterName,
					intctrlutil.AppComponentLabelKey, statefulCompName,
					cfgcore.CMConfigurationTplNameLabelKey, configTplName,
					cfgcore.CMConfigurationConstraintsNameLabelKey, cmName,
					cfgcore.CMConfigurationISVTplLabelKey, configTplName,
					cfgcore.CMConfigurationTypeLabelKey, cfgcore.ConfigInstanceType,
				))

			constraint := testdbaas.CreateCustomizedObj(&testCtx,
				"resources/mysql_config_template.yaml",
				&dbaasv1alpha1.ConfigConstraint{})

			By("Create a clusterDefinition obj")
			clusterDefObj := testdbaas.NewClusterDefFactory(clusterDefName, testdbaas.MySQLType).
				AddComponent(testdbaas.StatefulMySQLComponent, statefulCompType).
				AddConfigTemplate(configTplName, configmap.Name, constraint.Name, configVolumeName, nil).
				AddLabels(cfgcore.GenerateTPLUniqLabelKeyWithConfig(configTplName), configmap.Name,
					cfgcore.GenerateConstraintsUniqLabelKeyWithConfig(constraint.Name), constraint.Name).
				Create(&testCtx).GetClusterDef()

			By("Create a clusterVersion obj")
			clusterVersionObj := testdbaas.NewClusterVersionFactory(clusterVersionName, clusterDefObj.GetName()).
				AddComponent(statefulCompType).
				AddLabels(cfgcore.GenerateTPLUniqLabelKeyWithConfig(configTplName), configmap.Name,
					cfgcore.GenerateConstraintsUniqLabelKeyWithConfig(constraint.Name), constraint.Name).
				Create(&testCtx).GetClusterVersion()

			By("Creating a cluster")
			clusterObj := testdbaas.NewClusterFactory(testCtx.DefaultNamespace, clusterName,
				clusterDefObj.Name, clusterVersionObj.Name).
				AddComponent(statefulCompName, statefulCompType).Create(&testCtx).GetCluster()

			container := corev1.Container{
				Name: "mock-container",
				VolumeMounts: []corev1.VolumeMount{{
					Name:      configVolumeName,
					MountPath: "/mnt/config",
				}},
			}
			_ = testdbaas.NewStatefulSetFactory(testCtx.DefaultNamespace, statefulSetName, clusterObj.Name, statefulCompName).
				AddConfigmapVolume(configVolumeName, configmap.Name).
				AddContainer(container).
				AddLabels(intctrlutil.AppNameLabelKey, clusterName,
					intctrlutil.AppInstanceLabelKey, clusterName,
					intctrlutil.AppComponentLabelKey, statefulCompName,
					cfgcore.GenerateTPLUniqLabelKeyWithConfig(configTplName), configmap.Name,
				).Create(&testCtx).GetStatefulSet()

			By("check config constraint")
			Eventually(testdbaas.CheckObj(&testCtx, client.ObjectKeyFromObject(constraint), func(g Gomega, tpl *dbaasv1alpha1.ConfigConstraint) {
				g.Expect(tpl.Status.Phase).Should(BeEquivalentTo(dbaasv1alpha1.AvailablePhase))
			})).Should(Succeed())

			By("Check config for instance")
			var configHash string
			Eventually(func(g Gomega) {
				cm := &corev1.ConfigMap{}
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(configmap), cm)).Should(Succeed())
				g.Expect(cm.Labels[intctrlutil.AppInstanceLabelKey]).To(Equal(clusterObj.Name))
				g.Expect(cm.Labels[cfgcore.CMConfigurationTplNameLabelKey]).To(Equal(configTplName))
				g.Expect(cm.Labels[cfgcore.CMConfigurationTypeLabelKey]).NotTo(Equal(""))
				g.Expect(cm.Labels[cfgcore.CMInsLastReconfigureMethodLabelKey]).To(Equal(ReconfigureFirstConfigType))
				configHash = cm.Labels[cfgcore.CMInsConfigurationHashLabelKey]
				g.Expect(configHash).NotTo(Equal(""))
			}).Should(Succeed())

			By("Update config, old version: " + configHash)
			updatedCM := testdbaas.NewCustomizedObj("resources/mysql_ins_config_update.yaml", &corev1.ConfigMap{})
			Eventually(testdbaas.GetAndChangeObj(&testCtx, client.ObjectKeyFromObject(configmap), func(cm *corev1.ConfigMap) {
				cm.Data = updatedCM.Data
			})).Should(Succeed())

			By("check config new version")
			Eventually(func(g Gomega) {
				cm := &corev1.ConfigMap{}
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(configmap), cm)).Should(Succeed())
				newHash := cm.Labels[cfgcore.CMInsConfigurationHashLabelKey]
				g.Expect(newHash).NotTo(Equal(configHash))
				g.Expect(cm.Labels[cfgcore.CMInsLastReconfigureMethodLabelKey]).To(Equal(ReconfigureAutoReloadType))
			}).Should(Succeed())

			By("invalid Update")
			invalidUpdatedCM := testdbaas.NewCustomizedObj("resources/mysql_ins_config_invalid_update.yaml", &corev1.ConfigMap{})
			Eventually(testdbaas.GetAndChangeObj(&testCtx, client.ObjectKeyFromObject(configmap), func(cm *corev1.ConfigMap) {
				cm.Data = invalidUpdatedCM.Data
			})).Should(Succeed())

			By("check invalid update")
			Eventually(func(g Gomega) {
				cm := &corev1.ConfigMap{}
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(configmap), cm)).Should(Succeed())
				g.Expect(cm.Labels[cfgcore.CMInsLastReconfigureMethodLabelKey]).Should(BeEquivalentTo(ReconfigureNoChangeType))
			}).Should(Succeed())

			By("restart Update")
			restartUpdatedCM := testdbaas.NewCustomizedObj("resources/mysql_ins_config_update_with_restart.yaml", &corev1.ConfigMap{})
			Eventually(testdbaas.GetAndChangeObj(&testCtx, client.ObjectKeyFromObject(configmap), func(cm *corev1.ConfigMap) {
				cm.Data = restartUpdatedCM.Data
			})).Should(Succeed())

			By("check invalid update")
			Eventually(func(g Gomega) {
				cm := &corev1.ConfigMap{}
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(configmap), cm)).Should(Succeed())
				g.Expect(cm.Labels[cfgcore.CMInsLastReconfigureMethodLabelKey]).Should(BeEquivalentTo(ReconfigureSimpleType))
			}).Should(Succeed())
		})
	})

})
