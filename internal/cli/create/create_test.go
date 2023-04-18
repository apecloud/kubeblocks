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

package create

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	clientfake "k8s.io/client-go/rest/fake"
	cmdtesting "k8s.io/kubectl/pkg/cmd/testing"

	"github.com/apecloud/kubeblocks/internal/cli/testing"
	"github.com/apecloud/kubeblocks/internal/cli/types"
)

var _ = Describe("Create", func() {
	var (
		tf          *cmdtesting.TestFactory
		streams     genericclioptions.IOStreams
		baseOptions BaseOptions
	)

	BeforeEach(func() {
		streams, _, _, _ = genericclioptions.NewTestIOStreams()
		tf = cmdtesting.NewTestFactory().WithNamespace(testing.Namespace)
		tf.Client = &clientfake.RESTClient{}
		baseOptions = BaseOptions{
			Name:      "test",
			IOStreams: streams,
		}
	})

	AfterEach(func() {
		tf.Cleanup()
	})

	Context("Create Objects", func() {

		It("test Create run", func() {
			clusterOptions := map[string]interface{}{
				"name":              "test",
				"namespace":         testing.Namespace,
				"clusterDefRef":     "test-def",
				"clusterVersionRef": "test-clusterversion-ref",
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
					cmd.Flags().String("dry-run", "none", `Must be "server", or "client". If client strategy, only print the object that would be sent, without sending it. If server strategy, submit server-side request without persisting the resource.`)
				},
			}
			cmd := BuildCommand(inputs)

			Expect(cmd).ShouldNot(BeNil())
			Expect(cmd.Flags().Lookup("clusterDefRef")).ShouldNot(BeNil())
			Expect(cmd.Flags().Lookup("dry-run")).ShouldNot(BeNil())

			Expect(baseOptions.Complete(inputs, []string{})).Should(Succeed())
			Expect(baseOptions.Validate(inputs)).Should(Succeed())
			Expect(baseOptions.Run(inputs)).Should(Succeed())
		})

		It("test do not use dry-run strategy", func() {
			cmd := &cobra.Command{}
			cmd.Flags().String("dry-run", "none", `Must be "server", or "client". If client strategy, only print the object that would be sent, without sending it. If server strategy, submit server-side request without persisting the resource.`)
			cmd.Flags().Lookup("dry-run").NoOptDefVal = "unchanged"

			dryRun, _ := GetDryRunStrategy(cmd)
			Expect(dryRun == DryRunNone).Should(BeTrue())
		})

		It("test no parameters strategy", func() {
			cmd := &cobra.Command{}
			cmd.Flags().String("dry-run", "none", `Must be "server", or "client". If client strategy, only print the object that would be sent, without sending it. If server strategy, submit server-side request without persisting the resource.`)
			cmd.Flags().Lookup("dry-run").NoOptDefVal = "unchanged"

			Expect(cmd.Flags().Set("dry-run", "unchanged")).Should(Succeed())
			dryRun, _ := GetDryRunStrategy(cmd)
			Expect(dryRun == DryRunClient).Should(BeTrue())
		})

		It("test client strategy", func() {
			cmd := &cobra.Command{}
			cmd.Flags().String("dry-run", "none", `Must be "server", or "client". If client strategy, only print the object that would be sent, without sending it. If server strategy, submit server-side request without persisting the resource.`)
			cmd.Flags().Lookup("dry-run").NoOptDefVal = "unchanged"

			Expect(cmd.Flags().Set("dry-run", "client")).Should(Succeed())
			dryRun, _ := GetDryRunStrategy(cmd)

			Expect(dryRun == DryRunClient).Should(BeTrue())
		})

		It("test server strategy", func() {
			cmd := &cobra.Command{}
			cmd.Flags().String("dry-run", "none", `Must be "server", or "client". If client strategy, only print the object that would be sent, without sending it. If server strategy, submit server-side request without persisting the resource.`)
			cmd.Flags().Lookup("dry-run").NoOptDefVal = "unchanged"

			Expect(cmd.Flags().Set("dry-run", "server")).Should(Succeed())
			dryRun, _ := GetDryRunStrategy(cmd)
			Expect(dryRun == DryRunServer).Should(BeTrue())
		})

		It("test error parameter", func() {
			cmd := &cobra.Command{}
			cmd.Flags().String("dry-run", "none", `Must be "server", or "client". If client strategy, only print the object that would be sent, without sending it. If server strategy, submit server-side request without persisting the resource.`)
			cmd.Flags().Lookup("dry-run").NoOptDefVal = "unchanged"

			Expect(cmd.Flags().Set("dry-run", "ape")).Should(Succeed())
			dryRun, _ := GetDryRunStrategy(cmd)
			Expect(dryRun == DryRunServer).Should(BeFalse())
		})

		It("test Create runAsApply", func() {
			clusterOptions := map[string]interface{}{
				"name":              "test-apply",
				"namespace":         testing.Namespace,
				"clusterDefRef":     "test-def",
				"clusterVersionRef": "test-clusterversion-ref",
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
			// create
			Expect(baseOptions.RunAsApply(inputs)).Should(Succeed())
			// apply if exists
			Expect(baseOptions.RunAsApply(inputs)).Should(Succeed())
		})
	})
})
