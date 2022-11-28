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

package delete

import (
	"fmt"
	"net/http"
	"strings"

	. "github.com/onsi/ginkgo"
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

	"github.com/apecloud/kubeblocks/internal/dbctl/types"
	"github.com/apecloud/kubeblocks/internal/dbctl/util/builder"
)

var _ = Describe("Delete", func() {
	buildTestCmd := func(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
		return builder.NewCmdBuilder().
			Use("test-delete").
			Short("Test a delete command").
			Example("Test command example").
			Factory(f).
			IOStreams(streams).
			GVR(schema.GroupVersionResource{Resource: "pods", Version: types.VersionV1}).
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

	It("build a delete command", func() {
		pods, _, _ := cmdtesting.TestData()
		tf := mockClient(pods)
		streams, in, buf, _ := genericclioptions.NewTestIOStreams()
		cmd := buildTestCmd(tf, streams)
		Expect(cmd).ShouldNot(BeNil())
		deleteFlag := &DeleteFlags{}
		input := strings.NewReader("foo\n")
		Expect(validate(deleteFlag, []string{}, input)).Should(MatchError("missing name"))
		// prompt test always return error
		Expect(validate(deleteFlag, []string{"foo"}, input)).Should(Succeed())
		input = strings.NewReader("test1\n")
		deleteFlag.ResourceNames = []string{"test1"}
		Expect(validate(deleteFlag, []string{"foo"}, input)).Should(Succeed())

		_, _ = in.Write([]byte("foo\n"))
		cmd.Run(cmd, []string{"foo"})
		Expect(buf.String()).Should(Equal("pod \"foo\" deleted\n"))
	})

	It("test", func() {
		test := []string{}
		fmt.Printf("%d\n", len(test))

		test = nil
		fmt.Printf("%d\n", len(test))
	})
})
