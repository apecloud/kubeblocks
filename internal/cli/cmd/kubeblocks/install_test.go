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
	"bytes"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	clientfake "k8s.io/client-go/rest/fake"
	cmdtesting "k8s.io/kubectl/pkg/cmd/testing"

	extensionsv1alpha1 "github.com/apecloud/kubeblocks/apis/extensions/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/cli/testing"
	"github.com/apecloud/kubeblocks/internal/cli/util"
	"github.com/apecloud/kubeblocks/internal/cli/util/helm"
	"github.com/apecloud/kubeblocks/version"
)

const namespace = "test"

var _ = Describe("kubeblocks install", func() {
	var (
		cmd     *cobra.Command
		streams genericclioptions.IOStreams
		tf      *cmdtesting.TestFactory
	)

	BeforeEach(func() {
		streams, _, _, _ = genericclioptions.NewTestIOStreams()
		tf = cmdtesting.NewTestFactory().WithNamespace(namespace)
		tf.Client = &clientfake.RESTClient{}
	})

	AfterEach(func() {
		tf.Cleanup()
	})

	It("check install", func() {
		var cfg string
		cmd = newInstallCmd(tf, streams)
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

	It("run install", func() {
		o := &InstallOptions{
			Options: Options{
				IOStreams: streams,
				HelmCfg:   helm.NewFakeConfig(namespace),
				Client:    testing.FakeClientSet(),
				Dynamic:   testing.FakeDynamicClient(),
			},
			Version:         version.DefaultKubeBlocksVersion,
			Monitor:         true,
			CreateNamespace: true,
		}
		Expect(o.Install()).Should(HaveOccurred())
		Expect(o.ValueOpts.Values).Should(HaveLen(1))
		Expect(o.ValueOpts.Values[0]).To(Equal(fmt.Sprintf(kMonitorParam, true)))
		Expect(o.installChart()).Should(HaveOccurred())
		o.printNotes()
	})

	It("checkVersion", func() {
		o := &InstallOptions{
			Options: Options{
				IOStreams: genericclioptions.NewTestIOStreamsDiscard(),
				Client:    testing.FakeClientSet(),
			},
			Check: true,
		}
		By("kubernetes version is empty")
		v := util.Version{}
		Expect(o.checkVersion(v).Error()).Should(ContainSubstring("failed to get kubernetes version"))

		By("kubernetes is provided by cloud provider")
		v.Kubernetes = "v1.25.0-eks"
		Expect(o.checkVersion(v)).Should(Succeed())

		By("kubernetes is not provided by cloud provider")
		v.Kubernetes = "v1.25.0"
		Expect(o.checkVersion(v)).Should(Succeed())
	})

	It("printAddonTimeoutMsg", func() {
		const (
			reason = "test-failed-reason"
		)

		fakeAddOn := func(name string, conditionTrue bool, msg string) *extensionsv1alpha1.Addon {
			addon := &extensionsv1alpha1.Addon{}
			addon.Name = name
			addon.Status = extensionsv1alpha1.AddonStatus{}
			if conditionTrue {
				addon.Status.Phase = extensionsv1alpha1.AddonEnabled
			} else {
				addon.Status.Phase = extensionsv1alpha1.AddonFailed
				addon.Status.Conditions = []metav1.Condition{
					{
						Message: msg,
						Reason:  reason,
					},
					{
						Message: msg,
						Reason:  reason,
					},
				}
			}
			return addon
		}

		testCases := []struct {
			desc     string
			addons   []*extensionsv1alpha1.Addon
			expected string
		}{
			{
				desc:     "addons is nil",
				addons:   nil,
				expected: "",
			},
			{
				desc: "addons without false condition",
				addons: []*extensionsv1alpha1.Addon{
					fakeAddOn("addon", true, ""),
				},
				expected: "",
			},
			{
				desc: "addons with false condition",
				addons: []*extensionsv1alpha1.Addon{
					fakeAddOn("addon1", true, ""),
					fakeAddOn("addon2", false, "failed to enable addon2"),
				},
				expected: "failed to enable addon2",
			},
		}

		for _, c := range testCases {
			By(c.desc)
			out := &bytes.Buffer{}
			printAddonTimeoutMsg(out, c.addons)
			Expect(out.String()).To(ContainSubstring(c.expected))
		}
	})
})
