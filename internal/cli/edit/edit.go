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

package edit

import (
	"fmt"

	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/cmd/util/editor"

	"github.com/apecloud/kubeblocks/internal/cli/util"
)

type EditOptions struct {
	editor.EditOptions
	Factory cmdutil.Factory
	// Name are the resource name
	Name string
	GVR  schema.GroupVersionResource
}

func NewEditOptions(f cmdutil.Factory, streams genericclioptions.IOStreams,
	gvr schema.GroupVersionResource) *EditOptions {
	return &EditOptions{
		Factory:     f,
		GVR:         gvr,
		EditOptions: *editor.NewEditOptions(editor.NormalEditMode, streams),
	}
}

func (o *EditOptions) AddFlags(cmd *cobra.Command) {
	// bind flag structs
	o.RecordFlags.AddFlags(cmd)
	o.PrintFlags.AddFlags(cmd)

	usage := "to use to edit the resource"
	cmdutil.AddFilenameOptionFlags(cmd, &o.FilenameOptions, usage)
	cmdutil.AddValidateFlags(cmd)
	cmd.Flags().BoolVarP(&o.OutputPatch, "output-patch", "", o.OutputPatch, "Output the patch if the resource is edited.")
	cmd.Flags().BoolVar(&o.WindowsLineEndings, "windows-line-endings", o.WindowsLineEndings,
		"Defaults to the line ending native to your platform.")
	cmdutil.AddFieldManagerFlagVar(cmd, &o.FieldManager, "kubectl-edit")
	cmdutil.AddApplyAnnotationVarFlags(cmd, &o.ApplyAnnotation)
	cmdutil.AddSubresourceFlags(cmd, &o.Subresource, "If specified, edit will operate on the subresource of the requested object.", editor.SupportedSubresources...)
}

func (o *EditOptions) Complete(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("missing the name")
	}
	if len(args) > 0 {
		o.Name = args[0]
	}
	return o.EditOptions.Complete(o.Factory, []string{util.GVRToString(o.GVR), o.Name}, cmd)
}
