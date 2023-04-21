/*
Copyright (C) 2022 ApeCloud Co., Ltd

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

package util

import (
	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/runtime/schema"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	utilcomp "k8s.io/kubectl/pkg/util/completion"
)

func ResourceNameCompletionFunc(f cmdutil.Factory, gvr schema.GroupVersionResource) func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
	return func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		comps := utilcomp.CompGetResource(f, cmd, GVRToString(gvr), toComplete)
		seen := make(map[string]bool)

		var availableComps []string
		for _, arg := range args {
			seen[arg] = true
		}
		for _, comp := range comps {
			if !seen[comp] {
				availableComps = append(availableComps, comp)
			}
		}
		return availableComps, cobra.ShellCompDirectiveNoFileComp
	}
}
