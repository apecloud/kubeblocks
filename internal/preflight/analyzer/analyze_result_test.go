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

package analyzer

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	troubleshoot "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
)

var _ = Describe("kb_storage_class_test", func() {
	var (
		outcomes []*troubleshoot.Outcome
	)
	Context("analyze storage class test", func() {
		BeforeEach(func() {
			outcomes = []*troubleshoot.Outcome{
				{
					Pass: &troubleshoot.SingleOutcome{

						Message: "analyze storage class success",
					},
					Fail: &troubleshoot.SingleOutcome{
						Message: "analyze storage class fail",
					},
					Warn: &troubleshoot.SingleOutcome{
						Message: "warn message",
					},
				},
			}
		})
		It("AnalyzeResult test, and expected that fail is true", func() {
			Eventually(func(g Gomega) {
				res := newAnalyzeResult("test", FailType, outcomes)
				g.Expect(res.IsFail).Should(BeTrue())
				g.Expect(res.Message).Should(Equal(outcomes[0].Fail.Message))
			}).Should(Succeed())
		})

		It("AnalyzeResult test, and expected that warn is true", func() {
			Eventually(func(g Gomega) {
				res := newAnalyzeResult("test", WarnType, outcomes)
				g.Expect(res.IsWarn).Should(BeTrue())
				g.Expect(res.Message).Should(Equal(outcomes[0].Warn.Message))
			}).Should(Succeed())
		})
		It("AnalyzeResult test, and expected that pass is true", func() {
			Eventually(func(g Gomega) {
				res := newAnalyzeResult("test", PassType, outcomes)
				g.Expect(res.IsPass).Should(BeTrue())
				g.Expect(res.Message).Should(Equal(outcomes[0].Pass.Message))
			}).Should(Succeed())
		})
		It("AnalyzeResult with message test, and expected that fail is true", func() {
			Eventually(func(g Gomega) {
				message := "test"
				res := newFailedResultWithMessage("test", message)
				g.Expect(res.IsFail).Should(BeTrue())
				g.Expect(res.Message).Should(Equal(message))
			}).Should(Succeed())
		})
	})
})
