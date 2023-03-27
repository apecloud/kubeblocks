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

package addon

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/spf13/viper"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/version"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	cmdtesting "k8s.io/kubectl/pkg/cmd/testing"

	extensionsv1alpha1 "github.com/apecloud/kubeblocks/apis/extensions/v1alpha1"
	clitesting "github.com/apecloud/kubeblocks/internal/cli/testing"
	"github.com/apecloud/kubeblocks/internal/constant"
)

const (
	testNamespace = "test"
)

var _ = Describe("Manage applications related to KubeBlocks", func() {
	var streams genericclioptions.IOStreams
	var tf *cmdtesting.TestFactory

	const addonObjName = "test-addon"

	BeforeEach(func() {
		streams, _, _, _ = genericclioptions.NewTestIOStreams()
		tf = cmdtesting.NewTestFactory().WithNamespace(testNamespace)
		addonObj := &extensionsv1alpha1.Addon{
			ObjectMeta: metav1.ObjectMeta{
				Name: addonObjName,
			},
			Spec: extensionsv1alpha1.AddonSpec{
				Type: extensionsv1alpha1.HelmType,
			},
		}
		tf.FakeDynamicClient = clitesting.FakeDynamicClient(addonObj)
		fakeVer := version.Info{
			Major:      "1",
			Minor:      "26",
			GitVersion: "v1.26.1",
		}
		viper.SetDefault(constant.CfgKeyServerInfo, fakeVer)
	})

	AfterEach(func() {
		tf.Cleanup()
	})

	When("Iterate addon sub-cmds", func() {
		It("do sanity check", func() {
			addonCmd := NewAddonCmd(tf, streams)
			Expect(addonCmd).ShouldNot(BeNil())
			Expect(addonCmd.HasSubCommands()).Should(BeTrue())

			listCmd := newListCmd(tf, streams)
			Expect(listCmd).ShouldNot(BeNil())
			Expect(listCmd.HasSubCommands()).ShouldNot(BeTrue())

			enableCmd := newEnableCmd(tf, streams)
			Expect(enableCmd).ShouldNot(BeNil())
			Expect(enableCmd.HasSubCommands()).ShouldNot(BeTrue())

			disableCmd := newDisableCmd(tf, streams)
			Expect(disableCmd).ShouldNot(BeNil())
			Expect(disableCmd.HasSubCommands()).ShouldNot(BeTrue())

			describeCmd := newDescribeCmd(tf, streams)
			Expect(describeCmd).ShouldNot(BeNil())
			Expect(describeCmd.HasSubCommands()).ShouldNot(BeTrue())
		})
	})

	// When("Run addon sub-cmds with no args", func() {
	// 	It("Run list sub-cmd", func() {
	// 		By("run list cmd")
	// 		listCmd := newListCmd(tf, streams)
	// 		listCmd.Run(listCmd, nil)
	// 	})
	// })

	When("Run addon sub-cmds with addon args", func() {
		It("Run describe sub-cmd", func() {
			By("run describe cmd")
			cmd := newDescribeCmd(tf, streams)
			cmd.Run(cmd, []string{addonObjName})
		})

		// It("Run disable sub-cmd", func() {
		// 	By("run disable cmd")
		// 	cmd := newDisableCmd(tf, streams)
		// 	cmd.Run(cmd, []string{addonObjName})
		// })

		// It("Run enable sub-cmd", func() {
		// 	By("run enable cmd")
		// 	cmd := newEnableCmd(tf, streams)
		// 	cmd.Run(cmd, []string{addonObjName})
		// })
	})

	// When("Enable an addon", func() {
	// 	It("should set addon.spec.install.enabled=true", func() {
	// 		By("Checking install helm chart by fake helm action config")
	// 		enableCmd := newEnableCmd(tf, streams)
	// 		enableCmd.Run(enableCmd, []string{"my-addon"})
	// 	})
	// })
	//
	// When("Disable an addon", func() {
	// 	It("should set addon.spec.install.enabled=false", func() {
	// 		By("Checking install helm chart by fake helm action config")
	// 		disableCmd := newDisableCmd(tf, streams)
	// 		disableCmd.Run(disableCmd, []string{"my-addon"})
	// 	})
	// })

	// When("Enable an addon", func() {
	// 	It("should set addon.spec.install.enabled=true", func() {
	// 		By("Checking install helm chart by fake helm action config")
	// 		enableCmd := newEnableCmd(tf, streams)
	// 		enableCmd.Run(enableCmd, []string{"my-addon"})
	// 	})
	// })
	//
	// When("Disable an addon", func() {
	// 	It("should set addon.spec.install.enabled=false", func() {
	// 		By("Checking install helm chart by fake helm action config")
	// 		disableCmd := newDisableCmd(tf, streams)
	// 		disableCmd.Run(disableCmd, []string{"my-addon"})
	// 	})
	// })
})
