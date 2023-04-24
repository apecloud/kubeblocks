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
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/util/templates"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/class"
	"github.com/apecloud/kubeblocks/internal/cli/cluster"
	"github.com/apecloud/kubeblocks/internal/cli/create"
	"github.com/apecloud/kubeblocks/internal/cli/delete"
	"github.com/apecloud/kubeblocks/internal/cli/printer"
	"github.com/apecloud/kubeblocks/internal/cli/types"
	"github.com/apecloud/kubeblocks/internal/cli/util"
)

type OperationsOptions struct {
	create.CreateOptions
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

func newBaseOperationsOptions(f cmdutil.Factory, streams genericclioptions.IOStreams,
	opsType appsv1alpha1.OpsType, hasComponentNamesFlag bool) *OperationsOptions {
	o := &OperationsOptions{
		// nil cannot be set to a map struct in CueLang, so init the map of KeyValues.
		KeyValues:             map[string]string{},
		CreateOptions:         create.CreateOptions{IOStreams: streams},
		OpsType:               opsType,
		HasComponentNamesFlag: hasComponentNamesFlag,
		RequireConfirm:        true,
	}
	o.buildCreateOptions(f)
	return o
}

// addCommonFlags build common flags for operations command
func (o *OperationsOptions) addCommonFlags(cmd *cobra.Command) {
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

func (o *OperationsOptions) validateVScale(cluster *appsv1alpha1.Cluster) error {
	componentClasses, err := class.ListClassesByClusterDefinition(o.Dynamic, cluster.Spec.ClusterDefRef)
	if err != nil {
		return err
	}

	fillClassParams := func(comp *appsv1alpha1.ClusterComponentSpec) {
		if o.Class != "" {
			comp.ClassDefRef = &appsv1alpha1.ClassDefRef{Class: o.Class}
		}

		requests := make(corev1.ResourceList)
		if o.CPU != "" {
			requests[corev1.ResourceCPU] = resource.MustParse(o.CPU)
		}
		if o.Memory != "" {
			requests[corev1.ResourceMemory] = resource.MustParse(o.Memory)
		}
		requests.DeepCopyInto(&comp.Resources.Requests)
		requests.DeepCopyInto(&comp.Resources.Limits)
	}

	for _, name := range o.ComponentNames {
		for _, comp := range cluster.Spec.ComponentSpecs {
			if comp.Name != name {
				continue
			}
			fillClassParams(&comp)
			if _, err = class.ValidateComponentClass(&comp, componentClasses); err != nil {
				return err
			}
		}
	}
	return nil
}

// Validate command flags or args is legal
func (o *OperationsOptions) Validate() error {
	if o.Name == "" {
		return makeMissingClusterNameErr()
	}

	// check if cluster exist
	unstructuredObj, err := o.Dynamic.Resource(types.ClusterGVR()).Namespace(o.Namespace).Get(context.TODO(), o.Name, metav1.GetOptions{})
	if err != nil {
		return err
	}
	var cluster appsv1alpha1.Cluster
	if err = runtime.DefaultUnstructuredConverter.FromUnstructured(unstructuredObj.Object, &cluster); err != nil {
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
	case appsv1alpha1.VerticalScalingType:
		if err := o.validateVScale(&cluster); err != nil {
			return err
		}
	}
	if o.RequireConfirm {
		return delete.Confirm([]string{o.Name}, o.In)
	}
	return nil
}

// buildCreateOptions builds create options for operation
func (o *OperationsOptions) buildCreateOptions(f cmdutil.Factory) {
	o.OpsTypeLower = strings.ToLower(string(o.OpsType))
	customOutPut := func(opt *create.CreateOptions) {
		output := fmt.Sprintf("OpsRequest %s created successfully, you can view the progress:", opt.Name)
		printer.PrintLine(output)
		nextLine := fmt.Sprintf("\tkbcli cluster describe-ops %s -n %s", opt.Name, opt.Namespace)
		printer.PrintLine(nextLine)
	}
	o.CreateOptions = create.CreateOptions{
		CueTemplateName: "cluster_operations_template.cue",
		GVR:             types.OpsGVR(),
		Options:         o,
		Factory:         f,
		CustomOutPut:    customOutPut,
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
	o := newBaseOperationsOptions(f, streams, appsv1alpha1.RestartType, true)
	cmd := &cobra.Command{
		Use:               "restart NAME",
		Short:             "Restart the specified components in the cluster.",
		Example:           restartExample,
		ValidArgsFunction: util.ResourceNameCompletionFunc(f, types.ClusterGVR()),
		Run: func(cmd *cobra.Command, args []string) {
			o.Cmd, o.Args = cmd, args
			cmdutil.CheckErr(o.Complete())
			cmdutil.CheckErr(o.CompleteRestartOps())
			cmdutil.CheckErr(o.Validate())
			cmdutil.CheckErr(o.Run())
		},
	}
	o.addCommonFlags(cmd)
	return cmd
}

var upgradeExample = templates.Examples(`
		# upgrade the cluster to the specified version 
		kbcli cluster upgrade <my-cluster> --cluster-version=<cluster-version>
`)

// NewUpgradeCmd creates a upgrade command
func NewUpgradeCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := newBaseOperationsOptions(f, streams, appsv1alpha1.UpgradeType, false)
	cmd := &cobra.Command{
		Use:               "upgrade NAME",
		Short:             "Upgrade the cluster version.",
		Example:           upgradeExample,
		ValidArgsFunction: util.ResourceNameCompletionFunc(f, types.ClusterGVR()),
		Run: func(cmd *cobra.Command, args []string) {
			o.Cmd, o.Args = cmd, args
			cmdutil.CheckErr(o.Complete())
			cmdutil.CheckErr(o.Validate())
			cmdutil.CheckErr(o.Run())
		},
	}
	o.addCommonFlags(cmd)
	cmd.Flags().StringVar(&o.ClusterVersionRef, "cluster-version", "", "Reference cluster version (required)")
	return cmd
}

var verticalScalingExample = templates.Examples(`
		# scale the computing resources of specified components, separate with commas when <component-name> more than one
		kbcli cluster vscale <my-cluster> --components=<component-name> --cpu=500m --memory=500Mi 

		# scale the computing resources of specified components by class, available classes can be get by executing the command "kbcli class list --cluster-definition <cluster-definition-name>"
		kbcli cluster vscale <my-cluster> --components=<component-name> --class=<class-name>
`)

// NewVerticalScalingCmd creates a vertical scaling command
func NewVerticalScalingCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := newBaseOperationsOptions(f, streams, appsv1alpha1.VerticalScalingType, true)
	cmd := &cobra.Command{
		Use:               "vscale NAME",
		Short:             "Vertically scale the specified components in the cluster.",
		Example:           verticalScalingExample,
		ValidArgsFunction: util.ResourceNameCompletionFunc(f, types.ClusterGVR()),
		Run: func(cmd *cobra.Command, args []string) {
			o.Cmd, o.Args = cmd, args
			cmdutil.CheckErr(o.Complete())
			cmdutil.CheckErr(o.CompleteComponentsFlag())
			cmdutil.CheckErr(o.Validate())
			cmdutil.CheckErr(o.Run())
		},
	}
	o.addCommonFlags(cmd)
	cmd.Flags().StringVar(&o.CPU, "cpu", "", "Requested and limited size of component cpu")
	cmd.Flags().StringVar(&o.Memory, "memory", "", "Requested and limited size of component memory")
	cmd.Flags().StringVar(&o.Class, "class", "", "Component class")
	return cmd
}

var horizontalScalingExample = templates.Examples(`
		# expand storage resources of specified components, separate with commas when <component-name> more than one
		kbcli cluster hscale <my-cluster> --components=<component-name> --replicas=3
`)

// NewHorizontalScalingCmd creates a horizontal scaling command
func NewHorizontalScalingCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := newBaseOperationsOptions(f, streams, appsv1alpha1.HorizontalScalingType, true)
	cmd := &cobra.Command{
		Use:               "hscale NAME",
		Short:             "Horizontally scale the specified components in the cluster.",
		Example:           horizontalScalingExample,
		ValidArgsFunction: util.ResourceNameCompletionFunc(f, types.ClusterGVR()),
		Run: func(cmd *cobra.Command, args []string) {
			o.Cmd, o.Args = cmd, args
			cmdutil.CheckErr(o.Complete())
			cmdutil.CheckErr(o.CompleteComponentsFlag())
			cmdutil.CheckErr(o.Validate())
			cmdutil.CheckErr(o.Run())
		},
	}

	o.addCommonFlags(cmd)
	cmd.Flags().IntVar(&o.Replicas, "replicas", o.Replicas, "Replicas with the specified components")
	_ = cmd.MarkFlagRequired("replicas")
	return cmd
}

