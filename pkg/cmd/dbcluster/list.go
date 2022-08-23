/*
Copyright © 2022 The dbctl Authors

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

package dbcluster

import (
	"fmt"

	"github.com/gosuri/uitable"
	"github.com/spf13/cobra"
	"helm.sh/helm/v3/pkg/cli/output"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/resource"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/describe"

	"jihulab.com/infracreate/dbaas-system/dbctl/pkg/utils"
)

func getClusterGVK() schema.GroupVersionKind {
	return schema.GroupVersionKind{
		Group:   "mysql.oracle.com",
		Version: "v2",
		Kind:    "InnoDBCluster",
	}
}

func NewListCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := &commandOptions{
		FilenameOptions: &resource.FilenameOptions{},
		DescriberSettings: &describe.DescriberSettings{
			ShowEvents: true,
		},

		IOStreams: streams,
	}

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all database cluster",
		Run: func(cmd *cobra.Command, args []string) {
			cmdutil.CheckErr(o.setup(f, args))
			table := uitable.New()
			table.AddRow("NAMESPACE", "NAME", "INSTANCES", "ONLINE", "STATUS", "DESCRIPTION", "TYPE", "LABEL")
			cmdutil.CheckErr(o.run(
				func(clusterInfo *utils.DBClusterInfo) {
					table.AddRow(clusterInfo.DBNamespace, clusterInfo.DBCluster, clusterInfo.Instances, clusterInfo.OnlineInstances,
						clusterInfo.Status, "Example MySQL", fmt.Sprintf("%s %s", clusterInfo.Engine, clusterInfo.Topology), clusterInfo.Labels)
				}, func() error {
					if err := output.EncodeTable(o.Out, table); err != nil {
						return err
					}
					return nil
				}))
		},
	}

	return cmd
}
