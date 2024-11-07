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

package apps

import (
	kbappsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	testk8s "github.com/apecloud/kubeblocks/pkg/testutil/k8s"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/util/sets"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	"github.com/apecloud/kubeblocks/pkg/controller/graph"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	testapps "github.com/apecloud/kubeblocks/pkg/testutil/apps"
)

var _ = Describe("Component Workload Operations Test", func() {
	const (
		clusterName    = "test-cluster"
		compName       = "test-comp"
		kubeblocksName = "kubeblocks"
	)

	var (
		reader         *mockReader
		dag            *graph.DAG
		synthesizeComp *component.SynthesizedComponent
	)

	newDAG := func(graphCli model.GraphClient, comp *appsv1alpha1.Component) *graph.DAG {
		d := graph.NewDAG()
		graphCli.Root(d, comp, comp, model.ActionStatusPtr())
		return d
	}

	BeforeEach(func() {
		reader = &mockReader{}
		comp := &appsv1alpha1.Component{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: testCtx.DefaultNamespace,
				Name:      constant.GenerateClusterComponentName(clusterName, compName),
				Labels: map[string]string{
					constant.AppManagedByLabelKey:   constant.AppName,
					constant.AppInstanceLabelKey:    clusterName,
					constant.KBAppComponentLabelKey: compName,
				},
			},
			Spec: appsv1alpha1.ComponentSpec{},
		}

		synthesizeComp = &component.SynthesizedComponent{
			Namespace:   testCtx.DefaultNamespace,
			ClusterName: clusterName,
			Name:        compName,
			Roles: []kbappsv1.ReplicaRole{
				{Name: "leader", Serviceable: true, Writable: true, Votable: true},
				{Name: "follower", Serviceable: false, Writable: false, Votable: false},
			},
			LifecycleActions: &kbappsv1.ComponentLifecycleActions{
				MemberJoin: &kbappsv1.Action{
					Exec: &kbappsv1.ExecAction{
						Image: "test-image",
					},
				},
				MemberLeave: &kbappsv1.Action{
					Exec: &kbappsv1.ExecAction{
						Image: "test-image",
					},
				},
				Switchover: &kbappsv1.Action{
					Exec: &kbappsv1.ExecAction{
						Image: "test-image",
					},
				},
			},
		}

		graphCli := model.NewGraphClient(reader)
		dag = newDAG(graphCli, comp)
	})

	Context("Member Leave Operations", func() {
		var (
			ops  *componentWorkloadOps
			pod0 *corev1.Pod
			pod1 *corev1.Pod
			pods []*corev1.Pod
		)

		BeforeEach(func() {
			pod0 = testapps.NewPodFactory(testCtx.DefaultNamespace, "test-pod-0").
				AddAnnotations(constant.MemberJoinStatusAnnotationKey, "test-pod").
				AddContainer(corev1.Container{
					Image: "test-image",
					Name:  "test-container",
				}).
				AddLabels(
					constant.AppManagedByLabelKey, kubeblocksName,
					constant.AppInstanceLabelKey, clusterName,
					constant.KBAppComponentLabelKey, compName,
				).
				GetObject()

			pod1 = testapps.NewPodFactory(testCtx.DefaultNamespace, "test-pod-1").
				AddAnnotations(constant.MemberJoinStatusAnnotationKey, "test-pod").
				AddContainer(corev1.Container{
					Image: "test-image",
					Name:  "test-container",
				}).
				AddLabels(
					constant.AppManagedByLabelKey, kubeblocksName,
					constant.AppInstanceLabelKey, clusterName,
					constant.KBAppComponentLabelKey, compName,
				).
				GetObject()

			pods = []*corev1.Pod{}
			pods = append(pods, pod0)
			pods = append(pods, pod1)

			container := corev1.Container{
				Name:            "mock-container-name",
				Image:           testapps.ApeCloudMySQLImage,
				ImagePullPolicy: corev1.PullIfNotPresent,
			}

			mockITS := testapps.NewInstanceSetFactory(testCtx.DefaultNamespace,
				"test-its", clusterName, compName).
				AddFinalizers([]string{constant.DBClusterFinalizerName}).
				AddContainer(container).
				AddAppInstanceLabel(clusterName).
				AddAppComponentLabel(compName).
				AddAppManagedByLabel().
				AddAnnotations(constant.MemberJoinStatusAnnotationKey, "").
				SetReplicas(2).
				SetRoles([]workloads.ReplicaRole{
					{Name: "leader", AccessMode: workloads.ReadWriteMode, CanVote: true, IsLeader: true},
					{Name: "follower", AccessMode: workloads.ReadonlyMode, CanVote: true, IsLeader: false},
				}).
				Create(&testCtx).GetObject()

			mockCluster := testapps.NewClusterFactory(testCtx.DefaultNamespace, "test-cluster", "test-def").
				Create(&testCtx).GetObject()

			ops = &componentWorkloadOps{
				cli:            k8sClient,
				reqCtx:         intctrlutil.RequestCtx{Ctx: ctx, Log: logger},
				synthesizeComp: synthesizeComp,
				cluster:        mockCluster,
				runningITS:     mockITS,
				protoITS:       mockITS,
				dag:            dag,
			}

			testapps.MockKBAgentClient4Workload(&testCtx, pods)
		})

		It("should handle member leave process correctly", func() {
			for _, pod := range pods {
				Expect(ops.cli.Create(ctx, pod)).Should(BeNil())
			}

			ops.desiredCompPodNameSet = make(sets.Set[string])
			ops.desiredCompPodNameSet.Insert(pod0.Name)

			By("setting up member join status")
			ops.runningITS.Annotations[constant.MemberJoinStatusAnnotationKey] = ""

			By("executing leave member operation")
			err := ops.leaveMember4ScaleIn()
			Expect(err).Should(BeNil())
			Expect(pod0.Labels["test.kubeblock.io/memberleave-completed"]).Should(Equal(""))
			Expect(pod1.Labels["test.kubeblock.io/memberleave-completed"]).ShouldNot(Equal(""))

		})

		It("should return requeueError when exec memberleave with memberjoin processing ", func() {
			for _, pod := range pods {
				Expect(ops.cli.Create(ctx, pod)).Should(BeNil())
			}

			ops.desiredCompPodNameSet = make(sets.Set[string])
			ops.desiredCompPodNameSet.Insert(pod0.Name)

			By("setting up member join status")
			ops.runningITS.Annotations[constant.MemberJoinStatusAnnotationKey] = pod1.Name

			By("executing leave member operation")
			err := ops.leaveMember4ScaleIn()
			Expect(err).ShouldNot(BeNil())
			Expect(pod0.Labels["test.kubeblock.io/memberleave-completed"]).Should(Equal(""))
			Expect(pod1.Labels["test.kubeblock.io/memberleave-completed"]).Should(Equal(""))

		})

		It("should handle switchover for leader pod", func() {
			By("setting up leader pod")
			pod1.Labels[constant.RoleLabelKey] = "follower"
			pod1.Labels[constant.RoleLabelKey] = "leader"

			for _, pod := range pods {
				Expect(ops.cli.Create(ctx, pod)).Should(BeNil())
			}

			ops.desiredCompPodNameSet = make(sets.Set[string])
			ops.desiredCompPodNameSet.Insert(pod0.Name)

			By("executing leave member for leader")
			err := ops.leaveMemberForPod(pod1, []*corev1.Pod{pod1})
			Expect(err).ShouldNot(BeNil())
			Expect(pod0.Labels[constant.RoleLabelKey]).Should(Equal("leader"))
			Expect(pod1.Labels[constant.RoleLabelKey]).ShouldNot(Equal("leader"))
		})

		It("should handle member join process correctly", func() {

			for _, pod := range pods {
				Expect(ops.cli.Create(ctx, pod)).Should(BeNil())
			}

			ops.desiredCompPodNameSet = make(sets.Set[string])
			ops.desiredCompPodNameSet.Insert(pod0.Name)

			By("setting up pod status")
			ops.runningITS.Annotations[constant.MemberJoinStatusAnnotationKey] = pod1.Name
			testk8s.MockPodIsRunning(ctx, testCtx, pod1)

			By("executing leave member operation")
			err := ops.checkAndDoMemberJoin()
			Expect(err).Should(BeNil())
			Expect(pod0.Labels["test.kubeblock.io/memberjoin-completed"]).Should(Equal(""))
			Expect(pod1.Labels["test.kubeblock.io/memberjoin-completed"]).ShouldNot(Equal(""))
			Expect(ops.protoITS.Annotations[constant.MemberJoinStatusAnnotationKey]).Should(Equal(""))
		})

		It("should annotate instance for member join correctly", func() {
			Expect(ops.cli.Create(ctx, pod0)).Should(BeNil())

			ops.desiredCompPodNameSet = make(sets.Set[string])
			ops.desiredCompPodNameSet.Insert(pod0.Name)
			ops.desiredCompPodNameSet.Insert(pod1.Name)

			ops.runningItsPodNameSet = make(sets.Set[string])
			ops.runningItsPodNameSet.Insert(pod0.Name)

			ops.annotateInstanceSetForMemberJoin()

			Expect(ops.protoITS.Annotations[constant.MemberJoinStatusAnnotationKey]).Should(Equal(pod1.Name))
		})
	})
})
