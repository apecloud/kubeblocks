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
