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
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/cli/create"
	"github.com/apecloud/kubeblocks/internal/cli/delete"
	"github.com/apecloud/kubeblocks/internal/cli/printer"
	"github.com/apecloud/kubeblocks/internal/cli/types"
	"github.com/apecloud/kubeblocks/internal/cli/util"
	"github.com/apecloud/kubeblocks/internal/cli/util/prompt"
	cfgcore "github.com/apecloud/kubeblocks/internal/configuration"
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
	URLPath         string            `json:"urlPath"`
	Parameters      []string          `json:"parameters"`
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
		BaseOptions: create.BaseOptions{IOStreams: streams},
		OpsType:     opsType,
		// nil cannot be set to a map struct in CueLang, so init the map of KeyValues.
		KeyValues:             map[string]string{},
		HasComponentNamesFlag: hasComponentNamesFlag,
		RequireConfirm:        true,
	}
}

var (
	createReconfigureExample = templates.Examples(`
		# update component params 
		kbcli cluster configure <cluster-name> --component-name=<component-name> --template-name=<template-name> --configure-file=<configure-file> --set max_connections=1000,general_log=OFF

		# update mysql max_connections, cluster name is mycluster
		kbcli cluster configure mycluster --component-name=mysql --template-name=mysql-3node-tpl --configure-file=my.cnf --set max_connections=2000
	`)
)

// buildCommonFlags build common flags for operations command
func (o *OperationsOptions) buildCommonFlags(cmd *cobra.Command) {
	cmd.Flags().StringVar(&o.OpsRequestName, "name", "", "OpsRequest name. if not specified, it will be randomly generated ")
	cmd.Flags().IntVar(&o.TTLSecondsAfterSucceed, "ttlSecondsAfterSucceed", 0, "Time to live after the OpsRequest succeed")
	if o.HasComponentNamesFlag {
		cmd.Flags().StringSliceVar(&o.ComponentNames, "component-names", nil, " Component names to this operations")
	}
}

