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
	"k8s.io/apimachinery/pkg/util/intstr"
)

var _ = Describe("service builder", func() {
	It("should work well", func() {
		const (
			name                         = "foo"
			ns                           = "default"
			selectorKey1, selectorValue1 = "foo-1", "bar-1"
			selectorKey2, selectorValue2 = "foo-2", "bar-2"
			selectorKey3, selectorValue3 = "foo-3", "bar-3"
			selectorKey4, selectorValue4 = "foo-4", "bar-4"
			port                         = int32(12345)
		)
		selectors := map[string]string{selectorKey4: selectorValue4}
		ports := []corev1.ServicePort{
			{
				Name:     "foo-1",
				Protocol: corev1.ProtocolTCP,
				Port:     port,
			},
		}
		containerPorts := []corev1.ContainerPort{
			{
				Name:          "foo-2",
				Protocol:      corev1.ProtocolTCP,
				ContainerPort: port,
			},
		}
		serviceType := corev1.ServiceTypeLoadBalancer
		svc := NewHeadlessServiceBuilder(ns, name).
			AddSelector(selectorKey1, selectorValue1).
			AddSelectors(selectorKey2, selectorValue2, selectorKey3, selectorValue3).
			AddSelectorsInMap(selectors).
			AddPorts(ports...).
			AddContainerPorts(containerPorts...).
			SetType(serviceType).
			GetObject()

		Expect(svc.Name).Should(Equal(name))
		Expect(svc.Namespace).Should(Equal(ns))
		Expect(svc.Spec.Selector).ShouldNot(BeNil())
		Expect(len(svc.Spec.Selector)).Should(Equal(4))
		Expect(svc.Spec.Selector[selectorKey1]).Should(Equal(selectorValue1))
		Expect(svc.Spec.Selector[selectorKey2]).Should(Equal(selectorValue2))
		Expect(svc.Spec.Selector[selectorKey3]).Should(Equal(selectorValue3))
		Expect(svc.Spec.Selector[selectorKey4]).Should(Equal(selectorValue4))
		Expect(svc.Spec.Ports).ShouldNot(BeNil())
		Expect(len(svc.Spec.Ports)).Should(Equal(2))
		Expect(svc.Spec.Ports[0]).Should(Equal(ports[0]))
		Expect(svc.Spec.Type).Should(Equal(serviceType))
		Expect(svc.Spec.ExternalTrafficPolicy).Should(Equal(corev1.ServiceExternalTrafficPolicyTypeLocal))
		hasPort := func(containerPort corev1.ContainerPort, servicePorts []corev1.ServicePort) bool {
			for _, servicePort := range servicePorts {
				if containerPort.Protocol == servicePort.Protocol &&
					intstr.FromString(containerPort.Name) == servicePort.TargetPort {
					return true
				}
			}
			return false
		}
		for _, containerPort := range containerPorts {
			Expect(hasPort(containerPort, svc.Spec.Ports)).Should(BeTrue())
		}
	})
})
