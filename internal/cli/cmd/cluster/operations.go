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
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/util/templates"

	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/cli/create"
	"github.com/apecloud/kubeblocks/internal/cli/delete"
	"github.com/apecloud/kubeblocks/internal/cli/types"
)

type OperationsOptions struct {
	create.BaseOptions
	ComponentNames         []string `json:"componentNames,omitempty"`
	OpsRequestName         string   `json:"opsRequestName"`
	TTLSecondsAfterSucceed int      `json:"ttlSecondsAfterSucceed"`

	// OpsType operation type
	OpsType dbaasv1alpha1.OpsType `json:"type"`

	// OpsTypeLower lower OpsType
	OpsTypeLower string `json:"typeLower"`

	// Upgrade options
	ClusterVersionRef string `json:"clusterVersionRef"`

	// VerticalScaling options
	RequestCPU    string `json:"requestCPU"`
	RequestMemory string `json:"requestMemory"`
	LimitCPU      string `json:"limitCPU"`
	LimitMemory   string `json:"limitMemory"`

	// HorizontalScaling options
	Replicas int `json:"replicas"`

	// VolumeExpansion options.
	// VCTNames VolumeClaimTemplate names
	VCTNames []string `json:"vctNames,omitempty"`
	Storage  string   `json:"storage"`
}

func newBaseOperationsOptions(streams genericclioptions.IOStreams, opsType dbaasv1alpha1.OpsType) *OperationsOptions {
	return &OperationsOptions{
		BaseOptions: create.BaseOptions{IOStreams: streams},
		OpsType:     opsType,
	}
}

// buildCommonFlags build common flags for operations command
func (o *OperationsOptions) buildCommonFlags(cmd *cobra.Command) {
	cmd.Flags().StringVar(&o.OpsRequestName, "name", "", "OpsRequest name. if not specified, it will be randomly generated ")
	cmd.Flags().IntVar(&o.TTLSecondsAfterSucceed, "ttlSecondsAfterSucceed", 0, "Time to live after the OpsRequest succeed")
	if o.OpsType != dbaasv1alpha1.UpgradeType {
		cmd.Flags().StringSliceVar(&o.ComponentNames, "component-names", nil, " Component names to this operations")
	}
}

// CompleteRestartOps when restart a cluster and component-names is null, represents restarting the entire cluster.
// we should set all component names to ComponentNames
func (o *OperationsOptions) CompleteRestartOps() error {
	if len(o.ComponentNames) != 0 {
		return nil
	}
	gvr := schema.GroupVersionResource{Group: types.Group, Version: types.Version, Resource: types.ResourceClusters}
	if unstructuredObj, err := o.Client.Resource(gvr).Namespace(o.Namespace).Get(context.TODO(), o.Name, metav1.GetOptions{}); err != nil {
		return err
	} else {
		if o.ComponentNames, _, err = unstructured.NestedStringSlice(unstructuredObj.Object, "status", "operations", "restartable"); err != nil {
			return err
		}
	}
	return nil
}

func (o *OperationsOptions) validateUpgrade() error {
	if len(o.ClusterVersionRef) == 0 {
		return fmt.Errorf("missing cluster-version")
	}
	return nil
}

func (o *OperationsOptions) validateVolumeExpansion() error {
	if len(o.VCTNames) == 0 {
		return fmt.Errorf("missing volume-claim-template-names")
	}
	if len(o.Storage) == 0 {
		return fmt.Errorf("missing storage")
	}
	return nil
}

func (o *OperationsOptions) validateHorizontalScaling() error {
	if o.Replicas < -1 {
		return fmt.Errorf("replicas required natural number")
	}
	return nil
}

// Validate command flags or args is legal
func (o *OperationsOptions) Validate() error {
	if o.Name == "" {
		return fmt.Errorf("missing cluster name")
	}

	if o.OpsType == dbaasv1alpha1.UpgradeType {
		return o.validateUpgrade()
	}

	// common validate for componentOps
	if len(o.ComponentNames) == 0 {
		return fmt.Errorf("missing component-names")
	}

	switch o.OpsType {
	case dbaasv1alpha1.VolumeExpansionType:
		return o.validateVolumeExpansion()
	case dbaasv1alpha1.HorizontalScalingType:
		return o.validateHorizontalScaling()
	}
	return delete.Confirm([]string{o.Name}, o.In)
}

// buildOperationsInputs build operations inputs
func buildOperationsInputs(f cmdutil.Factory, o *OperationsOptions) create.Inputs {
	o.OpsTypeLower = strings.ToLower(string(o.OpsType))
	return create.Inputs{
		CueTemplateName: "cluster_operations_template.cue",
		ResourceName:    types.ResourceOpsRequests,
		BaseOptionsObj:  &o.BaseOptions,
		Options:         o,
		Factory:         f,
		Validate:        o.Validate,
	}
}

