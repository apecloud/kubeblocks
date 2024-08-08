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
	"encoding/json"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	viper "github.com/apecloud/kubeblocks/pkg/viperx"
)

var _ = Describe("Lorry Utils", func() {

	Context("build probe containers", func() {
		var container *corev1.Container
		var component *SynthesizedComponent
		var lorryHTTPPort int
		var lorryGRPCPort int

		BeforeEach(func() {
			lorryHTTPPort = 3501
			lorryGRPCPort = 50001

			component = &SynthesizedComponent{}
			component.ComponentServices = append(component.ComponentServices, appsv1alpha1.ComponentService{
				Service: appsv1alpha1.Service{
					Spec: corev1.ServiceSpec{
						Ports: []corev1.ServicePort{{
							Protocol: corev1.ProtocolTCP,
							Port:     3306,
						}},
					},
				},
			})
			component.Roles = []appsv1alpha1.ReplicaRole{
				{
					Name:        "leader",
					Serviceable: true,
					Writable:    true,
					Votable:     true,
				},
				{
					Name:        "follower",
					Serviceable: true,
					Writable:    false,
					Votable:     true,
				},
				{
					Name:        "learner",
					Serviceable: true,
					Writable:    false,
					Votable:     false,
				},
			}
			component.LifecycleActions = &appsv1alpha1.ComponentLifecycleActions{
				RoleProbe: &appsv1alpha1.Probe{},
			}
			component.PodSpec = &corev1.PodSpec{
				Containers: []corev1.Container{},
			}

			container = buildBasicContainer(lorryHTTPPort)
		})

		It("build role probe containers", func() {
			reqCtx := intctrlutil.RequestCtx{
				Ctx: ctx,
				Log: logger,
			}
			defaultBuiltInHandler := appsv1alpha1.MySQLBuiltinActionHandler
			component.LifecycleActions = &appsv1alpha1.ComponentLifecycleActions{
				RoleProbe: &appsv1alpha1.Probe{
					BuiltinHandler: &defaultBuiltInHandler,
				},
			}
			Expect(buildLorryContainers(reqCtx, component, nil)).Should(Succeed())
			Expect(component.PodSpec.Containers).Should(HaveLen(1))
			Expect(component.PodSpec.InitContainers).Should(HaveLen(0))
			Expect(component.PodSpec.Containers[0].Name).Should(Equal(constant.LorryContainerName))
		})

		It("should build role service container", func() {
			buildLorryServiceContainer(component, container, lorryHTTPPort, lorryGRPCPort, nil)
			Expect(container.Command).ShouldNot(BeEmpty())
			Expect(container.Name).Should(Equal(constant.LorryContainerName))
			Expect(len(container.Ports)).Should(Equal(2))
		})

		It("build lorry container if any builtinhandler specified", func() {
			reqCtx := intctrlutil.RequestCtx{
				Ctx: ctx,
				Log: logger,
			}
			// all other services are disabled
			defaultBuiltInHandler := appsv1alpha1.MySQLBuiltinActionHandler
			component.LifecycleActions = &appsv1alpha1.ComponentLifecycleActions{
				MemberJoin: &appsv1alpha1.LifecycleActionHandler{
					BuiltinHandler: &defaultBuiltInHandler,
				},
			}
			Expect(buildLorryContainers(reqCtx, component, nil)).Should(Succeed())
			Expect(component.PodSpec.Containers).Should(HaveLen(1))
			Expect(component.PodSpec.InitContainers).Should(HaveLen(0))
			Expect(component.PodSpec.Containers[0].Name).Should(Equal(constant.LorryContainerName))
		})

		It("build lorry container if any exec specified", func() {
			reqCtx := intctrlutil.RequestCtx{
				Ctx: ctx,
				Log: logger,
			}
			image := "testimage"
			// all other services are disabled
			component.LifecycleActions = &appsv1alpha1.ComponentLifecycleActions{
				MemberJoin: &appsv1alpha1.LifecycleActionHandler{
					CustomHandler: &appsv1alpha1.Action{
						Exec: &appsv1alpha1.ExecAction{
							Image:   image,
							Command: []string{"test"},
						},
					},
				},
			}
			Expect(buildLorryContainers(reqCtx, component, nil)).Should(Succeed())
			Expect(component.PodSpec.Containers).Should(HaveLen(1))
			Expect(component.PodSpec.InitContainers).Should(HaveLen(1))
			Expect(component.PodSpec.Containers[0].Image).Should(Equal(image))
			Expect(component.PodSpec.Containers[0].Name).Should(Equal(constant.LorryContainerName))
		})

		It("build volume protection probe container without RBAC", func() {
			reqCtx := intctrlutil.RequestCtx{
				Ctx: ctx,
				Log: logger,
			}
			component.Volumes = []appsv1alpha1.ComponentVolume{
				{
					Name:          "volume-001",
					HighWatermark: 90,
				},
				{
					Name:          "volume-002",
					HighWatermark: 0,
				},
			}
			defaultBuiltInHandler := appsv1alpha1.MySQLBuiltinActionHandler
			component.LifecycleActions = &appsv1alpha1.ComponentLifecycleActions{
				RoleProbe: &appsv1alpha1.Probe{
					BuiltinHandler: &defaultBuiltInHandler,
				},
			}
			Expect(buildLorryContainers(reqCtx, component, nil)).Should(Succeed())
			Expect(component.PodSpec.Containers).Should(HaveLen(2))
			Expect(component.PodSpec.Containers[0].Name).Should(Equal(constant.LorryContainerName))
			Expect(component.PodSpec.Containers[1].Name).Should(Equal(constant.VolumeProtectionProbeContainerName))
		})

		It("build volume protection probe container with RBAC", func() {
			reqCtx := intctrlutil.RequestCtx{
				Ctx: ctx,
				Log: logger,
			}
			component.Volumes = []appsv1alpha1.ComponentVolume{
				{
					Name:          "volume-001",
					HighWatermark: 90,
				},
				{
					Name:          "volume-002",
					HighWatermark: 0,
				},
			}
			defaultBuiltInHandler := appsv1alpha1.MySQLBuiltinActionHandler
			component.LifecycleActions = &appsv1alpha1.ComponentLifecycleActions{
				RoleProbe: &appsv1alpha1.Probe{
					BuiltinHandler: &defaultBuiltInHandler,
				},
			}
			viper.SetDefault(constant.EnableRBACManager, true)
			Expect(buildLorryContainers(reqCtx, component, nil)).Should(Succeed())
			Expect(component.PodSpec.Containers).Should(HaveLen(2))
			spec := &appsv1alpha1.VolumeProtectionSpec{}
			for _, e := range component.PodSpec.Containers[0].Env {
				if e.Name == constant.KBEnvVolumeProtectionSpec {
					Expect(json.Unmarshal([]byte(e.Value), spec)).Should(Succeed())
					break
				}
			}
			Expect(spec.Volumes).Should(HaveLen(1))
			Expect(*spec.Volumes[0].HighWatermark).Should(Equal(90))
		})
	})
})

