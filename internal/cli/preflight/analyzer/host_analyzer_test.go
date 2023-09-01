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
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	preflightv1beta2 "github.com/apecloud/kubeblocks/externalapis/preflight/v1beta2"
)

var _ = Describe("host_analyzer_test", func() {
	It("GetHostAnalyzer test", func() {
		collector, ok := GetHostAnalyzer(&preflightv1beta2.ExtendHostAnalyze{HostUtility: &preflightv1beta2.HostUtilityAnalyze{}})
		Expect(collector).ShouldNot(BeNil())
		Expect(ok).Should(BeTrue())
		collector, ok = GetHostAnalyzer(&preflightv1beta2.ExtendHostAnalyze{ClusterRegion: &preflightv1beta2.ClusterRegionAnalyze{}})
		Expect(collector).ShouldNot(BeNil())
		Expect(ok).Should(BeTrue())
		collector, ok = GetHostAnalyzer(&preflightv1beta2.ExtendHostAnalyze{})
		Expect(collector).Should(BeNil())
		Expect(ok).Should(BeFalse())
	})
})
