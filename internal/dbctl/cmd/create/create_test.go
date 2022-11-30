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
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	cmdtesting "k8s.io/kubectl/pkg/cmd/testing"

	"github.com/apecloud/kubeblocks/internal/dbctl/types"
)

var _ = Describe("Create", func() {
	Context("Create Objects", func() {
		It("test Create run", func() {
			tf := cmdtesting.NewTestFactory().WithNamespace("default")
			defer tf.Cleanup()
			streams, _, _, _ := genericclioptions.NewTestIOStreams()
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
					cmd.Flags().StringVar(&baseOptions.Namespace, "clusterDefRef", "", "cluster definition")
				},
			}
			cmd := BuildCommand(inputs)
			Expect(cmd).ShouldNot(BeNil())
			Expect(cmd.Flags().Lookup("clusterDefRef")).ShouldNot(BeNil())

			Expect(baseOptions.Complete(inputs, []string{})).Should(Succeed())
			Expect(baseOptions.Validate(inputs)).Should(Succeed())
			Expect(baseOptions.Run(inputs)).Should(Succeed())
		})
	})
})
