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
	"strconv"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/graph"
	"github.com/apecloud/kubeblocks/pkg/controller/job"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	testapps "github.com/apecloud/kubeblocks/pkg/testutil/apps"
)

var _ = Describe("Component PostProvision Test", func() {
	Context("has the BuildComponent function", func() {
		const (
			compDefName   = "test-compdef"
			clusterName   = "test-cluster"
			mysqlCompName = "mysql"
		)

		var (
			compDef *appsv1alpha1.ComponentDefinition
			cluster *appsv1alpha1.Cluster
		)

		BeforeEach(func() {
			compDef = testapps.NewComponentDefinitionFactory(compDefName).
				SetDefaultSpec().
				GetObject()
			pvcSpec := testapps.NewPVCSpec("1Gi")
			cluster = testapps.NewClusterFactory(testCtx.DefaultNamespace, clusterName, "").
				SetUID(clusterName).
				AddComponent(mysqlCompName, compDefName).
				AddVolumeClaimTemplate(testapps.DataVolumeName, pvcSpec).
				GetObject()
		})

		mockPodsForTest := func(cluster *appsv1alpha1.Cluster, number int) []corev1.Pod {
			clusterDefName := cluster.Spec.ClusterDefRef
			componentName := cluster.Spec.ComponentSpecs[0].Name
			clusterName := cluster.Name
			stsName := cluster.Name + "-" + componentName
			pods := make([]corev1.Pod, 0)
			for i := 0; i < number; i++ {
				pod := &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      stsName + "-" + strconv.Itoa(i),
						Namespace: testCtx.DefaultNamespace,
						Labels: map[string]string{
							constant.AppManagedByLabelKey:         constant.AppName,
							constant.AppNameLabelKey:              clusterDefName,
							constant.AppInstanceLabelKey:          clusterName,
							constant.KBAppComponentLabelKey:       componentName,
							appsv1.ControllerRevisionHashLabelKey: "mock-version",
						},
					},
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{{
							Name:  "mock-container",
							Image: "mock-container",
						}},
					},
				}
				pods = append(pods, *pod)
			}
			return pods
		}

		It("should work as expected with various inputs", func() {
			By("test component definition without post provision")
			reqCtx := intctrlutil.RequestCtx{
				Ctx: ctx,
				Log: tlog,
			}

			comp, err := BuildComponent(cluster, &cluster.Spec.ComponentSpecs[0], nil, nil)
			Expect(err).Should(Succeed())
			Expect(comp).ShouldNot(BeNil())
			comp.UID = cluster.UID

			synthesizeComp, err := BuildSynthesizedComponent(reqCtx, testCtx.Cli, cluster, compDef, comp)
			Expect(err).Should(Succeed())
			Expect(synthesizeComp).ShouldNot(BeNil())
			Expect(synthesizeComp.LifecycleActions).ShouldNot(BeNil())
			Expect(synthesizeComp.LifecycleActions.PostProvision).Should(BeNil())

			dag := graph.NewDAG()
			dag.AddVertex(&model.ObjectVertex{Obj: cluster, Action: model.ActionUpdatePtr()})

			actionCtx, err := NewActionContext(cluster, comp, nil, synthesizeComp.LifecycleActions, synthesizeComp.ScriptTemplates, PostProvisionAction)
			Expect(err).Should(Succeed())
			err = ReconcileCompPostProvision(testCtx.Ctx, k8sClient, graphCli, actionCtx, dag)
			Expect(err).Should(Succeed())

			By("build component with postProvision without PodList, do not need to do PostProvision action")
			synthesizeComp.LifecycleActions = &appsv1alpha1.ComponentLifecycleActions{}
			defaultPreCondition := appsv1alpha1.ComponentReadyPreConditionType
			postProvision := &appsv1alpha1.Action{
				Exec: &appsv1alpha1.ExecAction{
					Image:   constant.KBToolsImage,
					Command: []string{"echo", "mock"},
					Args:    []string{},
				},
				PreCondition: &defaultPreCondition,
			}
			synthesizeComp.LifecycleActions.PostProvision = postProvision
			actionCtx, err = NewActionContext(cluster, comp, nil, synthesizeComp.LifecycleActions, synthesizeComp.ScriptTemplates, PostProvisionAction)
			Expect(err).Should(Succeed())
			need, err := NeedDoPostProvision(testCtx.Ctx, k8sClient, actionCtx)
			Expect(err).Should(Succeed())
			Expect(need).Should(BeFalse())
			err = ReconcileCompPostProvision(testCtx.Ctx, k8sClient, graphCli, actionCtx, dag)
			Expect(err).Should(Succeed())

			By("mock component status ready, should do postProvision action")
			comp.Status.Phase = appsv1alpha1.RunningClusterCompPhase
			actionCtx.component = comp
			need, err = NeedDoPostProvision(testCtx.Ctx, k8sClient, actionCtx)
			Expect(err).Should(Succeed())
			Expect(need).Should(BeTrue())

			By("build component with postProvision with PodList, do postProvision action and requeue waiting job")
			pods := mockPodsForTest(cluster, 1)
			for _, pod := range pods {
				Expect(testCtx.CheckedCreateObj(testCtx.Ctx, &pod)).Should(Succeed())
				pod.Status.Conditions = []corev1.PodCondition{{
					Type:   corev1.PodReady,
					Status: corev1.ConditionTrue,
				}}
				Expect(k8sClient.Status().Update(ctx, &pod)).Should(Succeed())
			}
			err = ReconcileCompPostProvision(testCtx.Ctx, k8sClient, graphCli, actionCtx, dag)
			Expect(err).ShouldNot(Succeed())
			Expect(err.Error()).Should(ContainSubstring("job not exist, pls check"))

			jobName, _ := genActionJobName(synthesizeComp.FullCompName, PostProvisionAction)
			err = job.CheckJobSucceed(testCtx.Ctx, testCtx.Cli, cluster, jobName)
			Expect(err).ShouldNot(Succeed())
			Expect(err.Error()).Should(ContainSubstring("job not exist, pls check"))
		})
	})
})
