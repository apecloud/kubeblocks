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

package create

import (
	"net/http"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/resource"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest/fake"
	cmdtesting "k8s.io/kubectl/pkg/cmd/testing"

	"github.com/apecloud/kubeblocks/internal/dbctl/types"
)

var _ = Describe("Create", func() {
	mockClient := func(data runtime.Object) *cmdtesting.TestFactory {
		tf := cmdtesting.NewTestFactory().WithNamespace("default")
		defer tf.Cleanup()

		codec := scheme.Codecs.LegacyCodec(scheme.Scheme.PrioritizedVersionsAllGroups()...)
		tf.UnstructuredClient = &fake.RESTClient{
			NegotiatedSerializer: resource.UnstructuredPlusDefaultContentConfig().NegotiatedSerializer,
			Resp:                 &http.Response{StatusCode: http.StatusOK, Header: cmdtesting.DefaultHeader(), Body: cmdtesting.ObjBody(codec, data)},
		}
		return tf
	}

	Context("Create Objects", func() {
		It("test Create run", func() {
			tf := mockClient(&corev1.ConfigMap{})
			streams, _, _, _ := genericclioptions.NewTestIOStreams()
			tf.ClientConfigVal = cfg
			baseOptions := BaseOptions{
				Name:      "test",
				IOStreams: streams,
			}
			clusterOptions := map[string]interface{}{
				"name":              "test",
				"namespace":         "default",
				"clusterDefRef":     "test-def",
				"appVersionRef":     "test-appversion-ref",
				"components":        []string{},
				"terminationPolicy": "Halt",
			}

			inputs := Inputs{
				CueTemplateName: "create_template_test.cue",
				ResourceName:    types.ResourceClusters,
				BaseOptionsObj:  &baseOptions,
				Options:         clusterOptions,
				Factory:         tf,
				Validate: func() error {
					return nil
				},
				Complete: func() error {
					return nil
				},
				BuildFlags: func(cmd *cobra.Command) {
					cmd.Flags().StringVar(&baseOptions.Namespace, "namespace", "", "ClusterDefinition reference")
				},
			}
			BuildCommand(inputs)
			Expect(baseOptions.Complete(inputs, []string{})).Should(Succeed())
			Expect(baseOptions.Run(inputs, []string{"test"})).Should(Succeed())
			Expect(baseOptions.Run(inputs, []string{"test", "more", "args"})).Should(HaveOccurred())
		})
	})
})
