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

package dbaas

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
	cfgcore "github.com/apecloud/kubeblocks/internal/configuration"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
	testdbaas "github.com/apecloud/kubeblocks/internal/testutil/dbaas"
)

var _ = Describe("ClusterDefinition Controller", func() {
	const clusterDefName = "test-clusterdef"
	const clusterVersionName = "test-clusterversion"

	const statefulCompType = "replicasets"

	const configVolumeName = "mysql-config"

	const cmName = "mysql-tree-node-template-8.0"

	cleanEnv := func() {
		// must wait until resources deleted and no longer exist before the testcases start,
		// otherwise if later it needs to create some new resource objects with the same name,
		// in race conditions, it will find the existence of old objects, resulting failure to
		// create the new objects.
		By("clean resources")

		inNS := client.InNamespace(testCtx.DefaultNamespace)
		ml := client.HasLabels{testCtx.TestObjLabelKey}

		// resources should be released in following order
		// non-namespaced
		testdbaas.ClearResources(&testCtx, intctrlutil.ClusterVersionSignature, ml)
		testdbaas.ClearResources(&testCtx, intctrlutil.ClusterDefinitionSignature, ml)
		testdbaas.ClearResources(&testCtx, intctrlutil.ConfigConstraintSignature, ml)

		// namespaced
		testdbaas.ClearResources(&testCtx, intctrlutil.ConfigMapSignature, inNS, ml)
	}

	BeforeEach(func() {
		cleanEnv()

	})

	AfterEach(func() {
		cleanEnv()
	})

	var (
		clusterDefObj     *dbaasv1alpha1.ClusterDefinition
		clusterVersionObj *dbaasv1alpha1.ClusterVersion
	)

	Context("with no ConfigSpec", func() {
		BeforeEach(func() {
			By("Create a clusterDefinition obj")
			clusterDefObj = testdbaas.NewClusterDefFactory(&testCtx, clusterDefName, testdbaas.MySQLType).
				AddComponent(testdbaas.StatefulMySQL8, statefulCompType).
				Create().GetClusterDef()

			By("Create a clusterVersion obj")
			clusterVersionObj = testdbaas.NewClusterVersionFactory(&testCtx, clusterVersionName, clusterDefObj.GetName()).
				AddComponent(statefulCompType).AddContainerShort("mysql", testdbaas.ApeCloudMySQLImage).
				Create().GetClusterVersion()
		})

		It("should update status of clusterVersion at the same time when updating clusterDefinition", func() {
			By("Check reconciled finalizer and status of ClusterDefinition")
			var cdGen int64
			Eventually(testdbaas.CheckObj(&testCtx, client.ObjectKeyFromObject(clusterDefObj),
				func(g Gomega, cd *dbaasv1alpha1.ClusterDefinition) {
					g.Expect(cd.Finalizers).NotTo(BeEmpty())
					g.Expect(cd.Status.ObservedGeneration).To(BeEquivalentTo(1))
					cdGen = cd.Status.ObservedGeneration
				})).Should(Succeed())

			By("Check reconciled finalizer and status of ClusterVersion")
			Eventually(testdbaas.CheckObj(&testCtx, client.ObjectKeyFromObject(clusterVersionObj),
				func(g Gomega, cv *dbaasv1alpha1.ClusterVersion) {
					g.Expect(cv.Finalizers).NotTo(BeEmpty())
					g.Expect(cv.Status.ObservedGeneration).To(BeEquivalentTo(1))
					g.Expect(cv.Status.ClusterDefGeneration).To(Equal(cdGen))
				})).Should(Succeed())

			By("updating clusterDefinition's spec which then update clusterVersion's status")
			Eventually(testdbaas.GetAndChangeObj(&testCtx, client.ObjectKeyFromObject(clusterDefObj),
				func(cd *dbaasv1alpha1.ClusterDefinition) {
					cd.Spec.Type = "state.redis"
				})).Should(Succeed())

			By("Check ClusterVersion.Status as updated")
			Eventually(testdbaas.CheckObj(&testCtx, client.ObjectKeyFromObject(clusterVersionObj),
				func(g Gomega, cv *dbaasv1alpha1.ClusterVersion) {
					g.Expect(cv.Status.Phase).To(Equal(dbaasv1alpha1.AvailablePhase))
					g.Expect(cv.Status.Message).To(Equal(""))
					g.Expect(cv.Status.ClusterDefGeneration > cdGen).To(BeTrue())
				})).Should(Succeed())

			// TODO: update components to break @validateClusterVersion, and transit ClusterVersion.Status.Phase to UnavailablePhase
		})
	})

	assureCfgTplConfigMapObj := func() *corev1.ConfigMap {
		By("Create a configmap and config template obj")
		cm := testdbaas.CreateCustomizedObj(&testCtx, "config/configcm.yaml", &corev1.ConfigMap{},
			testCtx.UseDefaultNamespace())

		cfgTpl := testdbaas.CreateCustomizedObj(&testCtx, "config/configtpl.yaml",
			&dbaasv1alpha1.ConfigConstraint{})
		Expect(testdbaas.ChangeObjStatus(&testCtx, cfgTpl, func() {
			cfgTpl.Status.Phase = dbaasv1alpha1.AvailablePhase
		})).Should(Succeed())
		return cm
	}

	Context("with ConfigSpec", func() {
		BeforeEach(func() {
			By("Create a clusterDefinition obj")
			clusterDefObj = testdbaas.NewClusterDefFactory(&testCtx, clusterDefName, testdbaas.MySQLType).
				AddComponent(testdbaas.StatefulMySQL8, statefulCompType).
				AddConfigTemplate(cmName, cmName, cmName, configVolumeName).
				Create().GetClusterDef()

			By("Create a clusterVersion obj")
			clusterVersionObj = testdbaas.NewClusterVersionFactory(&testCtx, clusterVersionName, clusterDefObj.GetName()).
				AddComponent(statefulCompType).AddContainerShort("mysql", testdbaas.ApeCloudMySQLImage).
				Create().GetClusterVersion()
		})

		It("should stop proceeding the status of clusterDefinition if configmap is invalid or doesn't exist", func() {
			By("check the reconciler won't update Status.ObservedGeneration if configmap doesn't exist.")
			// should use Consistently here, since cd.Status.ObservedGeneration is initialized to be zero,
			// we must watch the value for a while to tell it's not changed by the reconciler.
			Consistently(testdbaas.CheckObj(&testCtx, client.ObjectKeyFromObject(clusterDefObj),
				func(g Gomega, cd *dbaasv1alpha1.ClusterDefinition) {
					g.Expect(cd.Status.ObservedGeneration).To(BeEquivalentTo(0))
				})).Should(Succeed())

			assureCfgTplConfigMapObj()

			By("check the reconciler update Status.ObservedGeneration after configmap is created.")
			Eventually(testdbaas.CheckObj(&testCtx, client.ObjectKeyFromObject(clusterDefObj),
				func(g Gomega, cd *dbaasv1alpha1.ClusterDefinition) {
					g.Expect(cd.Status.ObservedGeneration).To(BeEquivalentTo(1))

					// check labels and finalizers
					g.Expect(cd.Finalizers).ShouldNot(BeEmpty())
					configCMLabel := cfgcore.GenerateTPLUniqLabelKeyWithConfig(cmName)
					configTPLLabel := cfgcore.GenerateConstraintsUniqLabelKeyWithConfig(cmName)
					g.Expect(cd.Labels[configCMLabel]).Should(BeEquivalentTo(cmName))
					g.Expect(cd.Labels[configTPLLabel]).Should(BeEquivalentTo(cmName))
				})).Should(Succeed())

			By("check the reconciler update configmap.Finalizer after configmap is created.")
			cmKey := types.NamespacedName{
				Namespace: testCtx.DefaultNamespace,
				Name:      cmName,
			}
			Eventually(testdbaas.CheckObj(&testCtx, cmKey, func(g Gomega, cmObj *corev1.ConfigMap) {
				g.Expect(controllerutil.ContainsFinalizer(cmObj, cfgcore.ConfigurationTemplateFinalizerName)).To(BeTrue())
			})).Should(Succeed())
		})
	})
})
