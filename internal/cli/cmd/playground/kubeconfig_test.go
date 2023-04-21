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
