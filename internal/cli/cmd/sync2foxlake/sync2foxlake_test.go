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

package sync2foxlake

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("sync2foxlake", func() {

	It("func test", func() {
		e := EndpointModel{}
		Expect(e.buildFromStr("")).Should(HaveOccurred())

		Expect(e.buildFromStr("test")).Should(Succeed())
		Expect(e.EndpointType).Should(Equal(ClusterNameEndpointType))
		Expect(e.Endpoint).Should(Equal("test"))

		Expect(e.buildFromStr("user:123456@127.0.0.1:5432")).Should(Succeed())
		Expect(e.EndpointType).Should(Equal(AddressEndpointType))
		Expect(e.Endpoint).Should(Equal("127.0.0.1:5432"))
		Expect(e.UserName).Should(Equal("user"))
		Expect(e.Password).Should(Equal("123456"))
		Expect(e.Host).Should(Equal("127.0.0.1"))
		Expect(e.Port).Should(Equal("5432"))

		Expect(e.buildFromStr("user@host")).Should(HaveOccurred())
	})

})
