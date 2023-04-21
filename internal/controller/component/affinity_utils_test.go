/*
Copyright (C) 2022 ApeCloud Co., Ltd

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
		const labelKey = "testNodeLabelKey"
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
					labelKey: labelValue,
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
			Expect(affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms[0].MatchExpressions[0].Key).Should(Equal(labelKey))
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
		const labelKey = "testNodeLabelKey"
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
					labelKey: labelValue,
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
			Expect(affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms[0].MatchExpressions[0].Key).Should(Equal(labelKey))
			Expect(affinity.PodAntiAffinity.RequiredDuringSchedulingIgnoredDuringExecution).Should(BeEmpty())
			Expect(affinity.PodAntiAffinity.PreferredDuringSchedulingIgnoredDuringExecution[0].Weight).ShouldNot(BeNil())
			Expect(affinity.PodAntiAffinity.PreferredDuringSchedulingIgnoredDuringExecution[0].PodAffinityTerm.TopologyKey).Should(Equal(topologyKey))

			topologySpreadConstraints := buildPodTopologySpreadConstraints(clusterObj, clusterObj.Spec.Affinity, component)
			Expect(topologySpreadConstraints[0].WhenUnsatisfiable).Should(Equal(corev1.ScheduleAnyway))
			Expect(topologySpreadConstraints[0].TopologyKey).Should(Equal(topologyKey))
		})
	})
})
