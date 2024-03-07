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

	ctrl "sigs.k8s.io/controller-runtime"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/graph"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	testapps "github.com/apecloud/kubeblocks/pkg/testutil/apps"
)

var tlog = ctrl.Log.WithName("component_testing")

var _ = Describe("Component PostProvision Test", func() {
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
			By("test component definition without post provision")
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
			Expect(synthesizeComp.LifecycleActions.PostProvision).Should(BeNil())

			comp, err := BuildComponent(cluster, &cluster.Spec.ComponentSpecs[0], nil, nil)
			comp.UID = cluster.UID
			Expect(err).Should(Succeed())
			Expect(comp).ShouldNot(BeNil())

			dag := graph.NewDAG()
			dag.AddVertex(&model.ObjectVertex{Obj: cluster, Action: model.ActionUpdatePtr()})
			err = ReconcileCompPostProvision(testCtx.Ctx, testCtx.Cli, cluster, comp, synthesizeComp, dag)
			Expect(err).Should(Succeed())

			By("build component with postProvision without PodList, do not need to do PostProvision action")
			synthesizeComp.LifecycleActions = &appsv1alpha1.ComponentLifecycleActions{}
			defaultPreCondition := appsv1alpha1.ComponentReadyPreConditionType
			postProvision := appsv1alpha1.LifecycleActionHandler{
				CustomHandler: &appsv1alpha1.Action{
					Image: constant.KBToolsImage,
					Exec: &appsv1alpha1.ExecAction{
						Command: []string{"echo", "mock"},
						Args:    []string{},
					},
					PreCondition: &defaultPreCondition,
				},
			}
			synthesizeComp.LifecycleActions.PostProvision = &postProvision

			By("check built-in envs of cluster component available in postProvision job")
			renderJob, err := renderActionCmdJob(testCtx.Ctx, testCtx.Cli, cluster, synthesizeComp, PostProvisionAction)
			Expect(err).Should(Succeed())
			Expect(renderJob).ShouldNot(BeNil())
			Expect(len(renderJob.Spec.Template.Spec.Containers[0].Env) == 9).Should(BeTrue())
			compListExist := false
			compPodNameListExist := false
			compPodIPListExist := false
			compPodHostNameListExist := false
			compPodHostIPListExist := false
			clusterPodNameListExist := false
			clusterPodIPListExist := false
			clusterPodHostNameListExist := false
			clusterPodHostIPListExist := false
			for _, env := range renderJob.Spec.Template.Spec.Containers[0].Env {
				switch env.Name {
				case kbLifecycleActionClusterCompList:
					compListExist = true
				case kbLifecycleActionClusterCompPodHostIPList:
					compPodHostIPListExist = true
				case kbLifecycleActionClusterCompPodHostNameList:
					compPodHostNameListExist = true
				case kbLifecycleActionClusterCompPodIPList:
					compPodIPListExist = true
				case kbLifecycleActionClusterCompPodNameList:
					compPodNameListExist = true
				case kbLifecycleActionClusterPodHostIPList:
					clusterPodHostIPListExist = true
				case kbLifecycleActionClusterPodHostNameList:
					clusterPodHostNameListExist = true
				case kbLifecycleActionClusterPodIPList:
					clusterPodIPListExist = true
				case kbLifecycleActionClusterPodNameList:
					clusterPodNameListExist = true
				}
			}
			Expect(compListExist).Should(BeTrue())
			Expect(compPodNameListExist).Should(BeTrue())
			Expect(compPodIPListExist).Should(BeTrue())
			Expect(compPodHostNameListExist).Should(BeTrue())
			Expect(compPodHostIPListExist).Should(BeTrue())
			Expect(clusterPodNameListExist).Should(BeTrue())
			Expect(clusterPodIPListExist).Should(BeTrue())
			Expect(clusterPodHostNameListExist).Should(BeTrue())
			Expect(clusterPodHostIPListExist).Should(BeTrue())
		})
	})
})
