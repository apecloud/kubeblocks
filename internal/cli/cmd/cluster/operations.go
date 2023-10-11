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

	jsonpatch "github.com/evanphx/json-patch"
	"github.com/spf13/cobra"
	"golang.org/x/exp/slices"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	apitypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/json"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/util/templates"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/class"
	"github.com/apecloud/kubeblocks/internal/cli/cluster"
	"github.com/apecloud/kubeblocks/internal/cli/create"
	"github.com/apecloud/kubeblocks/internal/cli/printer"
	"github.com/apecloud/kubeblocks/internal/cli/types"
	"github.com/apecloud/kubeblocks/internal/cli/util"
	"github.com/apecloud/kubeblocks/internal/cli/util/flags"
	"github.com/apecloud/kubeblocks/internal/cli/util/prompt"
	"github.com/apecloud/kubeblocks/internal/constant"
)

type OperationsOptions struct {
	create.CreateOptions  `json:"-"`
	HasComponentNamesFlag bool `json:"-"`
	// autoApprove when set true, skip the double check.
	autoApprove            bool     `json:"-"`
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
	CPU         string                   `json:"cpu"`
	Memory      string                   `json:"memory"`
	Class       string                   `json:"class"`
	ClassDefRef appsv1alpha1.ClassDefRef `json:"classDefRef,omitempty"`

	// HorizontalScaling options
	Replicas int `json:"replicas"`

	// Reconfiguring options
	KeyValues       map[string]*string `json:"keyValues"`
	CfgTemplateName string             `json:"cfgTemplateName"`
	CfgFile         string             `json:"cfgFile"`
	ForceRestart    bool               `json:"forceRestart"`
	FileContent     string             `json:"fileContent"`
	HasPatch        bool               `json:"hasPatch"`

	// VolumeExpansion options.
	// VCTNames VolumeClaimTemplate names
	VCTNames []string `json:"vctNames,omitempty"`
	Storage  string   `json:"storage"`

	// Expose options
	ExposeType    string                                 `json:"-"`
	ExposeEnabled string                                 `json:"-"`
	Services      []appsv1alpha1.ClusterComponentService `json:"services,omitempty"`

	// Switchover options
	Component string `json:"component"`
	Instance  string `json:"instance"`
}

func newBaseOperationsOptions(f cmdutil.Factory, streams genericclioptions.IOStreams,
	opsType appsv1alpha1.OpsType, hasComponentNamesFlag bool) *OperationsOptions {
	customOutPut := func(opt *create.CreateOptions) {
		output := fmt.Sprintf("OpsRequest %s created successfully, you can view the progress:", opt.Name)
		printer.PrintLine(output)
		nextLine := fmt.Sprintf("\tkbcli cluster describe-ops %s -n %s", opt.Name, opt.Namespace)
		printer.PrintLine(nextLine)
	}

	o := &OperationsOptions{
		// nil cannot be set to a map struct in CueLang, so init the map of KeyValues.
		KeyValues:             map[string]*string{},
		HasPatch:              true,
		OpsType:               opsType,
		HasComponentNamesFlag: hasComponentNamesFlag,
		autoApprove:           false,
		CreateOptions: create.CreateOptions{
			Factory:         f,
			IOStreams:       streams,
			CueTemplateName: "cluster_operations_template.cue",
			GVR:             types.OpsGVR(),
			CustomOutPut:    customOutPut,
		},
	}

	o.OpsTypeLower = strings.ToLower(string(o.OpsType))
	o.CreateOptions.Options = o
	return o
}

// addCommonFlags adds common flags for operations command
func (o *OperationsOptions) addCommonFlags(cmd *cobra.Command, f cmdutil.Factory) {
	// add print flags
	printer.AddOutputFlagForCreate(cmd, &o.Format, false)

	cmd.Flags().StringVar(&o.OpsRequestName, "name", "", "OpsRequest name. if not specified, it will be randomly generated ")
	cmd.Flags().IntVar(&o.TTLSecondsAfterSucceed, "ttlSecondsAfterSucceed", 0, "Time to live after the OpsRequest succeed")
	cmd.Flags().StringVar(&o.DryRun, "dry-run", "none", `Must be "client", or "server". If with client strategy, only print the object that would be sent, and no data is actually sent. If with server strategy, submit the server-side request, but no data is persistent.`)
	cmd.Flags().Lookup("dry-run").NoOptDefVal = "unchanged"
	if o.HasComponentNamesFlag {
		flags.AddComponentsFlag(f, cmd, &o.ComponentNames, "Component names to this operations")
	}
}

