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

package collector

import (
	"encoding/json"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	troubleshoot "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"

	preflightv1beta2 "github.com/apecloud/kubeblocks/externalapis/preflight/v1beta2"
)

var _ = Describe("host_utility_test", func() {
	var (
		timeOut     = 10 * time.Second
		hostUtility CollectHostUtility
		testName    = "testName"
	)
	BeforeEach(func() {
		hostUtility = CollectHostUtility{
			HostCollector: &preflightv1beta2.HostUtility{
				HostCollectorMeta: troubleshoot.HostCollectorMeta{
					CollectorName: testName},
				UtilityName: "UtilityName"},
			BundlePath: "",
		}
	})
	It("CollectHostUtility test, UtilityName is invalid and expect error", func() {
		Eventually(func(g Gomega) {
			g.Expect(hostUtility.Title()).Should(Equal(testName))
			g.Expect(hostUtility.IsExcluded()).Should(BeFalse())
			dataMap, err := hostUtility.Collect(nil)
			g.Expect(err).ShouldNot(HaveOccurred())
			collectd, ok := dataMap[fmt.Sprintf(UtilityPathFormat, hostUtility.HostCollector.CollectorName)]
			g.Expect(ok).Should(BeTrue())
			utilityInfo := new(HostUtilityInfo)
			err = json.Unmarshal(collectd, utilityInfo)
			g.Expect(err).ShouldNot(HaveOccurred())
			g.Expect(utilityInfo.Error).ShouldNot(BeNil())
		}, timeOut).Should(Succeed())

	})
})
