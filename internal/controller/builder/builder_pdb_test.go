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
	"k8s.io/apimachinery/pkg/util/intstr"
)

var _ = Describe("pdb builder", func() {
	It("should work well", func() {
		const (
			name                         = "foo"
			ns                           = "default"
			selectorKey1, selectorValue1 = "foo-1", "bar-1"
			selectorKey2, selectorValue2 = "foo-2", "bar-2"
			selectorKey3, selectorValue3 = "foo-3", "bar-3"
			selectorKey4, selectorValue4 = "foo-4", "bar-4"
		)
		selectors := map[string]string{selectorKey4: selectorValue4}
		minAvailable := intstr.FromInt(3)
		pdb := NewPDBBuilder(ns, name).
			AddSelector(selectorKey1, selectorValue1).
			AddSelectors(selectorKey2, selectorValue2, selectorKey3, selectorValue3).
			AddSelectorsInMap(selectors).
			SetMinAvailable(minAvailable).
			GetObject()

		Expect(pdb.Name).Should(Equal(name))
		Expect(pdb.Namespace).Should(Equal(ns))
		Expect(pdb.Spec.Selector).ShouldNot(BeNil())
		Expect(pdb.Spec.Selector.MatchLabels).ShouldNot(BeNil())
		Expect(pdb.Spec.Selector).ShouldNot(BeNil())
		Expect(pdb.Spec.Selector.MatchLabels).Should(HaveLen(4))
		Expect(pdb.Spec.Selector.MatchLabels[selectorKey1]).Should(Equal(selectorValue1))
		Expect(pdb.Spec.Selector.MatchLabels[selectorKey2]).Should(Equal(selectorValue2))
		Expect(pdb.Spec.Selector.MatchLabels[selectorKey3]).Should(Equal(selectorValue3))
		Expect(pdb.Spec.Selector.MatchLabels[selectorKey4]).Should(Equal(selectorValue4))
		Expect(pdb.Spec.MinAvailable).ShouldNot(BeNil())
		Expect(*pdb.Spec.MinAvailable).Should(Equal(minAvailable))
	})
})
