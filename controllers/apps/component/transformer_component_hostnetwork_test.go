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
	"strconv"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	appsutil "github.com/apecloud/kubeblocks/controllers/apps/util"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	"github.com/apecloud/kubeblocks/pkg/controller/graph"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	viper "github.com/apecloud/kubeblocks/pkg/viperx"
)

var _ = Describe("component hostnetwork transformer test", func() {
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
		It("avoids explicit ports on creation and after an update and allocator restart", func() {
			DeferCleanup(func() {
				Expect(intctrlutil.InitDefaultHostPortManager(k8sClient)).To(Succeed())
			})
			for key, value := range map[string]string{
				constant.CfgHostPortIncludeRanges: "15000-15002",
				constant.CfgHostPortExcludeRanges: "",
			} {
				previous := viper.Get(key)
				viper.Set(key, value)
				DeferCleanup(func() { viper.Set(key, previous) })
			}
			scheme := runtime.NewScheme()
			Expect(corev1.AddToScheme(scheme)).To(Succeed())
			cli := fake.NewClientBuilder().WithScheme(scheme).Build()
			Expect(intctrlutil.InitDefaultHostPortManager(cli)).To(Succeed())

			synth := transCtx.SynthesizeComponent
			synth.Network = &appsv1.ComponentNetwork{HostNetwork: true, HostPorts: []appsv1.HostPort{{Name: "mysql", Port: 3306}, {Name: "http", Port: 15000}}}
			synth.PodSpec.Containers[1].Args = []string{"--port", "3501", "--streaming-port", "3502"}
			transformer := &componentHostNetworkTransformer{}
			check := func(http, streaming int32) {
				Expect(transformer.Transform(transCtx, dag)).To(Succeed())
				Expect(synth.PodSpec.HostNetwork).To(BeTrue())
				Expect(synth.PodSpec.Containers[0].Ports[0].ContainerPort).To(Equal(int32(3306)))
				agent := synth.PodSpec.Containers[1]
				Expect(agent.Ports[0].ContainerPort).To(Equal(http))
				Expect(agent.Ports[1].ContainerPort).To(Equal(streaming))
				Expect(agent.Args).To(Equal([]string{"--port", strconv.Itoa(int(http)), "--streaming-port", strconv.Itoa(int(streaming))}))
				pm := intctrlutil.GetPortManager(synth.Network)
				cm := &corev1.ConfigMap{}
				Expect(cli.Get(context.Background(), client.ObjectKey{
					Namespace: viper.GetString(constant.CfgKeyCtrlrMgrNS),
					Name:      viper.GetString(constant.CfgHostPortConfigMapName),
				}, cm)).To(Succeed())
				key := pm.PortKey(clusterName, compName, "kbagent", "streaming")
				Expect(cm.Data).To(HaveKeyWithValue(key, strconv.Itoa(int(streaming))))
			}
			check(15000, 15001)
			check(15000, 15001)
			Expect(intctrlutil.InitDefaultHostPortManager(cli)).To(Succeed())
			check(15000, 15001)
			synth.Network.HostPorts[1].Port = 15001
			check(15001, 15000)
			Expect(intctrlutil.GetPortManager(nil).UsePort("other-component", 15001)).To(Succeed())
			Expect(intctrlutil.InitDefaultHostPortManager(cli)).To(Succeed())
			check(15001, 15000)

			// Explicit ports are local to this Component, so another allocation can use 15001.
			port, err := intctrlutil.GetPortManager(nil).AllocatePort("other-component")
			Expect(err).NotTo(HaveOccurred())
			Expect(port).To(Equal(int32(15001)))
			Expect(intctrlutil.GetPortManager(synth.Network).ReleaseByPrefix(clusterName + "-" + compName)).To(Succeed())
			pm := intctrlutil.GetPortManager(nil)
			port, err = pm.GetPort(pm.PortKey(clusterName, compName, "kbagent", "streaming"))
			Expect(err).NotTo(HaveOccurred())
			Expect(port).To(BeZero())
		})

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
