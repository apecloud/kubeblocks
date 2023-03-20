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

package cluster

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	cmdtesting "k8s.io/kubectl/pkg/cmd/testing"

	"github.com/apecloud/kubeblocks/internal/cli/patch"
	"github.com/apecloud/kubeblocks/internal/cli/testing"
	"github.com/apecloud/kubeblocks/internal/cli/types"
)

var _ = Describe("cluster update", func() {
	var streams genericclioptions.IOStreams
	var tf *cmdtesting.TestFactory

	BeforeEach(func() {
		streams, _, _, _ = genericclioptions.NewTestIOStreams()
		tf = cmdtesting.NewTestFactory().WithNamespace("default")
	})

	AfterEach(func() {
		tf.Cleanup()
	})

	It("update command", func() {
		cmd := NewUpdateCmd(tf, streams)
		Expect(cmd).ShouldNot(BeNil())
	})

	Context("complete", func() {
		var o *updateOptions
		var cmd *cobra.Command
		var args []string
		BeforeEach(func() {
			cmd = NewUpdateCmd(tf, streams)
			o = &updateOptions{Options: patch.NewOptions(tf, streams, types.ClusterGVR())}
			args = []string{"c1"}

		})

		It("args is empty", func() {
			Expect(o.complete(cmd, nil)).Should(HaveOccurred())
		})

		It("the length of args greater than 1", func() {
			Expect(o.complete(cmd, []string{"c1", "c2"})).Should(HaveOccurred())
		})

		It("args only contains one cluster name", func() {
			Expect(o.complete(cmd, args)).Should(Succeed())
			Expect(o.Names[0]).Should(Equal("c1"))
		})

		It("set termination-policy", func() {
			Expect(cmd.Flags().Set("termination-policy", "Delete")).Should(Succeed())
			Expect(o.complete(cmd, args)).Should(Succeed())
			Expect(o.namespace).Should(Equal("default"))
			Expect(o.dynamic).ShouldNot(BeNil())
			Expect(o.Patch).Should(ContainSubstring("terminationPolicy"))
		})

		It("set monitor", func() {
			fakeCluster := testing.FakeCluster("c1", "default")
			tf.FakeDynamicClient = testing.FakeDynamicClient(fakeCluster)
			Expect(cmd.Flags().Set("monitor", "true")).Should(Succeed())
			Expect(o.complete(cmd, args)).Should(Succeed())
			Expect(o.Patch).Should(ContainSubstring("\"monitor\":true"))
		})

		It("set enable-all-logs", func() {
			fakeCluster := testing.FakeCluster("c1", "default")
			tf.FakeDynamicClient = testing.FakeDynamicClient(fakeCluster)
			Expect(cmd.Flags().Set("enable-all-logs", "false")).Should(Succeed())
			Expect(o.complete(cmd, args)).Should(Succeed())
		})

		It("set node-labels", func() {
			fakeCluster := testing.FakeCluster("c1", "default")
			tf.FakeDynamicClient = testing.FakeDynamicClient(fakeCluster)
			Expect(cmd.Flags().Set("node-labels", "k1=v1,k2=v2")).Should(Succeed())
			Expect(o.complete(cmd, args)).Should(Succeed())
			Expect(o.Patch).Should(ContainSubstring("k1"))
		})
	})
})
