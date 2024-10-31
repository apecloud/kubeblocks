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
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"

	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
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
			SetCredential(credential).
			SetTemplate(template).
			SetCustomHandler(observeActions).
			GetObject()
	})

	Context("injectRoleProbeBaseContainer function", func() {
		It("should reuse container 'kb-role-probe' if exists", func() {
			templateCopy := template.DeepCopy()
			templateCopy.Spec.Containers = append(templateCopy.Spec.Containers, corev1.Container{
				Name:  roleProbeContainerName,
				Image: "bar",
				Ports: []corev1.ContainerPort{
					{
						Name:          roleProbeGRPCPortName,
						ContainerPort: defaultRoleProbeGRPCPort,
					},
					{
						Name:          roleProbeDaemonPortName,
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

		It("should not use default grpcPort in case of 'probe-grpc-port' existence", func() {
			its.Spec.RoleProbe.RoleUpdateMechanism = workloads.ReadinessProbeEventUpdate
			templateCopy := template.DeepCopy()
			templateCopy.Spec.Containers = append(templateCopy.Spec.Containers, corev1.Container{
				Name:  roleProbeContainerName,
				Image: "bar",
				Ports: []corev1.ContainerPort{
					{
						Name:          roleProbeGRPCPortName,
						ContainerPort: 9555,
					},
					{
						Name:          roleProbeDaemonPortName,
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
				Name:  roleProbeContainerName,
				Image: "bar",
				Ports: []corev1.ContainerPort{
					{
						Name:          roleProbeGRPCPortName,
						ContainerPort: defaultRoleProbeGRPCPort,
					},
					{
						Name:          roleProbeDaemonPortName,
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
				Name:  roleProbeContainerName,
				Image: "bar",
				Ports: []corev1.ContainerPort{
					{
						Name:          roleProbeGRPCPortName,
						ContainerPort: defaultRoleProbeGRPCPort,
					},
					{
						Name:          roleProbeDaemonPortName,
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
})
