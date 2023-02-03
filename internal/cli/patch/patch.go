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

package patch

import (
	"fmt"

	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	cmdpatch "k8s.io/kubectl/pkg/cmd/patch"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"

	"github.com/apecloud/kubeblocks/internal/cli/util"
)

type Options struct {
	Factory cmdutil.Factory

	// resource names
	Names []string
	GVR   schema.GroupVersionResource

	*cmdpatch.PatchOptions
}

func NewOptions(f cmdutil.Factory, streams genericclioptions.IOStreams, gvr schema.GroupVersionResource) *Options {
	return &Options{
		Factory:      f,
		GVR:          gvr,
		PatchOptions: cmdpatch.NewPatchOptions(streams),
	}
}

func (o *Options) AddFlags(cmd *cobra.Command) {
	o.PrintFlags.AddFlags(cmd)
	cmdutil.AddDryRunFlag(cmd)
}

func (o *Options) complete(cmd *cobra.Command) error {
	if len(o.Names) == 0 {
		return fmt.Errorf("missing %s name", o.GVR.Resource)
	}

	// for CRD, we always use Merge patch type
	o.PatchType = "merge"
	args := append([]string{util.GVRToString(o.GVR)}, o.Names...)
	if err := o.Complete(o.Factory, cmd, args); err != nil {
		return err
	}
	return nil
}

func (o *Options) Run(cmd *cobra.Command) error {
	if err := o.complete(cmd); err != nil {
		return err
	}

	if len(o.Patch) == 0 {
		return fmt.Errorf("the contents of the patch is empty")
	}

	return o.RunPatch()
}
