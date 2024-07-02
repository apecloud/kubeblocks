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

package instanceset

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/builder"
)

var _ = Describe("object generation transformer test.", func() {
	BeforeEach(func() {
		its = builder.NewInstanceSetBuilder(namespace, name).
			SetUID(uid).
			AddLabels(constant.AppComponentLabelKey, name).
			SetReplicas(3).
			AddMatchLabelsInMap(selectors).
			SetRoles(roles).
			SetService(service).
			SetCredential(credential).
			SetTemplate(template).
			SetCustomHandler(observeActions).
			GetObject()
	})

	Context("buildEnvConfigData function", func() {
		It("should work well", func() {
			By("build env config data")
			its.Status.MembersStatus = []workloads.MemberStatus{
				{
					PodName:     getPodName(its.Name, 1),
					ReplicaRole: &workloads.ReplicaRole{Name: "leader", IsLeader: true},
				},
				{
					PodName:     getPodName(its.Name, 0),
					ReplicaRole: &workloads.ReplicaRole{Name: "follower", CanVote: true},
				},
				{
					PodName:     getPodName(its.Name, 2),
					ReplicaRole: &workloads.ReplicaRole{Name: "follower", CanVote: true},
				},
			}
			requiredKeys := []string{
				"KB_REPLICA_COUNT",
				"KB_0_HOSTNAME",
			}
			cfg, err := buildEnvConfigData(*its)
			Expect(err).Should(BeNil())
			By("builds Env Config correctly")
			Expect(cfg).ShouldNot(BeNil())
			for _, k := range requiredKeys {
				_, ok := cfg[k]
				Expect(ok).Should(BeTrue())
			}

			By("builds Env Config with ConsensusSet status correctly")
			toCheckKeys := append(requiredKeys, []string{
				"KB_LEADER",
				"KB_FOLLOWERS",
			}...)
			for _, k := range toCheckKeys {
				_, ok := cfg[k]
				Expect(ok).Should(BeTrue())
			}
		})

		It("non-sequential ordinal", func() {
			By("build env config data")
			its.Spec.OfflineInstances = []string{
				getPodName(its.Name, 1),
			}
			hostname := func(i int) string {
				return fmt.Sprintf("%s.%s", getPodName(its.Name, i), getHeadlessSvcName(its.Name))
			}
			requiredKeys := map[string]string{
				"KB_REPLICA_COUNT": "3",
				"KB_0_HOSTNAME":    hostname(0),
				"KB_2_HOSTNAME":    hostname(2),
				"KB_3_HOSTNAME":    hostname(3),
			}
			cfg, err := buildEnvConfigData(*its)
			Expect(err).Should(BeNil())

			By("builds Env Config correctly")
			Expect(cfg).ShouldNot(BeNil())
			for k, v := range requiredKeys {
				Expect(cfg).Should(HaveKeyWithValue(k, v))
			}
			Expect(cfg).ShouldNot(HaveKey("KB_1_HOSTNAME"))
		})
	})

	Context("well-known service labels", func() {
		It("should work well", func() {
			svc := buildSvc(*its, getMatchLabels(its.Name), getSvcSelector(its, false))
			Expect(svc).ShouldNot(BeNil())
			for k, ev := range service.Labels {
				v, ok := svc.Labels[k]
				Expect(ok).Should(BeTrue())
				Expect(v).Should(Equal(ev))
			}
		})
	})

	Context("injectRoleProbeBaseContainer function", func() {
		It("should reuse container 'kb-checkrole' if exists", func() {
			templateCopy := template.DeepCopy()
			templateCopy.Spec.Containers = append(templateCopy.Spec.Containers, corev1.Container{
				Name:  constant.RoleProbeContainerName,
				Image: "bar",
				Ports: []corev1.ContainerPort{
					{
						Name:          constant.LorryGRPCPortName,
						ContainerPort: defaultRoleProbeGRPCPort,
					},
					{
						Name:          constant.LorryHTTPPortName,
						ContainerPort: defaultRoleProbeDaemonPort,
					},
				},
			})
			injectRoleProbeBaseContainer(its, templateCopy, "", nil)
			Expect(len(templateCopy.Spec.Containers)).Should(Equal(2))
			probeContainer := templateCopy.Spec.Containers[1]
			Expect(len(probeContainer.Ports)).Should(Equal(2))
			Expect(probeContainer.Ports[0].ContainerPort).Should(BeElementOf([]int32{int32(defaultRoleProbeGRPCPort), int32(defaultRoleProbeDaemonPort)}))
		})

		It("should not use default grpcPort in case of 'lorry-grpc-port' existence", func() {
			its.Spec.RoleProbe.RoleUpdateMechanism = workloads.ReadinessProbeEventUpdate
			templateCopy := template.DeepCopy()
			templateCopy.Spec.Containers = append(templateCopy.Spec.Containers, corev1.Container{
				Name:  constant.RoleProbeContainerName,
				Image: "bar",
				Ports: []corev1.ContainerPort{
					{
						Name:          constant.LorryGRPCPortName,
						ContainerPort: 9555,
					},
					{
						Name:          constant.LorryHTTPPortName,
						ContainerPort: defaultRoleProbeDaemonPort,
					},
				},
			})
			injectRoleProbeBaseContainer(its, templateCopy, "", nil)
			Expect(len(templateCopy.Spec.Containers)).Should(Equal(2))
			probeContainer := templateCopy.Spec.Containers[1]
			Expect(len(probeContainer.Ports)).Should(Equal(2))
			Expect(probeContainer.Ports[0].ContainerPort).Should(Equal(int32(9555)))
		})

		It("container.ports nil", func() {
			its.Spec.RoleProbe.RoleUpdateMechanism = workloads.ReadinessProbeEventUpdate
			templateCopy := template.DeepCopy()
			templateCopy.Spec.Containers = append(templateCopy.Spec.Containers, corev1.Container{
				Name:  constant.RoleProbeContainerName,
				Image: "bar",
				Ports: []corev1.ContainerPort{
					{
						Name:          constant.LorryGRPCPortName,
						ContainerPort: defaultRoleProbeGRPCPort,
					},
					{
						Name:          constant.LorryHTTPPortName,
						ContainerPort: defaultRoleProbeDaemonPort,
					},
				},
			})
			injectRoleProbeBaseContainer(its, templateCopy, "", nil)
			Expect(len(templateCopy.Spec.Containers)).Should(Equal(2))
			probeContainer := templateCopy.Spec.Containers[1]
			Expect(len(probeContainer.Ports)).Should(Equal(2))
			Expect(probeContainer.Ports[0].ContainerPort).Should(Equal(int32(defaultRoleProbeGRPCPort)))
		})

		It("container.ports.containerPort negative", func() {
			its.Spec.RoleProbe.RoleUpdateMechanism = workloads.ReadinessProbeEventUpdate
			templateCopy := template.DeepCopy()
			templateCopy.Spec.Containers = append(templateCopy.Spec.Containers, corev1.Container{
				Name:  constant.RoleProbeContainerName,
				Image: "bar",
				Ports: []corev1.ContainerPort{
					{
						Name:          constant.LorryGRPCPortName,
						ContainerPort: defaultRoleProbeGRPCPort,
					},
					{
						Name:          constant.LorryHTTPPortName,
						ContainerPort: defaultRoleProbeDaemonPort,
					},
				},
			})
			injectRoleProbeBaseContainer(its, templateCopy, "", nil)
			Expect(len(templateCopy.Spec.Containers)).Should(Equal(2))
			probeContainer := templateCopy.Spec.Containers[1]
			Expect(len(probeContainer.Ports)).Should(Equal(2))
			Expect(probeContainer.Ports[0].ContainerPort).Should(Equal(int32(defaultRoleProbeGRPCPort)))
		})
	})

	Context("getHeadlessSvcName function", func() {
		It("should work well", func() {
			Expect(getHeadlessSvcName(its.Name)).Should(Equal("bar-headless"))
		})
	})

	Context("findSvcPort function", func() {
		It("should work well", func() {
			By("set port name")
			its.Spec.Service.Spec.Ports = []corev1.ServicePort{
				{
					Name:       "svc-port",
					Protocol:   corev1.ProtocolTCP,
					Port:       12345,
					TargetPort: intstr.FromString("my-service"),
				},
			}
			containerPort := int32(54321)
			container := corev1.Container{
				Name: name,
				Ports: []corev1.ContainerPort{
					{
						Name:          "my-service",
						Protocol:      corev1.ProtocolTCP,
						ContainerPort: containerPort,
					},
				},
			}
			pod := builder.NewPodBuilder(namespace, getPodName(name, 0)).
				SetContainers([]corev1.Container{container}).
				GetObject()
			its.Spec.Template = corev1.PodTemplateSpec{
				ObjectMeta: pod.ObjectMeta,
				Spec:       pod.Spec,
			}
			Expect(findSvcPort(its)).Should(BeEquivalentTo(containerPort))

			By("set port number")
			its.Spec.Service.Spec.Ports = []corev1.ServicePort{
				{
					Name:       "svc-port",
					Protocol:   corev1.ProtocolTCP,
					Port:       12345,
					TargetPort: intstr.FromInt(int(containerPort)),
				},
			}
			Expect(findSvcPort(its)).Should(BeEquivalentTo(containerPort))

			By("set no matched port")
			its.Spec.Service.Spec.Ports = []corev1.ServicePort{
				{
					Name:       "svc-port",
					Protocol:   corev1.ProtocolTCP,
					Port:       12345,
					TargetPort: intstr.FromInt(int(containerPort - 1)),
				},
			}
			Expect(findSvcPort(its)).Should(BeZero())
		})
	})
})
