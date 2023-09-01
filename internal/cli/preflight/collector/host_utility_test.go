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

package collector

import (
	"encoding/json"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	troubleshoot "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"

	preflightv1beta2 "github.com/apecloud/kubeblocks/externalapis/preflight/v1beta2"
)

var _ = Describe("host_utility_test", func() {
	var (
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
		}).Should(Succeed())

	})
})
