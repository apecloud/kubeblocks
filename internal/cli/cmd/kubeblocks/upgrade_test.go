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

package kubeblocks

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/spf13/cobra"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	clientfake "k8s.io/client-go/rest/fake"
	cmdtesting "k8s.io/kubectl/pkg/cmd/testing"

	"github.com/apecloud/kubeblocks/internal/cli/testing"
	"github.com/apecloud/kubeblocks/internal/cli/types"
	"github.com/apecloud/kubeblocks/internal/cli/util/helm"
	"github.com/apecloud/kubeblocks/version"
)

var _ = Describe("kubeblocks upgrade", func() {
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

	It("check upgrade", func() {
		var cfg string
		cmd = newUpgradeCmd(tf, streams)
		Expect(cmd).ShouldNot(BeNil())
		Expect(cmd.HasSubCommands()).Should(BeFalse())

		o := &InstallOptions{
			Options: Options{
				IOStreams: streams,
			},
		}

		By("command without kubeconfig flag")
		Expect(o.Complete(tf, cmd)).Should(HaveOccurred())

		cmd.Flags().StringVar(&cfg, "kubeconfig", "", "Path to the kubeconfig file to use for CLI requests.")
		cmd.Flags().StringVar(&cfg, "context", "", "The name of the kubeconfig context to use.")
		Expect(o.Complete(tf, cmd)).To(Succeed())
		Expect(o.HelmCfg).ShouldNot(BeNil())
		Expect(o.Namespace).To(Equal("test"))
	})

	It("run upgrade", func() {
		mockDeploy := func() *appsv1.Deployment {
			deploy := &appsv1.Deployment{}
			deploy.SetLabels(map[string]string{
				"app.kubernetes.io/name":    types.KubeBlocksChartName,
				"app.kubernetes.io/version": "0.3.0",
			})
			return deploy
		}

		o := &InstallOptions{
			Options: Options{
				IOStreams: streams,
				HelmCfg:   helm.NewFakeConfig(namespace),
				Namespace: "default",
				Client:    testing.FakeClientSet(mockDeploy()),
				Dynamic:   testing.FakeDynamicClient(),
			},
			Version: version.DefaultKubeBlocksVersion,
			Monitor: true,
			Check:   false,
		}
		cmd := newUpgradeCmd(tf, streams)
		_ = cmd.Flags().Set("monitor", "true")
		Expect(o.Upgrade(cmd)).Should(HaveOccurred())
		Expect(len(o.ValueOpts.Values)).To(Equal(1))
		Expect(o.ValueOpts.Values[0]).To(Equal(fmt.Sprintf(kMonitorParam, true)))
		Expect(o.upgradeChart()).Should(HaveOccurred())

		o.printNotes()
	})
})
