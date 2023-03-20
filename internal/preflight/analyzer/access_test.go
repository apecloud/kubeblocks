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
