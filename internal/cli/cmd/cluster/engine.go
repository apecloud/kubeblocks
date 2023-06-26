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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/kube-openapi/pkg/validation/spec"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"sigs.k8s.io/yaml"

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

type EngineCreateOptions struct {
	// engine is the type of the engine to create.
	engine cluster.EngineType

	// values is used to render the cluster helm chart.
	values map[string]interface{}

	// schema is the cluster helm chart schema, used to render the command flag
	// and validate the values.
	schema *spec.Schema

	*create.CreateOptions
}

func BuildEngineCmds(createOptions *create.CreateOptions) []*cobra.Command {
	var (
		err  error
		cmds []*cobra.Command
	)

	for _, e := range cluster.SupportedEngines() {
		o := &EngineCreateOptions{
			CreateOptions: createOptions,
			engine:        e,
		}

		// get engine schema
		o.schema, err = cluster.GetEngineSchema(e)
		util.CheckErr(err)

		cmd := &cobra.Command{
			Use:   strings.ToLower(e.String()) + " NAME",
			Short: fmt.Sprintf("Create a %s cluster.", e),
			Run: func(cmd *cobra.Command, args []string) {
				o.Args = args
				cmdutil.CheckErr(o.CreateOptions.Complete())
				cmdutil.CheckErr(o.Complete(cmd, args))
				cmdutil.CheckErr(o.Validate())
				cmdutil.CheckErr(o.Run())
			},
		}

		util.CheckErr(addEngineFlags(cmd, o.Factory, o.schema))

		cmds = append(cmds, cmd)
	}
	return cmds
}

func (o *EngineCreateOptions) Complete(cmd *cobra.Command, args []string) error {
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

	// set cluster name
	o.values[cluster.NameSchemaProp.String()] = o.Name
	return nil
}

func (o *EngineCreateOptions) Validate() error {
	return cluster.ValidateValues(o.schema, o.values)
}

func (o *EngineCreateOptions) Run() error {
	// get cluster manifests
	manifests, err := cluster.GetManifests(o.engine, o.Namespace, o.Name, o.values)
	if err != nil {
		return err
	}

	// get objects to be created from manifests
	objs, err := getObjectsInfo(o.Factory, manifests)
	if err != nil {
		return err
	}

	getClusterObj := func() *unstructured.Unstructured {
		for _, obj := range objs {
			if obj.gvr == types.ClusterGVR() {
				return obj.obj
			}
		}
		return nil
	}

	// only edits the cluster object, other dependencies object is not allowed to edit
	if o.EditBeforeCreate {
		clusterObj := getClusterObj()
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

		// for dryrun, only output cluster resource
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

func getObjectsInfo(f cmdutil.Factory, manifests map[string]string) ([]*objectInfo, error) {
	mapper, err := f.ToRESTMapper()
	if err != nil {
		return nil, err
	}

	var objects []*objectInfo
	for _, manifest := range manifests {
		objInfo := &objectInfo{}

		// convert yaml to json
		jsonData, err := yaml.YAMLToJSON([]byte(manifest))
		if err != nil {
			return nil, err
		}

		// get resource gvk
		obj, gvk, err := unstructured.UnstructuredJSONScheme.Decode(jsonData, nil, nil)
		if err != nil {
			return nil, err
		}

		// convert gvk to gvr
		m, err := mapper.RESTMapping(gvk.GroupKind(), gvk.Version)
		if err != nil {
			return nil, err
		}

		objInfo.obj = obj.(*unstructured.Unstructured)
		objInfo.gvr = m.Resource
		objects = append(objects, objInfo)
	}
	return objects, nil
}
