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
)

var _ = Describe("configmap builder", func() {
	It("should work well", func() {
		const (
			name = "foo"
			ns   = "default"
		)
		cm := NewConfigMapBuilder(ns, name).
			SetData(map[string]string{
				"foo-1": "bar-1",
			}).
			PutData("foo-2", "bar-2").
			SetBinaryData(map[string][]byte{
				"foo-3": []byte("bar-3"),
			}).
			PutBinaryData("foo-4", []byte("bar-4")).
			SetImmutable(true).
			GetObject()

		Expect(cm.Name).Should(Equal(name))
		Expect(cm.Namespace).Should(Equal(ns))
		Expect(cm.Data).ShouldNot(BeNil())
		Expect(cm.Data["foo-1"]).Should(Equal("bar-1"))
		Expect(cm.Data["foo-2"]).Should(Equal("bar-2"))
		Expect(cm.BinaryData).ShouldNot(BeNil())
		Expect(cm.BinaryData["foo-3"]).Should(Equal([]byte("bar-3")))
		Expect(cm.BinaryData["foo-4"]).Should(Equal([]byte("bar-4")))
		Expect(cm.Immutable).ShouldNot(BeNil())
		Expect(*cm.Immutable).Should(BeTrue())
	})
})
