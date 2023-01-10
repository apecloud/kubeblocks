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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("printer", func() {
	Context("print cluster objects", func() {
		objs := FakeClusterObjs()

		printObjs := func(printer *Printer, objs *ClusterObjects) error {
			printer.AddRow(objs)
			printer.Print()
			return nil
		}

		It("print cluster info", func() {
			Expect(printObjs(NewPrinter(os.Stdout, PrintClusters), objs)).Should(Succeed())
		})

		It("print cluster wide info", func() {
			Expect(printObjs(NewPrinter(os.Stdout, PrintWide), objs)).Should(Succeed())
		})

		It("print component info", func() {
			Expect(printObjs(NewPrinter(os.Stdout, PrintComponents), objs)).Should(Succeed())
		})

		It("print instance info", func() {
			Expect(printObjs(NewPrinter(os.Stdout, PrintInstances), objs)).Should(Succeed())
		})
	})
})
