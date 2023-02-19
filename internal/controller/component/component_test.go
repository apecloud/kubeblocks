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

	ctrl "sigs.k8s.io/controller-runtime"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
	testapps "github.com/apecloud/kubeblocks/internal/testutil/apps"
)

const (
	kFake = "fake"
)

var tlog = ctrl.Log.WithName("component_testing")

var _ = Describe("component module", func() {

	Context("has the BuildComponent function", func() {
		const (
			clusterDefName     = "test-clusterdef"
			clusterVersionName = "test-clusterversion"
			clusterName        = "test-cluster"
			mysqlCompType      = "replicasets"
			mysqlCompName      = "mysql"
			nginxCompType      = "proxy"
		)

		var (
			clusterDef     *appsv1alpha1.ClusterDefinition
			clusterVersion *appsv1alpha1.ClusterVersion
			cluster        *appsv1alpha1.Cluster
		)

		BeforeEach(func() {
			clusterDef = testapps.NewClusterDefFactory(clusterDefName).
				AddComponent(testapps.StatefulMySQLComponent, mysqlCompType).
				AddComponent(testapps.StatelessNginxComponent, nginxCompType).
				GetObject()
			clusterVersion = testapps.NewClusterVersionFactory(clusterVersionName, clusterDefName).
				AddComponent(mysqlCompType).
				AddContainerShort("mysql", testapps.ApeCloudMySQLImage).
				AddComponent(nginxCompType).
				AddInitContainerShort("nginx-init", testapps.NginxImage).
				AddContainerShort("nginx", testapps.NginxImage).
				GetObject()
			pvcSpec := testapps.NewPVC("1Gi")
			cluster = testapps.NewClusterFactory(testCtx.DefaultNamespace, clusterName,
				clusterDef.Name, clusterVersion.Name).
				AddComponent(mysqlCompName, mysqlCompType).
				AddVolumeClaimTemplate(testapps.DataVolumeName, &pvcSpec).
				GetObject()
		})

		It("should work as expected with various inputs", func() {
			By("assign every available fields")
			reqCtx := intctrlutil.RequestCtx{
				Ctx: ctx,
				Log: tlog,
			}
			component := BuildComponent(
				reqCtx,
				cluster,
				clusterDef,
				&clusterDef.Spec.ComponentDefs[0],
				&clusterVersion.Spec.ComponentVersions[0],
				&cluster.Spec.ComponentSpecs[0])
			Expect(component).ShouldNot(BeNil())

			By("leave clusterVersion.podSpec nil")
			clusterVersion.Spec.ComponentVersions[0].PodSpec = nil
			component = BuildComponent(
				reqCtx,
				cluster,
				clusterDef,
				&clusterDef.Spec.ComponentDefs[0],
				&clusterVersion.Spec.ComponentVersions[0],
				&cluster.Spec.ComponentSpecs[0])
			Expect(component).ShouldNot(BeNil())

			By("new container in clusterVersion not in clusterDefinition")
			component = BuildComponent(
				reqCtx,
				cluster,
				clusterDef,
				&clusterDef.Spec.ComponentDefs[0],
				&clusterVersion.Spec.ComponentVersions[1],
				&cluster.Spec.ComponentSpecs[0])
			Expect(len(component.PodSpec.Containers)).Should(Equal(2))

			By("new init container in clusterVersion not in clusterDefinition")
			component = BuildComponent(
				reqCtx,
				cluster,
				clusterDef,
				&clusterDef.Spec.ComponentDefs[0],
				&clusterVersion.Spec.ComponentVersions[1],
				&cluster.Spec.ComponentSpecs[0])
			Expect(len(component.PodSpec.InitContainers)).Should(Equal(1))

			By("leave clusterComp nil")
			component = BuildComponent(
				reqCtx,
				cluster,
				clusterDef,
				&clusterDef.Spec.ComponentDefs[0],
				&clusterVersion.Spec.ComponentVersions[0],
				nil)
			Expect(component).ShouldNot(BeNil())

			By("leave clusterCompDef nil")
			component = BuildComponent(
				reqCtx,
				cluster,
				clusterDef,
				nil,
				&clusterVersion.Spec.ComponentVersions[0],
				&cluster.Spec.ComponentSpecs[0])
			Expect(component).Should(BeNil())
		})
	})
})
