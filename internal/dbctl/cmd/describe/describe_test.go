/*
Copyright 2022 The KubeBlocks Authors

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

package describe

import (
	"fmt"
	"net/http"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest/fake"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/resource"
	cmdtesting "k8s.io/kubectl/pkg/cmd/testing"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
)

var _ = Describe("Describe", func() {
	testCmd := func(f cmdutil.Factory, streams genericclioptions.IOStreams) *Command {
		cmd := &Command{
			Factory:   f,
			Streams:   streams,
			Short:     "Test describe.",
			GroupKind: []schema.GroupKind{{Group: "", Kind: "pods"}},
			Template:  []string{"test.tmpl"},
			PrintExtra: func() error {
				fmt.Fprintln(streams.Out, "test print fun")
				return nil
			},
		}
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
		tf := mockClient(&pods.Items[0])
		streams, _, _, _ := genericclioptions.NewTestIOStreams()
		cmd := testCmd(tf, streams)
		Expect(cmd.complete([]string{})).To(MatchError("You must specify the name of resource to describe."))

		cmd.Template = []string{}
		Expect(cmd.complete([]string{"test"})).To(MatchError("The number of resource type is not equal to template."))

		cmd.GroupKind = []schema.GroupKind{}
		Expect(cmd.complete([]string{"test"})).To(MatchError("You must specify the resource type to describe."))
	})

	It("run", func() {
		pods, _, _ := cmdtesting.TestData()
		tf := mockClient(&pods.Items[0])
		streams, _, buf, _ := genericclioptions.NewTestIOStreams()
		cmd := testCmd(tf, streams).Build()
		cmd.Run(cmd, []string{"foo"})

		expected := `Name:foo
Namespace:test
Kind:Pod
test print fun
`
		Expect(buf.String()).To(Equal(expected))
	})
})
