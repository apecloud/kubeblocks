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
	"github.com/replicatedhq/troubleshoot/pkg/collect"
	"k8s.io/apimachinery/pkg/version"

	preflightv1beta2 "github.com/apecloud/kubeblocks/externalapis/preflight/v1beta2"
)

var _ = Describe("access_test", func() {
	var (
		analyzer AnalyzeClusterAccess
		resInfo  collect.ClusterVersion
	)
	Context("AnalyzeHostUtility test", func() {
		BeforeEach(func() {
			analyzer = AnalyzeClusterAccess{
				analyzer: &preflightv1beta2.ClusterAccessAnalyze{
					Outcomes: []*troubleshoot.Outcome{
						{
							Pass: &troubleshoot.SingleOutcome{
								Message: "k8s cluster access success",
							},
							Fail: &troubleshoot.SingleOutcome{
								Message: "k8s cluster access fail",
							},
						},
					}}}
		})
		It("Analyze test, and get file failed", func() {
			Eventually(func(g Gomega) {
				getCollectedFileContents := func(filename string) ([]byte, error) {
					return nil, errors.New("get file failed")
				}
				res, err := analyzer.Analyze(getCollectedFileContents, nil)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(res[0].IsFail).Should(BeTrue())
			}).Should(Succeed())
		})

		It("Analyze test, and return of get file is not version.Info", func() {
			Eventually(func(g Gomega) {
				getCollectedFileContents := func(filename string) ([]byte, error) {
					return []byte("test"), nil
				}
				res, err := analyzer.Analyze(getCollectedFileContents, nil)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(res[0].IsFail).Should(BeTrue())
			}).Should(Succeed())
		})

		It("Analyze test, and analyzer result is expected that pass is true", func() {
			resInfo = collect.ClusterVersion{
				Info: &version.Info{
					Major:        "1",
					Minor:        "23",
					GitVersion:   "v1.23.15",
					GitCommit:    "b84cb8ab29366daa1bba65bc67f54de2f6c34848",
					GitTreeState: "clean",
					BuildDate:    "2022-12-08T10:42:57Z",
					GoVersion:    "go1.17.13",
					Compiler:     "gc",
					Platform:     "linux/arm64",
				},
				String: "v1.23.15",
			}
			Eventually(func(g Gomega) {
				g.Expect(analyzer.IsExcluded()).Should(BeFalse())
				b, err := json.Marshal(resInfo)
				g.Expect(err).NotTo(HaveOccurred())
				getCollectedFileContents := func(filename string) ([]byte, error) {
					return b, nil
				}
				res, err := analyzer.Analyze(getCollectedFileContents, nil)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(res[0].IsPass).Should(BeTrue())
			}).Should(Succeed())
		})
	})
})
