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

package app

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"k8s.io/cli-runtime/pkg/genericclioptions"
	cmdtesting "k8s.io/kubectl/pkg/cmd/testing"

	"github.com/apecloud/kubeblocks/internal/cli/testing"
	"github.com/apecloud/kubeblocks/internal/cli/types"
	"github.com/apecloud/kubeblocks/internal/cli/util/helm"
)

const (
	testAppName   = "test-app"
	testNamespace = "test"
)

var _ = Describe("Manage applications related to KubeBlocks", func() {
	var streams genericclioptions.IOStreams
	var tf *cmdtesting.TestFactory

	BeforeEach(func() {
		streams, _, _, _ = genericclioptions.NewTestIOStreams()
		tf = cmdtesting.NewTestFactory().WithNamespace(testNamespace)

		// use a fake URL to test
		types.KubeBlocksChartName = testing.KubeBlocksChartName
		types.KubeBlocksChartURL = testing.KubeBlocksChartURL
	})

	AfterEach(func() {
		tf.Cleanup()
	})

	When("Installing application", func() {
		It("should install application with default configuration", func() {

			By("Checking app cmd")
			appCmd := NewAppCmd(tf, streams)
			Expect(appCmd).ShouldNot(BeNil())
			Expect(appCmd.HasSubCommands()).Should(BeTrue())

			By("Checking install sub-cmd")
			installCmd := newInstallCmd(tf, streams)
			Expect(installCmd).ShouldNot(BeNil())
			Expect(installCmd.HasSubCommands()).Should(BeFalse())

			By("Checking install options without kubeconfig and context flag")
			var cfg string
			o := &options{
				IOStreams: streams,
				Factory:   tf,
				AppName:   testAppName,
			}
			installCmd.Flags().StringVar(&cfg, "kubeconfig", "", "Path to the kubeconfig file to use for CLI requests.")
			installCmd.Flags().StringVar(&cfg, "context", "", "The name of the kubeconfig context to use.")
			Expect(o.complete(installCmd, []string{testAppName})).To(Succeed())
			Expect(len(o.Sets)).To(Equal(1))
			Expect(o.Sets[0]).To(Equal(fmt.Sprintf("namespace=%s", testNamespace)))
			Expect(o.HelmCfg).ShouldNot(BeNil())
			Expect(o.Namespace).To(Equal(testNamespace))

			By("Checking install helm chart by fake helm action config")
			o = &options{
				IOStreams: streams,
				Factory:   tf,
				AppName:   testAppName,
			}
			Expect(o.complete(installCmd, []string{testAppName})).Should(Succeed())
			o.HelmCfg = helm.NewFakeConfig(testNamespace)
			Expect(o.install()).Should(HaveOccurred())
			notes, err := o.installChart()
			Expect(err).Should(HaveOccurred())
			Expect(notes).Should(Equal(""))
		})
	})

	When("Uninstall application", func() {
		It("should uninstall application successfully", func() {
			By("Checking application uninstall sub-cmd")
			uninstallCmd := newUninstallCmd(tf, streams)
			Expect(uninstallCmd).ShouldNot(BeNil())
			Expect(uninstallCmd.HasSubCommands()).Should(BeFalse())

			By("Checking uninstall helm chart by fake helm action config")
			o := &options{
				IOStreams: streams,
				HelmCfg:   helm.NewFakeConfig(testNamespace),
				AppName:   testAppName,
			}
			Expect(o.uninstall()).Should(HaveOccurred())
		})
	})
})
