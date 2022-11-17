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

package cluster

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/ghodss/yaml"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"

	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/dbctl/cmd/create"
	"github.com/apecloud/kubeblocks/internal/dbctl/types"
)

const (
	DefaultClusterDef = "apecloud-wesql"
	DefaultAppVersion = "wesql-8.0.30"

	CueTemplateName = "cluster_template.cue"
	monitorKey      = "monitor"
)

type CreateOptions struct {
	// ClusterDefRef reference clusterDefinition
	ClusterDefRef     string `json:"clusterDefRef"`
	AppVersionRef     string `json:"appVersionRef"`
	TerminationPolicy string `json:"terminationPolicy"`
	PodAntiAffinity   string `json:"podAntiAffinity"`
	Monitor           bool   `json:"monitor"`
	EnableAllLogs     bool   `json:"enableAllLogs"`
	// TopologyKeys if TopologyKeys is nil, add omitempty json tag.
	// because CueLang can not covert null to list.
	TopologyKeys []string                 `json:"topologyKeys,omitempty"`
	NodeLabels   map[string]string        `json:"nodeLabels,omitempty"`
	Components   []map[string]interface{} `json:"components"`
	// ComponentsFilePath components file path
	ComponentsFilePath string `json:"-"`

	// backup name to restore in creation
	Backup string `json:"backup,omitempty"`

	create.BaseOptions
}

func setMonitor(monitor bool, components []map[string]interface{}) {
	if components == nil {
		return
	}
	for _, component := range components {
		component[monitorKey] = monitor
	}
}

func setBackup(o *CreateOptions, components []map[string]interface{}) error {
	backup := o.Backup
	if len(backup) == 0 {
		return nil
	}
	if components == nil {
		return nil
	}

	gvr := schema.GroupVersionResource{Group: types.DPGroup, Version: types.DPVersion, Resource: types.ResourceBackupJobs}
	backupJobObj, err := o.Client.Resource(gvr).Namespace(o.Namespace).Get(context.TODO(), backup, metav1.GetOptions{})
	if err != nil {
		return err
	}
	backupType, _, _ := unstructured.NestedString(backupJobObj.Object, "spec", "backupType")
	if backupType != "snapshot" {
		return errors.Errorf("Only support snapshot backup, specified backup type is '%s'.", backupType)
	}

	dataSource := make(map[string]interface{}, 0)
	_ = unstructured.SetNestedField(dataSource, backup, "name")
	_ = unstructured.SetNestedField(dataSource, "VolumeSnapshot", "kind")
	_ = unstructured.SetNestedField(dataSource, "snapshot.storage.k8s.io", "apiGroup")

	for _, component := range components {
		templates := component["volumeClaimTemplates"].([]interface{})
		for _, t := range templates {
			templateMap := t.(map[string]interface{})
			_ = unstructured.SetNestedField(templateMap, dataSource, "spec", "dataSource")
		}
	}
	return nil
}

func (o *CreateOptions) Validate() error {
	if o.Name == "" {
		return fmt.Errorf("missing cluster name")
	}
	if len(o.ComponentsFilePath) == 0 {
		return fmt.Errorf("a valid component file path is needed")
	}
	return nil
}

func (o *CreateOptions) Complete() error {
	var (
		componentByte []byte
		err           error
		components    = o.Components
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
	setMonitor(o.Monitor, components)
	if err = setBackup(o, components); err != nil {
		return err
	}
	o.Components = components
	return nil
}

func NewCreateCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := &CreateOptions{BaseOptions: create.BaseOptions{IOStreams: streams}}
	inputs := create.Inputs{
		Use:             "create",
		Short:           "Create a database cluster",
		CueTemplateName: CueTemplateName,
		ResourceName:    types.ResourceClusters,
		BaseOptionsObj:  &o.BaseOptions,
		Options:         o,
		Factory:         f,
		Validate:        o.Validate,
		Complete:        o.Complete,
		PreCreate:       o.PreCreate,
		BuildFlags: func(cmd *cobra.Command) {
			cmd.Flags().StringVar(&o.ClusterDefRef, "cluster-definition", DefaultClusterDef, "ClusterDefinition reference")
			cmd.Flags().StringVar(&o.AppVersionRef, "app-version", DefaultAppVersion, "AppVersion reference")
			cmd.Flags().StringVar(&o.TerminationPolicy, "termination-policy", "Delete", "Termination policy, one of: (DoNotTerminate, Halt, Delete, WipeOut)")
			cmd.Flags().StringVar(&o.PodAntiAffinity, "pod-anti-affinity", "Preferred", "Pod anti-affinity type")
			cmd.Flags().BoolVar(&o.Monitor, "monitor", false, "Set monitor enabled (default false)")
			cmd.Flags().BoolVar(&o.EnableAllLogs, "enable-all-logs", false, "Enable advanced application all log extraction, and true will ignore enabledLogs of component level")
			cmd.Flags().StringArrayVar(&o.TopologyKeys, "topology-keys", nil, "Topology keys for affinity")
			cmd.Flags().StringToStringVar(&o.NodeLabels, "node-labels", nil, "Node label selector")
			cmd.Flags().StringVar(&o.ComponentsFilePath, "components", "", "Use yaml file to specify the cluster components")
			cmd.Flags().StringVar(&o.Backup, "backup", "", "Set a source backup to restore data")
		},
	}
	return create.BuildCommand(inputs)
}

// PreCreate before commit yaml to k8s, make changes on Unstructured yaml
func (o *CreateOptions) PreCreate(obj *unstructured.Unstructured) error {
	if !o.EnableAllLogs {
		// EnableAllLogs is false, nothing will change
		return nil
	}
	c := &dbaasv1alpha1.Cluster{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, c); err != nil {
		return err
	}
	// get cluster definition from k8s
	res, err := o.Client.Resource(types.ClusterDefGVR()).Namespace("").Get(context.TODO(), c.Spec.ClusterDefRef, metav1.GetOptions{}, "")
	if err != nil {
		return err
	}
	cd := &dbaasv1alpha1.ClusterDefinition{}
	if err = runtime.DefaultUnstructuredConverter.FromUnstructured(res.Object, cd); err != nil {
		return err
	}
	setEnableAllLogs(c, cd)
	data, e := runtime.DefaultUnstructuredConverter.ToUnstructured(c)
	if e != nil {
		return e
	}
	obj.SetUnstructuredContent(data)
	return nil
}

// setEnableAllLog set enable all logs, and ignore enabledLogs of component level.
func setEnableAllLogs(c *dbaasv1alpha1.Cluster, cd *dbaasv1alpha1.ClusterDefinition) {
	for idx, comCluster := range c.Spec.Components {
		for _, com := range cd.Spec.Components {
			if !strings.EqualFold(comCluster.Type, com.TypeName) {
				continue
			}
			typeList := make([]string, 0, len(com.LogConfigs))
			for _, logConf := range com.LogConfigs {
				typeList = append(typeList, logConf.Name)
			}
			c.Spec.Components[idx].EnabledLogs = typeList
		}
	}
}
