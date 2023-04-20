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

package class

import (
	"bytes"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/kubernetes/scheme"
	cmdtesting "k8s.io/kubectl/pkg/cmd/testing"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/cli/testing"
)

var _ = Describe("list", func() {
	var (
		out     *bytes.Buffer
		tf      *cmdtesting.TestFactory
		streams genericclioptions.IOStreams
	)

	BeforeEach(func() {
		streams, _, out, _ = genericclioptions.NewTestIOStreams()
		tf = testing.NewTestFactory(namespace)
		_ = appsv1alpha1.AddToScheme(scheme.Scheme)
		tf.FakeDynamicClient = testing.FakeDynamicClient(&classDef)
	})

	AfterEach(func() {
		tf.Cleanup()
	})

	It("should succeed", func() {
		cmd := NewListCommand(tf, streams)
		Expect(cmd).ShouldNot(BeNil())
		_ = cmd.Flags().Set("cluster-definition", "apecloud-mysql")
		cmd.Run(cmd, []string{})
		Expect(out.String()).To(ContainSubstring("general-1c1g"))
		Expect(out.String()).To(ContainSubstring("mysql"))
		Expect(out.String()).To(ContainSubstring(generalResourceConstraint.Name))
	})

	It("memory should be normalized", func() {
		cases := []struct {
			memory     string
			normalized string
		}{
			{
				memory:     "0.2Gi",
				normalized: "0.2Gi",
			},
			{
				memory:     "0.2Mi",
				normalized: "0.2Mi",
			},
			{
				memory:     "0.2Ki",
				normalized: "0.2Ki",
			},
			{
				memory:     "1024Mi",
				normalized: "1Gi",
			},
			{
				memory:     "1025Mi",
				normalized: "1025Mi",
			},
			{
				memory:     "1023Mi",
				normalized: "1023Mi",
			},
			{
				memory:     "1Gi",
				normalized: "1Gi",
			},
			{
				memory:     "512Mi",
				normalized: "512Mi",
			},
		}
		for _, item := range cases {
			Expect(normalizeMemory(resource.MustParse(item.memory))).Should(Equal(item.normalized))
		}
	})
})
