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
