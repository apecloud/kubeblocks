/*
Copyright (C) 2022-2025 ApeCloud Co., Ltd

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

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
)

var _ = Describe("TLS test", func() {
	It("ComposeTLSCertsWithSecret", func() {
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
			ClusterName: "foo",
			Name:        "bar",
		}
		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: testCtx.DefaultNamespace,
				Name:      "foo-bar-tls",
			},
			Data: map[string][]byte{},
		}
		_, err := ComposeTLSCertsWithSecret(compDef, synthesizedComp, secret)
		Expect(err).Should(BeNil())
		Expect(secret.Data).ShouldNot(BeNil())
		Expect(secret.Data[*compDef.Spec.TLS.CAFile]).ShouldNot(BeZero())
		Expect(secret.Data[*compDef.Spec.TLS.CertFile]).ShouldNot(BeZero())
		Expect(secret.Data[*compDef.Spec.TLS.KeyFile]).ShouldNot(BeZero())
	})
})