func TestGetAvailableContainerPorts(t *testing.T) {
	var containers []corev1.Container

	tests := []struct {
		inputPort  int32
		outputPort int32
	}{{
		inputPort:  80, // 80 is a privileged port
		outputPort: 80,
	}, {
		inputPort:  65536, // 65536 is an invalid port
		outputPort: minAvailPort,
	}, {
		inputPort:  3306, // 3306 is a qualified port
		outputPort: 3306,
	}}

	for _, test := range tests {
		containerPorts := []int32{test.inputPort}
		foundPorts, err := getAvailableContainerPorts(containers, containerPorts)
		if err != nil {
			t.Error("expect getAvailableContainerPorts success")
		}
		if len(foundPorts) != 1 || foundPorts[0] != test.outputPort {
			t.Error("expect getAvailableContainerPorts returns", test.outputPort)
		}
	}
}

func TestGetAvailableContainerPortsPartlyOccupied(t *testing.T) {
	var containers []corev1.Container

	destPort := 3306
	for p := minAvailPort; p < destPort; p++ {
		containers = append(containers, corev1.Container{Ports: []corev1.ContainerPort{{ContainerPort: int32(p)}}})
	}

	containerPorts := []int32{minAvailPort + 1}
	foundPorts, err := getAvailableContainerPorts(containers, containerPorts)
	if err != nil {
		t.Error("expect getAvailableContainerPorts success")
	}
	if len(foundPorts) != 1 || foundPorts[0] != int32(destPort) {
		t.Error("expect getAvailableContainerPorts returns 3306")
	}
}

func TestGetAvailableContainerPortsFullyOccupied(t *testing.T) {
	var containers []corev1.Container

	for p := minAvailPort; p <= maxAvailPort; p++ {
		containers = append(containers, corev1.Container{Ports: []corev1.ContainerPort{{ContainerPort: int32(p)}}})
	}

	containerPorts := []int32{3306}
	_, err := getAvailableContainerPorts(containers, containerPorts)
	if err == nil {
		t.Error("expect getAvailableContainerPorts return error")
	}
}
