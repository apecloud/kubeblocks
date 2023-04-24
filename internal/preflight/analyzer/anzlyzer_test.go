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
	"context"
	"encoding/json"
	"errors"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	troubleshoot "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/replicatedhq/troubleshoot/pkg/collect"
	"k8s.io/apimachinery/pkg/version"

	preflightv1beta2 "github.com/apecloud/kubeblocks/externalapis/preflight/v1beta2"
	kbcollector "github.com/apecloud/kubeblocks/internal/preflight/collector"
)

var _ = Describe("analyzer_test", func() {
	Context("KBAnalyze test", func() {
		It("KBAnalyze test, and ExtendAnalyze is nil", func() {
			Eventually(func(g Gomega) {
				res := KBAnalyze(context.TODO(), &preflightv1beta2.ExtendAnalyze{}, nil, nil)
				g.Expect(res[0].IsFail).Should(BeTrue())
			}).Should(Succeed())
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
			}).Should(Succeed())
		})
	})

	Context("HostKBAnalyze test", func() {
		It("HostKBAnalyze test, and ExtendHostAnalyze is nil", func() {
			Eventually(func(g Gomega) {
				res := HostKBAnalyze(context.TODO(), &preflightv1beta2.ExtendHostAnalyze{}, nil, nil)
				g.Expect(res[0].IsFail).Should(BeTrue())
			}).Should(Succeed())
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
			}).Should(Succeed())
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
		}).Should(Succeed())
	})

	It("NewAnalyzeResultError test, argument isn't nil", func() {
		res := NewAnalyzeResultError(&AnalyzeClusterAccess{analyzer: &preflightv1beta2.ClusterAccessAnalyze{}}, errors.New("mock error"))
		Expect(len(res)).Should(Equal(1))
		Expect(res[0].IsFail).Should(BeTrue())
	})
})
