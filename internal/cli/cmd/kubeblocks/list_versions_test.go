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
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/Masterminds/semver/v3"
	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	clientfake "k8s.io/client-go/rest/fake"
	cmdtesting "k8s.io/kubectl/pkg/cmd/testing"
)

var _ = Describe("kubeblocks list versions", func() {
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

	It("list versions command", func() {
		cmd = newListVersionsCmd(streams)
		Expect(cmd).ShouldNot(BeNil())
	})

	It("run list-versions", func() {
		o := listVersionsOption{
			IOStreams: streams,
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

		// TODO: use a mock helm chart to test
		By("list versions")
		Expect(o.listVersions()).Should(HaveOccurred())
	})
})
