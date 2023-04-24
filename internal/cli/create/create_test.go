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
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/printers"
	clientfake "k8s.io/client-go/rest/fake"
	cmdtesting "k8s.io/kubectl/pkg/cmd/testing"
	"k8s.io/kubectl/pkg/scheme"

	"github.com/apecloud/kubeblocks/internal/cli/printer"
	"github.com/apecloud/kubeblocks/internal/cli/testing"
	"github.com/apecloud/kubeblocks/internal/cli/types"
)

var _ = Describe("Create", func() {
	var (
		tf          *cmdtesting.TestFactory
		streams     genericclioptions.IOStreams
		baseOptions CreateOptions
	)

	BeforeEach(func() {
		streams, _, _, _ = genericclioptions.NewTestIOStreams()
		tf = cmdtesting.NewTestFactory().WithNamespace(testing.Namespace)
		tf.Client = &clientfake.RESTClient{}
		baseOptions = CreateOptions{
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

			options := CreateOptions{
				CueTemplateName: "create_template_test.cue",
				GVR:             types.ClusterGVR(),
				Options:         clusterOptions,
				Factory:         tf,
				Name:            "test",
				Namespace:       testing.Namespace,
			}
			Expect(options.Complete([]string{})).Should(Succeed())
		})

		It("test create dry-run", func() {
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
					baseOptions.ToPrinter = func(mapping *meta.RESTMapping, withNamespace bool) (printers.ResourcePrinterFunc, error) {
						var p printers.ResourcePrinter
						var err error
						switch baseOptions.Format {
						case printer.JSON:
							p = &printers.JSONPrinter{}
						case printer.YAML:
							p = &printers.YAMLPrinter{}
						default:
							return nil, genericclioptions.NoCompatiblePrinterError{AllowedFormats: []string{"JOSN", "YAML"}}
						}

						p, err = printers.NewTypeSetter(scheme.Scheme).WrapToPrinter(p, nil)
						if err != nil {
							return nil, err
						}
						return p.PrintObj, nil
					}
					return nil
				},
				BuildFlags: func(cmd *cobra.Command) {
					cmd.Flags().StringVar(&baseOptions.Namespace, "clusterDefRef", "", "cluster definition")
					cmd.Flags().String("dry-run", "none", `Must be "server", or "client". If client strategy, only print the object that would be sent, without sending it. If server strategy, submit server-side request without persisting the resource.`)
					cmd.Flags().Lookup("dry-run").NoOptDefVal = "unchanged"
					printer.AddOutputFlagForCreate(cmd, &baseOptions.Format)
				},
			}
			cmd := BuildCommand(inputs)
			inputs.Cmd = cmd

			testCases := []struct {
				clusterName   string
				isUseDryRun   bool
				mode          string
				dryRunStrateg DryRunStrategy
				success       bool
			}{
				{ // test do not use dry-run strategy
					"test1",
					false,
					"",
					DryRunNone,
					true,
				},
				{ // test no parameter strategy
					"test2",
					true,
					"unchanged",
					DryRunClient,
					true,
				},
				{ // test client strategy
					"test3",
					true,
					"client",
					DryRunClient,
					true,
				},
				{ // test server strategy
					"test4",
					true,
					"server",
					DryRunServer,
					true,
				},
				{ // test error parameter
					"test5",
					true,
					"ape",
					DryRunServer,
					false,
				},
			}

			for _, t := range testCases {
				clusterOptions["name"] = t.clusterName
				Expect(cmd).ShouldNot(BeNil())
				Expect(cmd.Flags().Lookup("clusterDefRef")).ShouldNot(BeNil())
				Expect(cmd.Flags().Lookup("dry-run")).ShouldNot(BeNil())
				Expect(cmd.Flags().Lookup("output")).ShouldNot(BeNil())
				if t.isUseDryRun {
					Expect(cmd.Flags().Set("dry-run", t.mode)).Should(Succeed())
				}

				Expect(baseOptions.Complete(inputs, []string{})).Should(Succeed())
				Expect(baseOptions.Validate(inputs)).Should(Succeed())

				dryRunStrateg, _ := GetDryRunStrategy(cmd)
				if t.success {
					Expect(dryRunStrateg == t.dryRunStrateg).Should(BeTrue())
					Expect(baseOptions.Run(inputs)).Should(Succeed())
				} else {
					Expect(dryRunStrateg == t.dryRunStrateg).Should(BeFalse())
				}
			}
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
