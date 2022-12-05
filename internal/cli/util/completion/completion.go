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

package completion

import (
	"github.com/spf13/cobra"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	utilcomp "k8s.io/kubectl/pkg/util/completion"
)

func SetFactoryForCompletion(f cmdutil.Factory) {
	utilcomp.SetFactoryForCompletion(f)
}

// CompGetResource gets the list of the resource specified which begin with `toComplete`.
func CompGetResource(f cmdutil.Factory, cmd *cobra.Command, resourceName string, toComplete string) []string {
	return utilcomp.CompGetResource(f, cmd, resourceName, toComplete)
}

// ListContextsInConfig returns a list of context names which begin with `toComplete`
func ListContextsInConfig(toComplete string) []string {
	return utilcomp.ListContextsInConfig(toComplete)
}

// ListClustersInConfig returns a list of cluster names which begin with `toComplete`
func ListClustersInConfig(toComplete string) []string {
	return utilcomp.ListClustersInConfig(toComplete)
}

// ListUsersInConfig returns a list of usernames which begin with `toComplete`
func ListUsersInConfig(toComplete string) []string {
	return utilcomp.ListUsersInConfig(toComplete)
}

func ResourceNameCompletionFunc(f cmdutil.Factory, resourceType string) func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
	return utilcomp.ResourceNameCompletionFunc(f, resourceType)
}
