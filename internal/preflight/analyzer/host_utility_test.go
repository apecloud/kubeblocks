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
