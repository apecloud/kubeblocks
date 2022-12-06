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

package describe

import (
	"net/http"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/resource"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest/fake"
	cmdtesting "k8s.io/kubectl/pkg/cmd/testing"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/describe"

	"github.com/apecloud/kubeblocks/internal/cli/types"
)

var _ = Describe("Describe", func() {
	options := func(f cmdutil.Factory, streams genericclioptions.IOStreams) *Options {
		o := &Options{
			Factory:   f,
			IOStreams: streams,
			Short:     "Test describe",
			DescriberSettings: &describe.DescriberSettings{
				ShowEvents: true,
				ChunkSize:  cmdutil.DefaultChunkSize,
			},
			GVR: schema.GroupVersionResource{Version: types.VersionV1, Resource: "pods"},
		}
		return o
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
		options := options(tf, streams)
		Expect(options.complete([]string{})).To(MatchError("You must specify the name of resource to describe."))
		Expect(options.complete([]string{"foo"})).To(Succeed())

		cmd := options.Build()
		Expect(cmd).ShouldNot(BeNil())
		Expect(options.run()).Should(HaveOccurred())
	})
})
