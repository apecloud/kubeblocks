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

package kubeblocks

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	clientfake "k8s.io/client-go/rest/fake"
	cmdtesting "k8s.io/kubectl/pkg/cmd/testing"

	"github.com/apecloud/kubeblocks/internal/cli/testing"
	"github.com/apecloud/kubeblocks/internal/cli/util/helm"
)

var _ = Describe("kubeblocks uninstall", func() {
	var cmd *cobra.Command
	var streams genericclioptions.IOStreams
	var tf *cmdtesting.TestFactory

	BeforeEach(func() {
		streams, _, _, _ = genericclioptions.NewTestIOStreams()
		tf = cmdtesting.NewTestFactory().WithNamespace(namespace)
		tf.Client = &clientfake.RESTClient{}
	})

	AfterEach(func() {
		tf.Cleanup()
	})

	It("check uninstall", func() {
		var cfg string
		cmd = newUninstallCmd(tf, streams)
		Expect(cmd).ShouldNot(BeNil())

		cmd.Flags().StringVar(&cfg, "kubeconfig", "", "Path to the kubeconfig file to use for CLI requests.")
		cmd.Flags().StringVar(&cfg, "context", "", "The name of the kubeconfig context to use.")
		Expect(cmd.HasSubCommands()).Should(BeFalse())

		o := &Options{
			IOStreams: streams,
		}
		Expect(o.Complete(tf, cmd)).Should(Succeed())
		Expect(o.Namespace).Should(Equal(namespace))
		Expect(o.HelmCfg).ShouldNot(BeNil())
	})

	It("run uninstall", func() {
		o := UninstallOptions{
			Options: Options{
				IOStreams: streams,
				HelmCfg:   helm.NewFakeConfig(namespace),
				Namespace: "default",
				Client:    testing.FakeClientSet(),
				Dynamic:   testing.FakeDynamicClient(testing.FakeVolumeSnapshotClass()),
			},
			AutoApprove: true,
		}
		Expect(o.Uninstall()).Should(Succeed())
	})

	It("checkResources", func() {
		fakeDynamic := testing.FakeDynamicClient()
		Expect(checkResources(fakeDynamic)).Should(Succeed())
	})
})
