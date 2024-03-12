/*
Copyright (C) 2022-2024 ApeCloud Co., Ltd

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
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/graph"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	testapps "github.com/apecloud/kubeblocks/pkg/testutil/apps"
)

var _ = Describe("Component PreTerminate Test", func() {
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
				SetUID(clusterName).
				AddComponent(mysqlCompName, mysqlCompDefName).
				AddVolumeClaimTemplate(testapps.DataVolumeName, pvcSpec).
				GetObject()
		})

		It("should work as expected with various inputs", func() {
			By("test component definition without pre terminate")
			reqCtx := intctrlutil.RequestCtx{
				Ctx: ctx,
				Log: tlog,
			}
			synthesizeComp, err := BuildSynthesizedComponentWrapper4Test(
				reqCtx,
				testCtx.Cli,
				clusterDef,
				clusterVersion,
				cluster,
				&cluster.Spec.ComponentSpecs[0])
			Expect(err).Should(Succeed())
			Expect(synthesizeComp).ShouldNot(BeNil())
			Expect(synthesizeComp.LifecycleActions).ShouldNot(BeNil())
			Expect(synthesizeComp.LifecycleActions.PreTerminate).Should(BeNil())

			comp, err := BuildComponent(cluster, &cluster.Spec.ComponentSpecs[0], nil, nil)
			comp.UID = cluster.UID
			Expect(err).Should(Succeed())
			Expect(comp).ShouldNot(BeNil())

			By("test component without preTerminate action and no need to do PreTerminate action")
			dag := graph.NewDAG()
			dag.AddVertex(&model.ObjectVertex{Obj: cluster, Action: model.ActionUpdatePtr()})
			need, err := needDoPreTerminate(testCtx.Ctx, testCtx.Cli, cluster, comp, synthesizeComp)
			Expect(err).Should(Succeed())
			Expect(need).Should(BeFalse())
			err = reconcileCompPreTerminate(testCtx.Ctx, testCtx.Cli, cluster, comp, synthesizeComp, dag)
			Expect(err).Should(Succeed())

			By("build component with preTerminate action and should do PreTerminate action")
			synthesizeComp.LifecycleActions = &appsv1alpha1.ComponentLifecycleActions{}
			PreTerminate := appsv1alpha1.LifecycleActionHandler{
				CustomHandler: &appsv1alpha1.Action{
					Image: constant.KBToolsImage,
					Exec: &appsv1alpha1.ExecAction{
						Command: []string{"echo", "mock"},
						Args:    []string{},
					},
				},
			}
			synthesizeComp.LifecycleActions.PreTerminate = &PreTerminate
			need, err = needDoPreTerminate(testCtx.Ctx, testCtx.Cli, cluster, comp, synthesizeComp)
			Expect(err).Should(Succeed())
			Expect(need).Should(BeTrue())
			err = reconcileCompPreTerminate(testCtx.Ctx, testCtx.Cli, cluster, comp, synthesizeComp, dag)
			Expect(err).Should(Succeed())

			By("requeue to waiting for job")
			jobName, _ := genActionJobName(cluster.Name, synthesizeComp.Name, PreTerminateAction)
			err = CheckJobSucceed(testCtx.Ctx, testCtx.Cli, cluster, jobName)
			Expect(err).ShouldNot(Succeed())
			Expect(err.Error()).Should(ContainSubstring("requeue to waiting for job"))
		})
	})
})
