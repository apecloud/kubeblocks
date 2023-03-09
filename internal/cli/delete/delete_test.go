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

package delete

import (
	"bytes"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	clientfake "k8s.io/client-go/rest/fake"
	cmdtesting "k8s.io/kubectl/pkg/cmd/testing"

	"github.com/apecloud/kubeblocks/internal/cli/testing"
	"github.com/apecloud/kubeblocks/internal/cli/types"
)

var _ = Describe("Delete", func() {
	const (
		testNamespace = "test"
	)

	var cmd *cobra.Command
	var streams genericclioptions.IOStreams
	var tf *cmdtesting.TestFactory
	var pods *corev1.PodList
	var in *bytes.Buffer
	var o *DeleteOptions

	BeforeEach(func() {
		pods, _, _ = cmdtesting.TestData()
		streams, in, _, _ = genericclioptions.NewTestIOStreams()
		tf = cmdtesting.NewTestFactory().WithNamespace(testNamespace)
		tf.Client = &clientfake.RESTClient{}
		tf.FakeDynamicClient = testing.FakeDynamicClient(&pods.Items[0])
		o = NewDeleteOptions(tf, streams, types.PODGVR())
	})

	AfterEach(func() {
		tf.Cleanup()
	})

	buildTestCmd := func(o *DeleteOptions) *cobra.Command {
		cmd = &cobra.Command{
			Use:     "test-delete",
			Short:   "Test a delete command",
			Example: "Test command example",
			RunE: func(cmd *cobra.Command, args []string) error {
				o.Names = args
				return o.Run()
			},
		}
		o.AddFlags(cmd)
		return cmd
	}

	It("complete", func() {
		o.Names = []string{"foo"}
		_, _ = in.Write([]byte("foo\n"))
		Expect(o.complete()).Should(Succeed())

		in.Reset()
		_, _ = in.Write([]byte("bar\n"))
		Expect(o.complete()).Should(MatchError(MatchRegexp("does not match")))
	})

	It("build a delete command", func() {
		cmd := buildTestCmd(o)
		Expect(cmd).ShouldNot(BeNil())

		_, _ = in.Write([]byte("foo\n"))
		Expect(cmd.RunE(cmd, []string{"foo"})).Should(Succeed())
		Expect(cmd.RunE(cmd, []string{"bar"})).Should(HaveOccurred())
	})

	It("validate", func() {
		o.Names = []string{"foo"}
		By("set force and GracePeriod")
		o.Force = true
		o.GracePeriod = 1
		o.Now = false
		Expect(o.validate()).Should(HaveOccurred())

		o.Force = true
		o.GracePeriod = 0
		o.Now = false
		Expect(o.validate()).Should(Succeed())

		By("set now and GracePeriod")
		o.Force = false
		o.Now = true
		o.GracePeriod = 1
		Expect(o.validate()).Should(HaveOccurred())

		o.Force = false
		o.Now = true
		o.GracePeriod = -1
		Expect(o.validate()).Should(Succeed())

		By("set force only")
		o.Force = true
		o.Now = false
		o.GracePeriod = -1
		Expect(o.validate()).Should(Succeed())

		By("set GracePeriod only")
		o.Force = false
		o.Now = false
		o.GracePeriod = 1
		Expect(o.validate()).Should(Succeed())

		o.Force = false
		o.GracePeriod = -1
		o.Now = false

		By("set name and label")
		o.Names = []string{"foo"}
		o.LabelSelector = "foo=bar"
		o.AllNamespaces = false
		Expect(o.validate()).Should(HaveOccurred())

		By("set name and all")
		o.Names = []string{"foo"}
		o.LabelSelector = ""
		o.AllNamespaces = true
		Expect(o.validate()).Should(HaveOccurred())

		By("set all and label")
		o.Names = nil
		o.AllNamespaces = true
		o.LabelSelector = "foo=bar"
		Expect(o.validate()).Should(Succeed())

		By("set name")
		o.Names = []string{"foo"}
		o.AllNamespaces = false
		o.LabelSelector = ""
		Expect(o.validate()).Should(Succeed())

		By("set nothing")
		o.Names = nil
		o.LabelSelector = ""
		Expect(o.validate()).Should(MatchError(MatchRegexp("no name was specified")))
	})
})