// CompleteRestartOps restarts all components of the cluster
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

// CompleteComponentsFlag when components flag is null and the cluster only has one component, auto complete it.
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

	for _, cName := range o.ComponentNames {
		for _, vctName := range o.VCTNames {
			labels := fmt.Sprintf("%s=%s,%s=%s,%s=%s",
				constant.AppInstanceLabelKey, o.Name,
				constant.KBAppComponentLabelKey, cName,
				constant.VolumeClaimTemplateNameLabelKey, vctName,
			)
			pvcs, err := o.Client.CoreV1().PersistentVolumeClaims(o.Namespace).List(context.Background(),
				metav1.ListOptions{LabelSelector: labels, Limit: 1})
			if err != nil {
				return err
			}
			if len(pvcs.Items) == 0 {
				continue
			}
			pvc := pvcs.Items[0]
			specStorage := pvc.Spec.Resources.Requests.Storage()
			statusStorage := pvc.Status.Capacity.Storage()
			targetStorage, err := resource.ParseQuantity(o.Storage)
			if err != nil {
				return fmt.Errorf("cannot parse '%v', %v", o.Storage, err)
			}
			// determine whether the opsRequest is a recovery action for volume expansion failure
			if specStorage.Cmp(targetStorage) > 0 &&
				statusStorage.Cmp(targetStorage) <= 0 {
				o.autoApprove = false
				fmt.Fprintln(o.Out, printer.BoldYellow("Warning: this opsRequest is a recovery action for volume expansion failure and will re-create the PersistentVolumeClaims when RECOVER_VOLUME_EXPANSION_FAILURE=false"))
				break
			}
		}
	}
	return nil
}

