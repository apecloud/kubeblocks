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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/pkg/errors"
	troubleshoot "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"helm.sh/helm/v3/pkg/cli/values"
	v1 "k8s.io/api/core/v1"

	preflightv1beta2 "github.com/apecloud/kubeblocks/externalapis/preflight/v1beta2"
)

var (
	nodeList1 = v1.NodeList{Items: []v1.Node{
		{Spec: v1.NodeSpec{Taints: []v1.Taint{
			{Key: "dev", Value: "true", Effect: v1.TaintEffectNoSchedule},
			{Key: "large", Value: "true", Effect: v1.TaintEffectNoSchedule},
		}}},
		{Spec: v1.NodeSpec{Taints: []v1.Taint{
			{Key: "dev", Value: "false", Effect: v1.TaintEffectNoSchedule},
		}}},
	}}
	nodeList2 = v1.NodeList{Items: []v1.Node{
		{Spec: v1.NodeSpec{Taints: []v1.Taint{
			{Key: "dev", Value: "false", Effect: v1.TaintEffectNoSchedule},
			{Key: "large", Value: "true", Effect: v1.TaintEffectNoSchedule},
		}}},
	}}
	nodeList3 = v1.NodeList{Items: []v1.Node{
		{Spec: v1.NodeSpec{}},
	}}
)

var _ = Describe("taint_class_test", func() {
	var (
		analyzer AnalyzeTaintClassByKb
	)
	Context("analyze taint test", func() {
		BeforeEach(func() {
			JSONStr := "tolerations=[ { \"key\": \"dev\", \"operator\": \"Equal\", \"effect\": \"NoSchedule\", \"value\": \"true\" }, " +
				"{ \"key\": \"large\", \"operator\": \"Equal\", \"effect\": \"NoSchedule\", \"value\": \"true\" } ]," +
				"prometheus.server.tolerations=[ { \"key\": \"dev\", \"operator\": \"Equal\", \"effect\": \"NoSchedule\", \"value\": \"true\" }, " +
				"{ \"key\": \"large\", \"operator\": \"Equal\", \"effect\": \"NoSchedule\", \"value\": \"true\" } ]," +
				"grafana.tolerations=[ { \"key\": \"dev\", \"operator\": \"Equal\", \"effect\": \"NoSchedule\", \"value\": \"true\" }, " +
				"{ \"key\": \"large\", \"operator\": \"Equal\", \"effect\": \"NoSchedule\", \"value\": \"true\" } ],"
			analyzer = AnalyzeTaintClassByKb{
				analyzer: &preflightv1beta2.KBTaintAnalyze{
					Outcomes: []*troubleshoot.Outcome{
						{
							Pass: &troubleshoot.SingleOutcome{
								Message: "analyze taint success",
							},
							Fail: &troubleshoot.SingleOutcome{
								Message: "analyze taint fail",
							},
						},
					},
				},
				HelmOpts: &values.Options{JSONValues: []string{JSONStr}},
			}

		})
		It("Analyze test, and get file failed", func() {
			Eventually(func(g Gomega) {
				getCollectedFileContents := func(filename string) ([]byte, error) {
					return nil, errors.New("get file failed")
				}
				res, err := analyzer.Analyze(getCollectedFileContents, nil)
				g.Expect(err).To(HaveOccurred())
				g.Expect(res[0].IsFail).Should(BeTrue())
				g.Expect(res[0].IsPass).Should(BeFalse())
			}).Should(Succeed())
		})

		It("Analyze test, and return of get file is not clusterResource", func() {
			Eventually(func(g Gomega) {
				getCollectedFileContents := func(filename string) ([]byte, error) {
					return []byte("test"), nil
				}
				res, err := analyzer.Analyze(getCollectedFileContents, nil)
				g.Expect(err).To(HaveOccurred())
				g.Expect(res[0].IsFail).Should(BeTrue())
				g.Expect(res[0].IsPass).Should(BeFalse())
			}).Should(Succeed())
		})

		It("Analyze test, and analyzer result is expected that pass is true", func() {
			Eventually(func(g Gomega) {
				g.Expect(analyzer.IsExcluded()).Should(BeFalse())
				b, err := json.Marshal(nodeList1)
				g.Expect(err).NotTo(HaveOccurred())
				getCollectedFileContents := func(filename string) ([]byte, error) {
					return b, nil
				}
				res, err := analyzer.Analyze(getCollectedFileContents, nil)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(res[0].IsPass).Should(BeTrue())
				g.Expect(res[0].IsFail).Should(BeFalse())
			}).Should(Succeed())
		})
		It("Analyze test, and analyzer result is expected that fail is true", func() {
			Eventually(func(g Gomega) {
				g.Expect(analyzer.IsExcluded()).Should(BeFalse())
				b, err := json.Marshal(nodeList2)
				g.Expect(err).NotTo(HaveOccurred())
				getCollectedFileContents := func(filename string) ([]byte, error) {
					return b, nil
				}
				res, err := analyzer.Analyze(getCollectedFileContents, nil)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(res[0].IsPass).Should(BeFalse())
				g.Expect(res[0].IsFail).Should(BeTrue())
			}).Should(Succeed())
		})
		It("Analyze test, the taints are nil, and analyzer result is expected that pass is true", func() {
			Eventually(func(g Gomega) {
				g.Expect(analyzer.IsExcluded()).Should(BeFalse())
				b, err := json.Marshal(nodeList3)
				g.Expect(err).NotTo(HaveOccurred())
				getCollectedFileContents := func(filename string) ([]byte, error) {
					return b, nil
				}
				res, err := analyzer.Analyze(getCollectedFileContents, nil)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(res[0].IsPass).Should(BeTrue())
				g.Expect(res[0].IsFail).Should(BeFalse())
			}).Should(Succeed())
		})
		It("Analyze test, the tolerations are nil, and analyzer result is expected that fail is true", func() {
			Eventually(func(g Gomega) {
				g.Expect(analyzer.IsExcluded()).Should(BeFalse())
				b, err := json.Marshal(nodeList2)
				g.Expect(err).NotTo(HaveOccurred())
				getCollectedFileContents := func(filename string) ([]byte, error) {
					return b, nil
				}
				analyzer.HelmOpts = nil
				res, err := analyzer.Analyze(getCollectedFileContents, nil)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(res[0].IsPass).Should(BeFalse())
				g.Expect(res[0].IsFail).Should(BeTrue())
			}).Should(Succeed())
		})
	})
})
