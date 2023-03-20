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

var _ = Describe("region_test", func() {
	var (
		analyzer AnalyzeClusterRegion
		resInfo  kbcollector.ClusterRegionInfo
	)
	Context("AnalyzeHostUtility test", func() {
		BeforeEach(func() {
			analyzer = AnalyzeClusterRegion{
				analyzer: &preflightv1beta2.ClusterRegionAnalyze{
					Outcomes: []*troubleshoot.Outcome{
						{
							Pass: &troubleshoot.SingleOutcome{
								Message: "k8s cluster region is matched",
							},
							Fail: &troubleshoot.SingleOutcome{
								Message: "k8s cluster access isn't matched",
							},
							Warn: &troubleshoot.SingleOutcome{
								Message: "k8s cluster access isn't matched",
							},
						},
					},
					RegionNames: []string{"beijing", "shanghai"}}}
		})
		It("Analyze test, and get file failed", func() {
			Eventually(func(g Gomega) {
				getCollectedFileContents := func(string) ([]byte, error) {
					return nil, errors.New("get file failed")
				}
				_, err := analyzer.Analyze(getCollectedFileContents)
				g.Expect(err).Should(HaveOccurred())
			}).Should(Succeed())
		})

		It("Analyze test, and return of get file is not ClusterRegionInfo", func() {
			Eventually(func(g Gomega) {
				getCollectedFileContents := func(string) ([]byte, error) {
					return []byte("test"), nil
				}
				_, err := analyzer.Analyze(getCollectedFileContents)
				g.Expect(err).Should(HaveOccurred())
			}).Should(Succeed())
		})

		It("Analyze test, and analyzer warn is expected", func() {
			resInfo = kbcollector.ClusterRegionInfo{
				RegionName: "",
			}
			Eventually(func(g Gomega) {
				g.Expect(analyzer.IsExcluded()).Should(BeFalse())
				b, err := json.Marshal(resInfo)
				g.Expect(err).NotTo(HaveOccurred())
				getCollectedFileContents := func(filename string) ([]byte, error) {
					return b, nil
				}
				res, err := analyzer.Analyze(getCollectedFileContents)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(res[0].IsWarn).Should(BeTrue())
			}).Should(Succeed())
		})

		It("Analyze test, and analyzer warn is expected ", func() {
			resInfo = kbcollector.ClusterRegionInfo{
				RegionName: "hangzhou",
			}
			Eventually(func(g Gomega) {
				g.Expect(analyzer.IsExcluded()).Should(BeFalse())
				b, err := json.Marshal(resInfo)
				g.Expect(err).NotTo(HaveOccurred())
				getCollectedFileContents := func(string) ([]byte, error) {
					return b, nil
				}
				res, err := analyzer.Analyze(getCollectedFileContents)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(res[0].IsWarn).Should(BeTrue())
			}).Should(Succeed())
		})

		It("Analyze test, and analyzer pass is expected ", func() {
			resInfo = kbcollector.ClusterRegionInfo{
				RegionName: "beijing",
			}
			Eventually(func(g Gomega) {
				g.Expect(analyzer.IsExcluded()).Should(BeFalse())
				b, err := json.Marshal(resInfo)
				g.Expect(err).NotTo(HaveOccurred())
				getCollectedFileContents := func(string) ([]byte, error) {
					return b, nil
				}
				res, err := analyzer.Analyze(getCollectedFileContents)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(res[0].IsPass).Should(BeTrue())
			}).Should(Succeed())
		})
	})
})
