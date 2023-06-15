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

package cluster

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("cluster builder", func() {
	It("get cluster chart name", func() {
		res := getChartName(MySQL)
		Expect(res).Should(Equal("apecloud-mysql-cluster"))
	})

	It("get cluster schema", func() {
		res, err := GetClusterSchema(MySQL)
		Expect(err).Should(Succeed())
		Expect(res).ShouldNot(BeEmpty())
	})

	It("get cluster manifest", func() {
		manifests, err := GetManifests(MySQL, "default", "my", nil)
		Expect(err).Should(Succeed())
		Expect(manifests).ShouldNot(BeEmpty())
	})
})
