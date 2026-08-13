/*
Copyright (C) 2022-2026 ApeCloud Co., Ltd

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

	"github.com/apecloud/kubeblocks/pkg/controller/component"
)

var _ = Describe("TLS test", func() {
	It("ComposeTLSCertsWithSecret", func() {
		keys := TLSSecretKeys{
			CA:   ptr.To("ca.pem"),
			Cert: ptr.To("cert.pem"),
			Key:  ptr.To("key.pem"),
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
		_, err := ComposeTLSCertsWithSecret(keys, synthesizedComp, secret)
		Expect(err).Should(BeNil())
		Expect(secret.Data).ShouldNot(BeNil())
		Expect(secret.Data[*keys.CA]).ShouldNot(BeZero())
		Expect(secret.Data[*keys.Cert]).ShouldNot(BeZero())
		Expect(secret.Data[*keys.Key]).ShouldNot(BeZero())
	})
})
