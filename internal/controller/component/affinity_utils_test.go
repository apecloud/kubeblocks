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

package component

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/constant"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
	testapps "github.com/apecloud/kubeblocks/internal/testutil/apps"
)

var _ = Describe("affinity utils", func() {
	const (
		clusterDefName     = "test-clusterdef"
		clusterVersionName = "test-clusterversion"
		clusterName        = "test-cluster"
		mysqlCompDefName   = "replicasets"
		mysqlCompName      = "mysql"
	)

	var (
		clusterObj *appsv1alpha1.Cluster
		component  *SynthesizedComponent
	)

	Context("with PodAntiAffinity set to Required", func() {
		const topologyKey = "testTopologyKey"
		const lableKey = "testNodeLabelKey"
		const labelValue = "testLabelValue"

		BeforeEach(func() {
			clusterDefObj := testapps.NewClusterDefFactory(clusterDefName).
				AddComponentDef(testapps.StatefulMySQLComponent, mysqlCompDefName).
				GetObject()

			clusterVersionObj := testapps.NewClusterVersionFactory(clusterVersionName, clusterDefObj.Name).
				AddComponent(mysqlCompDefName).AddContainerShort("mysql", testapps.ApeCloudMySQLImage).
				GetObject()

			affinity := &appsv1alpha1.Affinity{
				PodAntiAffinity: appsv1alpha1.Required,
				TopologyKeys:    []string{topologyKey},
				NodeLabels: map[string]string{
					lableKey: labelValue,
				},
			}
			clusterObj = testapps.NewClusterFactory(testCtx.DefaultNamespace, clusterName,
				clusterDefObj.Name, clusterVersionObj.Name).
				AddComponent(mysqlCompName, mysqlCompDefName).
				SetClusterAffinity(affinity).
				GetObject()

			reqCtx := intctrlutil.RequestCtx{
				Ctx: ctx,
				Log: tlog,
			}
			component = BuildComponent(
				reqCtx,
				*clusterObj,
				*clusterDefObj,
				clusterDefObj.Spec.ComponentDefs[0],
				clusterObj.Spec.ComponentSpecs[0],
				&clusterVersionObj.Spec.ComponentVersions[0])
			Expect(component).ShouldNot(BeNil())
		})

		It("should have correct Affinity and TopologySpreadConstraints", func() {
			affinity := buildPodAffinity(clusterObj, clusterObj.Spec.Affinity, component)
			Expect(affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms[0].MatchExpressions[0].Key).Should(Equal(lableKey))
			Expect(affinity.PodAntiAffinity.RequiredDuringSchedulingIgnoredDuringExecution[0].TopologyKey).Should(Equal(topologyKey))
			Expect(affinity.PodAntiAffinity.PreferredDuringSchedulingIgnoredDuringExecution).Should(BeEmpty())

			affinity = patchBuiltInAffinity(affinity)
			Expect(affinity.NodeAffinity.PreferredDuringSchedulingIgnoredDuringExecution[0].Preference.MatchExpressions[0].Key).Should(
				Equal(constant.KubeBlocksDataNodeLabelKey))

			topologySpreadConstraints := buildPodTopologySpreadConstraints(clusterObj, clusterObj.Spec.Affinity, component)
			Expect(topologySpreadConstraints[0].WhenUnsatisfiable).Should(Equal(corev1.DoNotSchedule))
			Expect(topologySpreadConstraints[0].TopologyKey).Should(Equal(topologyKey))
		})
	})

	Context("with PodAntiAffinity set to Preferred", func() {
		const topologyKey = "testTopologyKey"
		const lableKey = "testNodeLabelKey"
		const labelValue = "testLabelValue"

		BeforeEach(func() {
			clusterDefObj := testapps.NewClusterDefFactory(clusterDefName).
				AddComponentDef(testapps.StatefulMySQLComponent, mysqlCompDefName).
				GetObject()

			clusterVersionObj := testapps.NewClusterVersionFactory(clusterVersionName, clusterDefObj.Name).
				AddComponent(mysqlCompDefName).AddContainerShort("mysql", testapps.ApeCloudMySQLImage).
				GetObject()

			affinity := &appsv1alpha1.Affinity{
				PodAntiAffinity: appsv1alpha1.Preferred,
				TopologyKeys:    []string{topologyKey},
				NodeLabels: map[string]string{
					lableKey: labelValue,
				},
			}
			clusterObj = testapps.NewClusterFactory(testCtx.DefaultNamespace, clusterName,
				clusterDefObj.Name, clusterVersionObj.Name).
				AddComponent(mysqlCompName, mysqlCompDefName).
				SetClusterAffinity(affinity).
				GetObject()

			reqCtx := intctrlutil.RequestCtx{
				Ctx: ctx,
				Log: tlog,
			}
			component = BuildComponent(
				reqCtx,
				*clusterObj,
				*clusterDefObj,
				clusterDefObj.Spec.ComponentDefs[0],
				clusterObj.Spec.ComponentSpecs[0],
				&clusterVersionObj.Spec.ComponentVersions[0],
			)
			Expect(component).ShouldNot(BeNil())
		})

		It("should have correct Affinity and TopologySpreadConstraints", func() {
			affinity := buildPodAffinity(clusterObj, clusterObj.Spec.Affinity, component)
			Expect(affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms[0].MatchExpressions[0].Key).Should(Equal(lableKey))
			Expect(affinity.PodAntiAffinity.RequiredDuringSchedulingIgnoredDuringExecution).Should(BeEmpty())
			Expect(affinity.PodAntiAffinity.PreferredDuringSchedulingIgnoredDuringExecution[0].Weight).ShouldNot(BeNil())
			Expect(affinity.PodAntiAffinity.PreferredDuringSchedulingIgnoredDuringExecution[0].PodAffinityTerm.TopologyKey).Should(Equal(topologyKey))

			topologySpreadConstraints := buildPodTopologySpreadConstraints(clusterObj, clusterObj.Spec.Affinity, component)
			Expect(topologySpreadConstraints[0].WhenUnsatisfiable).Should(Equal(corev1.ScheduleAnyway))
			Expect(topologySpreadConstraints[0].TopologyKey).Should(Equal(topologyKey))
		})
	})
})
