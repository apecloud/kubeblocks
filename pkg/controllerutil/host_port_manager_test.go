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
	"context"
	"fmt"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

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

func TestInternalHostPortReallocation(t *testing.T) {
	for _, failure := range []string{"conflict", "exhaustion"} {
		t.Run(failure, func(t *testing.T) {
			previousManager := defaultPortManager
			t.Cleanup(func() { defaultPortManager = previousManager })
			for key, value := range map[string]string{
				constant.CfgKeyCtrlrMgrNS:         "default",
				constant.CfgHostPortConfigMapName: "test-host-ports",
				constant.CfgHostPortIncludeRanges: "15000-15003",
				constant.CfgHostPortExcludeRanges: "",
			} {
				previous := viper.Get(key)
				viper.Set(key, value)
				t.Cleanup(func() { viper.Set(key, previous) })
			}
			scheme := runtime.NewScheme()
			if err := corev1.AddToScheme(scheme); err != nil {
				t.Fatal(err)
			}
			cli := fake.NewClientBuilder().WithScheme(scheme).Build()
			if err := InitDefaultHostPortManager(cli); err != nil {
				t.Fatal(err)
			}
			key := defaultPortManager.PortKey("cluster", "comp", "kbagent", "streaming")
			if port, err := defaultPortManager.AllocatePort(key); err != nil || port != 15000 {
				t.Fatalf("initial allocation = %d, %v", port, err)
			}
			network := &appsv1.ComponentNetwork{HostNetwork: true, HostPorts: []appsv1.HostPort{{Name: "http", Port: 15000}}}
			cmKey := client.ObjectKey{Namespace: "default", Name: "test-host-ports"}
			if failure == "conflict" {
				cm := &corev1.ConfigMap{}
				if err := cli.Get(context.Background(), cmKey, cm); err != nil {
					t.Fatal(err)
				}
				cm.Data["other"] = "15002"
				if err := cli.Update(context.Background(), cm); err != nil {
					t.Fatal(err)
				}
			} else {
				network.HostPorts = append(network.HostPorts, appsv1.HostPort{Name: "mysql", Port: 15001},
					appsv1.HostPort{Name: "metrics", Port: 15002}, appsv1.HostPort{Name: "admin", Port: 15003})
			}
			_, err := GetPortManager(network).AllocatePort(key)
			if err == nil || (failure == "conflict" && !apierrors.IsConflict(err)) {
				t.Fatalf("expected %s failure, got %v", failure, err)
			}
			cm := &corev1.ConfigMap{}
			if err := cli.Get(context.Background(), cmKey, cm); err != nil {
				t.Fatal(err)
			}
			if cm.Data[key] != "15000" {
				t.Fatalf("failed replacement changed historical record to %q", cm.Data[key])
			}
			if failure == "exhaustion" {
				network.HostPorts = network.HostPorts[:1]
			}
			if port, err := GetPortManager(network).AllocatePort(key); err != nil || port != 15001 {
				t.Fatalf("retry allocation = %d, %v", port, err)
			}
			if err := defaultPortManager.UsePort("reuse-old", 15000); err != nil {
				t.Fatalf("old reservation was not released: %v", err)
			}
			if err := InitDefaultHostPortManager(cli); err != nil {
				t.Fatal(err)
			}
			if port, err := GetPortManager(network).AllocatePort(key); err != nil || port != 15001 {
				t.Fatalf("allocation after restart = %d, %v", port, err)
			}
		})
	}
}

func newHostPortTestClient(t *testing.T, includes, excludes string, data map[string]string) client.Client {
	t.Helper()
	previousManager := defaultPortManager
	t.Cleanup(func() { defaultPortManager = previousManager })
	for key, value := range map[string]string{
		constant.CfgKeyCtrlrMgrNS:         "default",
		constant.CfgHostPortConfigMapName: "test-host-ports",
		constant.CfgHostPortIncludeRanges: includes,
		constant.CfgHostPortExcludeRanges: excludes,
	} {
		previous := viper.Get(key)
		viper.Set(key, value)
		t.Cleanup(func() { viper.Set(key, previous) })
	}
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	return fake.NewClientBuilder().WithScheme(scheme).WithObjects(&corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "test-host-ports"},
		Data:       data,
	}).Build()
}

func TestHostPortPoolRangeChange(t *testing.T) {
	data := make(map[string]string)
	for port := 33000; port <= 35225; port++ {
		data[fmt.Sprintf("old-%d", port)] = fmt.Sprint(port)
	}
	cli := newHostPortTestClient(t, "1025-65536", "6443,10250,10257,10259,2379-2380,30000-32767", data)
	if err := InitDefaultHostPortManager(cli); err != nil {
		t.Fatal(err)
	}
	if port, err := defaultPortManager.AllocatePort("old-33000"); err != nil || port != 33000 {
		t.Fatalf("historical allocation = %d, %v", port, err)
	}
	if port, err := defaultPortManager.AllocatePort("old-range-new"); err != nil || port != 1025 {
		t.Fatalf("old range allocation = %d, %v", port, err)
	}
	viper.Set(constant.CfgHostPortIncludeRanges, "55000-59999")
	for restart := 0; restart < 2; restart++ {
		if err := InitDefaultHostPortManager(cli); err != nil {
			t.Fatal(err)
		}
		if port, err := defaultPortManager.AllocatePort("old-33000"); err != nil || port != 33000 {
			t.Fatalf("historical allocation after range change = %d, %v", port, err)
		}
		if port, err := defaultPortManager.AllocatePort("new-range"); err != nil || port != 55000 {
			t.Fatalf("new range allocation = %d, %v", port, err)
		}
		if err := defaultPortManager.UsePort("excluded", 6443); err == nil {
			t.Fatal("out-of-range exclusion was lost")
		}
	}
	cm := &corev1.ConfigMap{}
	if err := cli.Get(context.Background(), client.ObjectKey{Namespace: "default", Name: "test-host-ports"}, cm); err != nil {
		t.Fatal(err)
	}
	for key, value := range data {
		if cm.Data[key] != value {
			t.Fatalf("historical record %s changed to %q", key, cm.Data[key])
		}
	}
}

func TestHostPortPoolExhaustionAndRelease(t *testing.T) {
	cli := newHostPortTestClient(t, "15000-15002", "15001,3306", map[string]string{"historical": "33000"})
	if err := InitDefaultHostPortManager(cli); err != nil {
		t.Fatal(err)
	}
	for i, expected := range []int32{15000, 15002} {
		if port, err := defaultPortManager.AllocatePort(fmt.Sprintf("new-%d", i)); err != nil || port != expected {
			t.Fatalf("allocation %d = %d, %v; want %d", i, port, err, expected)
		}
	}
	for restart := 0; restart < 2; restart++ {
		if _, err := defaultPortManager.AllocatePort("full"); err == nil {
			t.Fatal("expected exhaustion")
		}
		if err := InitDefaultHostPortManager(cli); err != nil {
			t.Fatal(err)
		}
	}
	if err := defaultPortManager.ReleaseByPrefix("new-0"); err != nil {
		t.Fatal(err)
	}
	if port, err := defaultPortManager.AllocatePort("replacement"); err != nil || port != 15000 {
		t.Fatalf("released allocation = %d, %v", port, err)
	}
}
