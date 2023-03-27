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
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("aws cloud provider", func() {
	const (
		tfPath              = "../testing/testdata"
		expectedClusterName = "kb-playground-test"
		expectedContextName = "arn-kb-playground-test"
	)

	It("get cluster name from state file", func() {
		name, err := getOutputValue(clusterNameKey, tfPath)
		Expect(err).Should(Succeed())
		Expect(name).Should(Equal(expectedClusterName))

		contextName, err := getOutputValue(contextNameKey, tfPath)
		Expect(err).Should(Succeed())
		Expect(contextName).Should(Equal(expectedContextName))
	})
})
