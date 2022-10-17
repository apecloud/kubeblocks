/*
Copyright 2022 The KubeBlocks Authors

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
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"

	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"

	"github.com/leaanthony/debme"
	ctrl "sigs.k8s.io/controller-runtime"

	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var tlog = ctrl.Log.WithName("lifecycle_util_testing")

func TestReadCUETplFromEmbeddedFS(t *testing.T) {
	cueFS, err := debme.FS(cueTemplates, "cue")
	if err != nil {
		t.Error("Expected no error", err)
	}
	cueTpl, err := intctrlutil.NewCUETplFromBytes(cueFS.ReadFile("secret_template.cue"))

	if err != nil {
		t.Error("Expected no error", err)
	}

	tlog.Info("", "cueValue", cueTpl)
}

var _ = Describe("create", func() {
	Context("mergeMonitorConfig", func() {
		var component *Component
		var cluster *dbaasv1alpha1.Cluster
		var clusterComp *dbaasv1alpha1.ClusterComponent
		var clusterDef *dbaasv1alpha1.ClusterDefinition
		var clusterDefComp *dbaasv1alpha1.ClusterDefinitionComponent

		BeforeEach(func() {
			component = &Component{}
			component.PodSpec = &corev1.PodSpec{}
			cluster = &dbaasv1alpha1.Cluster{}
			cluster.Name = "mysql-instance-3"
			clusterComp = &dbaasv1alpha1.ClusterComponent{}
			clusterComp.Monitor = true
			cluster.Spec.Components = append(cluster.Spec.Components, *clusterComp)
			clusterComp = &cluster.Spec.Components[0]

			clusterDef = &dbaasv1alpha1.ClusterDefinition{}
			clusterDef.Spec.Type = "state-mysql"
			clusterDefComp = &dbaasv1alpha1.ClusterDefinitionComponent{}
			clusterDefComp.CharacterType = "mysql"
			clusterDefComp.Monitor = &dbaasv1alpha1.MonitorConfig{
				BuiltIn: false,
				Exporter: &dbaasv1alpha1.ExporterConfig{
					ScrapePort: 9144,
					ScrapePath: "/metrics",
				},
			}
			clusterDef.Spec.Components = append(clusterDef.Spec.Components, *clusterDefComp)
			clusterDefComp = &clusterDef.Spec.Components[0]
		})

		It("Monitor disable in ClusterComponent", func() {
			clusterComp.Monitor = false
			mergeMonitorConfig(cluster, clusterDef, clusterDefComp, clusterComp, component)
			monitorConfig := component.Monitor
			Expect(monitorConfig.Enable).Should(BeFalse())
			Expect(monitorConfig.ScrapePort).To(Equal(0))
			Expect(monitorConfig.ScrapePath).To(Equal(""))
			if component.PodSpec != nil {
				Expect(len(component.PodSpec.Containers)).To(Equal(0))
			}
		})

		It("Disable builtIn monitor in ClusterDefinitionComponent", func() {
			clusterComp.Monitor = true
			clusterDefComp.CharacterType = "fake"
			clusterDefComp.Monitor.BuiltIn = false
			mergeMonitorConfig(cluster, clusterDef, clusterDefComp, clusterComp, component)
			monitorConfig := component.Monitor
			Expect(monitorConfig.Enable).Should(BeTrue())
			Expect(monitorConfig.ScrapePort).To(Equal(9144))
			Expect(monitorConfig.ScrapePath).To(Equal("/metrics"))
			if component.PodSpec != nil {
				Expect(len(component.PodSpec.Containers)).To(Equal(0))
			}
		})

		It("Disable builtIn monitor with wrong monitorConfig in ClusterDefinitionComponent", func() {
			clusterComp.Monitor = true
			clusterDefComp.CharacterType = "fake"
			clusterDefComp.Monitor.BuiltIn = false
			clusterDefComp.Monitor.Exporter = nil
			mergeMonitorConfig(cluster, clusterDef, clusterDefComp, clusterComp, component)
			monitorConfig := component.Monitor
			Expect(monitorConfig.Enable).Should(BeFalse())
			Expect(monitorConfig.ScrapePort).To(Equal(0))
			Expect(monitorConfig.ScrapePath).To(Equal(""))
			if component.PodSpec != nil {
				Expect(len(component.PodSpec.Containers)).To(Equal(0))
			}
		})

		It("Enable builtIn with wrong CharacterType in ClusterDefinitionComponent", func() {
			clusterComp.Monitor = true
			clusterDefComp.CharacterType = "fake"
			clusterDefComp.Monitor.BuiltIn = true
			clusterDefComp.Monitor.Exporter = nil
			mergeMonitorConfig(cluster, clusterDef, clusterDefComp, clusterComp, component)
			monitorConfig := component.Monitor
			Expect(monitorConfig.Enable).Should(BeFalse())
			Expect(monitorConfig.ScrapePort).To(Equal(0))
			Expect(monitorConfig.ScrapePath).To(Equal(""))
			if component.PodSpec != nil {
				Expect(len(component.PodSpec.Containers)).To(Equal(0))
			}
		})

		It("Enable builtIn with empty CharacterType and wrong clusterType in ClusterDefinitionComponent", func() {
			clusterComp.Monitor = true
			clusterDef.Spec.Type = "fake"
			clusterDefComp.CharacterType = ""
			clusterDefComp.Monitor.BuiltIn = true
			clusterDefComp.Monitor.Exporter = nil
			mergeMonitorConfig(cluster, clusterDef, clusterDefComp, clusterComp, component)
			monitorConfig := component.Monitor
			Expect(monitorConfig.Enable).Should(BeFalse())
			Expect(monitorConfig.ScrapePort).To(Equal(0))
			Expect(monitorConfig.ScrapePath).To(Equal(""))
			if component.PodSpec != nil {
				Expect(len(component.PodSpec.Containers)).To(Equal(0))
			}
		})

		It("Enable builtIn with empty CharacterType and right clusterType in ClusterDefinitionComponent", func() {
			clusterComp.Monitor = true
			clusterDef.Spec.Type = "state.mysql"
			clusterDefComp.CharacterType = ""
			clusterDefComp.Monitor.BuiltIn = true
			clusterDefComp.Monitor.Exporter = nil
			mergeMonitorConfig(cluster, clusterDef, clusterDefComp, clusterComp, component)
			monitorConfig := component.Monitor
			Expect(monitorConfig.Enable).Should(BeTrue())
			Expect(monitorConfig.ScrapePort).To(Equal(9104))
			Expect(monitorConfig.ScrapePath).To(Equal("/metrics"))
			Expect(len(component.PodSpec.Containers)).To(Equal(1))
			Expect(strings.HasPrefix(component.PodSpec.Containers[0].Name, "inject-")).To(BeTrue())
		})
	})
})
