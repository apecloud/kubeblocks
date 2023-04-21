/*
Copyright ApeCloud, Inc.

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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/util/templates"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/cli/cluster"
	"github.com/apecloud/kubeblocks/internal/cli/create"
	"github.com/apecloud/kubeblocks/internal/cli/delete"
	"github.com/apecloud/kubeblocks/internal/cli/printer"
	"github.com/apecloud/kubeblocks/internal/cli/types"
	"github.com/apecloud/kubeblocks/internal/cli/util"
)

type OperationsOptions struct {
	create.BaseOptions
	HasComponentNamesFlag bool `json:"-"`
	// RequireConfirm if it is true, the second verification will be performed before creating ops.
	RequireConfirm         bool     `json:"-"`
	ComponentNames         []string `json:"componentNames,omitempty"`
	OpsRequestName         string   `json:"opsRequestName"`
	TTLSecondsAfterSucceed int      `json:"ttlSecondsAfterSucceed"`

	// OpsType operation type
	OpsType appsv1alpha1.OpsType `json:"type"`

	// OpsTypeLower lower OpsType
	OpsTypeLower string `json:"typeLower"`

	// Upgrade options
	ClusterVersionRef string `json:"clusterVersionRef"`

	// VerticalScaling options
	CPU    string `json:"cpu"`
	Memory string `json:"memory"`
	Class  string `json:"class"`

	// HorizontalScaling options
	Replicas int `json:"replicas"`

	// Reconfiguring options
	KeyValues       map[string]string `json:"keyValues"`
	CfgTemplateName string            `json:"cfgTemplateName"`
	CfgFile         string            `json:"cfgFile"`

	// VolumeExpansion options.
	// VCTNames VolumeClaimTemplate names
	VCTNames []string `json:"vctNames,omitempty"`
	Storage  string   `json:"storage"`

	// Expose options
	ExposeType    string                                 `json:"-"`
	ExposeEnabled string                                 `json:"-"`
	Services      []appsv1alpha1.ClusterComponentService `json:"services,omitempty"`
}

func newBaseOperationsOptions(streams genericclioptions.IOStreams, opsType appsv1alpha1.OpsType, hasComponentNamesFlag bool) *OperationsOptions {
	return &OperationsOptions{
		// nil cannot be set to a map struct in CueLang, so init the map of KeyValues.
		KeyValues:             map[string]string{},
		BaseOptions:           create.BaseOptions{IOStreams: streams},
		OpsType:               opsType,
		HasComponentNamesFlag: hasComponentNamesFlag,
		RequireConfirm:        true,
	}
}

// buildCommonFlags build common flags for operations command
func (o *OperationsOptions) buildCommonFlags(cmd *cobra.Command) {
	// add print flags
	printer.AddOutputFlagForCreate(cmd, &o.Format)

	cmd.Flags().StringVar(&o.OpsRequestName, "name", "", "OpsRequest name. if not specified, it will be randomly generated ")
	cmd.Flags().IntVar(&o.TTLSecondsAfterSucceed, "ttlSecondsAfterSucceed", 0, "Time to live after the OpsRequest succeed")
	cmd.Flags().String("dry-run", "none", `Must be "server", or "client". If client strategy, only print the object that would be sent, without sending it. If server strategy, submit server-side request without persisting the resource.`)
	cmd.Flags().Lookup("dry-run").NoOptDefVal = "unchanged"
	if o.HasComponentNamesFlag {
		cmd.Flags().StringSliceVar(&o.ComponentNames, "components", nil, " Component names to this operations")
	}
}

// CompleteRestartOps when restart a cluster and components is null, it means restarting all components of the cluster.
// we should set all component names to ComponentNames flag.
func (o *OperationsOptions) CompleteRestartOps() error {
	if o.Name == "" {
		return makeMissingClusterNameErr()
	}
	if len(o.ComponentNames) != 0 {
		return nil
	}
	clusterObj, err := cluster.GetClusterByName(o.Dynamic, o.Name, o.Namespace)
	if err != nil {
		return err
	}
	componentSpecs := clusterObj.Spec.ComponentSpecs
	o.ComponentNames = make([]string, len(componentSpecs))
	for i := range componentSpecs {
		o.ComponentNames[i] = componentSpecs[i].Name
	}
	return nil
}

// CompleteComponentsFlag when components flag is null and the cluster only has one component, should auto complete it.
func (o *OperationsOptions) CompleteComponentsFlag() error {
	if o.Name == "" {
		return makeMissingClusterNameErr()
	}
	if len(o.ComponentNames) != 0 {
		return nil
	}
	clusterObj, err := cluster.GetClusterByName(o.Dynamic, o.Name, o.Namespace)
	if err != nil {
		return err
	}
	if len(clusterObj.Spec.ComponentSpecs) == 1 {
		o.ComponentNames = []string{clusterObj.Spec.ComponentSpecs[0].Name}
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
		return fmt.Errorf("missing volume-claim-templates")
	}
	if len(o.Storage) == 0 {
		return fmt.Errorf("missing storage")
	}
	return nil
}

// Validate command flags or args is legal
func (o *OperationsOptions) Validate() error {
	if o.Name == "" {
		return makeMissingClusterNameErr()
	}

	// check if cluster exist
	_, err := o.Dynamic.Resource(types.ClusterGVR()).Namespace(o.Namespace).Get(context.TODO(), o.Name, metav1.GetOptions{})
	if err != nil {
		return err
	}

	// common validate for componentOps
	if o.HasComponentNamesFlag && len(o.ComponentNames) == 0 {
		return fmt.Errorf(`missing components, please specify the "--components" flag for multi-components cluster`)
	}

	switch o.OpsType {
	case appsv1alpha1.VolumeExpansionType:
		if err := o.validateVolumeExpansion(); err != nil {
			return err
		}
	case appsv1alpha1.UpgradeType:
		if err := o.validateUpgrade(); err != nil {
			return err
		}
	}
	if o.RequireConfirm {
		return delete.Confirm([]string{o.Name}, o.In)
	}
	return nil
}

// buildOperationsInputs builds operations inputs
func buildOperationsInputs(f cmdutil.Factory, o *OperationsOptions) create.Inputs {
	o.OpsTypeLower = strings.ToLower(string(o.OpsType))
	customOutPut := func(opt *create.BaseOptions) {
		output := fmt.Sprintf("OpsRequest %s created successfully, you can view the progress:", opt.Name)
		printer.PrintLine(output)
		nextLine := fmt.Sprintf("\tkbcli cluster describe-ops %s -n %s", opt.Name, opt.Namespace)
		printer.PrintLine(nextLine)
	}
	return create.Inputs{
		CueTemplateName:              "cluster_operations_template.cue",
		ResourceName:                 types.ResourceOpsRequests,
		BaseOptionsObj:               &o.BaseOptions,
		Options:                      o,
		Factory:                      f,
		Validate:                     o.Validate,
		CustomOutPut:                 customOutPut,
		Group:                        types.AppsAPIGroup,
		Version:                      types.AppsAPIVersion,
		ResourceNameGVRForCompletion: types.ClusterGVR(),
	}
}

func (o *OperationsOptions) validateExpose() error {
	switch util.ExposeType(o.ExposeType) {
	case "", util.ExposeToVPC, util.ExposeToInternet:
	default:
		return fmt.Errorf("invalid expose type %q", o.ExposeType)
	}

	switch strings.ToLower(o.ExposeEnabled) {
	case util.EnableValue, util.DisableValue:
	default:
		return fmt.Errorf("invalid value for enable flag: %s", o.ExposeEnabled)
	}
	return nil
}

func (o *OperationsOptions) fillExpose() error {
	version, err := util.GetK8sVersion(o.Client.Discovery())
	if err != nil {
		return err
	}
	provider, err := util.GetK8sProvider(version, o.Client)
	if err != nil {
		return err
	}
	if provider == util.UnknownProvider {
		return fmt.Errorf("unknown k8s provider")
	}

	// default expose to internet
	exposeType := util.ExposeType(o.ExposeType)
	if exposeType == "" {
		exposeType = util.ExposeToInternet
	}

	annotations, err := util.GetExposeAnnotations(provider, exposeType)
	if err != nil {
		return err
	}

	gvr := schema.GroupVersionResource{Group: types.AppsAPIGroup, Version: types.AppsAPIVersion, Resource: types.ResourceClusters}
	unstructuredObj, err := o.Dynamic.Resource(gvr).Namespace(o.Namespace).Get(context.TODO(), o.Name, metav1.GetOptions{})
	if err != nil {
		return err
	}
	cluster := appsv1alpha1.Cluster{}
	if err = runtime.DefaultUnstructuredConverter.FromUnstructured(unstructuredObj.UnstructuredContent(), &cluster); err != nil {
		return err
	}

	if len(o.ComponentNames) == 0 {
		if len(cluster.Spec.ComponentSpecs) == 1 {
			o.ComponentNames = append(o.ComponentNames, cluster.Spec.ComponentSpecs[0].Name)
		} else {
			return fmt.Errorf("please specify --components")
		}
	}

	compMap := make(map[string]appsv1alpha1.ClusterComponentSpec)
	for _, compSpec := range cluster.Spec.ComponentSpecs {
		compMap[compSpec.Name] = compSpec
	}

	var (
		// currently, we use the expose type as service name
		svcName = string(exposeType)
		enabled = strings.ToLower(o.ExposeEnabled) == util.EnableValue
	)
	for _, name := range o.ComponentNames {
		comp, ok := compMap[name]
		if !ok {
			return fmt.Errorf("component %s not found", name)
		}

		for _, svc := range comp.Services {
			if svc.Name != svcName {
				o.Services = append(o.Services, svc)
			}
		}

		if enabled {
			o.Services = append(o.Services, appsv1alpha1.ClusterComponentService{
				Name:        svcName,
				ServiceType: corev1.ServiceTypeLoadBalancer,
				Annotations: annotations,
			})
		}
	}
	return nil
}

var restartExample = templates.Examples(`
		# restart all components
		kbcli cluster restart <my-cluster>

		# restart specifies the component, separate with commas when <component-name> more than one
		kbcli cluster restart <my-cluster> --components=<component-name>
`)

// NewRestartCmd creates a restart command
func NewRestartCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := newBaseOperationsOptions(streams, appsv1alpha1.RestartType, true)
	inputs := buildOperationsInputs(f, o)
	inputs.Use = "restart"
	inputs.Short = "Restart the specified components in the cluster."
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

// NewUpgradeCmd creates a upgrade command
func NewUpgradeCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := newBaseOperationsOptions(streams, appsv1alpha1.UpgradeType, false)
	inputs := buildOperationsInputs(f, o)
	inputs.Use = "upgrade"
	inputs.Short = "Upgrade the cluster version."
	inputs.Example = upgradeExample
	inputs.BuildFlags = func(cmd *cobra.Command) {
		o.buildCommonFlags(cmd)
		cmd.Flags().StringVar(&o.ClusterVersionRef, "cluster-version", "", "Reference cluster version (required)")
	}
	return create.BuildCommand(inputs)
}

var verticalScalingExample = templates.Examples(`
		# scale the computing resources of specified components, separate with commas when <component-name> more than one
		kbcli cluster vscale <my-cluster> --components=<component-name> --cpu=500m --memory=500Mi 

		# scale the computing resources of specified components by class, available classes can be get by executing the command "kbcli class list --cluster-definition <cluster-definition-name>"
		kbcli cluster vscale <my-cluster> --components=<component-name> --class=<class-name>
`)

// NewVerticalScalingCmd creates a vertical scaling command
func NewVerticalScalingCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := newBaseOperationsOptions(streams, appsv1alpha1.VerticalScalingType, true)
	inputs := buildOperationsInputs(f, o)
	inputs.Use = "vscale"
	inputs.Short = "Vertically scale the specified components in the cluster."
	inputs.Example = verticalScalingExample
	inputs.Complete = o.CompleteComponentsFlag
	inputs.BuildFlags = func(cmd *cobra.Command) {
		o.buildCommonFlags(cmd)
		cmd.Flags().StringVar(&o.CPU, "cpu", "", "Requested and limited size of component cpu")
		cmd.Flags().StringVar(&o.Memory, "memory", "", "Requested and limited size of component memory")
		cmd.Flags().StringVar(&o.Class, "class", "", "Component class")
	}
	return create.BuildCommand(inputs)
}

var horizontalScalingExample = templates.Examples(`
		# expand storage resources of specified components, separate with commas when <component-name> more than one
		kbcli cluster hscale <my-cluster> --components=<component-name> --replicas=3
`)

// NewHorizontalScalingCmd creates a horizontal scaling command
func NewHorizontalScalingCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := newBaseOperationsOptions(streams, appsv1alpha1.HorizontalScalingType, true)
	inputs := buildOperationsInputs(f, o)
	inputs.Use = "hscale"
	inputs.Short = "Horizontally scale the specified components in the cluster."
	inputs.Example = horizontalScalingExample
	inputs.Complete = o.CompleteComponentsFlag
	inputs.BuildFlags = func(cmd *cobra.Command) {
		o.buildCommonFlags(cmd)
		cmd.Flags().IntVar(&o.Replicas, "replicas", o.Replicas, "Replicas with the specified components")
		_ = cmd.MarkFlagRequired("replicas")
	}
	return create.BuildCommand(inputs)
}

var volumeExpansionExample = templates.Examples(`
		# restart specifies the component, separate with commas when <component-name> more than one
		kbcli cluster volume-expand <my-cluster> --components=<component-name> \ 
  		--volume-claim-templates=data --storage=10Gi
`)

// NewVolumeExpansionCmd creates a vertical scaling command
func NewVolumeExpansionCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := newBaseOperationsOptions(streams, appsv1alpha1.VolumeExpansionType, true)
	inputs := buildOperationsInputs(f, o)
	inputs.Use = "volume-expand"
	inputs.Short = "Expand volume with the specified components and volumeClaimTemplates in the cluster."
	inputs.Example = volumeExpansionExample
	inputs.Complete = o.CompleteComponentsFlag
	inputs.BuildFlags = func(cmd *cobra.Command) {
		o.buildCommonFlags(cmd)
		cmd.Flags().StringSliceVarP(&o.VCTNames, "volume-claim-templates", "t", nil, "VolumeClaimTemplate names in components (required)")
		cmd.Flags().StringVar(&o.Storage, "storage", "", "Volume storage size (required)")
	}
	return create.BuildCommand(inputs)
}

var (
	exposeExamples = templates.Examples(`
		# Expose a cluster to vpc
		kbcli cluster expose mycluster --type vpc --enable=true

		# Expose a cluster to internet
		kbcli cluster expose mycluster --type internet --enable=true
		
		# Stop exposing a cluster
		kbcli cluster expose mycluster --type vpc --enable=false
	`)
)

// NewExposeCmd creates an expose command
func NewExposeCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := newBaseOperationsOptions(streams, appsv1alpha1.ExposeType, true)
	inputs := buildOperationsInputs(f, o)
	inputs.Use = "expose NAME --enable=[true|false] --type=[vpc|internet]"
	inputs.Short = "Expose a cluster with a new endpoint, the new endpoint can be found by executing 'kbcli cluster describe NAME'."
	inputs.Example = exposeExamples
	inputs.BuildFlags = func(cmd *cobra.Command) {
		o.buildCommonFlags(cmd)
		cmd.Flags().StringVar(&o.ExposeType, "type", "", "Expose type, currently supported types are 'vpc', 'internet'")
		util.CheckErr(cmd.RegisterFlagCompletionFunc("type", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			return []string{string(util.ExposeToVPC), string(util.ExposeToInternet)}, cobra.ShellCompDirectiveNoFileComp
		}))
		cmd.Flags().StringVar(&o.ExposeEnabled, "enable", "", "Enable or disable the expose, values can be true or false")
		util.CheckErr(cmd.RegisterFlagCompletionFunc("enable", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			return []string{"true", "false"}, cobra.ShellCompDirectiveNoFileComp
		}))
		_ = cmd.MarkFlagRequired("enable")
	}
	inputs.Validate = o.validateExpose
	inputs.Complete = o.fillExpose
	return create.BuildCommand(inputs)
}

var stopExample = templates.Examples(`
		# stop the cluster and release all the pods of the cluster
		kbcli cluster stop <my-cluster>
`)

// NewStopCmd creates a stop command
func NewStopCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := newBaseOperationsOptions(streams, appsv1alpha1.StopType, false)
	inputs := buildOperationsInputs(f, o)
	inputs.Use = "stop"
	inputs.Short = "Stop the cluster and release all the pods of the cluster."
	inputs.Example = stopExample
	inputs.BuildFlags = func(cmd *cobra.Command) {
		o.buildCommonFlags(cmd)
	}
	return create.BuildCommand(inputs)
}

var startExample = templates.Examples(`
		# start the cluster when cluster is stopped
		kbcli cluster start <my-cluster>
`)

// NewStartCmd creates a start command
func NewStartCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := newBaseOperationsOptions(streams, appsv1alpha1.StartType, false)
	o.RequireConfirm = false
	inputs := buildOperationsInputs(f, o)
	inputs.Use = "start"
	inputs.Short = "Start the cluster if cluster is stopped."
	inputs.Example = startExample
	inputs.BuildFlags = func(cmd *cobra.Command) {
		o.buildCommonFlags(cmd)
	}
	return create.BuildCommand(inputs)
}