func (o *OperationsOptions) validateVScale(cluster *appsv1alpha1.Cluster) error {
	if o.Class != "" && (o.CPU != "" || o.Memory != "") {
		return fmt.Errorf("class and cpu/memory cannot be both specified")
	}
	if o.Class == "" && o.CPU == "" && o.Memory == "" {
		return fmt.Errorf("class or cpu/memory must be specified")
	}

	clsMgr, err := class.GetManager(o.Dynamic, cluster.Spec.ClusterDefRef)
	if err != nil {
		return err
	}

	fillClassParams := func(comp *appsv1alpha1.ClusterComponentSpec) error {
		if o.Class != "" {
			clsDefRef := appsv1alpha1.ClassDefRef{}
			parts := strings.SplitN(o.Class, ":", 2)
			if len(parts) == 1 {
				clsDefRef.Class = parts[0]
			} else {
				clsDefRef.Name = parts[0]
				clsDefRef.Class = parts[1]
			}
			comp.ClassDefRef = &clsDefRef
			comp.Resources = corev1.ResourceRequirements{}
		} else {
			comp.ClassDefRef = &appsv1alpha1.ClassDefRef{}
			requests := make(corev1.ResourceList)
			if o.CPU != "" {
				cpu, err := resource.ParseQuantity(o.CPU)
				if err != nil {
					return fmt.Errorf("cannot parse '%v', %v", o.CPU, err)
				}
				requests[corev1.ResourceCPU] = cpu
			}
			if o.Memory != "" {
				memory, err := resource.ParseQuantity(o.Memory)
				if err != nil {
					return fmt.Errorf("cannot parse '%v', %v", o.Memory, err)
				}
				requests[corev1.ResourceMemory] = memory
			}
			requests.DeepCopyInto(&comp.Resources.Requests)
			requests.DeepCopyInto(&comp.Resources.Limits)
		}
		return nil
	}

	for _, name := range o.ComponentNames {
		for _, comp := range cluster.Spec.ComponentSpecs {
			if comp.Name != name {
				continue
			}
			if err = fillClassParams(&comp); err != nil {
				return err
			}
			if err = clsMgr.ValidateResources(cluster.Spec.ClusterDefRef, &comp); err != nil {
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
	obj, err := o.Dynamic.Resource(types.ClusterGVR()).Namespace(o.Namespace).Get(context.TODO(), o.Name, metav1.GetOptions{})
	if err != nil {
		return err
	}
	var cluster appsv1alpha1.Cluster
	if err = runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, &cluster); err != nil {
		return err
	}

	// common validate for componentOps
	if o.HasComponentNamesFlag && len(o.ComponentNames) == 0 {
		return fmt.Errorf(`missing components, please specify the "--components" flag for multi-components cluster`)
	}

	switch o.OpsType {
	case appsv1alpha1.VolumeExpansionType:
		if err = o.validateVolumeExpansion(); err != nil {
			return err
		}
	case appsv1alpha1.UpgradeType:
		if err = o.validateUpgrade(); err != nil {
			return err
		}
	case appsv1alpha1.VerticalScalingType:
		if err = o.validateVScale(&cluster); err != nil {
			return err
		}
	case appsv1alpha1.ExposeType:
		if err = o.validateExpose(); err != nil {
			return err
		}
	case appsv1alpha1.SwitchoverType:
		if err = o.validatePromote(&cluster); err != nil {
			return err
		}
	}
	if !o.autoApprove && o.DryRun == "none" {
		return prompt.Confirm([]string{o.Name}, o.In, "", "")
	}
	return nil
}

func (o *OperationsOptions) validatePromote(cluster *appsv1alpha1.Cluster) error {
	var (
		clusterDefObj = appsv1alpha1.ClusterDefinition{}
		podObj        = &corev1.Pod{}
		componentName string
	)

	if len(cluster.Spec.ComponentSpecs) == 0 {
		return fmt.Errorf("cluster.Spec.ComponentSpecs cannot be empty")
	}

	if o.Component != "" {
		componentName = o.Component
	} else {
		if len(cluster.Spec.ComponentSpecs) > 1 {
			return fmt.Errorf("there are multiple components in cluster, please use --component to specify the component for promote")
		}
		componentName = cluster.Spec.ComponentSpecs[0].Name
	}

	if o.Instance != "" {
		// checks the validity of the instance whether it belongs to the current component and ensure it is not the primary or leader instance currently.
		podKey := client.ObjectKey{
			Namespace: cluster.Namespace,
			Name:      o.Instance,
		}
		if err := util.GetResourceObjectFromGVR(types.PodGVR(), podKey, o.Dynamic, podObj); err != nil || podObj == nil {
			return fmt.Errorf("instance %s not found, please check the validity of the instance using \"kbcli cluster list-instances\"", o.Instance)
		}
		v, ok := podObj.Labels[constant.RoleLabelKey]
		if !ok || v == "" {
			return fmt.Errorf("instance %s cannot be promoted because it had a invalid role label", o.Instance)
		}
		if v == constant.Primary || v == constant.Leader {
			return fmt.Errorf("instance %s cannot be promoted because it is already the primary or leader instance", o.Instance)
		}
		if !strings.HasPrefix(podObj.Name, fmt.Sprintf("%s-%s", cluster.Name, componentName)) {
			return fmt.Errorf("instance %s does not belong to the current component, please check the validity of the instance using \"kbcli cluster list-instances\"", o.Instance)
		}
	}

	// check clusterDefinition switchoverSpec exist
	clusterDefKey := client.ObjectKey{
		Namespace: "",
		Name:      cluster.Spec.ClusterDefRef,
	}
	if err := util.GetResourceObjectFromGVR(types.ClusterDefGVR(), clusterDefKey, o.Dynamic, &clusterDefObj); err != nil {
		return err
	}
	var compDefObj *appsv1alpha1.ClusterComponentDefinition
	for _, compDef := range clusterDefObj.Spec.ComponentDefs {
		if compDef.Name == cluster.Spec.GetComponentDefRefName(componentName) {
			compDefObj = &compDef
			break
		}
	}
	if compDefObj == nil {
		return fmt.Errorf("cluster component %s is invalid", componentName)
	}
	if compDefObj.SwitchoverSpec == nil {
		return fmt.Errorf("cluster component %s does not support switchover", componentName)
	}
	switch o.Instance {
	case "":
		if compDefObj.SwitchoverSpec.WithoutCandidate == nil {
			return fmt.Errorf("cluster component %s does not support promote without specifying an instance. Please specify a specific instance for the promotion", componentName)
		}
	default:
		if compDefObj.SwitchoverSpec.WithCandidate == nil {
			return fmt.Errorf("cluster component %s does not support specifying an instance for promote. If you want to perform a promote operation, please do not specify an instance", componentName)
		}
	}
	return nil
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
		kbcli cluster restart mycluster

		# specified component to restart, separate with commas for multiple components
		kbcli cluster restart mycluster --components=mysql
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
			o.Args = args
			cmdutil.BehaviorOnFatal(printer.FatalWithRedColor)
			cmdutil.CheckErr(o.Complete())
			cmdutil.CheckErr(o.CompleteRestartOps())
			cmdutil.CheckErr(o.Validate())
			cmdutil.CheckErr(o.Run())
		},
	}
	o.addCommonFlags(cmd, f)
	cmd.Flags().BoolVar(&o.autoApprove, "auto-approve", false, "Skip interactive approval before restarting the cluster")
	return cmd
}

