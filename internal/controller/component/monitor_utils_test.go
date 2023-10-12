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
			buildMonitorConfigLegacy(clusterCompDef, clusterCompSpec, component)
			monitorConfig := component.Monitor
			Expect(monitorConfig.Enable).Should(BeFalse())
			Expect(monitorConfig.BuiltIn).Should(BeFalse())
			Expect(monitorConfig.ScrapePort).To(BeEquivalentTo(0))
			Expect(monitorConfig.ScrapePath).To(Equal(""))
		})

		It("should disable builtin monitor if ClusterComponentDefinition.Monitor.BuiltIn is false and has valid ExporterConfig", func() {
			clusterCompSpec.Monitor = true
			clusterCompDef.Monitor.BuiltIn = false
			buildMonitorConfigLegacy(clusterCompDef, clusterCompSpec, component)
			monitorConfig := component.Monitor
			Expect(monitorConfig.Enable).Should(BeTrue())
			Expect(monitorConfig.BuiltIn).Should(BeFalse())
			Expect(monitorConfig.ScrapePort).To(BeEquivalentTo(9144))
			Expect(monitorConfig.ScrapePath).To(Equal("/metrics"))
		})

		It("should disable monitor if ClusterComponentDefinition.Monitor.BuiltIn is false and lack of ExporterConfig", func() {
			clusterCompSpec.Monitor = true
			clusterCompDef.Monitor.BuiltIn = false
			clusterCompDef.Monitor.Exporter = nil
			buildMonitorConfigLegacy(clusterCompDef, clusterCompSpec, component)
			monitorConfig := component.Monitor
			Expect(monitorConfig.Enable).Should(BeFalse())
			Expect(monitorConfig.BuiltIn).Should(BeFalse())
			Expect(monitorConfig.ScrapePort).To(BeEquivalentTo(0))
			Expect(monitorConfig.ScrapePath).To(Equal(""))
		})

		It("should enable monitor if ClusterComponentDefinition.Monitor.BuiltIn is true", func() {
			clusterCompSpec.Monitor = true
			clusterCompDef.Monitor.BuiltIn = true
			clusterCompDef.Monitor.Exporter = nil
			buildMonitorConfigLegacy(clusterCompDef, clusterCompSpec, component)
			monitorConfig := component.Monitor
			Expect(monitorConfig.Enable).Should(BeTrue())
			Expect(monitorConfig.BuiltIn).Should(BeTrue())
			Expect(monitorConfig.ScrapePort).To(BeEquivalentTo(0))
			Expect(monitorConfig.ScrapePath).To(Equal(""))
		})
	})
})
