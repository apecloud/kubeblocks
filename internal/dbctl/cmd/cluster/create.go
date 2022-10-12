/*
Copyright 2022 The KubeBlocks Authors

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

package cluster

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/ghodss/yaml"
	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/dynamic"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
)

var defaultClusterDef = "wesql-clusterdefinition"
var defaultAppVersion = "wesql-appversion-8.0.29"

type CreateOptions struct {
	Namespace         string
	Name              string
	ClusterDefRef     string
	AppVersionRef     string
	TerminationPolicy string
	Components        string

	client dynamic.Interface
	genericclioptions.IOStreams
}

func NewCreateCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := &CreateOptions{IOStreams: streams}

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a database cluster",
		Run: func(cmd *cobra.Command, args []string) {
			cmdutil.CheckErr(o.Validate(args))
			cmdutil.CheckErr(o.Complete(f, args))
			cmdutil.CheckErr(o.Run())
		},
	}

	cmd.Flags().StringVar(&o.ClusterDefRef, "cluster-definition", defaultClusterDef, "ClusterDefinition reference")
	cmd.Flags().StringVar(&o.AppVersionRef, "app-version", defaultAppVersion, "AppVersion reference")
	cmd.Flags().StringVar(&o.TerminationPolicy, "termination-policy", "Halt", "Termination policy")
	cmd.Flags().StringVar(&o.Components, "components", "", "Use yaml file to specify the cluster components")

	return cmd
}

func (o *CreateOptions) Validate(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("missing cluster name")
	}
	if len(o.ClusterDefRef) == 0 {
		return fmt.Errorf("cluster-definition can not be empty")
	}
	if len(o.AppVersionRef) == 0 {
		return fmt.Errorf("app-version can not be empty")
	}
	return nil
}

func (o *CreateOptions) Complete(f cmdutil.Factory, args []string) error {
	var err error
	o.Namespace, _, err = f.ToRawKubeConfigLoader().Namespace()
	if err != nil {
		return err
	}

	if len(args) > 0 {
		o.Name = args[0]
	}

	// used to fetch the resource
	config, err := f.ToRESTConfig()
	if err != nil {
		return nil
	}

	client, err := dynamic.NewForConfig(config)
	if err != nil {
		return err
	}

	o.client = client

	return nil
}

func (o *CreateOptions) Run() error {
	clusterObj := unstructured.Unstructured{}
	components := "[]"
	if len(o.Components) > 0 {
		yamlByte, err := os.ReadFile(o.Components)
		if err != nil {
			return err
		}
		jsonByte, err := yaml.YAMLToJSON(yamlByte)
		if err != nil {
			return err
		}
		components = string(jsonByte)
	}
	clusterJsonByte := []byte(fmt.Sprintf(`
{
  "apiVersion": "dbaas.infracreate.com/v1alpha1",
  "kind": "Cluster",
  "metadata": {
    "name": "%s",
    "namespace": "%s"
  },
  "spec": {
    "clusterDefinitionRef": "%s",
    "appVersionRef": "%s",
    "components": %s
  }
}
`, o.Name, o.Namespace, o.ClusterDefRef, o.AppVersionRef, components))
	if err := json.Unmarshal(clusterJsonByte, &clusterObj); err != nil {
		return err
	}
	gvr := schema.GroupVersionResource{Group: "dbaas.infracreate.com", Version: "v1alpha1", Resource: "clusters"}
	_, err := o.client.Resource(gvr).Namespace(o.Namespace).Create(context.TODO(), &clusterObj, metav1.CreateOptions{})
	if err != nil {
		return err
	}
	return nil
}