var upgradeExample = templates.Examples(`
		# upgrade the cluster to the target version 
		kbcli cluster upgrade mycluster --cluster-version=ac-mysql-8.0.30
`)

// NewUpgradeCmd creates an upgrade command
func NewUpgradeCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := newBaseOperationsOptions(f, streams, appsv1alpha1.UpgradeType, false)
	cmd := &cobra.Command{
		Use:               "upgrade NAME",
		Short:             "Upgrade the cluster version.",
		Example:           upgradeExample,
		ValidArgsFunction: util.ResourceNameCompletionFunc(f, types.ClusterGVR()),
		Run: func(cmd *cobra.Command, args []string) {
			o.Args = args
			cmdutil.BehaviorOnFatal(printer.FatalWithRedColor)
			cmdutil.CheckErr(o.Complete())
			cmdutil.CheckErr(o.Validate())
			cmdutil.CheckErr(o.Run())
		},
	}
	o.addCommonFlags(cmd, f)
	cmd.Flags().StringVar(&o.ClusterVersionRef, "cluster-version", "", "Reference cluster version (required)")
	cmd.Flags().BoolVar(&o.autoApprove, "auto-approve", false, "Skip interactive approval before upgrading the cluster")
	_ = cmd.MarkFlagRequired("cluster-version")
	return cmd
}

var verticalScalingExample = templates.Examples(`
		# scale the computing resources of specified components, separate with commas for multiple components
		kbcli cluster vscale mycluster --components=mysql --cpu=500m --memory=500Mi 

		# scale the computing resources of specified components by class, run command 'kbcli class list --cluster-definition cluster-definition-name' to get available classes
		kbcli cluster vscale mycluster --components=mysql --class=general-2c4g
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
			o.Args = args
			cmdutil.BehaviorOnFatal(printer.FatalWithRedColor)
			cmdutil.CheckErr(o.Complete())
			cmdutil.CheckErr(o.CompleteComponentsFlag())
			cmdutil.CheckErr(o.Validate())
			cmdutil.CheckErr(o.Run())
		},
	}
	o.addCommonFlags(cmd, f)
	cmd.Flags().StringVar(&o.CPU, "cpu", "", "Request and limit size of component cpu")
	cmd.Flags().StringVar(&o.Memory, "memory", "", "Request and limit size of component memory")
	cmd.Flags().StringVar(&o.Class, "class", "", "Component class")
	cmd.Flags().BoolVar(&o.autoApprove, "auto-approve", false, "Skip interactive approval before vertically scaling the cluster")
	_ = cmd.MarkFlagRequired("components")
	return cmd
}

var horizontalScalingExample = templates.Examples(`
		# expand storage resources of specified components, separate with commas for multiple components
		kbcli cluster hscale mycluster --components=mysql --replicas=3
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
			o.Args = args
			cmdutil.BehaviorOnFatal(printer.FatalWithRedColor)
			cmdutil.CheckErr(o.Complete())
			cmdutil.CheckErr(o.CompleteComponentsFlag())
			cmdutil.CheckErr(o.Validate())
			cmdutil.CheckErr(o.Run())
		},
	}

	o.addCommonFlags(cmd, f)
	cmd.Flags().IntVar(&o.Replicas, "replicas", o.Replicas, "Replicas with the specified components")
	cmd.Flags().BoolVar(&o.autoApprove, "auto-approve", false, "Skip interactive approval before horizontally scaling the cluster")
	_ = cmd.MarkFlagRequired("replicas")
	_ = cmd.MarkFlagRequired("components")
	return cmd
}

var volumeExpansionExample = templates.Examples(`
		# restart specifies the component, separate with commas for multiple components
		kbcli cluster volume-expand mycluster --components=mysql --volume-claim-templates=data --storage=10Gi
`)

