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
	"k8s.io/apimachinery/pkg/util/intstr"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
)

var _ = Describe("monitor_utils", func() {
	Context("has the buildMonitorConfig function", func() {
		var component *SynthesizedComponent
		var clusterCompSpec *appsv1alpha1.ClusterComponentSpec
		var clusterCompDef *appsv1alpha1.ClusterComponentDefinition

		BeforeEach(func() {
			component = &SynthesizedComponent{}
			clusterCompSpec = &appsv1alpha1.ClusterComponentSpec{}
			clusterCompSpec.Monitor = true
			clusterCompDef = &appsv1alpha1.ClusterComponentDefinition{}
			clusterCompDef.Monitor = &appsv1alpha1.MonitorConfig{
				BuiltIn: false,
				Exporter: &appsv1alpha1.ExporterConfig{
					ScrapePort: intstr.FromInt(9144),
					ScrapePath: "/metrics",
				},
			}
		})

		It("should disable monitor if ClusterComponentSpec.Monitor is false", func() {
			clusterCompSpec.Monitor = false
			buildMonitorConfig(clusterCompDef, clusterCompSpec, component)
			monitorConfig := component.Monitor
			Expect(monitorConfig.Enable).Should(BeFalse())
			Expect(monitorConfig.ScrapePort).To(BeEquivalentTo(0))
			Expect(monitorConfig.ScrapePath).To(Equal(""))
		})

		It("should disable builtin monitor if ClusterComponentDefinition.Monitor.BuiltIn is false and has valid ExporterConfig", func() {
			clusterCompSpec.Monitor = true
			clusterCompDef.Monitor.BuiltIn = false
			buildMonitorConfig(clusterCompDef, clusterCompSpec, component)
			monitorConfig := component.Monitor
			Expect(monitorConfig.Enable).Should(BeTrue())
			Expect(monitorConfig.ScrapePort).To(BeEquivalentTo(9144))
			Expect(monitorConfig.ScrapePath).To(Equal("/metrics"))
		})

		It("should disable monitor if ClusterComponentDefinition.Monitor.BuiltIn is false and lacks ExporterConfig", func() {
			clusterCompSpec.Monitor = true
			clusterCompDef.Monitor.BuiltIn = false
			clusterCompDef.Monitor.Exporter = nil
			buildMonitorConfig(clusterCompDef, clusterCompSpec, component)
			monitorConfig := component.Monitor
			Expect(monitorConfig.Enable).Should(BeFalse())
			Expect(monitorConfig.ScrapePort).To(BeEquivalentTo(0))
			Expect(monitorConfig.ScrapePath).To(Equal(""))
		})

		It("should disable monitor if ClusterComponentDefinition.Monitor.BuiltIn is true", func() {
			clusterCompSpec.Monitor = true
			clusterCompDef.Monitor.BuiltIn = true
			clusterCompDef.Monitor.Exporter = nil
			buildMonitorConfig(clusterCompDef, clusterCompSpec, component)
			monitorConfig := component.Monitor
			Expect(monitorConfig.Enable).Should(BeFalse())
			Expect(monitorConfig.ScrapePort).To(BeEquivalentTo(0))
			Expect(monitorConfig.ScrapePath).To(Equal(""))
		})
	})
})
