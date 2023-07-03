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

var _ = Describe("secret builder", func() {
	It("should work well", func() {
		const (
			name = "foo"
			ns   = "default"
		)
		secret := NewSecretBuilder(ns, name).
			SetStringData(map[string]string{
				"foo-1": "bar-1",
			}).
			PutStringData("foo-2", "bar-2").
			SetData(map[string][]byte{
				"foo-3": []byte("bar-3"),
			}).
			PutData("foo-4", []byte("bar-4")).
			SetImmutable(true).
			SetSecretType(corev1.SecretTypeOpaque).
			GetObject()

		Expect(secret.Name).Should(Equal(name))
		Expect(secret.Namespace).Should(Equal(ns))
		Expect(secret.StringData).ShouldNot(BeNil())
		Expect(secret.StringData["foo-1"]).Should(Equal("bar-1"))
		Expect(secret.StringData["foo-2"]).Should(Equal("bar-2"))
		Expect(secret.Data).ShouldNot(BeNil())
		Expect(secret.Data["foo-3"]).Should(Equal([]byte("bar-3")))
		Expect(secret.Data["foo-4"]).Should(Equal([]byte("bar-4")))
		Expect(secret.Immutable).ShouldNot(BeNil())
		Expect(*secret.Immutable).Should(BeTrue())
		Expect(secret.Type).Should(Equal(corev1.SecretTypeOpaque))
	})
})
