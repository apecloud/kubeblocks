/*
Copyright ApeCloud, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package component

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
)

var _ = Describe("probe_utils", func() {

	Context("build probe containers", func() {
		var container *corev1.Container
		var component *SynthesizedComponent
		var probeServiceHTTPPort, probeServiceGrpcPort int
		var clusterDefProbe *appsv1alpha1.ClusterDefinitionProbe

		BeforeEach(func() {
			var err error
			container, err = buildProbeContainer()
			Expect(err).NotTo(HaveOccurred())
			probeServiceHTTPPort, probeServiceGrpcPort = 3501, 50001

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
				RunningProbe:     &appsv1alpha1.ClusterDefinitionProbe{},
				StatusProbe:      &appsv1alpha1.ClusterDefinitionProbe{},
				RoleChangedProbe: &appsv1alpha1.ClusterDefinitionProbe{},
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
			Expect(buildProbeContainers(reqCtx, component)).Should(Succeed())
			Expect(len(component.PodSpec.Containers)).Should(Equal(3))
			Expect(component.PodSpec.Containers[0].Command).ShouldNot(BeEmpty())
		})

		It("should build role changed probe container", func() {
			buildRoleChangedProbeContainer("wesql", container, clusterDefProbe, probeServiceHTTPPort)
			Expect(container.ReadinessProbe.Exec.Command).ShouldNot(BeEmpty())
		})

		It("should build role service container", func() {
			buildProbeServiceContainer(component, container, probeServiceHTTPPort, probeServiceGrpcPort)
			Expect(container.Command).ShouldNot(BeEmpty())
		})

		It("should build status probe container", func() {
			buildStatusProbeContainer(container, clusterDefProbe, probeServiceHTTPPort)
			Expect(container.ReadinessProbe.HTTPGet).ShouldNot(BeNil())
		})

		It("should build running probe container", func() {
			buildRunningProbeContainer(container, clusterDefProbe, probeServiceHTTPPort)
			Expect(container.ReadinessProbe.HTTPGet).ShouldNot(BeNil())
		})
	})
})
