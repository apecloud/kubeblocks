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

package plan

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"k8s.io/utils/ptr"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
)

var _ = Describe("TLSUtilsTest", func() {
	Context("ComposeTLSSecret function", func() {
		It("should work well", func() {
			compDef := &appsv1.ComponentDefinition{
				Spec: appsv1.ComponentDefinitionSpec{
					TLS: &appsv1.TLS{
						CAFile:   ptr.To("ca.pem"),
						CertFile: ptr.To("cert.pem"),
						KeyFile:  ptr.To("key.pem"),
					},
				},
			}
			synthesizedComp := component.SynthesizedComponent{
				Namespace:   testCtx.DefaultNamespace,
				ClusterName: "bar",
				Name:        "test",
			}
			secret, err := ComposeTLSSecret(compDef, synthesizedComp, nil)
			Expect(err).Should(BeNil())
			Expect(secret).ShouldNot(BeNil())
			Expect(secret.Name).Should(Equal(GenerateTLSSecretName(synthesizedComp.ClusterName, synthesizedComp.Name)))
			Expect(secret.Labels).ShouldNot(BeNil())
			Expect(secret.Labels[constant.AppInstanceLabelKey]).Should(Equal(synthesizedComp.ClusterName))
			Expect(secret.Labels[constant.AppManagedByLabelKey]).Should(Equal(constant.AppName))
			Expect(secret.Labels[constant.KBAppComponentLabelKey]).Should(Equal(synthesizedComp.Name))
			Expect(secret.StringData).ShouldNot(BeNil())
			Expect(secret.StringData[*compDef.Spec.TLS.CAFile]).ShouldNot(BeZero())
			Expect(secret.StringData[*compDef.Spec.TLS.CertFile]).ShouldNot(BeZero())
			Expect(secret.StringData[*compDef.Spec.TLS.KeyFile]).ShouldNot(BeZero())
		})
	})
})
