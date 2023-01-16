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

package nyancat

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

const testNamespace = "test"

var _ = Describe("nyancat", func() {
	var streams genericclioptions.IOStreams
	var tf *cmdtesting.TestFactory

	BeforeEach(func() {
		streams, _, _, _ = genericclioptions.NewTestIOStreams()
		tf = cmdtesting.NewTestFactory().WithNamespace(testNamespace)

		// use a fake URL to test
		types.NyanCatChartName = testing.NyanCatChartName
		types.KubeBlocksChartURL = testing.KubeBlocksChartURL
	})

	AfterEach(func() {
		tf.Cleanup()
	})

	When("Installing Nyan Cat", func() {
		It("should install application with default configuration", func() {

			By("Checking Nyan Cat cmd")
			nyanCatCmd := NewNyancatCmd(tf, streams)
			Expect(nyanCatCmd).ShouldNot(BeNil())
			Expect(nyanCatCmd.HasSubCommands()).Should(BeTrue())

			By("Checking Nyan Cat install sub-cmd")
			installCmd := newInstallCmd(tf, streams)
			Expect(installCmd).ShouldNot(BeNil())
			Expect(installCmd.HasSubCommands()).Should(BeFalse())

			By("Checking install options without kubeconfig and context flag")
			var cfg string
			o := &options{
				IOStreams: streams,
			}
			installCmd.Flags().StringVar(&cfg, "kubeconfig", "", "Path to the kubeconfig file to use for CLI requests.")
			installCmd.Flags().StringVar(&cfg, "context", "", "The name of the kubeconfig context to use.")
			Expect(o.complete(tf, installCmd)).To(Succeed())
			Expect(len(o.Sets)).To(Equal(1))
			Expect(o.Sets[0]).To(Equal(fmt.Sprintf("namespace=%s", testNamespace)))
			Expect(o.HelmCfg).ShouldNot(BeNil())
			Expect(o.Namespace).To(Equal(testNamespace))

			By("Checking install helm chart by fake helm action config")
			o = &options{
				IOStreams: streams,
			}
			Expect(o.complete(tf, installCmd)).Should(Succeed())
			o.HelmCfg = helm.FakeActionConfig()
			Expect(o.install()).Should(HaveOccurred())
			Expect(o.installChart()).Should(HaveOccurred())
			o.printNotes()
		})
	})

	When("Uninstall Nyan Cat", func() {
		It("should uninstall application successfully", func() {
			By("Checking Nyan Cat uninstall sub-cmd")
			uninstallCmd := newUninstallCmd(tf, streams)
			Expect(uninstallCmd).ShouldNot(BeNil())
			Expect(uninstallCmd.HasSubCommands()).Should(BeFalse())

			By("Checking uninstall helm chart by fake helm action config")
			o := &options{
				IOStreams: streams,
				HelmCfg:   helm.FakeActionConfig(),
			}
			Expect(o.uninstall()).Should(HaveOccurred())
		})
	})
})
