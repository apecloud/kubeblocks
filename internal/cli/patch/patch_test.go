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

package patch

import (
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"k8s.io/cli-runtime/pkg/genericclioptions"
	cmdtesting "k8s.io/kubectl/pkg/cmd/testing"

	"github.com/apecloud/kubeblocks/internal/cli/builder"
	"github.com/apecloud/kubeblocks/internal/cli/types"
)

type testOptions struct {
	*Options
}

func (o *testOptions) complete(c *builder.Command) error {
	if len(c.Args) == 0 {
		return fmt.Errorf("missing cluster name")
	}
	o.Patch = "{terminationPolicy: Delete}"
	return nil
}

func (o *testOptions) run(c *builder.Command) (bool, error) {
	return c != nil, nil
}

var _ = Describe("Patch", func() {
	var streams genericclioptions.IOStreams
	var tf *cmdtesting.TestFactory
	var o *testOptions
	var cb *builder.CmdBuilder

	BeforeEach(func() {
		streams, _, _, _ = genericclioptions.NewTestIOStreams()
		tf = cmdtesting.NewTestFactory().WithNamespace("default")
		o = &testOptions{Options: NewOptions(streams)}
		cb = builder.NewCmdBuilder().
			Factory(tf).
			IOStreams(streams).
			Short("Test patch.").
			GVR(types.ClusterGVR()).
			CustomRun(o.run).
			CustomComplete(o.complete)
	})

	AfterEach(func() {
		tf.Cleanup()
	})

	It("build", func() {
		cmd := cb.Build(o.Build)
		Expect(cmd).ShouldNot(BeNil())
	})

	It("complete", func() {
		c := cb.GetCmd()
		c.Args = []string{"c1"}
		cb.Build(o.Build)
		Expect(o.Options.complete(c)).Should(Succeed())
		Expect(o.Patch).Should(ContainSubstring("terminationPolicy"))
		Expect(o.Options.validate()).Should(Succeed())
	})

	It("run", func() {
		c := cb.GetCmd()
		c.Args = []string{"c1"}
		cb.Build(o.Build)
		Expect(o.Options.complete(c))
		Expect(o.Options.run(c)).Should(HaveOccurred())
	})
})
