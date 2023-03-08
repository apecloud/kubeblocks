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
	"helm.sh/helm/v3/pkg/cli/values"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	clientfake "k8s.io/client-go/rest/fake"
	cmdtesting "k8s.io/kubectl/pkg/cmd/testing"

	"github.com/apecloud/kubeblocks/internal/cli/testing"
	"github.com/apecloud/kubeblocks/internal/cli/types"
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

		// use a fake URL to test
		types.KubeBlocksChartName = testing.KubeBlocksChartName
		types.KubeBlocksChartURL = testing.KubeBlocksChartURL
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
		Expect(len(o.ValueOpts.Values)).To(Equal(1))
		Expect(o.ValueOpts.Values[0]).To(Equal(fmt.Sprintf(kMonitorParam, true)))
		Expect(o.installChart()).Should(HaveOccurred())
		o.printNotes()
	})

	It("create volumeSnapshotClass", func() {
		o := &InstallOptions{
			Options: Options{
				IOStreams: streams,
				HelmCfg:   helm.NewFakeConfig(namespace),
				Namespace: "default",
				Client:    testing.FakeClientSet(),
				Dynamic:   testing.FakeDynamicClient(testing.FakeVolumeSnapshotClass()),
			},
			Version:         version.DefaultKubeBlocksVersion,
			Monitor:         true,
			CreateNamespace: true,
			ValueOpts:       values.Options{Values: []string{"snapshot-controller.enabled=true"}},
		}
		Expect(o.createVolumeSnapshotClass()).Should(HaveOccurred())
	})

	It("preCheck", func() {
		o := &InstallOptions{
			Options: Options{
				IOStreams: genericclioptions.NewTestIOStreamsDiscard(),
			},
			Check: true,
		}
		By("kubernetes version is empty")
		versionInfo := map[util.AppName]string{}
		Expect(o.preCheck(versionInfo).Error()).Should(ContainSubstring("failed to get kubernetes version"))

		versionInfo[util.KubernetesApp] = ""
		Expect(o.preCheck(versionInfo).Error()).Should(ContainSubstring("failed to get kubernetes version"))

		By("kubernetes version is smaller than required version")
		versionInfo[util.KubernetesApp] = "v1.20.0"
		Expect(o.preCheck(versionInfo).Error()).Should(ContainSubstring("should be greater than"))

		By("kubernetes is provided by cloud provider")
		versionInfo[util.KubernetesApp] = "v1.25.0-eks"
		Expect(o.preCheck(versionInfo)).Should(Succeed())

		By("kubernetes is not provided by cloud provider")
		versionInfo[util.KubernetesApp] = "v1.25.0"
		Expect(o.preCheck(versionInfo)).Should(Succeed())
	})

	It("disableOrEnableSets", func() {
		o := &InstallOptions{
			Options: Options{
				IOStreams: genericclioptions.NewTestIOStreamsDiscard(),
			},
		}
		cases := []struct {
			desc     string
			sets     []string
			expected map[util.K8sProvider][]string
		}{
			{
				"sets is empty",
				[]string{},
				map[util.K8sProvider][]string{
					util.UnknownProvider: {},
					util.EKSProvider:     {"snapshot-controller.enabled=true"},
				},
			},
			{
				"sets is nil", nil,
				map[util.K8sProvider][]string{
					util.UnknownProvider: nil,
					util.EKSProvider:     {"snapshot-controller.enabled=true"},
				},
			},
			{
				"sets without unsupported flag",
				[]string{"test=false"},
				map[util.K8sProvider][]string{
					util.UnknownProvider: {"test=false"},
					util.EKSProvider:     {"test=false", "snapshot-controller.enabled=true"},
				},
			},
			{
				"sets with unsupported flag and its value is false",
				[]string{"test=false", "loadbalancer.enabled=false"},
				map[util.K8sProvider][]string{
					util.UnknownProvider: {"test=false", "loadbalancer.enabled=false"},
					util.EKSProvider:     {"test=false", "loadbalancer.enabled=false", "snapshot-controller.enabled=true"},
				},
			},
			{
				"sets with unsupported flag and its value is true",
				[]string{"test=false", "loadbalancer.enabled=true"},
				map[util.K8sProvider][]string{
					util.UnknownProvider: {"test=false"},
					util.EKSProvider:     {"test=false", "loadbalancer.enabled=true", "snapshot-controller.enabled=true"},
				},
			},
			{
				"sets with more unsupported flags and the value is true",
				[]string{"test=false", "loadbalancer.enabled=true", "snapshot-controller.enabled=true"},
				map[util.K8sProvider][]string{
					util.UnknownProvider: {"test=false", "snapshot-controller.enabled=true"},
					util.EKSProvider:     {"test=false", "loadbalancer.enabled=true", "snapshot-controller.enabled=true"},
				},
			},
			{
				"sets with more unsupported flags",
				[]string{"test=false", "snapshot-controller.enabled=false", "loadbalancer.enabled=true"},
				map[util.K8sProvider][]string{
					util.UnknownProvider: {"test=false", "snapshot-controller.enabled=false"},
					util.EKSProvider:     {"test=false", "loadbalancer.enabled=true", "snapshot-controller.enabled=true"},
				},
			},
			{
				"sets with more unsupported flags and some values are true, some values are false",
				[]string{"test=false", "loadbalancer.enabled=false"},
				map[util.K8sProvider][]string{
					util.UnknownProvider: {"test=false", "loadbalancer.enabled=false"},
					util.EKSProvider:     {"test=false", "loadbalancer.enabled=false", "snapshot-controller.enabled=true"},
				},
			},
			{
				"sets with more unsupported flags and some values are true, some values are false",
				[]string{"test=false,snapshot-controller.enabled=true,loadbalancer.enabled=false,snapshot-controller.enabled=false"},
				map[util.K8sProvider][]string{
					util.UnknownProvider: {"test=false,snapshot-controller.enabled=true,loadbalancer.enabled=false,snapshot-controller.enabled=false"},
					util.EKSProvider:     {"test=false", "loadbalancer.enabled=false", "snapshot-controller.enabled=true"},
				},
			},
		}

		for _, c := range cases {
			By(c.desc)
			for _, p := range []util.K8sProvider{util.UnknownProvider, util.EKSProvider} {
				o.ValueOpts.Values = c.sets
				o.disableOrEnableSets(p)
				Expect(o.ValueOpts.Values).Should(Equal(c.expected[p]))
			}
		}
	})
})
