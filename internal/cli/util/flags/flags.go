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

package flags

import (
	"strings"

	"github.com/spf13/cobra"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	utilcomp "k8s.io/kubectl/pkg/util/completion"

	"github.com/apecloud/kubeblocks/internal/cli/cluster"
	"github.com/apecloud/kubeblocks/internal/cli/types"
	"github.com/apecloud/kubeblocks/internal/cli/util"
)

// AddClusterDefinitionFlag adds a flag "cluster-definition" for the cmd and stores the value of the flag
// in string p
func AddClusterDefinitionFlag(f cmdutil.Factory, cmd *cobra.Command, p *string) {
	cmd.Flags().StringVar(p, "cluster-definition", *p, "Specify cluster definition, run \"kbcli clusterdefinition list\" to show all available cluster definition")
	util.CheckErr(cmd.RegisterFlagCompletionFunc("cluster-definition",
		func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			return utilcomp.CompGetResource(f, cmd, util.GVRToString(types.ClusterDefGVR()), toComplete), cobra.ShellCompDirectiveNoFileComp
		}))
}

func AddComponentsFlag(f cmdutil.Factory, cmd *cobra.Command, isPlural bool, p any, usage string) {
	autoComplete := func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		var components []string
		if len(args) == 0 {
			return components, cobra.ShellCompDirectiveNoFileComp
		}
		namespace, _, _ := f.ToRawKubeConfigLoader().Namespace()
		dynamic, _ := f.DynamicClient()
		cluster, _ := cluster.GetClusterByName(dynamic, args[0], namespace)
		for _, comp := range cluster.Spec.ComponentSpecs {
			if strings.HasPrefix(comp.Name, toComplete) {
				components = append(components, comp.Name)
			}
		}
		return components, cobra.ShellCompDirectiveNoFileComp
	}

	if isPlural {
		cmd.Flags().StringSliceVar(p.(*[]string), "components", nil, usage)
		util.CheckErr(cmd.RegisterFlagCompletionFunc("components", autoComplete))
	} else {
		cmd.Flags().StringVar(p.(*string), "component", "", usage)
		util.CheckErr(cmd.RegisterFlagCompletionFunc("component", autoComplete))
	}
}
