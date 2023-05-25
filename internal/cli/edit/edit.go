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
