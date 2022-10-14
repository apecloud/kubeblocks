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
	"testing"

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
		var clusterComp *dbaasv1alpha1.ClusterComponent
		var clusterDef *dbaasv1alpha1.ClusterDefinition
		var clusterDefComp *dbaasv1alpha1.ClusterDefinitionComponent

		BeforeEach(func() {
			component = &Component{}
			clusterComp = &dbaasv1alpha1.ClusterComponent{}
			clusterComp.Monitor = true
			clusterDef = &dbaasv1alpha1.ClusterDefinition{}
			clusterDefComp = &dbaasv1alpha1.ClusterDefinitionComponent{}
			clusterDefComp.CharacterType = "mysql"
			clusterDefComp.Monitor = &dbaasv1alpha1.MonitorConfig{
				BuiltInEnable: false,
				Exporter: dbaasv1alpha1.ExporterConfig{
					ScrapePort: 9144,
					ScrapePath: "/metrics",
				},
			}
			clusterDef.Spec.Components = append(clusterDef.Spec.Components, *clusterDefComp)
		})

		It("monitor disable in ClusterComponent", func() {
			clusterComp.Monitor = false
			mergeMonitorConfig(clusterDef, clusterDefComp, clusterComp, component)
			monitorConfig := component.Monitor
			Expect(monitorConfig.Enable).Should(BeFalse())
			Expect(monitorConfig.ScrapePort).To(Equal(0))
			Expect(monitorConfig.ScrapePath).To(Equal(""))
		})

		It("not set CharacterType in ClusterDefinitionComponent", func() {
			clusterDefComp.CharacterType = ""
			mergeMonitorConfig(clusterDef, clusterDefComp, clusterComp, component)
			monitorConfig := component.Monitor
			Expect(monitorConfig.Enable).Should(BeFalse())
			Expect(monitorConfig.ScrapePort).To(Equal(0))
			Expect(monitorConfig.ScrapePath).To(Equal(""))
		})

		It("set wrong CharacterType in ClusterDefinitionComponent", func() {
			clusterDefComp.CharacterType = "fake"
			mergeMonitorConfig(clusterDef, clusterDefComp, clusterComp, component)
			monitorConfig := component.Monitor
			Expect(monitorConfig.Enable).Should(BeFalse())
			Expect(monitorConfig.ScrapePort).To(Equal(0))
			Expect(monitorConfig.ScrapePath).To(Equal(""))
		})

		It("normal case", func() {
			mergeMonitorConfig(clusterDef, clusterDefComp, clusterComp, component)
			monitorConfig := component.Monitor
			Expect(monitorConfig.Enable).Should(BeTrue())
			Expect(monitorConfig.ScrapePort).To(Equal(9144))
			Expect(monitorConfig.ScrapePath).To(Equal("/metrics"))
		})
	})
})
