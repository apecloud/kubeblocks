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

package builder

import (
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"

	"github.com/apecloud/kubeblocks/internal/cli/types"
	"github.com/apecloud/kubeblocks/internal/cli/util"
)

type testOptions struct {
	a string
}

var _ = Describe("builder", func() {
	It("build command", func() {
		cmd := NewCmdBuilder().
			IOStreams(genericclioptions.NewTestIOStreamsDiscard()).
			Factory(nil).
			Use("test").
			Short("test command short description").
			Example("test command examples").
			Options(&testOptions{}).
			CustomComplete(customCompleteFn).
			CustomFlags(customFlags).
			CustomRun(customRunFn).
			Example("test command example").
			GVR(types.ClusterGVR()).Build(buildFn)

		Expect(cmd).ShouldNot(BeNil())
		Expect(cmd.Use).Should(Equal("test"))
		Expect(cmd.Flags().Lookup("a").Value.String()).Should(Equal("a"))
	})
})

func buildFn(c *Command) *cobra.Command {
	cmd := &cobra.Command{
		Use:     c.Use,
		Short:   c.Short,
		Example: c.Example,
		Run: func(cmd *cobra.Command, args []string) {
			var (
				goon = true
				err  error
			)
			if c.CustomComplete != nil {
				util.CheckErr(c.CustomComplete(c))
			}

			if c.CustomRun != nil {
				goon, err = c.CustomRun(c)
			}

			if goon && err == nil {
				fmt.Fprint(c.Out, "run")
			}
		},
	}

	c.Cmd = cmd
	if c.CustomFlags != nil {
		c.CustomFlags(c)
	}
	return cmd
}

func customCompleteFn(c *Command) error {
	o := c.Options.(*testOptions)
	if len(o.a) == 0 {
		o.a = "auto complete"
	}
	return nil
}

func customFlags(c *Command) {
	o := c.Options.(*testOptions)
	c.Cmd.Flags().StringVar(&o.a, "a", "a", "a test flag")
}

func customRunFn(c *Command) (bool, error) {
	o := c.Options.(*testOptions)
	fmt.Fprint(c.Out, o.a)
	return true, nil
}
