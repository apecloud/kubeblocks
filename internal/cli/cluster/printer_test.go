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

		printerWithLabels := &PrinterOptions{
			ShowLabels: true,
		}

		It("print cluster info", func() {
			Expect(printObjs(NewPrinter(os.Stdout, PrintClusters, nil), objs)).Should(Succeed())
		})

		It("print cluster info with label", func() {
			Expect(printObjs(NewPrinter(os.Stdout, PrintClusters, printerWithLabels), objs)).Should(Succeed())
		})

		It("print cluster wide info", func() {
			Expect(printObjs(NewPrinter(os.Stdout, PrintWide, nil), objs)).Should(Succeed())
		})

		It("print cluster wide info with label", func() {
			Expect(printObjs(NewPrinter(os.Stdout, PrintWide, printerWithLabels), objs)).Should(Succeed())
		})

		It("print component info", func() {
			Expect(printObjs(NewPrinter(os.Stdout, PrintComponents, nil), objs)).Should(Succeed())
		})

		It("print instance info", func() {
			Expect(printObjs(NewPrinter(os.Stdout, PrintInstances, nil), objs)).Should(Succeed())
		})
	})
})
