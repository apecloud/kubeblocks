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

package reconcile

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/apecloud/kubeblocks/controllers/monitor/types"
)

var _ = Describe("monitor_controller", func() {
	var (
		instance *types.OteldInstance
	)

	BeforeEach(func() {
		instance = &types.OteldInstance{}
	})

	It("should generate config correctly from config yaml", func() {
		Eventually(func(g Gomega) {
			otel := buildDaemonSetForOteld(instance, "test", "test")
			g.Expect(otel).ShouldNot(BeNil())
			g.Expect(otel.Name).Should(Equal("test"))
		}).Should(Succeed())
	})

})
