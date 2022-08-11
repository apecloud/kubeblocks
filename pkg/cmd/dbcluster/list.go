/*
Copyright © 2022 The OpenCli Authors

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
	"context"
	"fmt"

	"github.com/gosuri/uitable"
	"github.com/spf13/cobra"
	"helm.sh/helm/v3/pkg/cli/output"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/resource"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/describe"
	ctrlcli "sigs.k8s.io/controller-runtime/pkg/client"

	"jihulab.com/infracreate/dbaas-system/opencli/pkg/cmd/playground"
	"jihulab.com/infracreate/dbaas-system/opencli/pkg/types"
	"jihulab.com/infracreate/dbaas-system/opencli/pkg/utils"
)

type ListOptions struct {
	Namespace string

	Describer  func(*meta.RESTMapping) (describe.ResourceDescriber, error)
	NewBuilder func() *resource.Builder

	BuilderArgs []string

	EnforceNamespace bool
	AllNamespaces    bool

	DescriberSettings *describe.DescriberSettings
	FilenameOptions   *resource.FilenameOptions

	client ctrlcli.Client
	genericclioptions.IOStreams
}

func getClusterGVK() schema.GroupVersionKind {
	return schema.GroupVersionKind{
		Group:   "mysql.oracle.com",
		Version: "v2",
		Kind:    "InnoDBCluster",
	}
}

func NewListCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := &ListOptions{
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
			cmdutil.CheckErr(o.Complete(f, args))
			cmdutil.CheckErr(o.Run())
		},
	}

	return cmd
}

func (o *ListOptions) Complete(f cmdutil.Factory, args []string) error {
	var err error
	o.Namespace, o.EnforceNamespace, err = f.ToRawKubeConfigLoader().Namespace()
	if err != nil {
		return err
	}

	if o.AllNamespaces {
		o.EnforceNamespace = false
	}

	o.BuilderArgs = append([]string{types.PlaygroundSourceName}, args...)

	o.Describer = func(mapping *meta.RESTMapping) (describe.ResourceDescriber, error) {
		return describe.DescriberFn(f, mapping)
	}

	// used to fetch the resource
	config, err := f.ToRESTConfig()
	if err != nil {
		return err
	}

	c, err := ctrlcli.New(config, ctrlcli.Options{})
	if err != nil {
		return err
	}
	o.client = c
	o.NewBuilder = f.NewBuilder

	return nil
}

func (o *ListOptions) Run() error {

	ctx := context.Background()
	ul := &unstructured.UnstructuredList{}
	ul.SetGroupVersionKind(getClusterGVK())

	// TODO: need to apply  MatchingLabels
	//ml := ctrlcli.MatchingLabels()
	if err := o.client.List(ctx, ul, ctrlcli.InNamespace("default")); err != nil {
		return err
	}

	table := uitable.New()
	table.AddRow("NAMESPACE", "NAME", "INSTANCES", "ONLINE", "STATUS", "DESCRIPTION", "TYPE", "LABEL")
	for _, dbCluster := range ul.Items {
		clusterInfo := utils.DBClusterInfo{
			RootUser: playground.DefaultRootUser,
			DBPort:   playground.DefaultPort,
		}

		clusterInfo.DBNamespace = dbCluster.GetNamespace()
		clusterInfo.DBCluster = dbCluster.GetName()
		buildClusterInfo(&dbCluster, &clusterInfo)
		table.AddRow(clusterInfo.DBNamespace, clusterInfo.DBCluster, clusterInfo.Instances, clusterInfo.OnlineInstances,
			clusterInfo.Status, "Example MySQL", fmt.Sprintf("%s %s", clusterInfo.Engine, clusterInfo.Topology), clusterInfo.Labels)
	}

	if err := output.EncodeTable(o.Out, table); err != nil {
		return err
	}
	if len(ul.Items) == 0 {
		// if we wrote no output, and had no errors, be sure we output something.
		if o.AllNamespaces {
			fmt.Fprintln(o.ErrOut, "No resources found")
		} else {
			fmt.Fprintf(o.ErrOut, "No resources found in %s namespace.\n", o.Namespace)
		}
	}
	return nil
}
