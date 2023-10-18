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

package infrastructure

import (
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"k8s.io/cli-runtime/pkg/genericiooptions"
	cmdtesting "k8s.io/kubectl/pkg/cmd/testing"

	"github.com/apecloud/kubeblocks/pkg/cli/testing"
	"github.com/apecloud/kubeblocks/test/testdata"
)

var _ = Describe("infra create test", func() {

	var (
		tf      *cmdtesting.TestFactory
		streams genericiooptions.IOStreams
	)

	BeforeEach(func() {
		streams, _, _, _ = genericiooptions.NewTestIOStreams()
		tf = cmdtesting.NewTestFactory().WithNamespace(testing.Namespace)
	})

	AfterEach(func() {
		tf.Cleanup()
	})

	mockPrivateKeyFile := func(tmpDir string) string {
		privateKeyFile := filepath.Join(tmpDir, "id_rsa.pem")
		Expect(os.WriteFile(privateKeyFile, []byte("private key"), os.ModePerm)).Should(Succeed())
		return privateKeyFile
	}

	It("test create k8s cluster with config file", func() {
		tmpDir, _ := os.MkdirTemp(os.TempDir(), "test-")
		defer os.RemoveAll(tmpDir)

		By("Create cluster with config file")
		o := &createOptions{
			clusterOptions: clusterOptions{
				IOStreams: streams,
			}}
		o.checkAndSetDefaultVersion()
		o.clusterConfig = testdata.SubTestDataPath("infrastructure/infra-cluster.yaml")
		Expect(o.Complete()).To(Succeed())
		o.Cluster.User.PrivateKeyPath = mockPrivateKeyFile(tmpDir)
		Expect(o.Validate()).To(Succeed())
	})

	It("test create k8s cluster with params", func() {
		tmpDir, _ := os.MkdirTemp(os.TempDir(), "test-")
		defer os.RemoveAll(tmpDir)

		By("Create cluster with config file")
		o := &createOptions{
			clusterOptions: clusterOptions{
				IOStreams: streams,
			}}
		o.checkAndSetDefaultVersion()

		o.nodes = []string{
			"node0:1.1.1.1:10.128.0.1",
			"node1:1.1.1.2:10.128.0.2",
			"node2:1.1.1.3:10.128.0.3",
		}
		o.Cluster.User.PrivateKeyPath = mockPrivateKeyFile(tmpDir)
		Expect(o.Complete()).Should(Succeed())
		Expect(o.Validate()).ShouldNot(Succeed())

		o.Cluster.RoleGroup.Master = []string{"node0"}
		o.Cluster.RoleGroup.ETCD = []string{"node0"}
		o.Cluster.RoleGroup.Worker = []string{"node1", "node2"}
		Expect(o.Validate()).Should(Succeed())
	})

})
