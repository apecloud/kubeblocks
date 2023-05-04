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
