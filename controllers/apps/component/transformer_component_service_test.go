/*
Copyright (C) 2022-2026 ApeCloud Co., Ltd

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
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	workloadsv1 "github.com/apecloud/kubeblocks/apis/workloads/v1"
	appsutil "github.com/apecloud/kubeblocks/controllers/apps/util"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	"github.com/apecloud/kubeblocks/pkg/controller/graph"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
	"github.com/apecloud/kubeblocks/pkg/controller/multicluster"
	"github.com/apecloud/kubeblocks/pkg/controllerutil"
	kbacli "github.com/apecloud/kubeblocks/pkg/kbagent/client"
	kbagentproto "github.com/apecloud/kubeblocks/pkg/kbagent/proto"
	testapps "github.com/apecloud/kubeblocks/pkg/testutil/apps"
)

var _ = Describe("component service transformer test", func() {
	const (
		clusterName = "test-cluster"
		compName    = "comp"
	)

	var (
		reader   *appsutil.MockReader
		dag      *graph.DAG
		transCtx *componentTransformContext
	)

	newDAG := func(graphCli model.GraphClient, comp *appsv1.Component) *graph.DAG {
		d := graph.NewDAG()
		graphCli.Root(d, comp, comp, model.ActionStatusPtr())
		return d
	}

	BeforeEach(func() {
		reader = &appsutil.MockReader{}
		comp := &appsv1.Component{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: testCtx.DefaultNamespace,
				Name:      constant.GenerateClusterComponentName(clusterName, compName),
				Labels: map[string]string{
					constant.AppManagedByLabelKey:   constant.AppName,
					constant.AppInstanceLabelKey:    clusterName,
					constant.KBAppComponentLabelKey: compName,
				},
			},
			Spec: appsv1.ComponentSpec{},
		}
		graphCli := model.NewGraphClient(reader)
		dag = newDAG(graphCli, comp)
		transCtx = &componentTransformContext{
			Context:       ctx,
			Client:        graphCli,
			EventRecorder: nil,
			Logger:        logger,
			Component:     comp,
			ComponentOrig: comp.DeepCopy(),
			SynthesizeComponent: &component.SynthesizedComponent{
				Namespace:   testCtx.DefaultNamespace,
				ClusterName: clusterName,
				Name:        compName,
				ComponentServices: []appsv1.ComponentService{
					{
						Service: appsv1.Service{
							Name:        "default",
							ServiceName: "default",
						},
					},
				},
				PodSpec:      &corev1.PodSpec{},
				Replicas:     3,
				FullCompName: constant.GenerateClusterComponentName(clusterName, compName),
			},
			RunningWorkload: &workloadsv1.InstanceSet{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: testCtx.DefaultNamespace,
					Name:      constant.GenerateClusterComponentName(clusterName, compName),
				},
				Spec: workloadsv1.InstanceSetSpec{
					Replicas: ptr.To[int32](3),
				},
			},
		}
	})

	AfterEach(func() {})

	truep := func() *bool { t := true; return &t }
	falsep := func() *bool { f := false; return &f }

	podServiceName := func(ordinal int32) string {
		return fmt.Sprintf("%s-%d", constant.GenerateComponentServiceName(clusterName, compName, "default"), ordinal)
	}

	podService := func(ordinal int32) *corev1.Service {
		svc := &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: transCtx.Component.Namespace,
				Name:      podServiceName(ordinal),
				Labels:    transCtx.Component.Labels,
			},
		}
		err := controllerutil.SetOwnerReference(transCtx.Component, svc)
		Expect(err).Should(BeNil())
		return svc
	}

	Context("pod service", func() {
		BeforeEach(func() {
			// set as pod service
			transCtx.SynthesizeComponent.ComponentServices[0].PodService = truep()
		})

		newPod := func(suffix string) *corev1.Pod {
			return &corev1.Pod{ObjectMeta: metav1.ObjectMeta{
				Namespace: transCtx.Component.Namespace,
				Name:      transCtx.Component.Name + "-" + suffix,
				Labels:    transCtx.Component.Labels,
			}}
		}

		It("retains pod services until memberLeave succeeds and the Pod disappears", func() {
			pod0, pod1 := newPod("0"), newPod("1")
			svc0, svc1 := podService(0), podService(1)
			cli := fake.NewClientBuilder().WithScheme(k8sClient.Scheme()).WithObjects(pod0, pod1, svc0, svc1).Build()
			graphCli := model.NewGraphClient(cli)
			transCtx.Client = graphCli
			transCtx.EventRecorder = &capturingEventRecorder{}
			transCtx.SynthesizeComponent.Replicas = 2
			transCtx.SynthesizeComponent.LifecycleActions.ComponentLifecycleActions = &appsv1.ComponentLifecycleActions{
				MemberLeave: &appsv1.Action{Exec: &appsv1.ExecAction{Image: "test-image"}},
			}
			var err error
			transCtx.RunningWorkload, err = component.BuildInstanceSet(transCtx.SynthesizeComponent, nil)
			Expect(err).NotTo(HaveOccurred())
			Expect(component.StatusReplicasStatus(transCtx.RunningWorkload, []string{pod0.Name, pod1.Name}, true, false)).To(Succeed())
			transCtx.SynthesizeComponent.Replicas = 1

			allowLeave := false
			testapps.MockKBAgentClient(func(recorder *kbacli.MockClientMockRecorder) {
				recorder.Action(gomock.Any(), gomock.Any()).Times(2).DoAndReturn(func(_ context.Context, req kbagentproto.ActionRequest) (kbagentproto.ActionResponse, error) {
					Expect(req.Action).To(Equal("memberLeave"))
					Expect(req.Parameters["KB_LEAVE_MEMBER_POD_NAME"]).To(Equal(pod1.Name))
					if !allowLeave {
						return kbagentproto.ActionResponse{Error: kbagentproto.Error2Type(kbagentproto.ErrFailed), Message: "leave blocked"}, nil
					}
					return kbagentproto.ActionResponse{}, nil
				})
			})
			DeferCleanup(func() { kbacli.SetMockClient(nil, nil) })
			chain := graph.TransformerChain{&componentServiceTransformer{}, &componentWorkloadTransformer{Client: cli}}
			reconcile := func() error {
				dag = newDAG(graphCli, transCtx.Component)
				return chain.ApplyTo(transCtx, dag)
			}

			By("executing memberLeave despite deferred Service cleanup, and retaining the workload on failure")
			err = reconcile()
			Expect(controllerutil.IsRequeueError(err)).To(BeTrue())
			Expect(err.Error()).To(ContainSubstring("leave blocked"))
			Expect(graphCli.IsAction(dag, svc1, model.ActionDeletePtr())).To(BeFalse())
			Expect(graphCli.FindAll(dag, &corev1.Service{})).To(HaveLen(2))
			Expect(graphCli.FindAll(dag, &workloadsv1.InstanceSet{})).To(BeEmpty())

			By("allowing the workload update after memberLeave succeeds")
			allowLeave = true
			Expect(controllerutil.IsDelayedRequeueError(reconcile())).To(BeTrue())
			Expect(graphCli.IsAction(dag, svc1, model.ActionDeletePtr())).To(BeFalse())
			workloads := graphCli.FindAll(dag, &workloadsv1.InstanceSet{})
			Expect(workloads).To(HaveLen(1))
			updated := workloads[0].(*workloadsv1.InstanceSet)
			Expect(*updated.Spec.Replicas).To(Equal(int32(1)))
			Expect(graphCli.IsAction(dag, updated, model.ActionUpdatePtr())).To(BeTrue())
			transCtx.RunningWorkload = updated

			By("retaining the address after the ITS update while the Pod still exists")
			Expect(controllerutil.IsDelayedRequeueError(reconcile())).To(BeTrue())
			Expect(graphCli.IsAction(dag, svc1, model.ActionDeletePtr())).To(BeFalse())

			By("retaining the address while the Pod is terminating")
			Expect(cli.Get(ctx, client.ObjectKeyFromObject(pod1), pod1)).To(Succeed())
			pod1.Finalizers = []string{"test.kubeblocks.io/hold"}
			Expect(cli.Update(ctx, pod1)).To(Succeed())
			Expect(cli.Delete(ctx, pod1)).To(Succeed())
			Expect(controllerutil.IsDelayedRequeueError(reconcile())).To(BeTrue())
			Expect(graphCli.IsAction(dag, svc1, model.ActionDeletePtr())).To(BeFalse())

			By("cleaning up only after the Pod object disappears")
			Expect(cli.Get(ctx, client.ObjectKeyFromObject(pod1), pod1)).To(Succeed())
			pod1.Finalizers = nil
			Expect(cli.Update(ctx, pod1)).To(Succeed())
			Expect(reconcile()).To(Succeed())
			Expect(graphCli.IsAction(dag, svc1, model.ActionDeletePtr())).To(BeTrue())
			Expect(graphCli.IsAction(dag, svc0, model.ActionDeletePtr())).To(BeFalse())
		})

		It("recreates services for current replicas even when their Pods are temporarily absent", func() {
			transCtx.SynthesizeComponent.Replicas = 1
			err := (&componentServiceTransformer{}).Transform(transCtx, dag)
			Expect(controllerutil.IsDelayedRequeueError(err)).To(BeTrue())
			graphCli := transCtx.Client.(model.GraphClient)
			Expect(graphCli.FindAll(dag, &corev1.Service{})).To(HaveLen(3))
			Expect(graphCli.IsAction(dag, podService(2), model.ActionCreatePtr())).To(BeTrue())
		})

		DescribeTable("precreates services before Pods exist",
			func(currentReplicas *int32) {
				if currentReplicas == nil {
					transCtx.RunningWorkload = nil
				} else {
					transCtx.RunningWorkload.Spec.Replicas = currentReplicas
				}
				Expect((&componentServiceTransformer{}).Transform(transCtx, dag)).To(Succeed())
				graphCli := transCtx.Client.(model.GraphClient)
				Expect(graphCli.FindAll(dag, &corev1.Service{})).To(HaveLen(3))
				Expect(graphCli.IsAction(dag, podService(2), model.ActionCreatePtr())).To(BeTrue())
			},
			Entry("on creation", nil),
			Entry("on scale-out", ptr.To[int32](1)),
		)

		It("retains named and offline instances by name instead of replica count", func() {
			transCtx.SynthesizeComponent.Replicas = 1
			transCtx.SynthesizeComponent.Instances = []appsv1.InstanceTemplate{{Name: "reader", Replicas: ptr.To[int32](1)}}
			transCtx.RunningWorkload.Spec.Instances = []workloadsv1.InstanceTemplate{{Name: "reader", Replicas: ptr.To[int32](2)}}
			transCtx.RunningWorkload.Spec.Replicas = ptr.To[int32](2)
			transCtx.SynthesizeComponent.OfflineInstances = []string{newPod("reader-0").Name}
			reader.Objects = append(reader.Objects, newPod("reader-7"))
			err := (&componentServiceTransformer{}).Transform(transCtx, dag)
			Expect(controllerutil.IsDelayedRequeueError(err)).To(BeTrue())
			graphCli := transCtx.Client.(model.GraphClient)
			services := graphCli.FindAll(dag, &corev1.Service{})
			selectors := make([]string, 0, len(services))
			for _, obj := range services {
				svc := obj.(*corev1.Service)
				selectors = append(selectors, svc.Spec.Selector[constant.KBAppPodNameLabelKey])
			}
			Expect(selectors).To(ConsistOf(newPod("reader-0").Name, newPod("reader-1").Name, newPod("reader-7").Name))
		})

		It("retains services across changes to non-contiguous ordinals", func() {
			transCtx.SynthesizeComponent.Replicas = 2
			transCtx.SynthesizeComponent.Ordinals = appsv1.Ordinals{Discrete: []int32{2, 8}}
			transCtx.RunningWorkload.Spec.Replicas = ptr.To[int32](2)
			transCtx.RunningWorkload.Spec.Ordinals = appsv1.Ordinals{Discrete: []int32{2, 5}}
			reader.Objects = append(reader.Objects, newPod("11"))
			Expect(controllerutil.IsDelayedRequeueError((&componentServiceTransformer{}).Transform(transCtx, dag))).To(BeTrue())
			graphCli := transCtx.Client.(model.GraphClient)
			services := graphCli.FindAll(dag, &corev1.Service{})
			names := make([]string, 0, len(services))
			for _, svc := range services {
				names = append(names, svc.GetName())
			}
			Expect(names).To(ConsistOf(podServiceName(2), podServiceName(5), podServiceName(8), podServiceName(11)))
		})

		It("queries Pods in the data placement and retains their Service assistant objects", func() {
			transCtx.SynthesizeComponent.Replicas = 1
			transCtx.RunningWorkload.Spec.Replicas = ptr.To[int32](1)
			transCtx.RunningWorkload.Annotations = map[string]string{constant.KBAppMultiClusterPlacementKey: "test-context"}
			transCtx.SynthesizeComponent.EnableInstanceAPI = ptr.To(true)
			transCtx.SynthesizeComponent.Annotations = transCtx.RunningWorkload.Annotations
			listedPods := false
			cli := fake.NewClientBuilder().WithScheme(k8sClient.Scheme()).WithObjects(newPod("1")).
				WithInterceptorFuncs(interceptor.Funcs{List: func(ctx context.Context, cli client.WithWatch, list client.ObjectList, opts ...client.ListOption) error {
					if _, ok := list.(*corev1.PodList); ok {
						placement, err := multicluster.FromContext(ctx)
						Expect(err).NotTo(HaveOccurred())
						Expect(placement).To(Equal("test-context"))
						Expect(opts).To(ContainElement(multicluster.InDataContext()))
						listedPods = true
					}
					return cli.List(ctx, list, opts...)
				}}).Build()
			transCtx.Client = model.NewGraphClient(cli)
			Expect(controllerutil.IsDelayedRequeueError((&componentServiceTransformer{}).Transform(transCtx, dag))).To(BeTrue())
			Expect(listedPods).To(BeTrue())
			Expect(transCtx.SynthesizeComponent.InstanceAssistantObjects).To(ContainElement(corev1.ObjectReference{
				Kind: "Service", Namespace: transCtx.Component.Namespace, Name: podServiceName(1),
			}))
		})

		It("does not delete Services when listing Pods fails", func() {
			cli := fake.NewClientBuilder().WithScheme(k8sClient.Scheme()).WithObjects(podService(1)).
				WithInterceptorFuncs(interceptor.Funcs{List: func(ctx context.Context, cli client.WithWatch, list client.ObjectList, opts ...client.ListOption) error {
					if _, ok := list.(*corev1.PodList); ok {
						return fmt.Errorf("pod list unavailable")
					}
					return cli.List(ctx, list, opts...)
				}}).Build()
			transCtx.Client = model.NewGraphClient(cli)
			transCtx.SynthesizeComponent.Replicas = 1
			transCtx.RunningWorkload.Spec.Replicas = ptr.To[int32](1)
			Expect((&componentServiceTransformer{}).Transform(transCtx, dag)).To(MatchError("pod list unavailable"))
			Expect(transCtx.Client.(model.GraphClient).FindAll(dag, &corev1.Service{})).To(BeEmpty())
		})

		DescribeTable("cleans up explicitly removed pod services even while Pods exist",
			func(disable bool) {
				svc := podService(1)
				reader.Objects = append(reader.Objects, newPod("1"), svc)
				transCtx.SynthesizeComponent.Replicas = 1
				if disable {
					transCtx.SynthesizeComponent.ComponentServices[0].DisableAutoProvision = truep()
				} else {
					transCtx.SynthesizeComponent.ComponentServices = nil
				}
				Expect((&componentServiceTransformer{}).Transform(transCtx, dag)).To(Succeed())
				Expect(transCtx.Client.(model.GraphClient).IsAction(dag, svc, model.ActionDeletePtr())).To(BeTrue())
			},
			Entry("removed from the configuration", false),
			Entry("auto provision disabled", true),
		)

		It("provision", func() {
			transformer := &componentServiceTransformer{}
			err := transformer.Transform(transCtx, dag)
			Expect(err).Should(BeNil())

			// check services to provision
			graphCli := transCtx.Client.(model.GraphClient)
			objs := graphCli.FindAll(dag, &corev1.Service{})
			Expect(len(objs)).Should(Equal(int(transCtx.SynthesizeComponent.Replicas)))
			slices.SortFunc(objs, func(a, b client.Object) int {
				return strings.Compare(a.GetName(), b.GetName())
			})
			for i := int32(0); i < transCtx.SynthesizeComponent.Replicas; i++ {
				svc := objs[i].(*corev1.Service)
				Expect(svc.Name).Should(Equal(podServiceName(i)))
				Expect(graphCli.IsAction(dag, svc, model.ActionCreatePtr())).Should(BeTrue())
			}
		})

		It("deletion", func() {
			services := make([]client.Object, 0)
			for i := int32(0); i < transCtx.SynthesizeComponent.Replicas; i++ {
				services = append(services, podService(i))
			}
			reader.Objects = append(reader.Objects, services...)

			// remove component services
			transCtx.SynthesizeComponent.ComponentServices = nil
			transformer := &componentServiceTransformer{}
			err := transformer.Transform(transCtx, dag)
			Expect(err).Should(BeNil())

			// check services to delete
			graphCli := transCtx.Client.(model.GraphClient)
			objs := graphCli.FindAll(dag, &corev1.Service{})
			Expect(len(objs)).Should(Equal(int(transCtx.SynthesizeComponent.Replicas)))
			slices.SortFunc(objs, func(a, b client.Object) int {
				return strings.Compare(a.GetName(), b.GetName())
			})
			for i := int32(0); i < transCtx.SynthesizeComponent.Replicas; i++ {
				svc := objs[i].(*corev1.Service)
				Expect(svc.Name).Should(Equal(podServiceName(i)))
				Expect(graphCli.IsAction(dag, svc, model.ActionDeletePtr())).Should(BeTrue())
			}
		})

		It("deletion at scale-in", func() {
			services := make([]client.Object, 0)
			for i := int32(0); i < transCtx.SynthesizeComponent.Replicas; i++ {
				services = append(services, podService(i))
			}
			reader.Objects = append(reader.Objects, services...)

			// scale-in
			replicas := transCtx.SynthesizeComponent.Replicas
			transCtx.SynthesizeComponent.Replicas = 1
			transCtx.RunningWorkload.Spec.Replicas = ptr.To[int32](1)
			transformer := &componentServiceTransformer{}
			err := transformer.Transform(transCtx, dag)
			Expect(err).Should(BeNil())

			// check services to delete
			graphCli := transCtx.Client.(model.GraphClient)
			objs := graphCli.FindAll(dag, &corev1.Service{})
			Expect(len(objs)).Should(Equal(int(replicas)))
			slices.SortFunc(objs, func(a, b client.Object) int {
				return strings.Compare(a.GetName(), b.GetName())
			})
			for i := int32(0); i < replicas; i++ {
				svc := objs[i].(*corev1.Service)
				Expect(svc.Name).Should(Equal(podServiceName(i)))
				if i < transCtx.SynthesizeComponent.Replicas {
					Expect(graphCli.IsAction(dag, svc, model.ActionUpdatePtr())).Should(BeTrue())
				} else {
					Expect(graphCli.IsAction(dag, svc, model.ActionDeletePtr())).Should(BeTrue())
				}
			}
		})
	})

	Context("auto provision", func() {
		It("disabled", func() {
			// disable auto provision
			transCtx.SynthesizeComponent.ComponentServices[0].DisableAutoProvision = truep()

			transformer := &componentServiceTransformer{}
			err := transformer.Transform(transCtx, dag)
			Expect(err).Should(BeNil())

			// check services to provision
			graphCli := transCtx.Client.(model.GraphClient)
			objs := graphCli.FindAll(dag, &corev1.Service{})
			Expect(len(objs)).Should(Equal(0))
		})

		It("enabled", func() {
			// enable auto provision
			transCtx.SynthesizeComponent.ComponentServices[0].DisableAutoProvision = falsep()

			transformer := &componentServiceTransformer{}
			err := transformer.Transform(transCtx, dag)
			Expect(err).Should(BeNil())

			// check services to provision
			graphCli := transCtx.Client.(model.GraphClient)
			objs := graphCli.FindAll(dag, &corev1.Service{})
			Expect(len(objs)).Should(Equal(1))
			svc := objs[0].(*corev1.Service)
			Expect(svc.Name).Should(Equal(constant.GenerateComponentServiceName(clusterName, compName, "default")))
			Expect(graphCli.IsAction(dag, svc, model.ActionCreatePtr())).Should(BeTrue())
		})
	})
})
