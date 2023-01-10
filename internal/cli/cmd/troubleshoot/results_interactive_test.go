/*
Copyright ApeCloud Inc.

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

package troubleshoot

import (
	"os"
	"time"

	tb "github.com/nsf/termbox-go"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	analyzerunner "github.com/replicatedhq/troubleshoot/pkg/analyze"
)

var _ = Describe("Results Interactive Test", func() {
	var (
		timeout        = time.Second * 10
		preflightName  = "interactivePreflightName"
		outputPath     = ""
		analyzeResults = []*analyzerunner.AnalyzeResult{
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
			},
		}
	)
	It("ShowInteractiveResults Test", func() {
		go func() {
			time.Sleep(5 * time.Second)
			tb.Interrupt()
		}()

		Eventually(func(g Gomega) {
			err := showInteractiveResults(preflightName, analyzeResults, outputPath)
			g.Expect(err).NotTo(HaveOccurred())
		}, timeout).Should(Succeed())
	})

	It("Save Test", func() {
		defer GinkgoRecover()
		Eventually(func(g Gomega) {
			fileName, err := save(preflightName, "", analyzeResults)
			g.Expect(fileName).Should(HaveSuffix(".txt"))
			g.Expect(err).NotTo(HaveOccurred())
			Expect(os.Remove(fileName)).NotTo(HaveOccurred())
		}, timeout).Should(Succeed())
	})
})
