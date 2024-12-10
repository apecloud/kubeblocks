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
			GetObject()
	})

	Context("headless service", func() {
		It("getHeadlessSvcName", func() {
			Expect(getHeadlessSvcName(its.Name)).Should(Equal("bar-headless"))
		})

		It("buildHeadlessSvc - duplicate port names", func() {
			port := its.Spec.Template.Spec.Containers[0].Ports[0]
			its.Spec.Template.Spec.Containers = append(its.Spec.Template.Spec.Containers, corev1.Container{
				Name:  "duplicate-port-name",
				Image: "image",
				Ports: []corev1.ContainerPort{
					{
						Name:          port.Name,
						Protocol:      port.Protocol,
						ContainerPort: port.ContainerPort + 1,
					},
				},
			})
			svc := buildHeadlessSvc(*its, nil, nil)
			Expect(svc).ShouldNot(BeNil())
			Expect(len(svc.Spec.Ports)).Should(Equal(2))
			Expect(svc.Spec.Ports[0].Name).Should(Equal(port.Name))
			Expect(svc.Spec.Ports[1].Name).ShouldNot(Equal(port.Name))
		})
	})
})
