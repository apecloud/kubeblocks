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

package builder

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
)

var _ = Describe("pod builder", func() {
	It("should work well", func() {
		name := "foo"
		ns := "default"
		port := int32(12345)
		container := *NewContainerBuilder("foo-1").
			SetImage("bar-1").
			AddPorts(corev1.ContainerPort{
				Name:          "foo-1",
				Protocol:      corev1.ProtocolTCP,
				ContainerPort: port,
			}).GetObject()
		containers := []corev1.Container{
			*NewContainerBuilder("foo-2").SetImage("bar-2").
				AddPorts(corev1.ContainerPort{
					Name:          "foo-2",
					Protocol:      corev1.ProtocolTCP,
					ContainerPort: port,
				}).GetObject(),
		}
		volumes := []corev1.Volume{
			{
				Name: "data",
				VolumeSource: corev1.VolumeSource{
					EmptyDir: &corev1.EmptyDirVolumeSource{},
				},
			},
		}
		restartPolicy := corev1.RestartPolicyOnFailure
		user := int64(0)
		ctx := corev1.PodSecurityContext{
			RunAsUser: &user,
		}
		tolerations := []corev1.Toleration{
			{
				Key:      "node",
				Operator: corev1.TolerationOpEqual,
				Value:    "node-0",
			},
		}
		nodeSelector := map[string]string{
			"label1": "value1",
			"label2": "value2",
		}
		pod := NewPodBuilder(ns, name).
			SetContainers(containers).
			AddContainer(container).
			AddVolumes(volumes...).
			SetRestartPolicy(restartPolicy).
			SetSecurityContext(ctx).
			AddServiceAccount("my_test").
			SetNodeSelector(nodeSelector).
			AddTolerations(tolerations...).
			GetObject()

		Expect(pod.Name).Should(Equal(name))
		Expect(pod.Namespace).Should(Equal(ns))
		Expect(pod.Spec.Containers).Should(HaveLen(2))
		Expect(pod.Spec.Containers[0]).Should(Equal(containers[0]))
		Expect(pod.Spec.Containers[1]).Should(Equal(container))
		Expect(pod.Spec.Volumes).Should(HaveLen(1))
		Expect(pod.Spec.Volumes[0]).Should(Equal(volumes[0]))
		Expect(pod.Spec.RestartPolicy).Should(Equal(restartPolicy))
		Expect(pod.Spec.SecurityContext).ShouldNot(BeNil())
		Expect(*pod.Spec.SecurityContext).Should(Equal(ctx))
		Expect(pod.Spec.Tolerations).Should(HaveLen(1))
		Expect(pod.Spec.Tolerations[0]).Should(Equal(tolerations[0]))
		Expect(pod.Spec.ServiceAccountName).Should(BeEquivalentTo("my_test"))
		Expect(pod.Spec.NodeSelector).Should(BeEquivalentTo(nodeSelector))
	})
})
