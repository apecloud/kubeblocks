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

package preflight

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	troubleshoot "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"

	preflightv1beta2 "github.com/apecloud/kubeblocks/externalapis/preflight/v1beta2"
)

var _ = Describe("analyze_test", func() {
	var (
		ctx              context.Context
		allCollectedData map[string][]byte
		analyzers        []*troubleshoot.Analyze
		kbAnalyzers      []*preflightv1beta2.ExtendAnalyze
		hostAnalyzers    []*troubleshoot.HostAnalyze
		kbhHostAnalyzers []*preflightv1beta2.ExtendHostAnalyze
		timeout          = time.Second * 10
		clusterVersion   = `
{
  "info": {
    "major": "1",
    "minor": "23",
    "gitVersion": "v1.23.15",
    "gitCommit": "b84cb8ab29366daa1bba65bc67f54de2f6c34848",
    "gitTreeState": "clean",
    "buildDate": "2022-12-08T10:42:57Z",
    "goVersion": "go1.17.13",
    "compiler": "gc",
    "platform": "linux/arm64"
  },
  "string": "v1.23.15"
}`
	)

	BeforeEach(func() {
		ctx = context.TODO()
		allCollectedData = map[string][]byte{"cluster-info/cluster_version.json": []byte(clusterVersion)}
		analyzers = []*troubleshoot.Analyze{
			{ClusterVersion: &troubleshoot.ClusterVersion{
				AnalyzeMeta: troubleshoot.AnalyzeMeta{
					CheckName: "ClusterVersionCheck",
				},
				Outcomes: []*troubleshoot.Outcome{
					{
						Pass: &troubleshoot.SingleOutcome{
							Message: "version is ok.",
						}}}}},
		}
		kbAnalyzers = []*preflightv1beta2.ExtendAnalyze{{}}
		hostAnalyzers = []*troubleshoot.HostAnalyze{{}}
		kbhHostAnalyzers = []*preflightv1beta2.ExtendHostAnalyze{{}}
	})

	It("doAnalyze test, and expect success", func() {
		Eventually(func(g Gomega) {
			analyzeList := doAnalyze(ctx, allCollectedData, analyzers, kbAnalyzers, hostAnalyzers, kbhHostAnalyzers)
			g.Expect(len(analyzeList)).Should(Equal(4))
			g.Expect(analyzeList[0].IsPass).Should(Equal(true))
			g.Expect(analyzeList[1].IsFail).Should(Equal(true))
		}, timeout).Should(Succeed())
	})
})
