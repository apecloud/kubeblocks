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

package patch

import (
	"fmt"

	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	cmdpatch "k8s.io/kubectl/pkg/cmd/patch"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	utilcomp "k8s.io/kubectl/pkg/util/completion"

	"github.com/apecloud/kubeblocks/internal/cli/builder"
	"github.com/apecloud/kubeblocks/internal/cli/util"
)

type Options struct {
	*cmdpatch.PatchOptions
}

func NewOptions(streams genericclioptions.IOStreams) *Options {
	return &Options{PatchOptions: cmdpatch.NewPatchOptions(streams)}
}

func (o *Options) Build(c *builder.Command) *cobra.Command {
	cmd := &cobra.Command{
		Use:               c.Use,
		Short:             c.Short,
		Example:           c.Example,
		ValidArgsFunction: utilcomp.ResourceNameCompletionFunc(c.Factory, util.GVRToString(c.GVR)),
		Run: func(cmd *cobra.Command, args []string) {
			c.Args = args
			util.CheckErr(o.complete(c))
			util.CheckErr(o.validate())
			util.CheckErr(o.run(c))
		},
	}

	c.Cmd = cmd
	o.addFlags(c)
	return cmd
}

func (o *Options) addFlags(c *builder.Command) {
	o.PrintFlags.AddFlags(c.Cmd)
	cmdutil.AddDryRunFlag(c.Cmd)
	if c.CustomFlags != nil {
		c.CustomFlags(c)
	}
}

func (o *Options) complete(c *builder.Command) error {
	if len(c.Args) == 0 {
		return fmt.Errorf("missing %s name", c.GVR.Resource)
	}

	// for CRD, we always use Merge patch type
	o.PatchType = "merge"
	args := append([]string{util.GVRToString(c.GVR)}, c.Args...)
	if err := o.Complete(c.Factory, c.Cmd, args); err != nil {
		return err
	}

	if c.CustomComplete != nil {
		return c.CustomComplete(c)
	}
	return nil
}

func (o *Options) validate() error {
	if len(o.Patch) == 0 {
		return fmt.Errorf("the contents of the patch is empty")
	}
	return nil
}

func (o *Options) run(c *builder.Command) error {
	var (
		goon = true
		err  error
	)
	if c.CustomRun != nil {
		goon, err = c.CustomRun(c)
	}
	if goon && err == nil {
		return o.RunPatch()
	}
	return err
}
