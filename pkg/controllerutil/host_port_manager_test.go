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

package controllerutil

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/kbagent"
	testutil "github.com/apecloud/kubeblocks/pkg/testutil/k8s"
	viper "github.com/apecloud/kubeblocks/pkg/viperx"
)

var _ = Describe("host port manager test", func() {
	var (
		clusterName   = "test-cluster"
		compName      = "comp"
		containerName = "container"
		portName      = "app"
		portNumber    = int32(1234)
		manager       PortManager
	)

	Context("defined host-port manager", func() {
		var (
			network = &appsv1.ComponentNetwork{
				HostNetwork: true,
				HostPorts: []appsv1.HostPort{
					{
						Name: portName,
						Port: portNumber,
					},
					{
						Name: kbagent.DefaultHTTPPortName,
						Port: kbagent.DefaultHTTPPort,
					},
					{
						Name: kbagent.DefaultStreamingPortName,
						Port: kbagent.DefaultStreamingPort,
					},
				},
			}
		)

		BeforeEach(func() {
			defaultPortManager = nil
			manager = GetPortManager(network, nil)
		})

		AfterEach(func() {
		})

		It("port key", func() {
			key := manager.PortKey(clusterName, compName, containerName, portName)
			Expect(key).To(Equal(portName))
		})

		It("port key - kbagent", func() {
			key := manager.PortKey(clusterName, compName, kbagent.ContainerName, kbagent.DefaultHTTPPortName)
			Expect(key).To(Equal(kbagent.DefaultHTTPPortName))
		})

		It("allocate port", func() {
			key := manager.PortKey(clusterName, compName, containerName, portName)
			port, err := manager.AllocatePort(key)
			Expect(err).Should(BeNil())
			Expect(port).Should(Equal(portNumber))
		})

		It("allocate port - kbagent", func() {
			key := manager.PortKey(clusterName, compName, kbagent.ContainerName, kbagent.DefaultHTTPPortName)
			port, err := manager.AllocatePort(key)
			Expect(err).Should(BeNil())
			Expect(port).Should(Equal(int32(kbagent.DefaultHTTPPort)))
		})

		It("allocate port - not defined", func() {
			errPortName := fmt.Sprintf("%s-not-defined", portName)
			key := manager.PortKey(clusterName, compName, containerName, errPortName)
			_, err := manager.AllocatePort(key)
			Expect(err).ShouldNot(BeNil())
			Expect(err.Error()).Should(ContainSubstring("no available port"))
		})

		It("get port", func() {
			key := manager.PortKey(clusterName, compName, containerName, portName)
			port, err := manager.GetPort(key)
			Expect(err).Should(BeNil())
			Expect(port).Should(Equal(portNumber))
		})

		It("get port - kbagent", func() {
			key := manager.PortKey(clusterName, compName, kbagent.ContainerName, kbagent.DefaultHTTPPortName)
			port, err := manager.GetPort(key)
			Expect(err).Should(BeNil())
			Expect(port).Should(Equal(int32(kbagent.DefaultHTTPPort)))
		})

		It("get port - not defined", func() {
			errPortName := fmt.Sprintf("%s-not-defined", portName)
			key := manager.PortKey(clusterName, compName, containerName, errPortName)
			port, err := manager.GetPort(key)
			Expect(err).Should(BeNil())
			Expect(port).Should(Equal(int32(0)))
		})

		It("use port", func() {
			key := manager.PortKey(clusterName, compName, containerName, portName)
			err := manager.UsePort(key, portNumber)
			Expect(err).Should(BeNil())
		})

		It("use port - kbagent", func() {
			key := manager.PortKey(clusterName, compName, kbagent.ContainerName, kbagent.DefaultHTTPPortName)
			err := manager.UsePort(key, kbagent.DefaultHTTPPort)
			Expect(err).Should(BeNil())
		})

		It("use port - not defined", func() {
			errPortName := fmt.Sprintf("%s-not-defined", portName)
			key := manager.PortKey(clusterName, compName, containerName, errPortName)
			err := manager.UsePort(key, portNumber)
			Expect(err).Should(BeNil())
		})

		It("release port", func() {
			key := manager.PortKey(clusterName, compName, containerName, portName)
			err := manager.ReleaseByPrefix(key)
			Expect(err).Should(BeNil())
		})
	})

	Context("defined host-port manager - w/o kbagent", func() {
		var (
			mockClient *testutil.K8sClientMockHelper
			network    = &appsv1.ComponentNetwork{
				HostNetwork: true,
				HostPorts: []appsv1.HostPort{
					{
						Name: portName,
						Port: portNumber,
					},
				},
			}
			minPort, maxPort       = int32(1024), int32(65536)
			dataCM                 = map[string]string{}
			definedPortManagerInst *definedPortManager
		)

		BeforeEach(func() {
			mockClient = testutil.NewK8sMockClient()
			mockClient.MockCreateMethod(testutil.WithCreateReturned(func(obj client.Object) error {
				dataCM = obj.(*corev1.ConfigMap).Data
				return nil
			}, testutil.WithAnyTimes()))
			mockClient.MockGetMethod(testutil.WithGetReturned(testutil.WithConstructSimpleGetResult([]client.Object{
				&corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: viper.GetString(constant.CfgKeyCtrlrMgrNS),
						Name:      viper.GetString(constant.CfgHostPortConfigMapName),
					},
					Data: dataCM,
				},
			}), testutil.WithAnyTimes()))
			mockClient.MockUpdateMethod(testutil.WithCreateReturned(func(obj client.Object) error {
				dataCM = obj.(*corev1.ConfigMap).Data
				return nil
			}, testutil.WithAnyTimes()))

			viper.Set(constant.CfgHostPortIncludeRanges, fmt.Sprintf("%d-%d", minPort, maxPort))

			err := InitDefaultHostPortManager(mockClient.Client())
			Expect(err).ShouldNot(HaveOccurred())

			manager = GetPortManager(network, nil)
			Expect(manager).ShouldNot(BeNil())
			definedPortManagerInst = manager.(*definedPortManager)
		})

		AfterEach(func() {
			mockClient.Finish()
		})

		It("port key", func() {
			key := manager.PortKey(clusterName, compName, containerName, portName)
			Expect(key).To(Equal(portName))
		})

		It("port key - kbagent", func() {
			key := manager.PortKey(clusterName, compName, kbagent.ContainerName, kbagent.DefaultHTTPPortName)
			Expect(key).To(Equal(fmt.Sprintf("%s-%s-%s-%s", clusterName, compName, kbagent.ContainerName, kbagent.DefaultHTTPPortName)))
		})

		It("allocate port", func() {
			key := manager.PortKey(clusterName, compName, containerName, portName)
			port, err := manager.AllocatePort(key)
			Expect(err).Should(BeNil())
			Expect(port).Should(Equal(portNumber))
		})

		It("allocate port - kbagent", func() {
			key := manager.PortKey(clusterName, compName, kbagent.ContainerName, kbagent.DefaultHTTPPortName)
			port, err := manager.AllocatePort(key)
			Expect(err).Should(BeNil())
			Expect(port).Should(Equal(minPort))
		})

		It("allocate port - not defined", func() {
			errPortName := fmt.Sprintf("%s-not-defined", portName)
			key := manager.PortKey(clusterName, compName, containerName, errPortName)
			_, err := manager.AllocatePort(key)
			Expect(err).ShouldNot(BeNil())
			Expect(err.Error()).Should(ContainSubstring("no available port"))
		})

		It("get port", func() {
			key := manager.PortKey(clusterName, compName, containerName, portName)
			port, err := manager.GetPort(key)
			Expect(err).Should(BeNil())
			Expect(port).Should(Equal(portNumber))
		})

		It("get port - kbagent, not allocated", func() {
			key := manager.PortKey(clusterName, compName, kbagent.ContainerName, kbagent.DefaultHTTPPortName)
			port, err := manager.GetPort(key)
			Expect(err).Should(BeNil())
			Expect(port).Should(Equal(int32(0)))
		})

		It("get port - kbagent", func() {
			key := manager.PortKey(clusterName, compName, kbagent.ContainerName, kbagent.DefaultHTTPPortName)
			allocated, err1 := manager.AllocatePort(key)
			Expect(err1).Should(BeNil())
			port, err2 := manager.GetPort(key)
			Expect(err2).Should(BeNil())
			Expect(port).Should(Equal(allocated))
		})

		It("get port - not defined", func() {
			errPortName := fmt.Sprintf("%s-not-defined", portName)
			key := manager.PortKey(clusterName, compName, containerName, errPortName)
			port, err := manager.GetPort(key)
			Expect(err).Should(BeNil())
			Expect(port).Should(Equal(int32(0)))
		})

		It("use port", func() {
			key := manager.PortKey(clusterName, compName, containerName, portName)
			err := manager.UsePort(key, portNumber)
			Expect(err).Should(BeNil())
			Expect(definedPortManagerInst.hostPorts).Should(HaveKeyWithValue(key, portNumber))
		})

		It("use port - kbagent", func() {
			key := manager.PortKey(clusterName, compName, kbagent.ContainerName, kbagent.DefaultHTTPPortName)
			err := manager.UsePort(key, int32(kbagent.DefaultHTTPPort))
			Expect(err).Should(BeNil())
			Expect(definedPortManagerInst.hostPorts).ShouldNot(HaveKey(key))
			Expect(dataCM).Should(HaveKeyWithValue(key, fmt.Sprintf("%d", kbagent.DefaultHTTPPort)))
		})

		It("use port - not defined", func() {
			errPortName := fmt.Sprintf("%s-not-defined", portName)
			key := manager.PortKey(clusterName, compName, containerName, errPortName)
			err := manager.UsePort(key, portNumber)
			Expect(err).Should(BeNil())
			Expect(definedPortManagerInst.hostPorts).ShouldNot(HaveKey(key))
		})

		It("release port", func() {
			key := manager.PortKey(clusterName, compName, containerName, portName)
			err := manager.ReleaseByPrefix(key)
			Expect(err).Should(BeNil())
		})

		It("release port - kbagent", func() {
			key := manager.PortKey(clusterName, compName, kbagent.ContainerName, kbagent.DefaultHTTPPortName)
			err := manager.UsePort(key, int32(kbagent.DefaultHTTPPort))
			Expect(err).Should(BeNil())
			Expect(dataCM).Should(HaveKeyWithValue(key, fmt.Sprintf("%d", kbagent.DefaultHTTPPort)))
			err = manager.ReleaseByPrefix(key)
			Expect(err).Should(BeNil())
			Expect(dataCM).Should(BeEmpty())
		})

		It("allocate port - kbagent adopts allocation recorded under legacy port name", func() {
			legacyKey := fmt.Sprintf("%s-%s-%s-%s", clusterName, compName, kbagent.ContainerName, kbagent.LegacyHTTPPortName)
			err := definedPortManagerInst.defaultPortManager.UsePort(legacyKey, int32(30001))
			Expect(err).Should(BeNil())

			key := manager.PortKey(clusterName, compName, kbagent.ContainerName, kbagent.DefaultHTTPPortName)
			port, err := manager.AllocatePort(key)
			Expect(err).Should(BeNil())
			Expect(port).Should(Equal(int32(30001)))
			Expect(dataCM).Should(HaveKeyWithValue(key, "30001"))
		})

		It("release - deletion-path nil context still releases default-manager allocations", func() {
			// allocation side: the component owns real ports named "http" and
			// "streaming", so both legacy aliases are suppressed and the two
			// kbagent ports are dynamically allocated in the default manager.
			// unique cluster/comp names keep this spec independent from the
			// allocations other specs may have recorded in the shared manager
			delCluster, delComp := "del-cluster", "del-comp"
			aliasNetwork := &appsv1.ComponentNetwork{
				HostNetwork: true,
				HostPorts: []appsv1.HostPort{
					{Name: kbagent.LegacyHTTPPortName, Port: 7001},
					{Name: kbagent.LegacyStreamingPortName, Port: 7002},
				},
			}
			alloc := GetPortManager(aliasNetwork, sets.New(kbagent.LegacyHTTPPortName, kbagent.LegacyStreamingPortName))
			cmData := func() map[string]string {
				return alloc.(*definedPortManager).defaultPortManager.cm.Data
			}
			for _, pn := range []string{kbagent.DefaultHTTPPortName, kbagent.DefaultStreamingPortName} {
				key := alloc.PortKey(delCluster, delComp, kbagent.ContainerName, pn)
				_, err := alloc.AllocatePort(key)
				Expect(err).Should(BeNil())
				Expect(cmData()).Should(HaveKey(key))
			}

			// deletion side: the component-port context is unavailable (nil),
			// so the legacy aliases resolve and hasKBAgentPortDefined() is
			// true — release must not consult alias resolution and still has
			// to remove the default-manager allocations, or the host ports
			// leak forever
			del := GetPortManager(aliasNetwork, nil)
			Expect(del.ReleaseByPrefix(fmt.Sprintf("%s-%s", delCluster, delComp))).Should(Succeed())
			for _, pn := range []string{kbagent.DefaultHTTPPortName, kbagent.DefaultStreamingPortName} {
				key := alloc.PortKey(delCluster, delComp, kbagent.ContainerName, pn)
				Expect(cmData()).ShouldNot(HaveKey(key))
			}
		})
	})

	Context("defined host-port manager - legacy kbagent port names", func() {
		var (
			network = &appsv1.ComponentNetwork{
				HostNetwork: true,
				HostPorts: []appsv1.HostPort{
					{
						Name: kbagent.LegacyHTTPPortName,
						Port: kbagent.DefaultHTTPPort,
					},
					{
						Name: kbagent.LegacyStreamingPortName,
						Port: kbagent.DefaultStreamingPort,
					},
				},
			}
		)

		BeforeEach(func() {
			defaultPortManager = nil
			manager = GetPortManager(network, nil)
		})

		It("port key resolves the legacy mapping as defined", func() {
			key := manager.PortKey(clusterName, compName, kbagent.ContainerName, kbagent.DefaultHTTPPortName)
			Expect(key).To(Equal(kbagent.DefaultHTTPPortName))
		})

		It("allocate port via legacy alias", func() {
			key := manager.PortKey(clusterName, compName, kbagent.ContainerName, kbagent.DefaultHTTPPortName)
			port, err := manager.AllocatePort(key)
			Expect(err).Should(BeNil())
			Expect(port).Should(Equal(int32(kbagent.DefaultHTTPPort)))
		})

		It("get port via legacy alias", func() {
			key := manager.PortKey(clusterName, compName, kbagent.ContainerName, kbagent.DefaultStreamingPortName)
			port, err := manager.GetPort(key)
			Expect(err).Should(BeNil())
			Expect(port).Should(Equal(int32(kbagent.DefaultStreamingPort)))
		})

		It("treats both kbagent ports as defined via legacy names", func() {
			Expect(manager.(*definedPortManager).hasKBAgentPortDefined()).Should(BeTrue())
		})

		It("does not treat a real component port name as a kbagent alias", func() {
			// an engine that declares its own "http" port owns the "http"
			// mapping; kbagent must not consume it as a legacy alias
			scoped := GetPortManager(network, sets.New(kbagent.LegacyHTTPPortName))
			pm := scoped.(*definedPortManager)

			_, defined := pm.definedKBAgentPort(kbagent.DefaultHTTPPortName)
			Expect(defined).Should(BeFalse())

			// streaming is not a component port, the alias still applies
			port, defined := pm.definedKBAgentPort(kbagent.DefaultStreamingPortName)
			Expect(defined).Should(BeTrue())
			Expect(port).Should(Equal(int32(kbagent.DefaultStreamingPort)))

			// with one alias suppressed, kbagent ports are no longer fully defined
			Expect(pm.hasKBAgentPortDefined()).Should(BeFalse())
		})
	})
})
