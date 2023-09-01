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

package testing

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("test fake in internal/preflight", func() {
	It("test FakeKbPreflight", func() {
		kbPreflight := FakeKbPreflight()
		Expect(kbPreflight).ShouldNot(BeNil())
		Expect(len(kbPreflight.Spec.Collectors)).Should(BeNumerically(">", 0))
	})

	It("test FakeKbHostPreflight", func() {
		hostKbPreflight := FakeKbHostPreflight()
		Expect(hostKbPreflight).ShouldNot(BeNil())
		Expect(len(hostKbPreflight.Spec.RemoteCollectors)).Should(BeNumerically(">", 0))
		Expect(len(hostKbPreflight.Spec.ExtendCollectors)).Should(BeNumerically(">", 0))
	})

	It("test FakeAnalyzers", func() {
		analyzers := FakeAnalyzers()
		Expect(analyzers).ShouldNot(BeNil())
		Expect(len(analyzers)).Should(BeNumerically(">", 0))
	})
})
