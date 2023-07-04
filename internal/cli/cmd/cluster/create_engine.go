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
	"strings"

	"github.com/spf13/cobra"
	"helm.sh/helm/v3/pkg/chart"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"

	"github.com/apecloud/kubeblocks/internal/cli/cluster"
	"github.com/apecloud/kubeblocks/internal/cli/create"
	"github.com/apecloud/kubeblocks/internal/cli/edit"
	"github.com/apecloud/kubeblocks/internal/cli/types"
	"github.com/apecloud/kubeblocks/internal/cli/util"
)

type objectInfo struct {
	gvr schema.GroupVersionResource
	obj *unstructured.Unstructured
}

type CreateEngineOptions struct {
	// engine is the type of the engine to create.
	engine cluster.EngineType

	// values is used to render the cluster helm chart.
	values map[string]interface{}

	// schema is the cluster helm chart schema, used to render the command flag
	// and validate the values.
	schema *cluster.EngineSchema

	// chart is the cluster helm chart.
	chart *chart.Chart

	// clusterDef is the engine cluster definition.
	clusterDef string

	*create.CreateOptions
}

func newEngineOptions(createOptions *create.CreateOptions, e cluster.EngineType) (*CreateEngineOptions, error) {
	var err error
	o := &CreateEngineOptions{
		CreateOptions: createOptions,
		engine:        e,
	}

	if o.chart, err = cluster.GetHelmChart(e); err != nil {
		return nil, err
	}

	if o.schema, err = cluster.GetEngineSchema(o.chart); err != nil {
		return nil, err
	}

	if o.clusterDef, err = cluster.GetEngineClusterDef(o.chart); err != nil {
		return nil, err
	}
	return o, nil
}

func buildCreateEngineCmds(createOptions *create.CreateOptions) []*cobra.Command {
	var cmds []*cobra.Command

	for _, e := range cluster.SupportedEngines() {
		o, err := newEngineOptions(createOptions, e)
		util.CheckErr(err)

		cmd := &cobra.Command{
			Use:     strings.ToLower(e.String()) + " NAME",
			Short:   fmt.Sprintf("Create a %s cluster.", e),
			Example: buildEngineCreateExamples(e),
			Run: func(cmd *cobra.Command, args []string) {
				o.Args = args
				cmdutil.CheckErr(o.CreateOptions.Complete())
				cmdutil.CheckErr(o.complete(cmd, args))
				cmdutil.CheckErr(o.validate())
				cmdutil.CheckErr(o.run())
			},
		}

		util.CheckErr(addEngineFlags(cmd, o.Factory, o.schema))
		cmds = append(cmds, cmd)
	}
	return cmds
}

func (o *CreateEngineOptions) complete(cmd *cobra.Command, args []string) error {
	var err error

	// if name is not specified, generate a random cluster name
	if o.Name == "" {
		o.Name, err = generateClusterName(o.Dynamic, o.Namespace)
		if err != nil {
			return err
		}
	}

	// get values from flags
	o.values = getValuesFromFlags(cmd.LocalNonPersistentFlags())
	return nil
}

func (o *CreateEngineOptions) validate() error {
	if err := o.validateVersion(); err != nil {
		return err
	}
	return cluster.ValidateValues(o.schema, o.values)
}

func (o *CreateEngineOptions) run() error {
	// move values that belong to sub chart to sub map
	values := buildHelmValues(o.schema, o.values)

	// get cluster manifests
	manifests, err := cluster.GetManifests(o.chart, o.Namespace, o.Name, values)
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
		return nil, fmt.Errorf("failed to find cluster object from manifests rendered from engine %s template", o.engine)
	}

	// only edits the cluster object, other dependencies object is not allowed to edit
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

		// for dry-run, only output cluster resource
		if !isCluster {
			continue
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

func (o *CreateEngineOptions) validateVersion() error {
	var err error

	cv := o.values[cluster.VersionSchemaProp.String()].(string)
	if cv != "" {
		if err = cluster.ValidateClusterVersion(o.Dynamic, o.clusterDef, cv); err != nil {
			return fmt.Errorf("cluster version \"%s\" does not exist, run following command to get the available cluster versions\n\tkbcli cv list --cluster-definition=%s",
				o.clusterDef, cv)
		}
		return nil
	}

	cv, err = cluster.GetDefaultVersion(o.Dynamic, o.clusterDef)
	if err != nil {
		return err
	}
	// set cluster version
	o.values[cluster.VersionSchemaProp.String()] = cv

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
