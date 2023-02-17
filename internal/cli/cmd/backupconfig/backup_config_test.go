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

package backupconfig

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/spf13/cobra"
	"helm.sh/helm/v3/pkg/cli/values"
	"helm.sh/helm/v3/pkg/repo"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	cmdtesting "k8s.io/kubectl/pkg/cmd/testing"

	"github.com/apecloud/kubeblocks/internal/cli/testing"
	"github.com/apecloud/kubeblocks/internal/cli/types"
	"github.com/apecloud/kubeblocks/internal/cli/util/helm"
)

var _ = Describe("backup_config", func() {
	var (
		cmd     *cobra.Command
		streams genericclioptions.IOStreams
		tf      *cmdtesting.TestFactory
	)

	BeforeEach(func() {
		streams, _, _, _ = genericclioptions.NewTestIOStreams()
		tf = cmdtesting.NewTestFactory().WithNamespace("test")
	})

	AfterEach(func() {
		tf.Cleanup()
	})

	It("backup_config", func() {
		cmd = NewBackupConfigCmd(tf, streams)
		Expect(cmd).ShouldNot(BeNil())
	})

	It("check backup_config", func() {
		var cfg string
		cmd = NewBackupConfigCmd(tf, streams)
		cmd.Flags().StringVar(&cfg, "kubeconfig", "", "Path to the kubeconfig file to use for CLI requests.")

		Expect(cmd).ShouldNot(BeNil())
		Expect(cmd.HasSubCommands()).Should(BeFalse())

		o := &upgradeOptions{
			IOStreams: streams,
		}
		Expect(o.complete(tf, cmd)).To(Succeed())
		Expect(o.namespace).To(Equal("test"))
		Expect(o.helmCfg).ShouldNot(BeNil())
	})

	It("run backup_config", func() {
		// use a fake URL to test
		types.KubeBlocksChartName = testing.KubeBlocksChartName
		types.KubeBlocksChartURL = testing.KubeBlocksChartURL

		// mock helm chart function for test
		old := helmAddRepo
		defer func() { helmAddRepo = old }()
		helmAddRepo = func(r *repo.Entry) error {
			return nil
		}

		o := &upgradeOptions{
			IOStreams: streams,
			helmCfg:   helm.FakeActionConfig(),
			namespace: "default",
			valueOpts: values.Options{Values: []string{"dataProtection=test"}},
		}
		Expect(o.run()).Should(HaveOccurred())
		Expect(len(o.valueOpts.Values)).To(Equal(1))
		Expect(o.valueOpts.Values[0]).To(Equal("dataProtection=test"))
	})
})
