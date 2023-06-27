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
		container := corev1.Container{
			Name:  "foo-1",
			Image: "bar-2",
			Ports: []corev1.ContainerPort{
				{
					Name:          "foo-1",
					Protocol:      corev1.ProtocolTCP,
					ContainerPort: port,
				},
			},
		}
		containers := []corev1.Container{
			{
				Name:  "foo-2",
				Image: "bar-2",
				Ports: []corev1.ContainerPort{
					{
						Name:          "foo-2",
						Protocol:      corev1.ProtocolTCP,
						ContainerPort: port,
					},
				},
			},
		}
		pod := NewPodBuilder(ns, name).
			SetContainers(containers).
			AddContainer(container).
			GetObject()

		Expect(pod.Name).Should(Equal(name))
		Expect(pod.Namespace).Should(Equal(ns))
		Expect(len(pod.Spec.Containers)).Should(Equal(2))
		Expect(pod.Spec.Containers[0]).Should(Equal(containers[0]))
		Expect(pod.Spec.Containers[1]).Should(Equal(container))
	})
})
