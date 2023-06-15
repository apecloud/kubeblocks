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
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"sigs.k8s.io/yaml"

	"github.com/apecloud/kubeblocks/internal/cli/cluster"
	"github.com/apecloud/kubeblocks/internal/cli/util"
)

type CreateOptionsV1 struct {
	Factory   cmdutil.Factory
	Client    kubernetes.Interface
	Dynamic   dynamic.Interface
	Engine    cluster.EngineType
	Namespace string
	SetFile   string
	Values    map[string]interface{}

	genericclioptions.IOStreams
}

func AddEngineSubCmds(parent *cobra.Command, f cmdutil.Factory, streams genericclioptions.IOStreams) {
	for _, e := range cluster.SupportedEngines() {
		o := &CreateOptionsV1{
			Factory:   f,
			IOStreams: streams,
			Engine:    e,
		}

		cmd := &cobra.Command{
			Use:   strings.ToLower(e.String()),
			Short: fmt.Sprintf("Create a %s cluster.", e.String()),
			Run: func(cmd *cobra.Command, args []string) {
				cmdutil.CheckErr(o.Complete(cmd, args))
				cmdutil.CheckErr(o.Validate())
				cmdutil.CheckErr(o.Run())
			},
		}

		util.CheckErr(addCreateFlags(cmd, e))
		parent.AddCommand(cmd)
	}
}

func (o *CreateOptionsV1) Complete(cmd *cobra.Command, args []string) error {
	var (
		err  error
		name string
	)
	o.Client, err = o.Factory.KubernetesClientSet()
	if err != nil {
		return err
	}

	o.Dynamic, err = o.Factory.DynamicClient()
	if err != nil {
		return err
	}

	o.Namespace, _, err = o.Factory.ToRawKubeConfigLoader().Namespace()
	if err != nil {
		return err
	}

	// if name is not specified, generate a random cluster name
	if len(args) == 0 {
		name, err = generateClusterName(o.Dynamic, o.Namespace)
		if err != nil {
			return err
		}
		if name == "" {
			return fmt.Errorf("failed to generate a random cluster name")
		}
	} else {
		name = args[0]
	}

	// get values from flags
	o.Values = getValuesFromFlags(cmd.LocalNonPersistentFlags())

	// set cluster name
	o.Values[cluster.NameProp.String()] = name

	return nil
}

func (o *CreateOptionsV1) Validate() error {
	if len(o.Values) > 0 && len(o.SetFile) > 0 {
		return fmt.Errorf("does not support --set and --set-file being specified at the same time")
	}

	return cluster.Validate(o.Engine, o.Values)
}

func (o *CreateOptionsV1) Run() error {
	// get cluster manifests
	manifests, err := cluster.GetManifests(o.Engine, o.Namespace, o.Values)
	if err != nil {
		return err
	}

	mapper, err := o.Factory.ToRESTMapper()
	if err != nil {
		return err
	}

	// create cluster and dependency resources
	for _, manifest := range manifests {
		// convert yaml to json
		jsonData, err := yaml.YAMLToJSON([]byte(manifest))
		if err != nil {
			return err
		}

		// get resource gvk
		obj, gvk, err := unstructured.UnstructuredJSONScheme.Decode(jsonData, nil, nil)
		if err != nil {
			return err
		}

		// convert gvk to gvr
		m, err := mapper.RESTMapping(gvk.GroupKind(), gvk.Version)
		if err != nil {
			return err
		}

		// create resource
		_, err = o.Dynamic.Resource(m.Resource).Namespace(o.Namespace).Create(context.TODO(), obj.(*unstructured.Unstructured), metav1.CreateOptions{})
		if err != nil {
			return err
		}
	}
	return nil
}
