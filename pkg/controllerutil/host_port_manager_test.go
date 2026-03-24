/*
Copyright (C) 2022-2025 ApeCloud Co., Ltd

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
			manager = GetPortManager(network)
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

			manager = GetPortManager(network)
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
	})
})
