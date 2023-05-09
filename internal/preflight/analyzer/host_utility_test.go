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

package analyzer

import (
	"encoding/json"
	"errors"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	troubleshoot "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"

	preflightv1beta2 "github.com/apecloud/kubeblocks/externalapis/preflight/v1beta2"
	kbcollector "github.com/apecloud/kubeblocks/internal/preflight/collector"
)

var _ = Describe("host_utility_test", func() {
	var (
		hostUtilityAnalyzer AnalyzeHostUtility
		resInfo             kbcollector.HostUtilityInfo
	)

	Context("AnalyzeHostUtility test", func() {
		BeforeEach(func() {
			hostUtilityAnalyzer = AnalyzeHostUtility{
				hostAnalyzer: &preflightv1beta2.HostUtilityAnalyze{
					Outcomes: []*troubleshoot.Outcome{
						{
							Pass: &troubleshoot.SingleOutcome{
								Message: "utility already installed",
							},
							Fail: &troubleshoot.SingleOutcome{
								Message: "utility isn't installed",
							},
							Warn: &troubleshoot.SingleOutcome{
								Message: "utility isn't installed",
							},
						},
					}}}
		})

		It("Analyze test, and returns fail error", func() {
			resInfo = kbcollector.HostUtilityInfo{
				Path:  "",
				Name:  "helm",
				Error: "helm isn't installed in localhost",
			}
			Eventually(func(g Gomega) {
				getCollectedFileContents := func(filename string) ([]byte, error) {
					return nil, errors.New("get file failed")
				}
				res, err := hostUtilityAnalyzer.Analyze(getCollectedFileContents)
				g.Expect(err).Should(HaveOccurred())
				g.Expect(res).Should(BeNil())
			}).Should(Succeed())
		})

		It("Analyze test, and analyzer result is expected that fail is true", func() {
			resInfo = kbcollector.HostUtilityInfo{
				Path:  "",
				Name:  "helm",
				Error: "helm isn't installed in localhost",
			}
			Eventually(func(g Gomega) {
				b, err := json.Marshal(resInfo)
				g.Expect(err).NotTo(HaveOccurred())
				getCollectedFileContents := func(filename string) ([]byte, error) {
					return b, nil
				}
				res, err := hostUtilityAnalyzer.Analyze(getCollectedFileContents)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(res[0].IsWarn).Should(BeTrue())
			}).Should(Succeed())
		})

		It("Analyze test, and analyzer result is expected that pass is true", func() {
			resInfo = kbcollector.HostUtilityInfo{
				Path:  "/usr/local/bin/helm",
				Name:  "helm",
				Error: "",
			}
			Eventually(func(g Gomega) {
				g.Expect(hostUtilityAnalyzer.IsExcluded()).Should(BeFalse())
				b, err := json.Marshal(resInfo)
				g.Expect(err).NotTo(HaveOccurred())
				getCollectedFileContents := func(filename string) ([]byte, error) {
					return b, nil
				}
				res, err := hostUtilityAnalyzer.Analyze(getCollectedFileContents)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(res[0].IsPass).Should(BeTrue())
			}).Should(Succeed())
		})
	})
})
