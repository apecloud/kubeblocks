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
	"strconv"

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
	monitorKey             = "monitor"
)

type CreateOptions struct {
	// ClusterDefRef reference clusterDefinition
	ClusterDefRef     string `json:"clusterDefRef"`
	AppVersionRef     string `json:"appVersionRef"`
	TerminationPolicy string `json:"terminationPolicy"`
	PodAntiAffinity   string `json:"podAntiAffinity"`
	Monitor           string                   `json:"monitor"`
	// TopologyKeys if TopologyKeys is nil, add omitempty json tag.
	// because CueLang can not covert null to list.
	TopologyKeys []string                 `json:"topologyKeys,omitempty"`
	NodeLabels   map[string]string        `json:"nodeLabels,omitempty"`
	Components   []map[string]interface{} `json:"components"`
	// ComponentsFilePath components file path
	ComponentsFilePath string `json:"-"`
	create.BaseOptions
}

func setMonitor(monitor string, components []map[string]interface{}) {
	enable, err := strconv.ParseBool(monitor)
	if err != nil {
		return
	}
	for _, component := range components {
		if _, ok := component[monitorKey]; ok {
			component[monitorKey] = enable
		}
	}
}

func (o *CreateOptions) Validate() error {
	if o.Name == "" {
		return fmt.Errorf("missing cluster name")
	}
	return nil
}

func (o *CreateOptions) Complete() error {
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
	if len(o.Monitor) == 0 || len(components) == 0 {
		o.Components = components
		return nil
	}
	setMonitor(o.Monitor, components)
	o.Components = components
	return nil
}

func NewCreateCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := &CreateOptions{BaseOptions: create.BaseOptions{IOStreams: streams}}
	inputs := create.Inputs{
		Use:             "create",
		Short:           "Create a database cluster",
		CueTemplateName: clusterCueTemplateName,
		ResourceName:    types.ResourceClusters,
		BaseOptionsObj:  &o.BaseOptions,
		Options:         o,
		Factory:         f,
		Validate:        o.Validate,
		Complete:        o.Complete,
		BuildFlags: func(cmd *cobra.Command) {
			cmd.Flags().StringVar(&o.ClusterDefRef, "cluster-definition", defaultClusterDef, "ClusterDefinition reference")
			cmd.Flags().StringVar(&o.AppVersionRef, "app-version", defaultAppVersion, "AppVersion reference")
			cmd.Flags().StringVar(&o.TerminationPolicy, "termination-policy", "Halt", "Termination policy")
			cmd.Flags().StringVar(&o.PodAntiAffinity, "pod-anti-affinity", "Preferred", "Pod anti-affinity type")
			cmd.Flags().StringVar(&o.Monitor, "monitor", "", "[true|false], override default monitor config if provided")
			cmd.Flags().StringArrayVar(&o.TopologyKeys, "topology-keys", nil, "Topology keys for affinity")
			cmd.Flags().StringToStringVar(&o.NodeLabels, "node-labels", nil, "Node label selector")
			cmd.Flags().StringVar(&o.ComponentsFilePath, "components", "", "Use yaml file to specify the cluster components")
		},
	}
	return create.BuildCommand(inputs)
}