// NewVolumeExpansionCmd creates a volume expanding command
func NewVolumeExpansionCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := newBaseOperationsOptions(f, streams, appsv1alpha1.VolumeExpansionType, true)
	cmd := &cobra.Command{
		Use:               "volume-expand NAME",
		Short:             "Expand volume with the specified components and volumeClaimTemplates in the cluster.",
		Example:           volumeExpansionExample,
		ValidArgsFunction: util.ResourceNameCompletionFunc(f, types.ClusterGVR()),
		Run: func(cmd *cobra.Command, args []string) {
			o.Args = args
			cmdutil.BehaviorOnFatal(printer.FatalWithRedColor)
			cmdutil.CheckErr(o.Complete())
			cmdutil.CheckErr(o.CompleteComponentsFlag())
			cmdutil.CheckErr(o.Validate())
			cmdutil.CheckErr(o.Run())
		},
	}
	o.addCommonFlags(cmd, f)
	cmd.Flags().StringSliceVarP(&o.VCTNames, "volume-claim-templates", "t", nil, "VolumeClaimTemplate names in components (required)")
	cmd.Flags().StringVar(&o.Storage, "storage", "", "Volume storage size (required)")
	cmd.Flags().BoolVar(&o.autoApprove, "auto-approve", false, "Skip interactive approval before expanding the cluster volume")
	_ = cmd.MarkFlagRequired("volume-claim-templates")
	_ = cmd.MarkFlagRequired("storage")
	_ = cmd.MarkFlagRequired("components")
	return cmd
}

