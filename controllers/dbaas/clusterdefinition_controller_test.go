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
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
	"github.com/apecloud/kubeblocks/controllers/dbaas/configuration"
	cfgcore "github.com/apecloud/kubeblocks/internal/configuration"
	testdbaas "github.com/apecloud/kubeblocks/internal/testutil/dbaas"
	"github.com/apecloud/kubeblocks/test/testdata"
)

var _ = Describe("ClusterDefinition Controller", func() {

	const timeout = time.Second * 10
	const interval = time.Second * 1
	const waitDuration = time.Second * 5

	var ctx = context.Background()

	cleanEnv := func() {
		// must wait until resources deleted and no longer exist before the testcases start,
		// otherwise if later it needs to create some new resource objects with the same name,
		// in race conditions, it will find the existence of old objects, resulting failure to
		// create the new objects.
		By("clean resources")

		// delete cluster(and all dependent sub-resources), clusterversion and clusterdef
		testdbaas.ClearClusterResources(&testCtx)
	}

	BeforeEach(cleanEnv)

	AfterEach(cleanEnv)

	Context("When updating clusterDefinition", func() {
		It("Should update status of clusterVersion at the same time", func() {
			By("By creating a clusterDefinition")

			testWrapper := configuration.NewFakeDBaasCRsFromISV(testCtx, ctx, configuration.FakeTest{
				TestDataPath:    "resources",
				CfgCCYaml:       "mysql_config_template.yaml",
				CDYaml:          "mysql_cd.yaml",
				CVYaml:          "mysql_cv.yaml",
				CfgCMYaml:       "mysql_config_cm.yaml",
				ComponentName:   configuration.TestComponentName,
				CDComponentType: configuration.TestCDComponentTypeName,

				DisableConfigTpl: true,
			})

			defer func() {
				By("clear TestWrapper created objects...")
				defer testWrapper.DeleteAllObjects()
			}()

			// clusterDefinition := &dbaasv1alpha1.ClusterDefinition{}
			// Expect(yaml.Unmarshal([]byte(clusterDefYaml), clusterDefinition)).Should(Succeed())
			// Expect(testCtx.CreateObj(ctx, clusterDefinition)).Should(Succeed())
			// check reconciled finalizer and status
			Eventually(func(g Gomega) {
				cd := &dbaasv1alpha1.ClusterDefinition{}
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(testWrapper.CD), cd)).To(Succeed())
				g.Expect(len(cd.Finalizers) > 0 &&
					cd.Status.ObservedGeneration == 1).To(BeTrue())
			}, timeout, interval).Should(Succeed())

			// By("creating an clusterVersion")
			// clusterVersion := &dbaasv1alpha1.ClusterVersion{}
			// Expect(yaml.Unmarshal([]byte(clusterVersionYaml), clusterVersion)).Should(Succeed())
			// Expect(testCtx.CreateObj(ctx, clusterVersion)).Should(Succeed())
			// check reconciled finalizer
			Eventually(func(g Gomega) {
				cv := &dbaasv1alpha1.ClusterVersion{}
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(testWrapper.CV), cv)).To(Succeed())
				g.Expect(len(cv.Finalizers) > 0 &&
					cv.Status.ObservedGeneration == 1).To(BeTrue())
			}, timeout, interval).Should(Succeed())

			By("updating clusterDefinition's spec which then mark clusterVersion's status as OutOfSync")
			Eventually(testdbaas.GetAndChangeObj(&testCtx, client.ObjectKeyFromObject(testWrapper.CD),
				func(cd *dbaasv1alpha1.ClusterDefinition) {
					cd.Spec.Type = "state.redis"
				}), timeout, interval).Should(Succeed())
			// check ClusterVersion.Status as updated
			Eventually(func(g Gomega) {
				cv := &dbaasv1alpha1.ClusterVersion{}
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(testWrapper.CV), cv)).To(Succeed())
				g.Expect(cv.Status.Phase == dbaasv1alpha1.AvailablePhase &&
					cv.Status.Message == "" &&
					cv.Status.ClusterDefGeneration > testWrapper.CD.Generation).To(BeTrue())
			}, timeout, interval).Should(Succeed())

			// TODO: update components to break @validateClusterVersion, and transit ClusterVersion.Status.Phase to UnavailablePhase
		})
	})

	Context("When configmap template refs in clusterDefinition is invalid", func() {
		It("Should stop proceeding the status of clusterDefinition", func() {
			By("creating a clusterDefinition")

			testWrapper := configuration.NewFakeDBaasCRsFromISV(testCtx, ctx, configuration.FakeTest{
				TestDataPath: "resources",
				CDYaml:       "mysql_cd.yaml",
				CVYaml:       "mysql_cv.yaml",
				CfgCCYaml:    "mysql_config_template.yaml",
				// CfgCMYaml:       "mysql_config_cm.yaml",
				ComponentName:   configuration.TestComponentName,
				CDComponentType: configuration.TestCDComponentTypeName,
			})
			defer func() {
				By("clear TestWrapper created objects...")
				defer testWrapper.DeleteAllObjects()
			}()

			namer := testWrapper.GetNamer()
			By("check the reconciler won't update Status.ObservedGeneration if configmap doesn't exist.")

			// should use Consistently here, since cd.Status.ObservedGeneration is initialized to be zero,
			// we must watch the value for a while to tell it's not changed by the reconciler.
			Consistently(func(g Gomega) {
				cd := &dbaasv1alpha1.ClusterDefinition{}
				g.Eventually(func() error {
					return k8sClient.Get(ctx, client.ObjectKeyFromObject(testWrapper.CD), cd)
				}, timeout, interval).Should(Succeed())
				g.Expect(cd.Status.ObservedGeneration == 0).To(BeTrue())
			}, waitDuration, interval).Should(Succeed())

			By("check the reconciler update Status.ObservedGeneration after configmap is created.")
			cmName := namer.TPLName
			configuration.NewFakeK8sObjectFromFile(testWrapper, "mysql_config_cm.yaml", &corev1.ConfigMap{}, testdata.WithNamespacedName(cmName, testCtx.DefaultNamespace))

			Eventually(testdbaas.CheckObjExists(&testCtx, client.ObjectKeyFromObject(testWrapper.CC), &dbaasv1alpha1.ConfigConstraint{}, true), timeout, interval).Should(Succeed())
			Eventually(testdbaas.ChangeObjStatus(&testCtx, testWrapper.CC, func() {
				testWrapper.CC.Status.Phase = dbaasv1alpha1.AvailablePhase
			})).Should(Succeed())

			Eventually(func(g Gomega) {
				cd := &dbaasv1alpha1.ClusterDefinition{}
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(testWrapper.CD), cd)).To(Succeed())
				g.Expect(cd.Status.ObservedGeneration == 1).To(BeTrue())

				// check labels and finalizers
				g.Expect(cd.Finalizers).ShouldNot(BeEmpty())
				configCMLabel := cfgcore.GenerateTPLUniqLabelKeyWithConfig(cmName)
				configTPLLabel := cfgcore.GenerateConstraintsUniqLabelKeyWithConfig(namer.CCName)
				g.Expect(cd.Labels[configCMLabel]).Should(BeEquivalentTo(cmName))
				g.Expect(cd.Labels[configTPLLabel]).Should(BeEquivalentTo(namer.CCName))
			}, waitDuration, interval).Should(Succeed())

			By("check the reconciler update configmap.Finalizer after configmap is created.")
			Eventually(func(g Gomega) {
				cmObj := &corev1.ConfigMap{}
				g.Expect(k8sClient.Get(ctx, types.NamespacedName{
					Namespace: testCtx.DefaultNamespace,
					Name:      cmName,
				}, cmObj)).Should(Succeed())
				g.Expect(controllerutil.ContainsFinalizer(cmObj, cfgcore.ConfigurationTemplateFinalizerName)).To(BeTrue())
			}, timeout, interval).Should(Succeed())
		})
	})

})
