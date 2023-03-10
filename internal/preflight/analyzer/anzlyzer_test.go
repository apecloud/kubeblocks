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
	"context"
	"encoding/json"
	"errors"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	troubleshoot "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/replicatedhq/troubleshoot/pkg/collect"
	"k8s.io/apimachinery/pkg/version"

	preflightv1beta2 "github.com/apecloud/kubeblocks/externalapis/preflight/v1beta2"
	kbcollector "github.com/apecloud/kubeblocks/internal/preflight/collector"
)

var _ = Describe("analyzer_test", func() {
	var (
		timeOut = time.Second * 10
	)

	Context("KBAnalyze test", func() {
		It("KBAnalyze test, and ExtendAnalyze is nil", func() {
			Eventually(func(g Gomega) {
				res := KBAnalyze(context.TODO(), &preflightv1beta2.ExtendAnalyze{}, nil, nil)
				g.Expect(res[0].IsFail).Should(BeTrue())
			}, timeOut).Should(Succeed())
		})

		It("KBAnalyze test, and expect success", func() {
			kbAnalyzer := &preflightv1beta2.ExtendAnalyze{
				ClusterAccess: &preflightv1beta2.ClusterAccessAnalyze{
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
			resInfo := collect.ClusterVersion{
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
				b, err := json.Marshal(resInfo)
				g.Expect(err).NotTo(HaveOccurred())
				getCollectedFileContents := func(string) ([]byte, error) {
					return b, nil
				}
				res := KBAnalyze(context.TODO(), kbAnalyzer, getCollectedFileContents, nil)
				Expect(len(res)).Should(Equal(1))
				g.Expect(res[0].IsPass).Should(BeTrue())
			}, timeOut).Should(Succeed())
		})
	})

	Context("HostKBAnalyze test", func() {
		It("HostKBAnalyze test, and ExtendHostAnalyze is nil", func() {
			Eventually(func(g Gomega) {
				res := HostKBAnalyze(context.TODO(), &preflightv1beta2.ExtendHostAnalyze{}, nil, nil)
				g.Expect(res[0].IsFail).Should(BeTrue())
			}, timeOut).Should(Succeed())
		})

		It("HostKBAnalyze test, and expect success", func() {
			kbHostAnalyzer := &preflightv1beta2.ExtendHostAnalyze{
				HostUtility: &preflightv1beta2.HostUtilityAnalyze{
					Outcomes: []*troubleshoot.Outcome{
						{
							Pass: &troubleshoot.SingleOutcome{
								Message: "utility already installed",
							},
							Fail: &troubleshoot.SingleOutcome{
								Message: "utility isn't installed",
							},
						},
					}}}
			resInfo := kbcollector.HostUtilityInfo{
				Path:  "/usr/local/bin/helm",
				Name:  "helm",
				Error: "",
			}
			Eventually(func(g Gomega) {
				b, err := json.Marshal(resInfo)
				g.Expect(err).NotTo(HaveOccurred())
				getCollectedFileContents := func(string) ([]byte, error) {
					return b, nil
				}
				res := HostKBAnalyze(context.TODO(), kbHostAnalyzer, getCollectedFileContents, nil)
				Expect(len(res)).Should(Equal(1))
				g.Expect(res[0].IsPass).Should(BeTrue())
			}, timeOut).Should(Succeed())
		})
	})

	It("GetAnalyzer test, and expect success", func() {
		Eventually(func(g Gomega) {
			collector, ok := GetAnalyzer(&preflightv1beta2.ExtendAnalyze{ClusterAccess: &preflightv1beta2.ClusterAccessAnalyze{}})
			g.Expect(collector).ShouldNot(BeNil())
			g.Expect(ok).Should(BeTrue())
			collector, ok = GetAnalyzer(&preflightv1beta2.ExtendAnalyze{})
			g.Expect(collector).Should(BeNil())
			g.Expect(ok).Should(BeFalse())
		}, timeOut).Should(Succeed())
	})

	It("NewAnalyzeResultError test, argument isn't nil", func() {
		res := NewAnalyzeResultError(&AnalyzeClusterAccess{analyzer: &preflightv1beta2.ClusterAccessAnalyze{}}, errors.New("mock error"))
		Expect(len(res)).Should(Equal(1))
		Expect(res[0].IsFail).Should(BeTrue())
	})
})
