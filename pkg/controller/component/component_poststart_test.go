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

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	testapps "github.com/apecloud/kubeblocks/pkg/testutil/apps"
)

var _ = Describe("Component PostStart Test", func() {
	Context("has the BuildComponent function", func() {
		const (
			clusterDefName     = "test-clusterdef"
			clusterVersionName = "test-clusterversion"
			clusterName        = "test-cluster"
			mysqlCompDefName   = "replicasets"
			mysqlCompName      = "mysql"
		)

		var (
			clusterDef     *appsv1alpha1.ClusterDefinition
			clusterVersion *appsv1alpha1.ClusterVersion
			cluster        *appsv1alpha1.Cluster
		)

		BeforeEach(func() {
			clusterDef = testapps.NewClusterDefFactory(clusterDefName).
				AddComponentDef(testapps.StatefulMySQLComponent, mysqlCompDefName).
				GetObject()
			clusterVersion = testapps.NewClusterVersionFactory(clusterVersionName, clusterDefName).
				AddComponentVersion(mysqlCompDefName).
				AddContainerShort("mysql", testapps.ApeCloudMySQLImage).
				GetObject()
			pvcSpec := testapps.NewPVCSpec("1Gi")
			cluster = testapps.NewClusterFactory(testCtx.DefaultNamespace, clusterName,
				clusterDef.Name, clusterVersion.Name).
				AddComponent(mysqlCompName, mysqlCompDefName).
				AddVolumeClaimTemplate(testapps.DataVolumeName, pvcSpec).
				GetObject()
		})

		It("should work as expected with various inputs", func() {
			By("test component definition without poststart")
			reqCtx := intctrlutil.RequestCtx{
				Ctx: ctx,
				Log: tlog,
			}
			component, err := BuildComponent(
				reqCtx,
				nil,
				cluster,
				clusterDef,
				&clusterDef.Spec.ComponentDefs[0],
				&cluster.Spec.ComponentSpecs[0],
				nil,
				&clusterVersion.Spec.ComponentVersions[0])
			Expect(err).Should(Succeed())
			Expect(component).ShouldNot(BeNil())
			Expect(component.PostStartSpec).Should(BeNil())

			err = ReconcileCompPostStart(testCtx.Ctx, testCtx.Cli, cluster, component)
			Expect(err).Should(Succeed())

			By("build component with poststartSpec without RSM")
			commandExecutorEnvItem := &appsv1alpha1.CommandExecutorEnvItem{
				Image: testapps.ApeCloudMySQLImage,
			}
			commandExecutorItem := &appsv1alpha1.CommandExecutorItem{
				Command: []string{"echo", "hello"},
				Args:    []string{},
			}
			postStartSpec := &appsv1alpha1.PostStartAction{
				CmdExecutorConfig: appsv1alpha1.CmdExecutorConfig{
					CommandExecutorEnvItem: *commandExecutorEnvItem,
					CommandExecutorItem:    *commandExecutorItem,
				},
			}
			component.PostStartSpec = postStartSpec
			err = ReconcileCompPostStart(testCtx.Ctx, testCtx.Cli, cluster, component)
			Expect(err).ShouldNot(Succeed())
		})
	})
})
