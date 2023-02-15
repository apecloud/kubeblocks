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
	ctrl "sigs.k8s.io/controller-runtime"

	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
	testdbaas "github.com/apecloud/kubeblocks/internal/testutil/dbaas"
)

const (
	kFake = "fake"
)

var tlog = ctrl.Log.WithName("component_testing")

var _ = Describe("component module", func() {

	Context("has the mergeMonitorConfig function", func() {
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
			clusterDef.Spec.Type = "state.mysql"
			clusterDefComp = &dbaasv1alpha1.ClusterDefinitionComponent{}
			clusterDefComp.CharacterType = kMysql
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

		It("should disable monitor if ClusterComponent.Monitor is false", func() {
			clusterComp.Monitor = false
			mergeMonitorConfig(cluster, clusterDef, clusterDefComp, clusterComp, component)
			monitorConfig := component.Monitor
			Expect(monitorConfig.Enable).Should(BeFalse())
			Expect(monitorConfig.ScrapePort).To(BeEquivalentTo(0))
			Expect(monitorConfig.ScrapePath).To(Equal(""))
			if component.PodSpec != nil {
				Expect(len(component.PodSpec.Containers)).To(BeEquivalentTo(0))
			}
		})

		It("should disable builtin monitor if ClusterDefinitionComponent.Monitor.BuiltIn is false and has valid ExporterConfig", func() {
			clusterComp.Monitor = true
			clusterDefComp.CharacterType = kFake
			clusterDefComp.Monitor.BuiltIn = false
			mergeMonitorConfig(cluster, clusterDef, clusterDefComp, clusterComp, component)
			monitorConfig := component.Monitor
			Expect(monitorConfig.Enable).Should(BeTrue())
			Expect(monitorConfig.ScrapePort).To(BeEquivalentTo(9144))
			Expect(monitorConfig.ScrapePath).To(Equal("/metrics"))
			if component.PodSpec != nil {
				Expect(len(component.PodSpec.Containers)).To(BeEquivalentTo(0))
			}
		})

		It("should disable monitor if ClusterDefinitionComponent.Monitor.BuiltIn is false and lacks ExporterConfig", func() {
			clusterComp.Monitor = true
			clusterDefComp.CharacterType = kFake
			clusterDefComp.Monitor.BuiltIn = false
			clusterDefComp.Monitor.Exporter = nil
			mergeMonitorConfig(cluster, clusterDef, clusterDefComp, clusterComp, component)
			monitorConfig := component.Monitor
			Expect(monitorConfig.Enable).Should(BeFalse())
			Expect(monitorConfig.ScrapePort).To(BeEquivalentTo(0))
			Expect(monitorConfig.ScrapePath).To(Equal(""))
			if component.PodSpec != nil {
				Expect(len(component.PodSpec.Containers)).To(Equal(0))
			}
		})

		It("should disable monitor if ClusterDefinitionComponent.Monitor.BuiltIn is true and CharacterType isn't recognizable", func() {
			clusterComp.Monitor = true
			clusterDefComp.CharacterType = kFake
			clusterDefComp.Monitor.BuiltIn = true
			clusterDefComp.Monitor.Exporter = nil
			mergeMonitorConfig(cluster, clusterDef, clusterDefComp, clusterComp, component)
			monitorConfig := component.Monitor
			Expect(monitorConfig.Enable).Should(BeFalse())
			Expect(monitorConfig.ScrapePort).To(BeEquivalentTo(0))
			Expect(monitorConfig.ScrapePath).To(Equal(""))
			if component.PodSpec != nil {
				Expect(len(component.PodSpec.Containers)).To(Equal(0))
			}
		})

		It("should disable monitor if ClusterDefinitionComponent's CharacterType is empty", func() {
			// TODO fixme: seems setting clusterDef.Spec.Type has no effect to mergeMonitorConfig
			clusterComp.Monitor = true
			clusterDef.Spec.Type = kFake
			clusterDefComp.CharacterType = ""
			clusterDefComp.Monitor.BuiltIn = true
			clusterDefComp.Monitor.Exporter = nil
			mergeMonitorConfig(cluster, clusterDef, clusterDefComp, clusterComp, component)
			monitorConfig := component.Monitor
			Expect(monitorConfig.Enable).Should(BeFalse())
			Expect(monitorConfig.ScrapePort).To(BeEquivalentTo(0))
			Expect(monitorConfig.ScrapePath).To(Equal(""))
			if component.PodSpec != nil {
				Expect(len(component.PodSpec.Containers)).To(Equal(0))
			}
		})
	})

	Context("has the MergeComponents function", func() {
		const (
			clusterDefName     = "test-clusterdef"
			clusterVersionName = "test-clusterversion"
			clusterName        = "test-cluster"
			mysqlCompType      = "replicasets"
			mysqlCompName      = "mysql"
			nginxCompType      = "proxy"
		)

		var (
			clusterDef     *dbaasv1alpha1.ClusterDefinition
			clusterVersion *dbaasv1alpha1.ClusterVersion
			cluster        *dbaasv1alpha1.Cluster
		)

		BeforeEach(func() {
			clusterDef = testdbaas.NewClusterDefFactory(clusterDefName, testdbaas.MySQLType).
				AddComponent(testdbaas.StatefulMySQLComponent, mysqlCompType).
				AddComponent(testdbaas.StatelessNginxComponent, nginxCompType).
				GetObject()
			clusterVersion = testdbaas.NewClusterVersionFactory(clusterVersionName, clusterDefName).
				AddComponent(mysqlCompType).
				AddContainerShort("mysql", testdbaas.ApeCloudMySQLImage).
				AddComponent(nginxCompType).
				AddInitContainerShort("nginx-init", testdbaas.NginxImage).
				AddContainerShort("nginx", testdbaas.NginxImage).
				GetObject()
			pvcSpec := testdbaas.NewPVC("1Gi")
			cluster = testdbaas.NewClusterFactory(testCtx.DefaultNamespace, clusterName,
				clusterDef.Name, clusterVersion.Name).
				AddComponent(mysqlCompName, mysqlCompType).
				AddVolumeClaimTemplate(testdbaas.DataVolumeName, &pvcSpec).
				GetObject()
		})

		It("should work as expected with various inputs", func() {
			By("assign every available fields")
			reqCtx := intctrlutil.RequestCtx{
				Ctx: ctx,
				Log: tlog,
			}
			component := MergeComponents(
				reqCtx,
				cluster,
				clusterDef,
				&clusterDef.Spec.Components[0],
				&clusterVersion.Spec.Components[0],
				&cluster.Spec.Components[0])
			Expect(component).ShouldNot(BeNil())

			By("leave clusterVersion.podSpec nil")
			clusterVersion.Spec.Components[0].PodSpec = nil
			component = MergeComponents(
				reqCtx,
				cluster,
				clusterDef,
				&clusterDef.Spec.Components[0],
				&clusterVersion.Spec.Components[0],
				&cluster.Spec.Components[0])
			Expect(component).ShouldNot(BeNil())

			By("new container in clusterVersion not in clusterDefinition")
			component = MergeComponents(
				reqCtx,
				cluster,
				clusterDef,
				&clusterDef.Spec.Components[0],
				&clusterVersion.Spec.Components[1],
				&cluster.Spec.Components[0])
			Expect(len(component.PodSpec.Containers)).Should(Equal(2))

			By("new init container in clusterVersion not in clusterDefinition")
			component = MergeComponents(
				reqCtx,
				cluster,
				clusterDef,
				&clusterDef.Spec.Components[0],
				&clusterVersion.Spec.Components[1],
				&cluster.Spec.Components[0])
			Expect(len(component.PodSpec.InitContainers)).Should(Equal(1))

			By("leave clusterComp nil")
			component = MergeComponents(
				reqCtx,
				cluster,
				clusterDef,
				&clusterDef.Spec.Components[0],
				&clusterVersion.Spec.Components[0],
				nil)
			Expect(component).ShouldNot(BeNil())

			By("leave clusterDefComp nil")
			component = MergeComponents(
				reqCtx,
				cluster,
				clusterDef,
				nil,
				&clusterVersion.Spec.Components[0],
				&cluster.Spec.Components[0])
			Expect(component).Should(BeNil())
		})
	})
})