var (
	exposeExamples = templates.Examples(`
		# Expose a cluster to vpc
		kbcli cluster expose mycluster --type vpc --enable=true

		# Expose a cluster to public internet
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
			o.Args = args
			cmdutil.BehaviorOnFatal(printer.FatalWithRedColor)
			cmdutil.CheckErr(o.Complete())
			cmdutil.CheckErr(o.CompleteComponentsFlag())
			cmdutil.CheckErr(o.fillExpose())
			cmdutil.CheckErr(o.Validate())
			cmdutil.CheckErr(o.Run())
		},
	}
	o.addCommonFlags(cmd, f)
	cmd.Flags().StringVar(&o.ExposeType, "type", "", "Expose type, currently supported types are 'vpc', 'internet'")
	cmd.Flags().StringVar(&o.ExposeEnabled, "enable", "", "Enable or disable the expose, values can be true or false")
	cmd.Flags().BoolVar(&o.autoApprove, "auto-approve", false, "Skip interactive approval before exposing the cluster")

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
		kbcli cluster stop mycluster
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
			o.Args = args
			cmdutil.BehaviorOnFatal(printer.FatalWithRedColor)
			cmdutil.CheckErr(o.Complete())
			cmdutil.CheckErr(o.Validate())
			cmdutil.CheckErr(o.Run())
		},
	}
	o.addCommonFlags(cmd, f)
	cmd.Flags().BoolVar(&o.autoApprove, "auto-approve", false, "Skip interactive approval before stopping the cluster")
	return cmd
}

var startExample = templates.Examples(`
		# start the cluster when cluster is stopped
		kbcli cluster start mycluster
`)

// NewStartCmd creates a start command
func NewStartCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := newBaseOperationsOptions(f, streams, appsv1alpha1.StartType, false)
	o.autoApprove = true
	cmd := &cobra.Command{
		Use:               "start NAME",
		Short:             "Start the cluster if cluster is stopped.",
		Example:           startExample,
		ValidArgsFunction: util.ResourceNameCompletionFunc(f, types.ClusterGVR()),
		Run: func(cmd *cobra.Command, args []string) {
			o.Args = args
			cmdutil.BehaviorOnFatal(printer.FatalWithRedColor)
			cmdutil.CheckErr(o.Complete())
			cmdutil.CheckErr(o.Validate())
			cmdutil.CheckErr(o.Run())
		},
	}
	o.addCommonFlags(cmd, f)
	return cmd
}

var cancelExample = templates.Examples(`
		# cancel the opsRequest which is not completed.
		kbcli cluster cancel-ops <opsRequestName>
`)

func cancelOps(o *OperationsOptions) error {
	opsRequest := &appsv1alpha1.OpsRequest{}
	if err := cluster.GetK8SClientObject(o.Dynamic, opsRequest, o.GVR, o.Namespace, o.Name); err != nil {
		return err
	}
	notSupportedPhases := []appsv1alpha1.OpsPhase{appsv1alpha1.OpsFailedPhase, appsv1alpha1.OpsSucceedPhase, appsv1alpha1.OpsCancelledPhase}
	if slices.Contains(notSupportedPhases, opsRequest.Status.Phase) {
		return fmt.Errorf("can not cancel the opsRequest when phase is %s", opsRequest.Status.Phase)
	}
	if opsRequest.Status.Phase == appsv1alpha1.OpsCancellingPhase {
		return fmt.Errorf(`opsRequest "%s" is cancelling`, opsRequest.Name)
	}
	supportedType := []appsv1alpha1.OpsType{appsv1alpha1.HorizontalScalingType, appsv1alpha1.VerticalScalingType}
	if !slices.Contains(supportedType, opsRequest.Spec.Type) {
		return fmt.Errorf("opsRequest type: %s not support cancel action", opsRequest.Spec.Type)
	}
	if !o.autoApprove {
		if err := prompt.Confirm([]string{o.Name}, o.In, "", ""); err != nil {
			return err
		}
	}
	oldOps := opsRequest.DeepCopy()
	opsRequest.Spec.Cancel = true
	oldData, err := json.Marshal(oldOps)
	if err != nil {
		return err
	}
	newData, err := json.Marshal(opsRequest)
	if err != nil {
		return err
	}
	patchBytes, err := jsonpatch.CreateMergePatch(oldData, newData)
	if err != nil {
		return err
	}
	if _, err = o.Dynamic.Resource(types.OpsGVR()).Namespace(opsRequest.Namespace).Patch(context.TODO(),
		opsRequest.Name, apitypes.MergePatchType, patchBytes, metav1.PatchOptions{}); err != nil {
		return err
	}
	fmt.Fprintf(o.Out, "start to cancel opsRequest \"%s\", you can view the progress:\n\tkbcli cluster list-ops --name %s\n", o.Name, o.Name)
	return nil
}

func NewCancelCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := newBaseOperationsOptions(f, streams, "", false)
	cmd := &cobra.Command{
		Use:               "cancel-ops NAME",
		Short:             "Cancel the pending/creating/running OpsRequest which type is vscale or hscale.",
		Example:           cancelExample,
		ValidArgsFunction: util.ResourceNameCompletionFunc(f, types.OpsGVR()),
		Run: func(cmd *cobra.Command, args []string) {
			o.Args = args
			cmdutil.BehaviorOnFatal(printer.FatalWithRedColor)
			cmdutil.CheckErr(o.Complete())
			cmdutil.CheckErr(cancelOps(o))
		},
	}
	cmd.Flags().BoolVar(&o.autoApprove, "auto-approve", false, "Skip interactive approval before cancel the opsRequest")
	return cmd
}

var promoteExample = templates.Examples(`
		# Promote the instance mycluster-mysql-1 as the new primary or leader.
		kbcli cluster promote mycluster --instance mycluster-mysql-1

		# Promote a non-primary or non-leader instance as the new primary or leader, the new primary or leader is determined by the system.
		kbcli cluster promote mycluster

		# If the cluster has multiple components, you need to specify a component, otherwise an error will be reported.
	    kbcli cluster promote mycluster --component=mysql --instance mycluster-mysql-1
`)

// NewPromoteCmd creates a promote command
func NewPromoteCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := newBaseOperationsOptions(f, streams, appsv1alpha1.SwitchoverType, false)
	cmd := &cobra.Command{
		Use:               "promote NAME [--component=<comp-name>] [--instance <instance-name>]",
		Short:             "Promote a non-primary or non-leader instance as the new primary or leader of the cluster",
		Example:           promoteExample,
		ValidArgsFunction: util.ResourceNameCompletionFunc(f, types.ClusterGVR()),
		Run: func(cmd *cobra.Command, args []string) {
			o.Args = args
			cmdutil.BehaviorOnFatal(printer.FatalWithRedColor)
			cmdutil.CheckErr(o.Complete())
			cmdutil.CheckErr(o.CompleteComponentsFlag())
			cmdutil.CheckErr(o.Validate())
			cmdutil.CheckErr(o.Run())
		},
	}
	cmd.Flags().StringVar(&o.Component, "component", "", "Specify the component name of the cluster, if the cluster has multiple components, you need to specify a component")
	cmd.Flags().StringVar(&o.Instance, "instance", "", "Specify the instance name as the new primary or leader of the cluster, you can get the instance name by running \"kbcli cluster list-instances\"")
	cmd.Flags().BoolVar(&o.autoApprove, "auto-approve", false, "Skip interactive approval before promote the instance")
	o.addCommonFlags(cmd, f)
	return cmd
}