var volumeExpansionExample = templates.Examples(`
		# restart specifies the component, separate with commas when <component-name> more than one
		kbcli cluster volume-expand <my-cluster> --components=<component-name> \ 
  		--volume-claim-templates=data --storage=10Gi
`)

// NewVolumeExpansionCmd creates a vertical scaling command
func NewVolumeExpansionCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := newBaseOperationsOptions(f, streams, appsv1alpha1.VolumeExpansionType, true)
	cmd := &cobra.Command{
		Use:               "volume-expand NAME",
		Short:             "Expand volume with the specified components and volumeClaimTemplates in the cluster.",
		Example:           volumeExpansionExample,
		ValidArgsFunction: util.ResourceNameCompletionFunc(f, types.ClusterGVR()),
		Run: func(cmd *cobra.Command, args []string) {
			o.Cmd, o.Args = cmd, args
			cmdutil.CheckErr(o.Complete())
			cmdutil.CheckErr(o.CompleteComponentsFlag())
			cmdutil.CheckErr(o.Validate())
			cmdutil.CheckErr(o.Run())
		},
	}
	o.addCommonFlags(cmd)
	cmd.Flags().StringSliceVarP(&o.VCTNames, "volume-claim-templates", "t", nil, "VolumeClaimTemplate names in components (required)")
	cmd.Flags().StringVar(&o.Storage, "storage", "", "Volume storage size (required)")
	return cmd
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
	o := newBaseOperationsOptions(f, streams, appsv1alpha1.ExposeType, true)
	cmd := &cobra.Command{
		Use:               "expose NAME --enable=[true|false] --type=[vpc|internet]",
		Short:             "Expose a cluster with a new endpoint, the new endpoint can be found by executing 'kbcli cluster describe NAME'.",
		Example:           exposeExamples,
		ValidArgsFunction: util.ResourceNameCompletionFunc(f, types.ClusterGVR()),
		Run: func(cmd *cobra.Command, args []string) {
			o.Cmd, o.Args = cmd, args
			cmdutil.CheckErr(o.Complete())
			cmdutil.CheckErr(o.fillExpose())
			cmdutil.CheckErr(o.validateExpose())
			cmdutil.CheckErr(o.Run())
		},
	}

	o.addCommonFlags(cmd)
	cmd.Flags().StringVar(&o.ExposeType, "type", "", "Expose type, currently supported types are 'vpc', 'internet'")
	cmd.Flags().StringVar(&o.ExposeEnabled, "enable", "", "Enable or disable the expose, values can be true or false")

	util.CheckErr(cmd.RegisterFlagCompletionFunc("type", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return []string{string(util.ExposeToVPC), string(util.ExposeToInternet)}, cobra.ShellCompDirectiveNoFileComp
	}))
	util.CheckErr(cmd.RegisterFlagCompletionFunc("enable", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return []string{"true", "false"}, cobra.ShellCompDirectiveNoFileComp
	}))

	_ = cmd.MarkFlagRequired("enable")
	return cmd
}

