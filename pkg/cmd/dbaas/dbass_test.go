/*
Copyright 2022.

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

package dbaas

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	cmdtesting "k8s.io/kubectl/pkg/cmd/testing"
)

var _ = Describe("dbaas", func() {
	var cmd *cobra.Command
	var streams genericclioptions.IOStreams

	BeforeEach(func() {
		streams, _, _, _ = genericclioptions.NewTestIOStreams()
	})

	Context("command", func() {
		It("dbaas", func() {
			tf := cmdtesting.NewTestFactory().WithNamespace("test")
			defer tf.Cleanup()

			cmd = NewDbaasCmd(tf, streams)
			Expect(cmd != nil).Should(BeTrue())
			Expect(cmd.HasSubCommands()).Should(BeTrue())
		})

		It("install", func() {
			tf := cmdtesting.NewTestFactory().WithNamespace("test")
			defer tf.Cleanup()

			var cfg string
			cmd = newInstallCmd(tf, streams)
			cmd.Flags().StringVar(&cfg, "kubeconfig", "", "Path to the kubeconfig file to use for CLI requests.")

			Expect(cmd != nil).Should(BeTrue())
			Expect(cmd.HasSubCommands()).Should(BeFalse())
			o := &InstallOptions{
				Options: Options{
					IOStreams: streams,
				},
			}

			Expect(o.Complete(tf, cmd)).To(Succeed())
			Expect(o.KubeConfig).To(Equal(""))
			Expect(o.Namespace).To(Equal("test"))
		})

		It("uninstall", func() {
			tf := cmdtesting.NewTestFactory().WithNamespace("test")
			defer tf.Cleanup()

			var cfg string
			cmd = newUninstallCmd(tf, streams)
			cmd.Flags().StringVar(&cfg, "kubeconfig", "", "Path to the kubeconfig file to use for CLI requests.")

			Expect(cmd != nil).Should(BeTrue())
			Expect(cmd.HasSubCommands()).Should(BeFalse())

			o := &InstallOptions{
				Options: Options{
					IOStreams: streams,
				},
			}
			Expect(o.Complete(tf, cmd)).To(Succeed())
			Expect(o.KubeConfig).To(Equal(""))
			Expect(o.Namespace).To(Equal("test"))
		})
	})
})
