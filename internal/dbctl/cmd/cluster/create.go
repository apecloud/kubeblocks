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
	"encoding/json"
	"fmt"
	"os"

	"github.com/ghodss/yaml"
	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"

	"github.com/apecloud/kubeblocks/internal/dbctl/cmd/create"
	"github.com/apecloud/kubeblocks/internal/dbctl/types"
)

const (
	defaultClusterDef      = "wesql-clusterdefinition"
	defaultAppVersion      = "wesql-appversion-8.0.29"
	clusterCueTemplateName = "cluster_template.cue"
)

type CreateOptions struct {
	// ClusterDefRef reference clusterDefinition
	ClusterDefRef     string                   `json:"clusterDefRef"`
	AppVersionRef     string                   `json:"appVersionRef"`
	TerminationPolicy string                   `json:"terminationPolicy"`
	PodAntiAffinity   string                   `json:"podAntiAffinity"`
	TopologyKeys      []string                 `json:"topologyKeys,omitempty"`
	NodeLabels        map[string]string        `json:"nodeLabels,omitempty"`
	Components        []map[string]interface{} `json:"components"`
	// ComponentsFilePath components file path
	ComponentsFilePath string
	create.BaseOptions
}

func (o *CreateOptions) Validate() error {
	if o.Name == "" {
		return fmt.Errorf("missing cluster name")
	}
	return nil
}

// CovertComponents get content from componentsFilePath and covert to components
func (o *CreateOptions) CovertComponents() error {
	var (
		componentByte []byte
		err           error
		components    = make([]map[string]interface{}, 0)
	)
	if len(o.ComponentsFilePath) > 0 {
		if componentByte, err = os.ReadFile(o.ComponentsFilePath); err != nil {
			return err
		}
		if componentByte, err = yaml.YAMLToJSON(componentByte); err != nil {
			return err
		}
		if err = json.Unmarshal(componentByte, &components); err != nil {
			return err
		}
	}
	o.Components = components
	return nil
}

func NewCreateCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := &CreateOptions{BaseOptions: create.BaseOptions{IOStreams: streams}}
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a database cluster",
		Run: func(cmd *cobra.Command, args []string) {
			inputs := create.Inputs{
				CueTemplateName:    clusterCueTemplateName,
				ResourceName:       types.ResourceClusters,
				Options:            o,
				Factory:            f,
				ValidateFunc:       o.Validate,
				OptionsConvertFunc: o.CovertComponents,
			}
			cmdutil.CheckErr(o.Run(inputs, args))
		},
	}

	cmd.Flags().StringVar(&o.ClusterDefRef, "cluster-definition", defaultClusterDef, "ClusterDefinition reference")
	cmd.Flags().StringVar(&o.AppVersionRef, "app-version", defaultAppVersion, "AppVersion reference")
	cmd.Flags().StringVar(&o.TerminationPolicy, "termination-policy", "Halt", "Termination policy")
	cmd.Flags().StringVar(&o.PodAntiAffinity, "pod-anti-affinity", "Preferred", "Pod anti-affinity type")
	cmd.Flags().StringArrayVar(&o.TopologyKeys, "topology-keys", nil, "Topology keys for affinity")
	cmd.Flags().StringToStringVar(&o.NodeLabels, "node-labels", nil, "Node label selector")
	cmd.Flags().StringVar(&o.ComponentsFilePath, "components", "", "Use yaml file to specify the cluster components")

	return cmd
}
