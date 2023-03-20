/*
Copyright ApeCloud, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
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
