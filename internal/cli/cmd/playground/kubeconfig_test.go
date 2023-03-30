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

package playground

import (
	"encoding/base64"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"k8s.io/client-go/tools/clientcmd"
)

const (
	testConfigPath = "./testdata/kubeconfig"
	testCluster    = "test-cluster"
	testUser       = "test-user"
	testContext    = "test-context"
)

var (
	testKubeConfig = fmt.Sprintf(`apiVersion: v1
clusters:
- cluster:
    certificate-authority-data: %[4]s
    server: test-server
  name: %[1]s
contexts:
- context:
    cluster: %[1]s
    user: %[2]s
  name: %[3]s
current-context: %[3]s
kind: Config
preferences: {}
users:
- name: %[2]s
  user:
    client-certificate-data: %[4]s
    client-key-data: %[4]s`,
		testCluster, testUser, testContext, base64.StdEncoding.EncodeToString([]byte("Hello KubeBlocks!")))
)

var _ = Describe("playground kubeconfig", func() {
	It("get kubeconfig default path", func() {
		path, err := kubeConfigGetDefaultPath()
		Expect(err).Should(Succeed())
		Expect(path).ShouldNot(BeEmpty())
	})

	It("write invalid config to file", func() {
		err := kubeConfigWrite("invalid config", testConfigPath, writeKubeConfigOptions{})
		Expect(err).Should(HaveOccurred())
	})

	It("write and remove valid config to file", func() {
		By("write valid config to file")
		err := kubeConfigWrite(testKubeConfig, testConfigPath, writeKubeConfigOptions{UpdateExisting: true})
		Expect(err).Should(Succeed())
		config, err := clientcmd.LoadFromFile(testConfigPath)
		Expect(err).Should(Succeed())
		Expect(config).ShouldNot(BeNil())
		Expect(config.CurrentContext).Should(Equal(testContext))

		By("remove config from file")
		err = kubeConfigRemove(testKubeConfig, testConfigPath)
		Expect(err).Should(Succeed())
	})
})
