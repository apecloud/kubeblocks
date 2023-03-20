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

package patch

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	cmdtesting "k8s.io/kubectl/pkg/cmd/testing"

	"github.com/apecloud/kubeblocks/internal/cli/types"
)

var _ = Describe("Patch", func() {
	var streams genericclioptions.IOStreams
	var tf *cmdtesting.TestFactory

	BeforeEach(func() {
		streams, _, _, _ = genericclioptions.NewTestIOStreams()
		tf = cmdtesting.NewTestFactory().WithNamespace("default")
	})

	AfterEach(func() {
		tf.Cleanup()
	})

	It("complete", func() {
		cmd := &cobra.Command{}
		o := NewOptions(tf, streams, types.ClusterGVR())
		o.AddFlags(cmd)
		Expect(o.complete(cmd)).Should(HaveOccurred())

		o.Names = []string{"c1"}
		Expect(o.complete(cmd)).Should(Succeed())
	})

	It("run", func() {
		cmd := &cobra.Command{}
		o := NewOptions(tf, streams, types.ClusterGVR())
		o.Names = []string{"c1"}
		o.AddFlags(cmd)

		o.Patch = "{terminationPolicy: Delete}"
		Expect(o.Run(cmd)).Should(HaveOccurred())
	})
})
