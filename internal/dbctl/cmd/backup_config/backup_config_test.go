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

package backup_config

import (
	"bytes"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	cmdtesting "k8s.io/kubectl/pkg/cmd/testing"

	"github.com/apecloud/kubeblocks/internal/dbctl/util/helm"
)

var _ = Describe("backup_config", func() {
	var cmd *cobra.Command
	var streams genericclioptions.IOStreams
	buf := new(bytes.Buffer)
	errbuf := new(bytes.Buffer)

	BeforeEach(func() {
		streams, _, buf, errbuf = genericclioptions.NewTestIOStreams()
	})

	It("backup_config", func() {
		tf := cmdtesting.NewTestFactory().WithNamespace("test")
		defer tf.Cleanup()

		cmd = NewBackupConfigCmd(tf, streams)
		Expect(cmd != nil).Should(BeTrue())
	})

	It("check backup_config", func() {
		tf := cmdtesting.NewTestFactory().WithNamespace("test")
		defer tf.Cleanup()

		var cfg string
		cmd = NewBackupConfigCmd(tf, streams)
		cmd.Flags().StringVar(&cfg, "kubeconfig", "", "Path to the kubeconfig file to use for CLI requests.")
		cmd.SetOut(buf)
		cmd.SetErr(errbuf)

		Expect(cmd != nil).Should(BeTrue())
		Expect(cmd.HasSubCommands()).Should(BeFalse())

		o := &upgradeOptions{
			IOStreams: streams,
		}
		Expect(o.complete(tf, cmd)).To(Succeed())
		Expect(o.Namespace).To(Equal("test"))
	})

	It("run backup_config", func() {
		o := &upgradeOptions{
			IOStreams: streams,
			cfg:       helm.FakeActionConfig(),
			Namespace: "default",
			Sets:      []string{"dataProtection=test"},
		}
		Expect(o.run()).To(Or(Succeed(), HaveOccurred()))
		Expect(len(o.Sets)).To(Equal(1))
		Expect(o.Sets[0]).To(Equal("dataProtection=test"))
	})
})
