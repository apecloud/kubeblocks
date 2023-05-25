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

package component

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/spf13/viper"
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

		clusterTolerationKey = "testClusterTolerationKey"
		topologyKey          = "testTopologyKey"
		labelKey             = "testNodeLabelKey"
		labelValue           = "testLabelValue"
		nodeKey              = "testNodeKey"
	)

	var (
		clusterObj *appsv1alpha1.Cluster
		component  *SynthesizedComponent

		buildObjs = func(podAntiAffinity appsv1alpha1.PodAntiAffinity) {
			clusterDefObj := testapps.NewClusterDefFactory(clusterDefName).
				AddComponentDef(testapps.StatefulMySQLComponent, mysqlCompDefName).
				GetObject()

			clusterVersionObj := testapps.NewClusterVersionFactory(clusterVersionName, clusterDefObj.Name).
				AddComponentVersion(mysqlCompDefName).AddContainerShort("mysql", testapps.ApeCloudMySQLImage).
				GetObject()

			affinity := &appsv1alpha1.Affinity{
				PodAntiAffinity: podAntiAffinity,
				TopologyKeys:    []string{topologyKey},
				NodeLabels: map[string]string{
					labelKey: labelValue,
				},
			}

			toleration := corev1.Toleration{
				Key:      clusterTolerationKey,
				Operator: corev1.TolerationOpExists,
				Effect:   corev1.TaintEffectNoExecute,
			}

			clusterObj = testapps.NewClusterFactory(testCtx.DefaultNamespace, clusterName,
				clusterDefObj.Name, clusterVersionObj.Name).
				AddComponent(mysqlCompName, mysqlCompDefName).
				SetClusterAffinity(affinity).
				AddClusterToleration(toleration).
				GetObject()

			reqCtx := intctrlutil.RequestCtx{
				Ctx: ctx,
				Log: tlog,
			}
			component, _ = BuildComponent(
				reqCtx,
				*clusterObj,
				*clusterDefObj,
				clusterDefObj.Spec.ComponentDefs[0],
				clusterObj.Spec.ComponentSpecs[0],
				&clusterVersionObj.Spec.ComponentVersions[0],
			)
		}
	)

	Context("with PodAntiAffinity set to Required", func() {
		BeforeEach(func() {
			buildObjs(appsv1alpha1.Required)
			Expect(component).ShouldNot(BeNil())
		})

		It("should have correct Affinity and TopologySpreadConstraints", func() {
			affinity, err := buildPodAffinity(clusterObj, clusterObj.Spec.Affinity, component)
			Expect(err).Should(Succeed())
			Expect(affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms[0].MatchExpressions[0].Key).Should(Equal(labelKey))
			Expect(affinity.PodAntiAffinity.RequiredDuringSchedulingIgnoredDuringExecution[0].TopologyKey).Should(Equal(topologyKey))
			Expect(affinity.PodAntiAffinity.PreferredDuringSchedulingIgnoredDuringExecution).Should(BeEmpty())
			Expect(affinity.NodeAffinity.PreferredDuringSchedulingIgnoredDuringExecution).Should(BeEmpty())

			topologySpreadConstraints := buildPodTopologySpreadConstraints(clusterObj, clusterObj.Spec.Affinity, component)
			Expect(topologySpreadConstraints[0].WhenUnsatisfiable).Should(Equal(corev1.DoNotSchedule))
			Expect(topologySpreadConstraints[0].TopologyKey).Should(Equal(topologyKey))
		})

		It("when data plane affinity is set, should have correct Affinity and TopologySpreadConstraints", func() {
			viper.Set(constant.CfgKeyDataPlaneAffinity,
				fmt.Sprintf("{\"nodeAffinity\":{\"preferredDuringSchedulingIgnoredDuringExecution\":[{\"preference\":{\"matchExpressions\":[{\"key\":\"%s\",\"operator\":\"In\",\"values\":[\"true\"]}]},\"weight\":100}]}}", nodeKey))
			defer viper.Set(constant.CfgKeyDataPlaneAffinity, "")

			affinity, err := buildPodAffinity(clusterObj, clusterObj.Spec.Affinity, component)
			Expect(err).Should(Succeed())
			Expect(affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms[0].MatchExpressions[0].Key).Should(Equal(labelKey))
			Expect(affinity.PodAntiAffinity.RequiredDuringSchedulingIgnoredDuringExecution[0].TopologyKey).Should(Equal(topologyKey))
			Expect(affinity.PodAntiAffinity.PreferredDuringSchedulingIgnoredDuringExecution).Should(BeEmpty())
			Expect(affinity.NodeAffinity.PreferredDuringSchedulingIgnoredDuringExecution[0].Preference.MatchExpressions[0].Key).Should(Equal(nodeKey))

			topologySpreadConstraints := buildPodTopologySpreadConstraints(clusterObj, clusterObj.Spec.Affinity, component)
			Expect(topologySpreadConstraints[0].WhenUnsatisfiable).Should(Equal(corev1.DoNotSchedule))
			Expect(topologySpreadConstraints[0].TopologyKey).Should(Equal(topologyKey))
		})
	})

	Context("with tolerations", func() {
		BeforeEach(func() {
			buildObjs(appsv1alpha1.Required)
		})

		It("should have correct tolerations", func() {
			tolerations, err := BuildTolerations(clusterObj, &clusterObj.Spec.ComponentSpecs[0])
			Expect(err).Should(Succeed())
			Expect(tolerations).ShouldNot(BeEmpty())
			Expect(tolerations[0].Key).Should(Equal(clusterTolerationKey))
		})

		It("when data plane tolerations is set, should have correct tolerations", func() {
			const dpTolerationKey = "dataPlaneTolerationKey"
			viper.Set(constant.CfgKeyDataPlaneTolerations, fmt.Sprintf("[{\"key\":\"%s\", \"operator\": \"Exists\", \"effect\": \"NoSchedule\"}]", dpTolerationKey))
			defer viper.Set(constant.CfgKeyDataPlaneTolerations, "")
			tolerations, err := BuildTolerations(clusterObj, &clusterObj.Spec.ComponentSpecs[0])
			Expect(err).Should(Succeed())
			Expect(tolerations).Should(HaveLen(2))
			Expect(tolerations[0].Key).Should(Equal(clusterTolerationKey))
			Expect(tolerations[1].Key).Should(Equal(dpTolerationKey))
		})
	})

	Context("with PodAntiAffinity set to Preferred", func() {
		BeforeEach(func() {
			buildObjs(appsv1alpha1.Preferred)
			Expect(component).ShouldNot(BeNil())
		})

		It("should have correct Affinity and TopologySpreadConstraints", func() {
			affinity, err := buildPodAffinity(clusterObj, clusterObj.Spec.Affinity, component)
			Expect(err).Should(Succeed())
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
