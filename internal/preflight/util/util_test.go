/*
Copyright (C) 2022 ApeCloud Co., Ltd

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

package util

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	troubleshoot "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/replicatedhq/troubleshoot/pkg/multitype"
)

var _ = Describe("util_test", func() {
	It("IsExcluded test", func() {
		Eventually(func(g Gomega) {
			By("tests with normal case, and expect success")
			tests := []*multitype.BoolOrString{nil, {Type: 1, BoolVal: true, StrVal: ""}, {Type: 0, BoolVal: false, StrVal: "true"}, {Type: 0, BoolVal: false, StrVal: ""}}
			var resList []bool
			for _, test := range tests {
				res, err := IsExcluded(test)
				g.Expect(err).NotTo(HaveOccurred())
				resList = append(resList, res)
			}
			g.Expect(resList).Should(Equal([]bool{false, true, true, false}))
			By("test with corner case, and expect error")
			cornerTest := &multitype.BoolOrString{Type: 0, BoolVal: false, StrVal: "i am true"}
			res, err := IsExcluded(cornerTest)
			g.Expect(err).To(HaveOccurred())
			g.Expect(res).Should(Equal(false))
		}).Should(Succeed())
	})

	It("TitleOrDefault test", func() {
		Eventually(func(g Gomega) {
			res := TitleOrDefault(troubleshoot.HostCollectorMeta{}, "default")
			g.Expect(res).Should(Equal("default"))
			res = TitleOrDefault(troubleshoot.HostCollectorMeta{CollectorName: "collectName"}, "default")
			g.Expect(res).Should(Equal("collectName"))
			res = TitleOrDefault(troubleshoot.AnalyzeMeta{}, "default")
			g.Expect(res).Should(Equal("default"))
			res = TitleOrDefault(troubleshoot.AnalyzeMeta{CheckName: "checkName"}, "default")
			g.Expect(res).Should(Equal("checkName"))
		}).Should(Succeed())
	})

})
