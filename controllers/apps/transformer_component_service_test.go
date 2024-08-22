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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"golang.org/x/exp/slices"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	"github.com/apecloud/kubeblocks/pkg/controller/graph"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
	"github.com/apecloud/kubeblocks/pkg/controllerutil"
)

var _ = Describe(" component service transformer test", func() {
	const (
		clusterName = "test-cluster"
		compName    = "comp"
	)

	var (
		reader   *mockReader
		dag      *graph.DAG
		transCtx *componentTransformContext
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
				ComponentServices: []appsv1alpha1.ComponentService{
					{
						Service: appsv1alpha1.Service{
							Name:        "default",
							ServiceName: "default",
						},
					},
				},
				Replicas: 3,
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

		It("provision", func() {
			transformer := &componentServiceTransformer{}
			err := transformer.Transform(transCtx, dag)
			Expect(err).Should(BeNil())

			// check services to provision
			graphCli := transCtx.Client.(model.GraphClient)
			objs := graphCli.FindAll(dag, &corev1.Service{})
			Expect(len(objs)).Should(Equal(int(transCtx.SynthesizeComponent.Replicas)))
			slices.SortFunc(objs, func(a, b client.Object) bool {
				return a.GetName() < b.GetName()
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
			reader.objs = append(reader.objs, services...)

			// remove component services
			transCtx.SynthesizeComponent.ComponentServices = nil
			transformer := &componentServiceTransformer{}
			err := transformer.Transform(transCtx, dag)
			Expect(err).Should(BeNil())

			// check services to delete
			graphCli := transCtx.Client.(model.GraphClient)
			objs := graphCli.FindAll(dag, &corev1.Service{})
			Expect(len(objs)).Should(Equal(int(transCtx.SynthesizeComponent.Replicas)))
			slices.SortFunc(objs, func(a, b client.Object) bool {
				return a.GetName() < b.GetName()
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
			reader.objs = append(reader.objs, services...)

			// scale-in
			replicas := transCtx.SynthesizeComponent.Replicas
			transCtx.SynthesizeComponent.Replicas = 1
			transformer := &componentServiceTransformer{}
			err := transformer.Transform(transCtx, dag)
			Expect(err).Should(BeNil())

			// check services to delete
			graphCli := transCtx.Client.(model.GraphClient)
			objs := graphCli.FindAll(dag, &corev1.Service{})

			// first service should not be updated or deleted
			Expect(len(objs)).Should(Equal(int(replicas) - 1))
			slices.SortFunc(objs, func(a, b client.Object) bool {
				return a.GetName() < b.GetName()
			})
			for i := int32(1); i < replicas; i++ {
				svc := objs[i-1].(*corev1.Service)
				Expect(svc.Name).Should(Equal(podServiceName(i)))
				Expect(graphCli.IsAction(dag, svc, model.ActionDeletePtr())).Should(BeTrue())
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
