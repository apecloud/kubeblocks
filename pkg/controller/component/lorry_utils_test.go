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

var _ = Describe("probe_utils", func() {

	Context("build probe containers", func() {
		var container *corev1.Container
		var component *SynthesizedComponent
		var probeServiceHTTPPort int
		var probeServiceGRPCPort int
		var clusterDefProbe *appsv1alpha1.ClusterDefinitionProbe

		BeforeEach(func() {
			container = buildBasicContainer()
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
			component.ConsensusSpec = &appsv1alpha1.ConsensusSetSpec{
				Leader: appsv1alpha1.ConsensusMember{
					Name:       "leader",
					AccessMode: appsv1alpha1.ReadWrite,
				},
				Followers: []appsv1alpha1.ConsensusMember{{
					Name:       "follower",
					AccessMode: appsv1alpha1.Readonly,
				}},
				Learner: &appsv1alpha1.ConsensusMember{
					Name:       "learner",
					AccessMode: appsv1alpha1.Readonly,
				},
			}
			component.Probes = &appsv1alpha1.ClusterDefinitionProbes{
				RunningProbe: &appsv1alpha1.ClusterDefinitionProbe{},
				StatusProbe:  &appsv1alpha1.ClusterDefinitionProbe{},
				RoleProbe:    &appsv1alpha1.ClusterDefinitionProbe{},
			}
			component.PodSpec = &corev1.PodSpec{
				Containers: []corev1.Container{},
			}
		})

		It("should build multiple probe containers", func() {
			reqCtx := intctrlutil.RequestCtx{
				Ctx: ctx,
				Log: logger,
			}
			Expect(buildLorryContainers(reqCtx, component)).Should(Succeed())
			Expect(len(component.PodSpec.Containers) >= 2).Should(BeTrue())
			Expect(component.PodSpec.Containers[0].Command).ShouldNot(BeEmpty())
		})

		It("should build role service container", func() {
			buildLorryServiceContainer(component, container, probeServiceHTTPPort, probeServiceGRPCPort)
			Expect(container.Command).ShouldNot(BeEmpty())
		})

		It("should build status probe container", func() {
			buildStatusProbeContainer("wesql", container, clusterDefProbe, probeServiceHTTPPort)
			Expect(container.ReadinessProbe.HTTPGet).ShouldNot(BeNil())
		})

		It("should build running probe container", func() {
			buildRunningProbeContainer("wesql", container, clusterDefProbe, probeServiceHTTPPort)
			Expect(container.ReadinessProbe.HTTPGet).ShouldNot(BeNil())
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
			Expect(buildLorryContainers(reqCtx, component)).Should(Succeed())
			Expect(len(component.PodSpec.Containers) >= 3).Should(BeTrue())
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
			viper.SetDefault(constant.EnableRBACManager, true)
			Expect(buildLorryContainers(reqCtx, component)).Should(Succeed())
			Expect(len(component.PodSpec.Containers) >= 3).Should(BeTrue())
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
