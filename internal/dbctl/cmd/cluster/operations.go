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

	"github.com/apecloud/kubeblocks/internal/dbctl/cmd/create"
	"github.com/apecloud/kubeblocks/internal/dbctl/types"
)

const (
	OpsTypeRestart           = "Restart"
	OpsTypeUpgrade           = "Upgrade"
	OpsTypeVerticalScaling   = "VerticalScaling"
	OpsTypeHorizontalScaling = "HorizontalScaling"
	OpsTypeVolumeExpansion   = "VolumeExpansion"
)

type OperationsOptions struct {
	create.BaseOptions
	ComponentNames         []string `json:"componentNames,omitempty"`
	OpsRequestName         string   `json:"opsRequestName"`
	TTLSecondsAfterSucceed int      `json:"ttlSecondsAfterSucceed"`

	// OpsType operation type
	OpsType string `json:"type"`

	// OpsTypeLower lower OpsType
	OpsTypeLower string `json:"typeLower"`

	// Upgrade options
	AppVersionRef string `json:"appVersionRef"`

	// VerticalScaling options
	RequestCPU    string `json:"requestCPU"`
	RequestMemory string `json:"requestMemory"`
	LimitCPU      string `json:"limitCPU"`
	LimitMemory   string `json:"limitMemory"`

	// HorizontalScaling options
	Replicas int `json:"replicas"`

	// VolumeExpansion options.
	// VctNames VolumeClaimTemplate names
	VctNames []string `json:"vctNames,omitempty"`
	Storage  string   `json:"storage"`
}

// buildCommonFlags build common flags for operations command
func (o *OperationsOptions) buildCommonFlags(cmd *cobra.Command) {
	cmd.Flags().StringVar(&o.OpsRequestName, "name", "", "OpsRequest name. if not specified, it will be randomly generated ")
	cmd.Flags().IntVar(&o.TTLSecondsAfterSucceed, "ttlSecondsAfterSucceed", 0, "Time to live after the OpsRequest succeed")
	if o.OpsType != OpsTypeUpgrade {
		cmd.Flags().StringSliceVar(&o.ComponentNames, "component-names", nil, " Component names to this operations (required)")
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
	if len(o.AppVersionRef) == 0 {
		return fmt.Errorf("missing app-version")
	}
	return nil
}

func (o *OperationsOptions) validateVolumeExpansion() error {
	if len(o.VctNames) == 0 {
		return fmt.Errorf("missing vct-names")
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

	if o.OpsType == OpsTypeUpgrade {
		return o.validateUpgrade()
	}

	// common validate for componentOps
	if len(o.ComponentNames) == 0 {
		return fmt.Errorf("missing component-names")
	}

	switch o.OpsType {
	case OpsTypeVolumeExpansion:
		return o.validateVolumeExpansion()
	case OpsTypeHorizontalScaling:
		return o.validateHorizontalScaling()
	}
	return nil
}

// buildOperationsInputs build operations inputs
func buildOperationsInputs(f cmdutil.Factory, o *OperationsOptions) create.Inputs {
	o.OpsTypeLower = strings.ToLower(o.OpsType)
	return create.Inputs{
		CueTemplateName: "cluster_operations_template.cue",
		ResourceName:    types.ResourceOpsRequests,
		BaseOptionsObj:  &o.BaseOptions,
		Options:         o,
		Factory:         f,
		Validate:        o.Validate,
	}
}

// NewRestartCmd create a restart command
func NewRestartCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := &OperationsOptions{BaseOptions: create.BaseOptions{IOStreams: streams}, OpsType: OpsTypeRestart}
	inputs := buildOperationsInputs(f, o)
	inputs.Use = "restart"
	inputs.Short = "restart the specified components in the cluster"
	inputs.BuildFlags = func(cmd *cobra.Command) {
		o.buildCommonFlags(cmd)
	}
	inputs.Complete = o.CompleteRestartOps
	return create.BuildCommand(inputs)
}

// NewUpgradeCmd create a upgrade command
func NewUpgradeCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := &OperationsOptions{BaseOptions: create.BaseOptions{IOStreams: streams}, OpsType: OpsTypeUpgrade}
	inputs := buildOperationsInputs(f, o)
	inputs.Use = "upgrade"
	inputs.Short = "upgrade the cluster"
	inputs.BuildFlags = func(cmd *cobra.Command) {
		o.buildCommonFlags(cmd)
		cmd.Flags().StringVar(&o.AppVersionRef, "app-version", "", "Reference app version (required)")
	}
	return create.BuildCommand(inputs)
}

// NewVerticalScalingCmd create a vertical scaling command
func NewVerticalScalingCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := &OperationsOptions{BaseOptions: create.BaseOptions{IOStreams: streams}, OpsType: OpsTypeVerticalScaling}
	inputs := buildOperationsInputs(f, o)
	inputs.Use = "vertical-scaling"
	inputs.Short = "vertical scaling the specified components in the cluster"
	inputs.BuildFlags = func(cmd *cobra.Command) {
		o.buildCommonFlags(cmd)
		cmd.Flags().StringVar(&o.RequestCPU, "requests.cpu", "", "CPU size requested by the component")
		cmd.Flags().StringVar(&o.RequestMemory, "requests.memory", "", "Memory size requested by the component")
		cmd.Flags().StringVar(&o.LimitCPU, "limits.cpu", "", "CPU size limited by the component")
		cmd.Flags().StringVar(&o.LimitMemory, "limits.memory", "", "Memory size limited by the component")
	}
	return create.BuildCommand(inputs)
}

// NewHorizontalScalingCmd create a horizontal scaling command
func NewHorizontalScalingCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := &OperationsOptions{BaseOptions: create.BaseOptions{IOStreams: streams}, OpsType: OpsTypeHorizontalScaling}
	inputs := buildOperationsInputs(f, o)
	inputs.Use = "horizontal-scaling"
	inputs.Short = "horizontal scaling the specified components in the cluster"
	inputs.BuildFlags = func(cmd *cobra.Command) {
		o.buildCommonFlags(cmd)
		cmd.Flags().IntVar(&o.Replicas, "replicas", -1, "Replicas with the specified components")
	}
	return create.BuildCommand(inputs)
}

// NewVolumeExpansionCmd create a vertical scaling command
func NewVolumeExpansionCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := &OperationsOptions{BaseOptions: create.BaseOptions{IOStreams: streams}, OpsType: OpsTypeVolumeExpansion}
	inputs := buildOperationsInputs(f, o)
	inputs.Use = "volume-expansion"
	inputs.Short = "expand volume with the specified components and volumeClaimTemplates in the cluster"
	inputs.BuildFlags = func(cmd *cobra.Command) {
		o.buildCommonFlags(cmd)
		cmd.Flags().StringSliceVar(&o.VctNames, "volume-claim-template-names", nil, "VolumeClaimTemplate names in components (required)")
		cmd.Flags().StringVar(&o.Storage, "storage", "", "Volume storage size (required)")
	}
	return create.BuildCommand(inputs)
}
