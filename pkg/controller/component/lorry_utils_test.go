/*
Copyright (C) 2022-2023 ApeCloud Co., Ltd

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
	"reflect"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	viper "github.com/apecloud/kubeblocks/pkg/viperx"
)

var _ = Describe("Lorry Utils", func() {

	Context("build probe containers", func() {
		var container *corev1.Container
		var component *SynthesizedComponent
		var probeServiceHTTPPort int
		var probeServiceGRPCPort int
		var clusterDefProbe *appsv1alpha1.ClusterDefinitionProbe

		BeforeEach(func() {
			probeServiceHTTPPort = 3501
			probeServiceGRPCPort = 50001

			clusterDefProbe = &appsv1alpha1.ClusterDefinitionProbe{}
			clusterDefProbe.PeriodSeconds = 1
			clusterDefProbe.TimeoutSeconds = 1
			clusterDefProbe.FailureThreshold = 1

			component = &SynthesizedComponent{}
			component.CharacterType = "mysql"
			component.Services = append(component.Services, corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name: "mysql",
				},
				Spec: corev1.ServiceSpec{
					Ports: []corev1.ServicePort{{
						Protocol: corev1.ProtocolTCP,
						Port:     3306,
					}},
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
			component.Probes = &appsv1alpha1.ClusterDefinitionProbes{
				RunningProbe: &appsv1alpha1.ClusterDefinitionProbe{},
				StatusProbe:  &appsv1alpha1.ClusterDefinitionProbe{},
				RoleProbe:    &appsv1alpha1.ClusterDefinitionProbe{},
			}
			component.LifecycleActions = &appsv1alpha1.ComponentLifecycleActions{
				RoleProbe: &appsv1alpha1.RoleProbe{},
			}
			component.PodSpec = &corev1.PodSpec{
				Containers: []corev1.Container{},
			}

			container = buildBasicContainer(component)
		})

		It("build role probe containers", func() {
			reqCtx := intctrlutil.RequestCtx{
				Ctx: ctx,
				Log: logger,
			}
			defaultBuiltInHandler := appsv1alpha1.MySQLBuiltinActionHandler
			component.LifecycleActions = &appsv1alpha1.ComponentLifecycleActions{
				RoleProbe: &appsv1alpha1.RoleProbe{
					LifecycleActionHandler: appsv1alpha1.LifecycleActionHandler{
						BuiltinHandler: &defaultBuiltInHandler,
					},
				},
			}
			Expect(buildLorryContainers(reqCtx, component, nil)).Should(Succeed())
			Expect(component.PodSpec.Containers).Should(HaveLen(1))
			Expect(component.PodSpec.Containers[0].Name).Should(Equal(constant.RoleProbeContainerName))
		})

		It("should build role service container", func() {
			buildLorryServiceContainer(component, container, probeServiceHTTPPort, probeServiceGRPCPort, nil)
			Expect(container.Command).ShouldNot(BeEmpty())
		})

		It("build we-syncer container", func() {
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
			Expect(component.PodSpec.Containers[0].Name).Should(Equal(constant.WeSyncerContainerName))
		})

		It("build volume protection probe container without RBAC", func() {
			reqCtx := intctrlutil.RequestCtx{
				Ctx: ctx,
				Log: logger,
			}
			zeroWatermark := 0
			component.VolumeProtection = &appsv1alpha1.VolumeProtectionSpec{
				HighWatermark: 90,
				Volumes: []appsv1alpha1.ProtectedVolume{
					{
						Name: "volume-001",
					},
					{
						Name:          "volume-002",
						HighWatermark: &zeroWatermark,
					},
				},
			}
			defaultBuiltInHandler := appsv1alpha1.MySQLBuiltinActionHandler
			component.LifecycleActions = &appsv1alpha1.ComponentLifecycleActions{
				RoleProbe: &appsv1alpha1.RoleProbe{
					LifecycleActionHandler: appsv1alpha1.LifecycleActionHandler{
						BuiltinHandler: &defaultBuiltInHandler,
					},
				},
			}
			Expect(buildLorryContainers(reqCtx, component, nil)).Should(Succeed())
			Expect(component.PodSpec.Containers).Should(HaveLen(2))
			Expect(component.PodSpec.Containers[0].Name).Should(Equal(constant.RoleProbeContainerName))
			Expect(component.PodSpec.Containers[1].Name).Should(Equal(constant.VolumeProtectionProbeContainerName))
		})

		It("build volume protection probe container with RBAC", func() {
			reqCtx := intctrlutil.RequestCtx{
				Ctx: ctx,
				Log: logger,
			}
			zeroWatermark := 0
			component.VolumeProtection = &appsv1alpha1.VolumeProtectionSpec{
				HighWatermark: 90,
				Volumes: []appsv1alpha1.ProtectedVolume{
					{
						Name: "volume-001",
					},
					{
						Name:          "volume-002",
						HighWatermark: &zeroWatermark,
					},
				},
			}
			defaultBuiltInHandler := appsv1alpha1.MySQLBuiltinActionHandler
			component.LifecycleActions = &appsv1alpha1.ComponentLifecycleActions{
				RoleProbe: &appsv1alpha1.RoleProbe{
					LifecycleActionHandler: appsv1alpha1.LifecycleActionHandler{
						BuiltinHandler: &defaultBuiltInHandler,
					},
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
			Expect(reflect.DeepEqual(component.VolumeProtection, spec)).Should(BeTrue())
		})
	})
})
