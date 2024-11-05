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
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

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
		clusterName = "test-cluster"
		compName    = "test-component"
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
		}

		graphCli := model.NewGraphClient(reader)
		dag = newDAG(graphCli, comp)
	})

	Context("TransformerError", func() {
		It("should handle error creation and checking", func() {
			// create test errors
			testErrs := []error{
				fmt.Errorf("test error 1"),
				fmt.Errorf("test error 2"),
			}
			requeueAfter := 5 * time.Second

			By("creating TransformerError")
			batchErr := NewTransformerError(testErrs, requeueAfter)
			Expect(batchErr).ShouldNot(BeNil())
			Expect(batchErr.Error()).Should(And(
				ContainSubstring("test error 1"),
				ContainSubstring("test error 2"),
			))

			By("checking error type and requeue duration")
			duration, ok := IsTransformerError(batchErr)
			Expect(ok).Should(BeTrue())
			Expect(duration).Should(Equal(requeueAfter))

			By("handling nil errors")
			Expect(NewTransformerError(nil, requeueAfter)).Should(BeNil())
			Expect(NewTransformerError([]error{}, requeueAfter)).Should(BeNil())
		})
	})

	Context("Member Leave Operations", func() {
		var (
			ops *componentWorkloadOps
			pod *corev1.Pod
		)

		BeforeEach(func() {
			pod = testapps.NewPodFactory(testCtx.DefaultNamespace, "test-pod").
				AddAnnotations(constant.MemberJoinStatusAnnotationKey, "test-pod").
				GetObject()

			ops = &componentWorkloadOps{
				cli:            k8sClient,
				reqCtx:         intctrlutil.RequestCtx{Ctx: ctx, Log: logger},
				synthesizeComp: synthesizeComp,
				dag:            dag,
			}
		})

		It("should handle member join process correctly", func() {
			By("setting up member join status")
			ops.runningITS.Annotations[constant.MemberJoinStatusAnnotationKey] = pod.Name

			By("executing leave member operation")
			err := ops.leaveMember4ScaleIn()
			Expect(err).ShouldNot(BeNil())

			By("verifying error type and message")
			reqErr, ok := err.(*TransformerError)
			Expect(ok).Should(BeTrue())
			Expect(reqErr.Error()).Should(ContainSubstring("is in memberjoin process"))
		})

		It("should handle lorry client errors", func() {
			By("creating test pod")
			Expect(k8sClient.Create(ctx, pod)).Should(Succeed())

			By("executing leave member with NotImplemented error")
			err := ops.leaveMemberForPod(pod, []*corev1.Pod{pod})
			Expect(err).Should(BeNil())
		})

		It("should handle switchover for leader pod", func() {
			By("setting up leader pod")
			pod.Labels[constant.RoleLabelKey] = "leader"
			Expect(k8sClient.Create(ctx, pod)).Should(Succeed())

			By("executing leave member for leader")
			err := ops.leaveMemberForPod(pod, []*corev1.Pod{pod})
			Expect(err).ShouldNot(BeNil())
			Expect(err.Error()).Should(ContainSubstring("wait role label to be updated"))
		})
	})
})
