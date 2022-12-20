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
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	cmdtesting "k8s.io/kubectl/pkg/cmd/testing"

	"github.com/apecloud/kubeblocks/internal/cli/builder"
	"github.com/apecloud/kubeblocks/internal/cli/testing"
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

	It("add flags", func() {
		o := newUpdateOptions(streams)
		c := &builder.Command{
			Cmd: &cobra.Command{},
		}
		o.addFlags(c)
	})

	Context("complete", func() {
		var o *updateOptions
		var c *builder.Command
		BeforeEach(func() {
			o = newUpdateOptions(streams)
			c = &builder.Command{
				Cmd:     NewUpdateCmd(tf, streams),
				Factory: tf,
				Args:    []string{"c1"},
			}
		})

		It("args is empty", func() {
			c.Args = []string{}
			Expect(o.complete(c)).Should(HaveOccurred())
		})

		It("the length of args greater than 1", func() {
			c.Args = []string{"c1", "c2"}
			Expect(o.complete(c)).Should(HaveOccurred())
		})

		It("args only contains one cluster name", func() {
			Expect(o.complete(c)).Should(Succeed())
			Expect(o.name).Should(Equal("c1"))
		})

		It("set termination-policy", func() {
			Expect(c.Cmd.Flags().Set("termination-policy", "Delete")).Should(Succeed())
			Expect(o.complete(c)).Should(Succeed())
			Expect(o.namespace).Should(Equal("default"))
			Expect(o.dynamic).ShouldNot(BeNil())
			Expect(o.Patch).Should(ContainSubstring("terminationPolicy"))
		})

		It("set monitor", func() {
			fakeCluster := testing.FakeCluster("c1", "default")
			tf.FakeDynamicClient = testing.FakeDynamicClient(fakeCluster)
			Expect(c.Cmd.Flags().Set("monitor", "true")).Should(Succeed())
			Expect(o.complete(c)).Should(Succeed())
			Expect(o.Patch).Should(ContainSubstring("\"monitor\":true"))
		})

		It("set enable-all-logs", func() {
			fakeCluster := testing.FakeCluster("c1", "default")
			tf.FakeDynamicClient = testing.FakeDynamicClient(fakeCluster)
			Expect(c.Cmd.Flags().Set("enable-all-logs", "false")).Should(Succeed())
			Expect(o.complete(c)).Should(Succeed())
		})
	})
})
