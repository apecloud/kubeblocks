/*
Copyright (C) 2022 ApeCloud Co., Ltd

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
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	preflightv1beta2 "github.com/apecloud/kubeblocks/externalapis/preflight/v1beta2"
)

var _ = Describe("host_collector_test", func() {
	It("GetExtendHostCollector test", func() {
		collector, ok := GetExtendHostCollector(&preflightv1beta2.ExtendHostCollect{HostUtility: &preflightv1beta2.HostUtility{}}, "bundlePath")
		Expect(collector).ShouldNot(BeNil())
		Expect(ok).Should(BeTrue())
		collector, ok = GetExtendHostCollector(&preflightv1beta2.ExtendHostCollect{ClusterRegion: &preflightv1beta2.ClusterRegion{}}, "bundlePath")
		Expect(collector).ShouldNot(BeNil())
		Expect(ok).Should(BeTrue())
		collector, ok = GetExtendHostCollector(&preflightv1beta2.ExtendHostCollect{}, "bundlePath")
		Expect(collector).Should(BeNil())
		Expect(ok).Should(BeFalse())
	})
})
