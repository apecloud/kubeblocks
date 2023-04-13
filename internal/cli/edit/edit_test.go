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

package edit

import (
	"net/http"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/resource"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest/fake"
	cmdtesting "k8s.io/kubectl/pkg/cmd/testing"

	"github.com/apecloud/kubeblocks/internal/cli/types"
)

var _ = Describe("List", func() {
	var (
		streams genericclioptions.IOStreams
		tf      *cmdtesting.TestFactory
	)

	mockClient := func() *corev1.PodList {
		pods, _, _ := cmdtesting.TestData()
		tf = cmdtesting.NewTestFactory().WithNamespace("test")
		defer tf.Cleanup()

		codec := scheme.Codecs.LegacyCodec(scheme.Scheme.PrioritizedVersionsAllGroups()...)
		tf.UnstructuredClient = &fake.RESTClient{
			NegotiatedSerializer: resource.UnstructuredPlusDefaultContentConfig().NegotiatedSerializer,
			Resp:                 &http.Response{StatusCode: http.StatusOK, Header: cmdtesting.DefaultHeader(), Body: cmdtesting.ObjBody(codec, pods)},
		}
		return pods
	}

	AfterEach(func() {
		tf.Cleanup()
	})

	It("test edit", func() {
		pods := mockClient()
		o := NewEditOptions(tf, streams, schema.GroupVersionResource{Group: "", Resource: "pods", Version: types.K8sCoreAPIVersion})
		cmd := &cobra.Command{
			Use:   "edit-test",
			Short: "edit test.",
			Run: func(cmd *cobra.Command, args []string) {

			},
		}
		o.AddFlags(cmd)
		podName := pods.Items[0].Name
		Expect(o.Complete(cmd, []string{})).Should(MatchError("missing the name"))
		Expect(o.Complete(cmd, []string{podName})).ShouldNot(HaveOccurred())
	})
})
