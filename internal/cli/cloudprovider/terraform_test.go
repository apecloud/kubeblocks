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

package cloudprovider

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("aws cloud provider", func() {
	const (
		tfPath              = "./testdata/aws/eks"
		expectedClusterName = "kb-playground-test"
	)

	It("get cluster name from state file", func() {
		By("get cluster name from state file")
		vals, err := getOutputValues(tfPath, clusterNameKey)
		Expect(err).Should(Succeed())
		Expect(vals).Should(HaveLen(1))
		Expect(vals).Should(ContainElement(expectedClusterName))

		By("get unknown key from state file")
		vals, err = getOutputValues(tfPath, "unknownKey")
		Expect(err).Should(Succeed())
		Expect(vals).ShouldNot(BeEmpty())
		Expect(vals).Should(ContainElement(""))
	})
})
