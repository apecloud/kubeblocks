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
	"github.com/Masterminds/semver/v3"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	clientfake "k8s.io/client-go/rest/fake"
	cmdtesting "k8s.io/kubectl/pkg/cmd/testing"

	"github.com/apecloud/kubeblocks/internal/cli/testing"
	"github.com/apecloud/kubeblocks/internal/cli/types"
	"github.com/apecloud/kubeblocks/internal/cli/util/helm"
)

var _ = Describe("kubeblocks list versions", func() {
	var cmd *cobra.Command
	var streams genericclioptions.IOStreams
	var tf *cmdtesting.TestFactory

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

	It("list versions command", func() {
		var cfg string
		cmd = newListVersionsCmd(tf, streams)
		Expect(cmd).ShouldNot(BeNil())

		cmd.Flags().StringVar(&cfg, "kubeconfig", "", "Path to the kubeconfig file to use for CLI requests.")
		cmd.Flags().StringVar(&cfg, "context", "", "The name of the kubeconfig context to use.")

		o := &Options{
			IOStreams: streams,
		}
		Expect(o.complete(tf, cmd)).Should(Succeed())
		Expect(o.Namespace).Should(Equal(namespace))
		Expect(o.HelmCfg).ShouldNot(BeNil())
	})

	It("run list-versions", func() {
		o := listVersionsOption{
			IOStreams: streams,
			HelmCfg:   helm.FakeActionConfig(),
			Namespace: namespace,
		}
		By("setup searched version")
		o.setupSearchedVersion()
		Expect(o.version).ShouldNot(BeEmpty())

		By("search version")
		versions := []string{"0.1.0", "0.1.0-alpha.0"}
		semverVersions := make([]*semver.Version, len(versions))
		for i, v := range versions {
			semVer, _ := semver.NewVersion(v)
			semverVersions[i] = semVer
		}
		res, err := o.applyConstraint(semverVersions)
		Expect(err).Should(Succeed())
		Expect(len(res)).Should(Equal(1))
		Expect(res[0].String()).Should(Equal("0.1.0"))

		By("search version with devel")
		o.devel = true
		o.setupSearchedVersion()
		res, err = o.applyConstraint(semverVersions)
		Expect(err).Should(Succeed())
		Expect(len(res)).Should(Equal(2))

		By("list versions")
		Expect(o.listVersions()).Should(HaveOccurred())
	})
})
