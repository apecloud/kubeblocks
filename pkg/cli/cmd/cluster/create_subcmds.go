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

package cluster

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"

	"github.com/apecloud/kubeblocks/pkg/cli/cluster"
	"github.com/apecloud/kubeblocks/pkg/cli/create"
	"github.com/apecloud/kubeblocks/pkg/cli/edit"
	"github.com/apecloud/kubeblocks/pkg/cli/types"
	"github.com/apecloud/kubeblocks/pkg/cli/util"
)

type objectInfo struct {
	gvr schema.GroupVersionResource
	obj *unstructured.Unstructured
}

type CreateSubCmdsOptions struct {
	// clusterType is the type of the cluster to create.
	clusterType cluster.ClusterType

	// Values is used to render the cluster helm chartInfo.
	Values map[string]interface{}

	// chartInfo is the cluster chart information, used to render the command flag
	// and validate the values.
	chartInfo *cluster.ChartInfo

	*create.CreateOptions
}

func NewSubCmdsOptions(createOptions *create.CreateOptions, t cluster.ClusterType) (*CreateSubCmdsOptions, error) {
	var err error
	o := &CreateSubCmdsOptions{
		CreateOptions: createOptions,
		clusterType:   t,
	}

	if o.chartInfo, err = cluster.BuildChartInfo(t); err != nil {
		return nil, err
	}
	return o, nil
}

func buildCreateSubCmds(createOptions *create.CreateOptions) []*cobra.Command {
	var cmds []*cobra.Command

	for _, t := range cluster.SupportedTypes() {
		o, err := NewSubCmdsOptions(createOptions, t)
		if err != nil {
			fmt.Fprintf(os.Stdout, "Failed add '%s' to 'create' sub command due to %s\n", t.String(), err.Error())
			cluster.ClearCharts(t)
			continue
		}

		cmd := &cobra.Command{
			Use:     t.String() + " NAME",
			Short:   fmt.Sprintf("Create a %s cluster.", t),
			Example: buildCreateSubCmdsExamples(t),
			Run: func(cmd *cobra.Command, args []string) {
				o.Args = args
				cmdutil.CheckErr(o.CreateOptions.Complete())
				cmdutil.CheckErr(o.complete(cmd))
				cmdutil.CheckErr(o.validate())
				cmdutil.CheckErr(o.Run())
			},
		}

		if o.chartInfo.Alias != "" {
			cmd.Aliases = []string{o.chartInfo.Alias}
		}

		util.CheckErr(addCreateFlags(cmd, o.Factory, o.chartInfo))
		cmds = append(cmds, cmd)
	}
	return cmds
}

func (o *CreateSubCmdsOptions) complete(cmd *cobra.Command) error {
	var err error

	// if name is not specified, generate a random cluster name
	if o.Name == "" {
		o.Name, err = generateClusterName(o.Dynamic, o.Namespace)
		if err != nil {
			return err
		}
	}

	// get values from flags
	o.Values = getValuesFromFlags(cmd.LocalNonPersistentFlags())
	return nil
}

func (o *CreateSubCmdsOptions) validate() error {
	if err := o.validateVersion(); err != nil {
		return err
	}
	return cluster.ValidateValues(o.chartInfo, o.Values)
}

func (o *CreateSubCmdsOptions) Run() error {
	// move values that belong to sub chart to sub map
	values := buildHelmValues(o.chartInfo, o.Values)

	// get Kubernetes version
	kubeVersion, err := util.GetK8sVersion(o.Client.Discovery())
	if err != nil || kubeVersion == "" {
		return fmt.Errorf("failed to get Kubernetes version %v", err)
	}

	// get cluster manifests
	manifests, err := cluster.GetManifests(o.chartInfo.Chart, o.Namespace, o.Name, kubeVersion, values)
	if err != nil {
		return err
	}

	// get objects to be created from manifests
	objs, err := getObjectsInfo(o.Factory, manifests)
	if err != nil {
		return err
	}

	getClusterObj := func() (*unstructured.Unstructured, error) {
		for _, obj := range objs {
			if obj.gvr == types.ClusterGVR() {
				return obj.obj, nil
			}
		}
		return nil, fmt.Errorf("failed to find cluster object from manifests rendered from %s chart", o.clusterType)
	}

	// only edits the cluster object, other dependency objects are created directly
	if o.EditBeforeCreate {
		clusterObj, err := getClusterObj()
		if err != nil {
			return err
		}
		customEdit := edit.NewCustomEditOptions(o.Factory, o.IOStreams, "create")
		if err = customEdit.Run(clusterObj); err != nil {
			return err
		}
	}

	dryRun, err := o.GetDryRunStrategy()
	if err != nil {
		return err
	}

	// create cluster and dependency resources
	for _, obj := range objs {
		isCluster := obj.gvr == types.ClusterGVR()
		resObj := obj.obj

		if dryRun != create.DryRunClient {
			createOptions := metav1.CreateOptions{}
			if dryRun == create.DryRunServer {
				createOptions.DryRun = []string{metav1.DryRunAll}
			}

			// create resource
			resObj, err = o.Dynamic.Resource(obj.gvr).Namespace(o.Namespace).Create(context.TODO(), resObj, createOptions)
			if err != nil {
				return err
			}

			// only output cluster resource
			if dryRun != create.DryRunServer && isCluster {
				if o.Quiet {
					continue
				}
				if o.CustomOutPut != nil {
					o.CustomOutPut(o.CreateOptions)
				}
				fmt.Fprintf(o.Out, "%s %s created\n", resObj.GetKind(), resObj.GetName())
				continue
			}
		}

		if len(objs) > 1 {
			fmt.Fprintf(o.Out, "---\n")
		}

		p, err := o.ToPrinter(nil, false)
		if err != nil {
			return err
		}

		if err = p.PrintObj(resObj, o.Out); err != nil {
			return err
		}
	}
	return nil
}

func (o *CreateSubCmdsOptions) validateVersion() error {
	var err error
	cv, ok := o.Values[cluster.VersionSchemaProp.String()].(string)
	if ok && cv != "" {
		if err = cluster.ValidateClusterVersion(o.Dynamic, o.chartInfo.ClusterDef, cv); err != nil {
			return fmt.Errorf("cluster version \"%s\" does not exist, run following command to get the available cluster versions\n\tkbcli cv list --cluster-definition=%s",
				cv, o.chartInfo.ClusterDef)
		}
		return nil
	}

	cv, err = cluster.GetDefaultVersion(o.Dynamic, o.chartInfo.ClusterDef)
	if err != nil {
		return err
	}
	// set cluster version
	o.Values[cluster.VersionSchemaProp.String()] = cv

	dryRun, err := o.GetDryRunStrategy()
	if err != nil {
		return err
	}
	// if dryRun is set, run in quiet mode, avoid to output yaml file with the info
	if dryRun != create.DryRunNone {
		return nil
	}

	fmt.Fprintf(o.Out, "Info: --version is not specified, %s is applied by default.\n", cv)
	return nil
}