var restartExample = templates.Examples(`
		# restart all components
		kbcli cluster restart <my-cluster>

		# restart specifies the component, separate with commas when <component-name> more than one
		kbcli cluster restart <my-cluster> --component-names=<component-name>
`)

// NewRestartCmd create a restart command
func NewRestartCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := newBaseOperationsOptions(streams, dbaasv1alpha1.RestartType)
	inputs := buildOperationsInputs(f, o)
	inputs.Use = "restart"
	inputs.Short = "Restart the specified components in the cluster"
	inputs.Example = restartExample
	inputs.BuildFlags = func(cmd *cobra.Command) {
		o.buildCommonFlags(cmd)
	}
	inputs.Complete = o.CompleteRestartOps
	return create.BuildCommand(inputs)
}

var upgradeExample = templates.Examples(`
		# upgrade the cluster to the specified version 
		kbcli cluster upgrade <my-cluster> --cluster-version=<cluster-version>
`)

// NewUpgradeCmd create a upgrade command
func NewUpgradeCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := newBaseOperationsOptions(streams, dbaasv1alpha1.UpgradeType)
	inputs := buildOperationsInputs(f, o)
	inputs.Use = "upgrade"
	inputs.Short = "Upgrade the cluster version"
	inputs.Example = upgradeExample
	inputs.BuildFlags = func(cmd *cobra.Command) {
		o.buildCommonFlags(cmd)
		cmd.Flags().StringVar(&o.ClusterVersionRef, "cluster-version", "", "Reference cluster version (required)")
	}
	return create.BuildCommand(inputs)
}

var verticalScalingExample = templates.Examples(`
		# scale the computing resources of specified components, separate with commas when <component-name> more than one
		kbcli cluster vertical-scale <my-cluster> --component-names=<component-name> --requests.cpu=500m \
        --requests.memory=500Mi --limits.cpu=500m --limits.memory=500Mi
`)

// NewVerticalScalingCmd create a vertical scaling command
func NewVerticalScalingCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := newBaseOperationsOptions(streams, dbaasv1alpha1.VerticalScalingType)
	inputs := buildOperationsInputs(f, o)
	inputs.Use = "vertical-scale"
	inputs.Short = "Vertical scale the specified components in the cluster"
	inputs.Example = verticalScalingExample
	inputs.BuildFlags = func(cmd *cobra.Command) {
		o.buildCommonFlags(cmd)
		cmd.Flags().StringVar(&o.RequestCPU, "requests.cpu", "", "CPU size requested by the component")
		cmd.Flags().StringVar(&o.RequestMemory, "requests.memory", "", "Memory size requested by the component")
		cmd.Flags().StringVar(&o.LimitCPU, "limits.cpu", "", "CPU size limited by the component")
		cmd.Flags().StringVar(&o.LimitMemory, "limits.memory", "", "Memory size limited by the component")
	}
	return create.BuildCommand(inputs)
}

var horizontalScalingExample = templates.Examples(`
		# expand storage resources of specified components, separate with commas when <component-name> more than one
		kbcli cluster horizontal-scale <my-cluster> --component-names=<component-name> --replicas=3
`)

// NewHorizontalScalingCmd create a horizontal scaling command
func NewHorizontalScalingCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := newBaseOperationsOptions(streams, dbaasv1alpha1.HorizontalScalingType)
	inputs := buildOperationsInputs(f, o)
	inputs.Use = "horizontal-scale"
	inputs.Short = "Horizontal scale the specified components in the cluster"
	inputs.Example = horizontalScalingExample
	inputs.BuildFlags = func(cmd *cobra.Command) {
		o.buildCommonFlags(cmd)
		cmd.Flags().IntVar(&o.Replicas, "replicas", -1, "Replicas with the specified components")
	}
	return create.BuildCommand(inputs)
}

var volumeExpansionExample = templates.Examples(`
		# restart specifies the component, separate with commas when <component-name> more than one
		kbcli cluster volume-expand <my-cluster> --component-names=<component-name> \ 
  		--volume-claim-template-names=data --storage=10Gi
`)

// NewVolumeExpansionCmd create a vertical scaling command
func NewVolumeExpansionCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := newBaseOperationsOptions(streams, dbaasv1alpha1.VolumeExpansionType)
	inputs := buildOperationsInputs(f, o)
	inputs.Use = "volume-expand"
	inputs.Short = "Expand volume with the specified components and volumeClaimTemplates in the cluster"
	inputs.Example = volumeExpansionExample
	inputs.BuildFlags = func(cmd *cobra.Command) {
		o.buildCommonFlags(cmd)
		cmd.Flags().StringSliceVar(&o.VCTNames, "volume-claim-template-names", nil, "VolumeClaimTemplate names in components (required)")
		cmd.Flags().StringVar(&o.Storage, "storage", "", "Volume storage size (required)")
	}
	return create.BuildCommand(inputs)
}
