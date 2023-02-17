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
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
)

func TestIsSupportedCharacterType(t *testing.T) {
	if !isSupportedCharacterType("mysql") {
		t.Error("mysql is supported characterType")
	}

	if isSupportedCharacterType("redis") {
		t.Error("redis is not supported characterType")
	}

	if isSupportedCharacterType("other") {
		t.Error("other is not supported characterType")
	}
}

var _ = Describe("monitor_utils", func() {
	Context("has the buildMonitorConfig function", func() {
		var component *SynthesizedComponent
		var cluster *appsv1alpha1.Cluster
		var clusterComp *appsv1alpha1.ClusterComponentSpec
		var clusterDef *appsv1alpha1.ClusterDefinition
		var clusterDefComp *appsv1alpha1.ClusterComponentDefinition

		BeforeEach(func() {
			component = &SynthesizedComponent{}
			component.PodSpec = &corev1.PodSpec{}
			cluster = &appsv1alpha1.Cluster{}
			cluster.Name = "mysql-instance-3"
			clusterComp = &appsv1alpha1.ClusterComponentSpec{}
			clusterComp.Monitor = true
			cluster.Spec.ComponentSpecs = append(cluster.Spec.ComponentSpecs, *clusterComp)
			clusterComp = &cluster.Spec.ComponentSpecs[0]

			clusterDef = &appsv1alpha1.ClusterDefinition{}
			clusterDefComp = &appsv1alpha1.ClusterComponentDefinition{}
			clusterDefComp.CharacterType = kMysql
			clusterDefComp.Monitor = &appsv1alpha1.MonitorConfig{
				BuiltIn: false,
				Exporter: &appsv1alpha1.ExporterConfig{
					ScrapePort: 9144,
					ScrapePath: "/metrics",
				},
			}
			clusterDef.Spec.ComponentDefs = append(clusterDef.Spec.ComponentDefs, *clusterDefComp)
			clusterDefComp = &clusterDef.Spec.ComponentDefs[0]
		})

		It("should disable monitor if ClusterComponentSpec.Monitor is false", func() {
			clusterComp.Monitor = false
			buildMonitorConfig(cluster, clusterDef, clusterDefComp, clusterComp, component)
			monitorConfig := component.Monitor
			Expect(monitorConfig.Enable).Should(BeFalse())
			Expect(monitorConfig.ScrapePort).To(BeEquivalentTo(0))
			Expect(monitorConfig.ScrapePath).To(Equal(""))
			if component.PodSpec != nil {
				Expect(len(component.PodSpec.Containers)).To(BeEquivalentTo(0))
			}
		})

		It("should disable builtin monitor if ClusterComponentDefinition.Monitor.BuiltIn is false and has valid ExporterConfig", func() {
			clusterComp.Monitor = true
			clusterDefComp.CharacterType = kFake
			clusterDefComp.Monitor.BuiltIn = false
			buildMonitorConfig(cluster, clusterDef, clusterDefComp, clusterComp, component)
			monitorConfig := component.Monitor
			Expect(monitorConfig.Enable).Should(BeTrue())
			Expect(monitorConfig.ScrapePort).To(BeEquivalentTo(9144))
			Expect(monitorConfig.ScrapePath).To(Equal("/metrics"))
			if component.PodSpec != nil {
				Expect(len(component.PodSpec.Containers)).To(BeEquivalentTo(0))
			}
		})

		It("should disable monitor if ClusterComponentDefinition.Monitor.BuiltIn is false and lacks ExporterConfig", func() {
			clusterComp.Monitor = true
			clusterDefComp.CharacterType = kFake
			clusterDefComp.Monitor.BuiltIn = false
			clusterDefComp.Monitor.Exporter = nil
			buildMonitorConfig(cluster, clusterDef, clusterDefComp, clusterComp, component)
			monitorConfig := component.Monitor
			Expect(monitorConfig.Enable).Should(BeFalse())
			Expect(monitorConfig.ScrapePort).To(BeEquivalentTo(0))
			Expect(monitorConfig.ScrapePath).To(Equal(""))
			if component.PodSpec != nil {
				Expect(len(component.PodSpec.Containers)).To(Equal(0))
			}
		})

		It("should disable monitor if ClusterComponentDefinition.Monitor.BuiltIn is true and CharacterType isn't recognizable", func() {
			clusterComp.Monitor = true
			clusterDefComp.CharacterType = kFake
			clusterDefComp.Monitor.BuiltIn = true
			clusterDefComp.Monitor.Exporter = nil
			buildMonitorConfig(cluster, clusterDef, clusterDefComp, clusterComp, component)
			monitorConfig := component.Monitor
			Expect(monitorConfig.Enable).Should(BeFalse())
			Expect(monitorConfig.ScrapePort).To(BeEquivalentTo(0))
			Expect(monitorConfig.ScrapePath).To(Equal(""))
			if component.PodSpec != nil {
				Expect(len(component.PodSpec.Containers)).To(Equal(0))
			}
		})

		It("should disable monitor if ClusterComponentDefinition's CharacterType is empty", func() {
			// TODO fixme: seems setting clusterDef.Spec.Type has no effect to buildMonitorConfig
			clusterComp.Monitor = true
			clusterDefComp.CharacterType = ""
			clusterDefComp.Monitor.BuiltIn = true
			clusterDefComp.Monitor.Exporter = nil
			buildMonitorConfig(cluster, clusterDef, clusterDefComp, clusterComp, component)
			monitorConfig := component.Monitor
			Expect(monitorConfig.Enable).Should(BeFalse())
			Expect(monitorConfig.ScrapePort).To(BeEquivalentTo(0))
			Expect(monitorConfig.ScrapePath).To(Equal(""))
			if component.PodSpec != nil {
				Expect(len(component.PodSpec.Containers)).To(Equal(0))
			}
		})
	})

	Context("has the setMysqlComponent function ", func() {
		It("which could check against other containers for port conflicts", func() {
			component := &SynthesizedComponent{
				PodSpec: &corev1.PodSpec{
					Containers: []corev1.Container{{
						Ports: []corev1.ContainerPort{{
							ContainerPort: defaultMonitorPort,
						}},
					}},
				}}
			cluster := &appsv1alpha1.Cluster{}
			cluster.SetName("mock-cluster")
			Expect(setMysqlComponent(cluster, component)).Should(Succeed())
			monitor := component.Monitor
			Expect(monitor.ScrapePort).Should(BeEquivalentTo(defaultMonitorPort + 1))
		})
	})
})
