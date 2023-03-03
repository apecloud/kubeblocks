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
