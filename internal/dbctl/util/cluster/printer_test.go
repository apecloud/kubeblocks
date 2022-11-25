/*
Copyright ApeCloud Inc.

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

package cluster

import (
	"os"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/gosuri/uitable"
)

var _ = Describe("printer", func() {
	Context("print cluster objects", func() {
		objs := FakeClusterObjs()

		printObjs := func(printer Printer, objs *ClusterObjects) error {
			printer.AddHeader()
			printer.AddRow(objs)
			return printer.Print(os.Stdout)
		}

		It("print cluster info", func() {
			Expect(printObjs(&ClusterPrinter{uitable.New()}, objs)).Should(Succeed())
		})

		It("print component info", func() {
			Expect(printObjs(&ComponentPrinter{uitable.New()}, objs)).Should(Succeed())
		})

		It("print instance info", func() {
			Expect(printObjs(&InstancePrinter{uitable.New()}, objs)).Should(Succeed())
		})
	})
})
