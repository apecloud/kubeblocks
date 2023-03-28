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
