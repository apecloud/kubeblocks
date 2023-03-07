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
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	analyzerunner "github.com/replicatedhq/troubleshoot/pkg/analyze"
)

var _ = Describe("text_results_test", func() {
	var (
		timeOut       = time.Second * 10
		preflightName = "stdoutPreflightName"
		humanFormat   = "human"
		jsonFormat    = "json"
		yamlFormat    = "yaml"
		unknownFormat = "unknown"
	)
	It("ShowStdoutResults Test", func() {
		analyzeResults := []*analyzerunner.AnalyzeResult{
			{
				IsPass:  true,
				Title:   "pass item",
				Message: "message for pass test",
				URI:     "https://kubernetes.io",
			},
			{
				IsFail:  true,
				Title:   "fail item",
				Message: "message for fail test",
				URI:     "https://kubernetes.io",
			},
			{
				IsWarn:  true,
				Title:   "warn item",
				Message: "message for warn test",
				URI:     "https://kubernetes.io",
				Strict:  true,
			},
		}
		Eventually(func(g Gomega) {
			err := ShowTextResults(preflightName, analyzeResults, humanFormat, false)
			g.Expect(err).To(HaveOccurred())
			err = ShowTextResults(preflightName, analyzeResults, jsonFormat, true)
			g.Expect(err).NotTo(HaveOccurred())
			err = ShowTextResults(preflightName, analyzeResults, yamlFormat, false)
			g.Expect(err).NotTo(HaveOccurred())
			err = ShowTextResults(preflightName, analyzeResults, unknownFormat, false)
			g.Expect(err).To(HaveOccurred())
		}, timeOut).Should(Succeed())
	})
})
