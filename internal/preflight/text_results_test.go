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

package preflight

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/cli-runtime/pkg/genericclioptions"

	analyzerunner "github.com/replicatedhq/troubleshoot/pkg/analyze"
)

var _ = Describe("text_results_test", func() {
	var (
		preflightName    = "stdoutPreflightName"
		jsonFormat       = "json"
		yamlFormat       = "yaml"
		kbcliFormat      = "kbcli"
		unknownFormat    = "unknown"
		streams, _, _, _ = genericclioptions.NewTestIOStreams()
		out              = streams.Out
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
				IsWarn:  true,
				Title:   "warn item",
				Message: "message for warn test",
				URI:     "https://kubernetes.io",
				Strict:  true,
			},
		}
		Eventually(func(g Gomega) {
			err := ShowTextResults(preflightName, analyzeResults, jsonFormat, true, out)
			g.Expect(err).NotTo(HaveOccurred())
			err = ShowTextResults(preflightName, analyzeResults, yamlFormat, false, out)
			g.Expect(err).NotTo(HaveOccurred())
			err = ShowTextResults(preflightName, analyzeResults, kbcliFormat, false, out)
			g.Expect(err).NotTo(HaveOccurred())
			err = ShowTextResults(preflightName, analyzeResults, unknownFormat, false, out)
			g.Expect(err).To(HaveOccurred())
		}).ShouldNot(HaveOccurred())
	})
	It("ShowStdoutResults Test", func() {
		analyzeResults := []*analyzerunner.AnalyzeResult{
			{
				IsFail:  true,
				Title:   "fail item",
				Message: "message for fail test",
				URI:     "https://kubernetes.io",
			},
		}
		Eventually(func(g Gomega) {
			err := ShowTextResults(preflightName, analyzeResults, jsonFormat, true, out)
			g.Expect(err).NotTo(HaveOccurred())
			err = ShowTextResults(preflightName, analyzeResults, yamlFormat, false, out)
			g.Expect(err).NotTo(HaveOccurred())
			err = ShowTextResults(preflightName, analyzeResults, kbcliFormat, false, out)
			g.Expect(err).NotTo(HaveOccurred())
			err = ShowTextResults(preflightName, analyzeResults, unknownFormat, false, out)
			g.Expect(err).To(HaveOccurred())
		}).Should(HaveOccurred())
	})
})