// CompleteRestartOps when restart a cluster and component-names is null, it means restarting all components of the cluster.
// we should set all component names to ComponentNames flag.
func (o *OperationsOptions) CompleteRestartOps() error {
	if o.Name == "" {
		return makeMissingClusterNameErr()
	}
	if len(o.ComponentNames) != 0 {
		return nil
	}
	unstructuredObj, err := o.Dynamic.Resource(types.ClusterGVR()).Namespace(o.Namespace).Get(context.TODO(), o.Name, metav1.GetOptions{})
	if err != nil {
		return err
	}
	cluster := appsv1alpha1.Cluster{}
	err = runtime.DefaultUnstructuredConverter.
		FromUnstructured(unstructuredObj.UnstructuredContent(), &cluster)
	if err != nil {
		return err
	}
	componentSpecs := cluster.Spec.ComponentSpecs
	o.ComponentNames = make([]string, len(componentSpecs))
	for i := range componentSpecs {
		o.ComponentNames[i] = componentSpecs[i].Name
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

func (o *OperationsOptions) validateReconfiguring() error {
	if len(o.ComponentNames) != 1 {
		return cfgcore.MakeError("reconfiguring only support one component.")
	}
	componentName := o.ComponentNames[0]
	if err := o.existClusterAndComponent(componentName); err != nil {
		return err
	}

	tplList, err := util.GetConfigTemplateList(o.Name, o.Namespace, o.Dynamic, componentName, true)
	if err != nil {
		return err
	}
	tpl, err := o.validateTemplateParam(tplList)
	if err != nil {
		return err
	}
	if err := o.validateConfigMapKey(tpl, componentName); err != nil {
		return err
	}
	if err := o.validateConfigParams(tpl); err != nil {
		return err
	}
	o.printConfigureTips()
	return nil
}

func (o *OperationsOptions) validateConfigParams(tpl *appsv1alpha1.ComponentConfigSpec) error {
	transKeyPair := func(pts map[string]string) map[string]interface{} {
		m := make(map[string]interface{}, len(pts))
		for key, value := range pts {
			m[key] = value
		}
		return m
	}

	configConstraintKey := client.ObjectKey{
		Namespace: "",
		Name:      tpl.ConfigConstraintRef,
	}
	configConstraint := appsv1alpha1.ConfigConstraint{}
	if err := util.GetResourceObjectFromGVR(types.ConfigConstraintGVR(), configConstraintKey, o.Dynamic, &configConstraint); err != nil {
		return err
	}

	newConfigData, err := cfgcore.MergeAndValidateConfigs(configConstraint.Spec, map[string]string{o.CfgFile: ""}, tpl.Keys, []cfgcore.ParamPairs{{
		Key:           o.CfgFile,
		UpdatedParams: transKeyPair(o.KeyValues),
	}})
	if err != nil {
		return err
	}
	return o.checkChangedParamsAndDoubleConfirm(&configConstraint.Spec, newConfigData, tpl)
}

func (o *OperationsOptions) checkChangedParamsAndDoubleConfirm(cc *appsv1alpha1.ConfigConstraintSpec, data map[string]string, tpl *appsv1alpha1.ComponentConfigSpec) error {
	mockEmptyData := func(m map[string]string) map[string]string {
		r := make(map[string]string, len(data))
		for key := range m {
			r[key] = ""
		}
		return r
	}

	configPatch, _, err := cfgcore.CreateConfigPatch(mockEmptyData(data), data, cc.FormatterConfig.Format, tpl.Keys, false)
	if err != nil {
		return err
	}

	dynamicUpdated, err := cfgcore.IsUpdateDynamicParameters(cc, configPatch)
	if err != nil {
		return nil
	}
	if dynamicUpdated {
		return nil
	}
	return o.confirmReconfigureWithRestart()
}

func (o *OperationsOptions) confirmReconfigureWithRestart() error {
	const confirmStr = "yes"
	printer.Warning(o.Out, restartConfirmPrompt)
	_, err := prompt.NewPrompt(fmt.Sprintf("Please type \"%s\" to confirm:", confirmStr),
		func(input string) error {
			if input != confirmStr {
				return fmt.Errorf("typed \"%s\" does not match \"%s\"", input, confirmStr)
			}
			return nil
		}, o.In).Run()
	return err
}

func (o *OperationsOptions) validateTemplateParam(tpls []appsv1alpha1.ComponentConfigSpec) (*appsv1alpha1.ComponentConfigSpec, error) {
	if len(tpls) == 0 {
		return nil, cfgcore.MakeError("not support reconfiguring because there is no config template.")
	}

	if len(o.CfgTemplateName) == 0 && len(tpls) > 1 {
		return nil, cfgcore.MakeError("when multi templates exist, must specify which template to use.")
	}

	// Autofill Config template name.
	if len(o.CfgTemplateName) == 0 && len(tpls) == 1 {
		tpl := &tpls[0]
		o.CfgTemplateName = tpl.Name
		return tpl, nil
	}

	for i := range tpls {
		tpl := &tpls[i]
		if tpl.Name == o.CfgTemplateName {
			return tpl, nil
		}
	}
	return nil, cfgcore.MakeError("specify template name[%s] is not exist.", o.CfgTemplateName)
}

func (o *OperationsOptions) validateConfigMapKey(tpl *appsv1alpha1.ComponentConfigSpec, componentName string) error {
	var (
		cmObj  = corev1.ConfigMap{}
		cmName = cfgcore.GetComponentCfgName(o.Name, componentName, tpl.VolumeName)
	)

	if err := util.GetResourceObjectFromGVR(types.ConfigmapGVR(), client.ObjectKey{
		Name:      cmName,
		Namespace: o.Namespace,
	}, o.Dynamic, &cmObj); err != nil {
		return err
	}
	if len(cmObj.Data) == 0 {
		return cfgcore.MakeError("not support reconfiguring because there is no config file.")
	}

	// Autofill ConfigMap key
	if o.CfgFile == "" && len(cmObj.Data) > 0 {
		o.fillKeyForReconfiguring(tpl, cmObj.Data)
	}
	if _, ok := cmObj.Data[o.CfgFile]; !ok {
		return cfgcore.MakeError("specify file name[%s] is not exist.", o.CfgFile)
	}
	return nil
}

func (o *OperationsOptions) parseUpdatedParams() error {
	if len(o.Parameters) == 0 && len(o.URLPath) == 0 {
		return cfgcore.MakeError("reconfiguring required configure file or updated parameters.")
	}

	o.KeyValues = make(map[string]string)
	for _, param := range o.Parameters {
		pp := strings.Split(param, ",")
		for _, p := range pp {
			fields := strings.SplitN(p, "=", 2)
			if len(fields) != 2 {
				return cfgcore.MakeError("updated parameter format: key=value")
			}
			o.KeyValues[fields[0]] = fields[1]
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
	_, err := o.Dynamic.Resource(types.ClusterGVR()).Namespace(o.Namespace).Get(context.TODO(), o.Name, metav1.GetOptions{})
	if err != nil {
		return err
	}

	// not require confirm for reconfigure
	if o.OpsType == appsv1alpha1.ReconfiguringType {
		return o.validateReconfiguring()
	}

	// common validate for componentOps
	if o.HasComponentNamesFlag && len(o.ComponentNames) == 0 {
		return fmt.Errorf("missing component-names")
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

func (o *OperationsOptions) fillTemplateArgForReconfiguring() error {
	if o.Name == "" {
		return makeMissingClusterNameErr()
	}

	if err := o.fillComponentNameForReconfiguring(); err != nil {
		return err
	}

	if err := o.parseUpdatedParams(); err != nil {
		return err
	}
	if len(o.KeyValues) == 0 {
		return cfgcore.MakeError(missingUpdatedParametersErrMessage)
	}

	componentName := o.ComponentNames[0]
	tplList, err := util.GetConfigTemplateList(o.Name, o.Namespace, o.Dynamic, componentName, true)
	if err != nil {
		return err
	}

	if len(tplList) == 0 {
		return makeNotFoundTemplateErr(o.Name, componentName)
	}

	if len(tplList) == 1 {
		o.CfgTemplateName = tplList[0].Name
		return nil
	}

	supportUpdatedTpl := make([]appsv1alpha1.ComponentConfigSpec, 0)
	for _, tpl := range tplList {
		if ok, err := util.IsSupportReconfigureParams(tpl, o.KeyValues, o.Dynamic); err == nil && ok {
			supportUpdatedTpl = append(supportUpdatedTpl, tpl)
		}
	}
	if len(supportUpdatedTpl) == 1 {
		o.CfgTemplateName = supportUpdatedTpl[0].Name
		return nil
	}

	return cfgcore.MakeError(multiConfigTemplateErrorMessage)
}

func (o *OperationsOptions) fillComponentNameForReconfiguring() error {
	if len(o.ComponentNames) != 0 {
		return nil
	}

	componentNames, err := util.GetComponentsFromClusterCR(client.ObjectKey{
		Namespace: o.Namespace,
		Name:      o.Name,
	}, o.Dynamic)
	if err != nil {
		return err
	}
	if len(componentNames) != 1 {
		return cfgcore.MakeError(multiComponentsErrorMessage)
	}
	o.ComponentNames = componentNames
	return nil
}

func (o *OperationsOptions) existClusterAndComponent(componentName string) error {
	clusterObj := appsv1alpha1.Cluster{}
	if err := util.GetResourceObjectFromGVR(types.ClusterGVR(), client.ObjectKey{
		Namespace: o.Namespace,
		Name:      o.Name,
	}, o.Dynamic, &clusterObj); err != nil {
		return makeClusterNotExistErr(o.Name)
	}

	for _, component := range clusterObj.Spec.ComponentSpecs {
		if component.Name == componentName {
			return nil
		}
	}
	return makeComponentNotExistErr(o.Name, componentName)
}

func (o *OperationsOptions) printConfigureTips() {
	fmt.Println("Will updated configure file meta:")
	printer.PrintLineWithTabSeparator(
		printer.NewPair("  TemplateName", printer.BoldYellow(o.CfgTemplateName)),
		printer.NewPair("  ConfigureFile", printer.BoldYellow(o.CfgFile)),
		printer.NewPair("ComponentName", o.ComponentNames[0]),
		printer.NewPair("ClusterName", o.Name))
}

func (o *OperationsOptions) fillKeyForReconfiguring(tpl *appsv1alpha1.ComponentConfigSpec, data map[string]string) {
	keys := make([]string, 0, len(data))
	for k := range data {
		if cfgcore.CheckConfigTemplateReconfigureKey(*tpl, k) {
			keys = append(keys, k)
		}
	}
	if len(keys) == 1 {
		o.CfgFile = keys[0]
	}
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
	provider, err := util.GetK8SProvider(o.Client)
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
			return fmt.Errorf("please specify --component-names")
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
		kbcli cluster restart <my-cluster> --component-names=<component-name>
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
		kbcli cluster vscale <my-cluster> --component-names=<component-name> --cpu=500m --memory=500Mi 
`)

// NewVerticalScalingCmd creates a vertical scaling command
func NewVerticalScalingCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := newBaseOperationsOptions(streams, appsv1alpha1.VerticalScalingType, true)
	inputs := buildOperationsInputs(f, o)
	inputs.Use = "vscale"
	inputs.Short = "Vertically scale the specified components in the cluster."
	inputs.Example = verticalScalingExample
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
		kbcli cluster hscale <my-cluster> --component-names=<component-name> --replicas=3
`)

// NewHorizontalScalingCmd creates a horizontal scaling command
func NewHorizontalScalingCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := newBaseOperationsOptions(streams, appsv1alpha1.HorizontalScalingType, true)
	inputs := buildOperationsInputs(f, o)
	inputs.Use = "hscale"
	inputs.Short = "Horizontally scale the specified components in the cluster."
	inputs.Example = horizontalScalingExample
	inputs.BuildFlags = func(cmd *cobra.Command) {
		o.buildCommonFlags(cmd)
		cmd.Flags().IntVar(&o.Replicas, "replicas", o.Replicas, "Replicas with the specified components")
		_ = cmd.MarkFlagRequired("replicas")
	}
	return create.BuildCommand(inputs)
}

var volumeExpansionExample = templates.Examples(`
		# restart specifies the component, separate with commas when <component-name> more than one
		kbcli cluster volume-expand <my-cluster> --component-names=<component-name> \ 
  		--volume-claim-template-names=data --storage=10Gi
`)

// NewVolumeExpansionCmd creates a vertical scaling command
func NewVolumeExpansionCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := newBaseOperationsOptions(streams, appsv1alpha1.VolumeExpansionType, true)
	inputs := buildOperationsInputs(f, o)
	inputs.Use = "volume-expand"
	inputs.Short = "Expand volume with the specified components and volumeClaimTemplates in the cluster."
	inputs.Example = volumeExpansionExample
	inputs.BuildFlags = func(cmd *cobra.Command) {
		o.buildCommonFlags(cmd)
		cmd.Flags().StringSliceVar(&o.VCTNames, "volume-claim-template-names", nil, "VolumeClaimTemplate names in components (required)")
		cmd.Flags().StringVar(&o.Storage, "storage", "", "Volume storage size (required)")
	}
	return create.BuildCommand(inputs)
}

// NewReconfigureCmd creates a Reconfiguring command
func NewReconfigureCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := newBaseOperationsOptions(streams, appsv1alpha1.ReconfiguringType, false)
	inputs := buildOperationsInputs(f, o)
	inputs.Use = "configure"
	inputs.Short = "Reconfigure parameters with the specified components in the cluster."
	inputs.Example = createReconfigureExample
	inputs.BuildFlags = func(cmd *cobra.Command) {
		o.buildCommonFlags(cmd)
		cmd.Flags().StringSliceVar(&o.Parameters, "set", nil, "Specify updated parameter list. For details about the parameters, refer to kbcli sub command: 'kbcli cluster describe-configure'.")
		cmd.Flags().StringSliceVar(&o.ComponentNames, "component-name", nil, "Specify the name of Component to be updated. If the cluster has only one component, unset the parameter.")
		cmd.Flags().StringVar(&o.CfgTemplateName, "template-name", "", "Specify the name of the configuration template to be updated (e.g. for apecloud-mysql: --template-name=mysql-3node-tpl). What templates or configure files are available for this cluster can refer to kbcli sub command: 'kbcli cluster describe-configure'.")
		cmd.Flags().StringVar(&o.CfgFile, "configure-file", "", "Specify the name of the configuration file to be updated (e.g. for mysql: --configure-file=my.cnf). What templates or configure files are available for this cluster can refer to kbcli sub command: 'kbcli cluster describe-configure'.")
	}
	inputs.Complete = o.fillTemplateArgForReconfiguring
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

// NewExposeCmd creates a Expose command
func NewExposeCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := newBaseOperationsOptions(streams, appsv1alpha1.ExposeType, true)
	inputs := buildOperationsInputs(f, o)
	inputs.Use = "expose"
	inputs.Short = "Expose a cluster."
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
