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

package clusterversion

import (
	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/util/templates"

	"github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/cli/list"
	"github.com/apecloud/kubeblocks/internal/cli/printer"
	"github.com/apecloud/kubeblocks/internal/cli/types"
	"github.com/apecloud/kubeblocks/internal/cli/util"
	"github.com/apecloud/kubeblocks/internal/cli/util/flags"
	"github.com/apecloud/kubeblocks/internal/constant"
)

var listExample = templates.Examples(`
		# list all ClusterVersions
		kbcli clusterversion list`)

type ListClusterVersionOptions struct {
	*list.ListOptions
	clusterDefinitionRef string
}

func NewClusterVersionCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "clusterversion",
		Short:   "ClusterVersion command.",
		Aliases: []string{"cv"},
	}

	cmd.AddCommand(NewListCmd(f, streams))
	cmd.AddCommand(newSetDefaultCMD(f, streams))
	cmd.AddCommand(newUnSetDefaultCMD(f, streams))
	return cmd
}

func NewListCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := &ListClusterVersionOptions{
		ListOptions: list.NewListOptions(f, streams, types.ClusterVersionGVR()),
	}
	cmd := &cobra.Command{
		Use:               "list",
		Short:             "List ClusterVersions.",
		Example:           listExample,
		Aliases:           []string{"ls"},
		ValidArgsFunction: util.ResourceNameCompletionFunc(f, o.GVR),
		Run: func(cmd *cobra.Command, args []string) {
			if len(o.clusterDefinitionRef) != 0 {
				o.LabelSelector = util.BuildClusterDefinitionRefLable(o.LabelSelector, []string{o.clusterDefinitionRef})
			}
			o.Names = args
			util.CheckErr(run(o))
		},
	}
	o.AddFlags(cmd, true)
	flags.AddClusterDefinitionFlag(f, cmd, &o.clusterDefinitionRef)
	return cmd
}

func run(o *ListClusterVersionOptions) error {
	if !o.Format.IsHumanReadable() {
		_, err := o.Run()
		return err
	}
	o.Print = false
	r, err := o.Run()
	if err != nil {
		return err
	}
	infos, err := r.Infos()
	if err != nil {
		return err
	}
	p := printer.NewTablePrinter(o.Out)
	p.SetHeader("NAME", "CLUSTER-DEFINITION", "STATUS", "IS-DEFAULT", "CREATED-TIME")
	p.SortBy(2)
	for _, info := range infos {
		var cv v1alpha1.ClusterVersion
		if err = runtime.DefaultUnstructuredConverter.FromUnstructured(info.Object.(*unstructured.Unstructured).Object, &cv); err != nil {
			return err
		}
		isDefaultValue := isDefault(&cv)
		if isDefaultValue == "true" {
			p.AddRow(printer.BoldGreen(cv.Name), cv.Labels[constant.ClusterDefLabelKey], cv.Status.Phase, isDefaultValue, util.TimeFormat(&cv.CreationTimestamp))
			continue
		}
		p.AddRow(cv.Name, cv.Labels[constant.ClusterDefLabelKey], cv.Status.Phase, isDefaultValue, util.TimeFormat(&cv.CreationTimestamp))
	}
	p.Print()
	return nil
}

func isDefault(cv *v1alpha1.ClusterVersion) string {
	if cv.Annotations == nil {
		return "false"
	}
	if _, ok := cv.Annotations[constant.DefaultClusterVersionAnnotationKey]; !ok {
		return "false"
	}
	return cv.Annotations[constant.DefaultClusterVersionAnnotationKey]
}
