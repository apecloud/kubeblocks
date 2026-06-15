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

package parameters

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"k8s.io/utils/ptr"
)

var _ = Describe("parameter overlay markers", func() {
	It("encodes nil values and decodes markers", func() {
		Expect(EncodeParameterOverlay(nil)).To(BeNil())
		Expect(DecodeParameterOverlay(nil)).To(BeNil())

		encoded := EncodeParameterOverlay(map[string]*string{
			"remove": nil,
			"keep":   ptr.To("value"),
		})
		Expect(encoded["remove"]).NotTo(BeNil())
		Expect(*encoded["remove"]).To(Equal(parameterRemoveMarker))
		Expect(encoded["keep"]).To(Equal(ptr.To("value")))

		decoded := DecodeParameterOverlay(encoded)
		Expect(decoded["remove"]).To(BeNil())
		Expect(decoded["keep"]).To(Equal(ptr.To("value")))
	})
})
