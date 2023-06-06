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

package cloudprovider

import (
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("aws cloud provider", func() {
	const (
		tfPath              = "./testdata"
		expectedClusterName = "kb-playground-test"
		expectedRegion      = "cn-northwest-1"
	)

	It("new cloud provider", func() {
		By("invalid cloud provider")
		provider, err := New("test", tfPath, os.Stdout, os.Stderr)
		Expect(err).Should(HaveOccurred())
		Expect(provider).Should(BeNil())

		By("valid cloud provider")
		provider, err = New("aws", tfPath, os.Stdout, os.Stderr)
		Expect(err).Should(Succeed())
		Expect(provider).ShouldNot(BeNil())
		Expect(provider.Name()).Should(Equal("aws"))

		By("get and check cluster info")
		clusterInfo, err := provider.GetClusterInfo()
		Expect(err).Should(Succeed())
		Expect(clusterInfo).ShouldNot(BeNil())
		Expect(clusterInfo.ClusterName).Should(Equal(expectedClusterName))
		Expect(clusterInfo.Region).Should(Equal(expectedRegion))
	})
})
