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

var _ = Describe("job builder", func() {
	It("should work well", func() {
		const (
			name = "foo"
			ns   = "default"
			port = int32(12345)
		)
		pod := NewPodBuilder(ns, "foo").
			AddContainer(corev1.Container{
				Name:  "foo",
				Image: "bar",
				Ports: []corev1.ContainerPort{
					{
						Name:          "foo",
						Protocol:      corev1.ProtocolTCP,
						ContainerPort: port,
					},
				},
			}).GetObject()
		template := corev1.PodTemplateSpec{
			ObjectMeta: pod.ObjectMeta,
			Spec:       pod.Spec,
		}
		selectorKey, selectorValue := "foo", "bar"
		suspend := true
		job := NewJobBuilder(ns, name).
			SetPodTemplateSpec(template).
			AddSelector(selectorKey, selectorValue).
			SetSuspend(suspend).
			GetObject()

		Expect(job.Name).Should(Equal(name))
		Expect(job.Namespace).Should(Equal(ns))
		Expect(job.Spec.Template).Should(Equal(template))
		Expect(job.Spec.Selector).ShouldNot(BeNil())
		Expect(job.Spec.Selector.MatchLabels).ShouldNot(BeNil())
		Expect(job.Spec.Selector.MatchLabels[selectorKey]).Should(Equal(selectorValue))
		Expect(job.Spec.Suspend).ShouldNot(BeNil())
		Expect(*job.Spec.Suspend).Should(Equal(suspend))
	})
})
