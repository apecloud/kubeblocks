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
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	"github.com/apecloud/kubeblocks/pkg/controller/graph"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
)

var _ = Describe("component hostnetwork transformer test", func() {
	const (
		clusterName = "test-cluster"
		compName    = "comp"
	)

	var (
		reader   *mockReader
		dag      *graph.DAG
		transCtx *componentTransformContext
	)

	newDAG := func(graphCli model.GraphClient, comp *appsv1.Component) *graph.DAG {
		d := graph.NewDAG()
		graphCli.Root(d, comp, comp, model.ActionStatusPtr())
		return d
	}

	BeforeEach(func() {
		reader = &mockReader{}
		comp := &appsv1.Component{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: testCtx.DefaultNamespace,
				Name:      constant.GenerateClusterComponentName(clusterName, compName),
				Labels: map[string]string{
					constant.AppManagedByLabelKey:   constant.AppName,
					constant.AppInstanceLabelKey:    clusterName,
					constant.KBAppComponentLabelKey: compName,
				},
				// host network enabled
				Annotations: map[string]string{
					constant.HostNetworkAnnotationKey: compName,
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
				Replicas:    2,
				HostNetwork: &appsv1.HostNetwork{
					ContainerPorts: []appsv1.HostNetworkContainerPort{
						{
							Container: "mysql",
							Ports:     []string{"mysql"},
						},
						{
							Container: "kbagent",
							Ports:     []string{"http"},
						},
						{
							Container: "kbagent",
							Ports:     []string{"streaming"},
						},
					},
				},
				PodSpec: &corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name: "mysql",
							Ports: []corev1.ContainerPort{
								{
									Name:          "mysql",
									ContainerPort: 3306,
								},
							},
						},
						{
							Name: "kbagent",
							Ports: []corev1.ContainerPort{
								{
									Name:          "http",
									ContainerPort: 3501,
								},
								{
									Name:          "streaming",
									ContainerPort: 3502,
								},
							},
						},
					},
				},
			},
		}
	})

	AfterEach(func() {})

	Context("allocate host network success ", func() {
		It("disabled", func() {
			transformer := &componentServiceTransformer{}
			err := transformer.Transform(transCtx, dag)
			Expect(err).Should(BeNil())

			// check the container ports in the synthesized component
			Expect(transCtx.SynthesizeComponent.PodSpec.Containers[0].Ports[0].ContainerPort).ShouldNot(Equal(3306))
			Expect(transCtx.SynthesizeComponent.PodSpec.Containers[1].Ports[0].ContainerPort).ShouldNot(Equal(3501))
			Expect(transCtx.SynthesizeComponent.PodSpec.Containers[1].Ports[1].ContainerPort).ShouldNot(Equal(3502))
		})
	})
})
