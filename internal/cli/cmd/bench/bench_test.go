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

package bench

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/pingcap/go-tpc/tpcc"
)

var _ = Describe("bench", func() {
	It("bench command", func() {
		cmd := NewBenchCmd()
		Expect(cmd != nil).Should(BeTrue())
		Expect(cmd.HasSubCommands()).Should(BeTrue())
	})

	It("tpcc command", func() {
		cmd := NewTpccCmd()
		Expect(cmd != nil).Should(BeTrue())
		Expect(cmd.HasSubCommands()).Should(BeTrue())

		cmd = newPrepareCmd()
		Expect(cmd != nil).Should(BeTrue())
		Expect(cmd.RunE(cmd, []string{})).Should(HaveOccurred())

		cmd = newRunCmd()
		Expect(cmd != nil).Should(BeTrue())
		Expect(cmd.RunE(cmd, []string{})).Should(HaveOccurred())

		cmd = newCleanCmd()
		Expect(cmd != nil).Should(BeTrue())
		Expect(cmd.RunE(cmd, []string{})).Should(HaveOccurred())
	})

	It("internal functions", func() {
		outputInterval = 120 * time.Second
		executeWorkload(context.Background(), &tpcc.Workloader{}, 1, "prepare")
	})

	It("util", func() {
		Expect(openDB()).Should(HaveOccurred())
		Expect(ping()).Should(Succeed())
		Expect(createDB()).Should(HaveOccurred())
		Expect(closeDB()).Should(Succeed())
	})
})
