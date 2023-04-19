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
