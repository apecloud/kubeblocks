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

package list

import (
	"fmt"
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
	cmdutil "k8s.io/kubectl/pkg/cmd/util"

	"github.com/apecloud/kubeblocks/internal/cli/builder"
	"github.com/apecloud/kubeblocks/internal/cli/get"
	"github.com/apecloud/kubeblocks/internal/cli/types"
)

var _ = Describe("List", func() {
	buildTestCmd := func(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
		return builder.NewCmdBuilder().
			Factory(f).
			IOStreams(streams).
			Short("Test list.").
			GVR(schema.GroupVersionResource{Group: "", Resource: "pods", Version: types.VersionV1}).
			Build(Build)
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

	It("run", func() {
		pods, _, _ := cmdtesting.TestData()
		tf := mockClient(pods)
		streams, _, buf, _ := genericclioptions.NewTestIOStreams()
		cmd := buildTestCmd(tf, streams)
		cmd.Run(cmd, []string{})

		expected := `NAME   AGE
foo    <unknown>
bar    <unknown>
`
		Expect(buf.String()).To(Equal(expected))
	})

	It("build list args", func() {
		cmd := &builder.Command{
			GVR:  types.ClusterGVR(),
			Args: []string{},
		}
		o := &get.Options{}
		buildListArgs(cmd, o)
		Expect(len(o.BuildArgs)).Should(Equal(0))

		By("list cluster with args")
		cmd.Args = []string{"test"}
		buildListArgs(cmd, o)
		Expect(len(o.BuildArgs)).Should(Equal(1))

		By("list ops with args")
		cmd = &builder.Command{
			GVR:  types.OpsGVR(),
			Args: []string{"test"},
		}
		buildListArgs(cmd, o)
		clusterLabel := fmt.Sprintf("%s in (%s)", types.InstanceLabelKey, "test")
		Expect(len(o.BuildArgs)).Should(Equal(1))
		Expect(o.LabelSelector).Should(Equal(clusterLabel))

		By("test list with cluster and custom label")
		testLabel := "kubeblocks.io/test=test"
		o.LabelSelector = testLabel
		buildListArgs(cmd, o)
		Expect(o.LabelSelector).Should(Equal(fmt.Sprintf("%s,%s", testLabel, clusterLabel)))
	})
})
