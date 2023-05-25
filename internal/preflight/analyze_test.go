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
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	troubleshoot "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"

	preflightv1beta2 "github.com/apecloud/kubeblocks/externalapis/preflight/v1beta2"
	preflightTesting "github.com/apecloud/kubeblocks/internal/preflight/testing"
)

var _ = Describe("analyze_test", func() {
	var (
		ctx              context.Context
		allCollectedData map[string][]byte
		analyzers        []*troubleshoot.Analyze
		kbAnalyzers      []*preflightv1beta2.ExtendAnalyze
		hostAnalyzers    []*troubleshoot.HostAnalyze
		kbhHostAnalyzers []*preflightv1beta2.ExtendHostAnalyze
	)

	BeforeEach(func() {
		ctx = context.TODO()
		allCollectedData = preflightTesting.FakeCollectedData()
		analyzers = preflightTesting.FakeAnalyzers()
		kbAnalyzers = []*preflightv1beta2.ExtendAnalyze{{}}
		hostAnalyzers = []*troubleshoot.HostAnalyze{{}}
		kbhHostAnalyzers = []*preflightv1beta2.ExtendHostAnalyze{{}}
	})

	It("doAnalyze test, and expect success", func() {
		Eventually(func(g Gomega) {
			analyzeList := doAnalyze(ctx, allCollectedData, analyzers, kbAnalyzers, hostAnalyzers, kbhHostAnalyzers, nil)
			g.Expect(len(analyzeList)).Should(Equal(4))
			g.Expect(analyzeList[0].IsPass).Should(Equal(true))
			g.Expect(analyzeList[1].IsFail).Should(Equal(true))
		}).Should(Succeed())
	})
})