var stopExample = templates.Examples(`
		# stop the cluster and release all the pods of the cluster
		kbcli cluster stop <my-cluster>
`)

// NewStopCmd creates a stop command
func NewStopCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := newBaseOperationsOptions(f, streams, appsv1alpha1.StopType, false)
	cmd := &cobra.Command{
		Use:               "stop NAME",
		Short:             "Stop the cluster and release all the pods of the cluster.",
		Example:           stopExample,
		ValidArgsFunction: util.ResourceNameCompletionFunc(f, types.ClusterGVR()),
		Run: func(cmd *cobra.Command, args []string) {
			o.Cmd, o.Args = cmd, args
			cmdutil.CheckErr(o.Complete())
			cmdutil.CheckErr(o.Validate())
			cmdutil.CheckErr(o.Run())
		},
	}
	o.addCommonFlags(cmd)
	return cmd
}

var startExample = templates.Examples(`
		# start the cluster when cluster is stopped
		kbcli cluster start <my-cluster>
`)

// NewStartCmd creates a start command
func NewStartCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := newBaseOperationsOptions(f, streams, appsv1alpha1.StartType, false)
	o.RequireConfirm = false
	cmd := &cobra.Command{
		Use:               "start NAME",
		Short:             "Start the cluster if cluster is stopped.",
		Example:           startExample,
		ValidArgsFunction: util.ResourceNameCompletionFunc(f, types.ClusterGVR()),
		Run: func(cmd *cobra.Command, args []string) {
			o.Cmd, o.Args = cmd, args
			cmdutil.CheckErr(o.Complete())
			cmdutil.CheckErr(o.Validate())
			cmdutil.CheckErr(o.Run())
		},
	}
	o.addCommonFlags(cmd)
	return cmd
}
