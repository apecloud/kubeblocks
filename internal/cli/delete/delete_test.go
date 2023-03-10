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
	"net/http"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/resource"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest/fake"
	cmdtesting "k8s.io/kubectl/pkg/cmd/testing"

	"github.com/apecloud/kubeblocks/internal/cli/types"
	"github.com/apecloud/kubeblocks/internal/cli/util"
)

var _ = Describe("Delete", func() {
	buildTestCmd := func(o *DeleteOptions) *cobra.Command {
		cmd := &cobra.Command{
			Use:     "test-delete",
			Short:   "Test a delete command",
			Example: "Test command example",
			Run: func(cmd *cobra.Command, args []string) {
				o.Names = args
				util.CheckErr(o.Run())
			},
		}
		o.AddFlags(cmd)
		return cmd
	}

	mockClient := func(data runtime.Object) *cmdtesting.TestFactory {
		tf := cmdtesting.NewTestFactory().WithNamespace("test")
		defer tf.Cleanup()
		codec := scheme.Codecs.LegacyCodec(scheme.Scheme.PrioritizedVersionsAllGroups()...)
		tf.UnstructuredClient = &fake.RESTClient{
			NegotiatedSerializer: resource.UnstructuredPlusDefaultContentConfig().NegotiatedSerializer,
			Resp:                 &http.Response{StatusCode: http.StatusOK, Header: cmdtesting.DefaultHeader(), Body: cmdtesting.ObjBody(codec, data)},
		}
		return tf
	}

	It("complete", func() {
		pods, _, _ := cmdtesting.TestData()
		tf := mockClient(pods)
		streams, in, _, _ := genericclioptions.NewTestIOStreams()
		o := NewDeleteOptions(tf, streams, schema.GroupVersionResource{Resource: "pods", Version: types.K8sCoreAPIVersion})

		By("set force and GracePeriod")
		o.Force = true
		o.GracePeriod = 1
		Expect(o.complete()).Should(HaveOccurred())

		By("set now and GracePeriod")
		o.Force = false
		o.Now = true
		o.GracePeriod = 1
		Expect(o.complete()).Should(HaveOccurred())

		By("confirm")
		o.Now = false
		Expect(o.complete()).Should(MatchError(MatchRegexp("no name was specified")))

		_, _ = in.Write([]byte("foo\n"))
		o.Names = []string{"foo"}
		Expect(o.complete()).Should(Succeed())
		Expect(o.Result).ShouldNot(BeNil())
	})

	It("build a delete command", func() {
		pods, _, _ := cmdtesting.TestData()
		tf := mockClient(pods)
		streams, in, _, _ := genericclioptions.NewTestIOStreams()
		o := NewDeleteOptions(tf, streams, schema.GroupVersionResource{Resource: "pods", Version: types.K8sCoreAPIVersion})
		cmd := buildTestCmd(o)
		Expect(cmd).ShouldNot(BeNil())

		_, _ = in.Write([]byte("foo\n"))
		cmd.Run(cmd, []string{"foo"})
	})
})
