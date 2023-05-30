/*
Copyright (C) 2022-2023 ApeCloud Co., Ltd

This file is part of KubeBlocks project

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program.  If not, see <http://www.gnu.org/licenses/>.
*/

package create

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"k8s.io/cli-runtime/pkg/genericclioptions"
	clientfake "k8s.io/client-go/rest/fake"
	cmdtesting "k8s.io/kubectl/pkg/cmd/testing"

	"github.com/apecloud/kubeblocks/internal/cli/printer"
	"github.com/apecloud/kubeblocks/internal/cli/testing"
	"github.com/apecloud/kubeblocks/internal/cli/types"
)

var _ = Describe("Create", func() {
	const (
		clusterName = "test"
		cueFileName = "create_template_test.cue"
	)

	var (
		tf      *cmdtesting.TestFactory
		streams genericclioptions.IOStreams
		options CreateOptions
	)

	BeforeEach(func() {
		streams, _, _, _ = genericclioptions.NewTestIOStreams()
		tf = cmdtesting.NewTestFactory().WithNamespace(testing.Namespace)
		tf.Client = &clientfake.RESTClient{}
		clusterOptions := map[string]interface{}{
			"clusterDefRef":     "test-def",
			"clusterVersionRef": "test-clusterversion-ref",
			"components":        []string{},
			"terminationPolicy": "Halt",
		}
		options = CreateOptions{
			Factory:         tf,
			Name:            clusterName,
			Namespace:       testing.Namespace,
			IOStreams:       streams,
			GVR:             types.ClusterGVR(),
			CueTemplateName: cueFileName,
			Options:         clusterOptions,
		}
	})

	AfterEach(func() {
		tf.Cleanup()
	})

	Context("Create Objects", func() {
		It("Complete", func() {
			options.Args = []string{}
			Expect(options.Complete()).Should(Succeed())
		})

		It("test edit the cluster resource before creating", func() {
			options.Format = printer.YAML
			Expect(options.Complete()).Should(Succeed())
			resObj, err := options.buildResourceObj()
			Expect(err).Should(Succeed())
			Expect(options.runEditOnCreate(resObj, true)).Should(Succeed())
		})

		It("test create with dry-run", func() {
			options.Format = printer.YAML
			testCases := []struct {
				clusterName    string
				isUseDryRun    bool
				mode           string
				dryRunStrategy DryRunStrategy
				success        bool
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
				By(fmt.Sprintf("when isDryRun %v, dryRunStrategy %v, mode %s",
					t.isUseDryRun, t.dryRunStrategy, t.mode))
				options.Name = t.clusterName
				if t.isUseDryRun {
					options.DryRun = t.mode
				}
				Expect(options.Complete()).Should(Succeed())

				s, _ := options.GetDryRunStrategy()
				if t.success {
					Expect(s == t.dryRunStrategy).Should(BeTrue())
					Expect(options.Run()).Should(Succeed())
				} else {
					Expect(s).ShouldNot(Equal(t.dryRunStrategy))
				}
			}
		})
	})
})
