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
	"fmt"
	"reflect"

	"github.com/spf13/cast"
	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericiooptions"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/util/templates"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/cli/printer"
	"github.com/apecloud/kubeblocks/pkg/cli/types"
	"github.com/apecloud/kubeblocks/pkg/cli/util"
	"github.com/apecloud/kubeblocks/pkg/configuration/core"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/unstructured"
)

type configDiffOptions struct {
	baseOptions *describeOpsOptions

	clusterName   string
	componentName string
	templateNames []string
	baseVersion   *appsv1alpha1.OpsRequest
	diffVersion   *appsv1alpha1.OpsRequest
}

var (
	diffConfigureExample = templates.Examples(`
		# compare config files
		kbcli cluster diff-config opsrequest1 opsrequest2`)
)

func (o *configDiffOptions) complete(args []string) error {
	isValidReconfigureOps := func(ops *appsv1alpha1.OpsRequest) bool {
		return ops.Spec.Type == appsv1alpha1.ReconfiguringType && ops.Spec.Reconfigure != nil
	}

	if len(args) != 2 {
		return core.MakeError("missing opsrequest name")
	}

	if err := o.baseOptions.complete(args); err != nil {
		return err
	}

	baseVersion := &appsv1alpha1.OpsRequest{}
	diffVersion := &appsv1alpha1.OpsRequest{}
	if err := util.GetResourceObjectFromGVR(types.OpsGVR(), client.ObjectKey{
		Namespace: o.baseOptions.namespace,
		Name:      args[0],
	}, o.baseOptions.dynamic, baseVersion); err != nil {
		return core.WrapError(err, "failed to get ops CR [%s]", args[0])
	}
	if err := util.GetResourceObjectFromGVR(types.OpsGVR(), client.ObjectKey{
		Namespace: o.baseOptions.namespace,
		Name:      args[1],
	}, o.baseOptions.dynamic, diffVersion); err != nil {
		return core.WrapError(err, "failed to get ops CR [%s]", args[1])
	}

	if !isValidReconfigureOps(baseVersion) {
		return core.MakeError("opsrequest is not valid reconfiguring operation [%s]", client.ObjectKeyFromObject(baseVersion))
	}

	if !isValidReconfigureOps(diffVersion) {
		return core.MakeError("opsrequest is not valid reconfiguring operation [%s]", client.ObjectKeyFromObject(diffVersion))
	}

	if !o.maybeCompareOps(baseVersion, diffVersion) {
		return core.MakeError("failed to diff, not same cluster, or same component, or template.")
	}

	o.baseVersion = baseVersion
	o.diffVersion = diffVersion
	return nil
}

func findTemplateStatusByName(status *appsv1alpha1.ReconfiguringStatus, tplName string) *appsv1alpha1.ConfigurationItemStatus {
	if status == nil {
		return nil
	}

	for i := range status.ConfigurationStatus {
		s := &status.ConfigurationStatus[i]
		if s.Name == tplName {
			return s
		}
	}
	return nil
}

func (o *configDiffOptions) validate() error {
	var (
		baseStatus = o.baseVersion.Status
		diffStatus = o.diffVersion.Status
	)

	if baseStatus.Phase != appsv1alpha1.OpsSucceedPhase {
		return core.MakeError("require reconfiguring phase is success!, name: %s, phase: %s", o.baseVersion.Name, baseStatus.Phase)
	}
	if diffStatus.Phase != appsv1alpha1.OpsSucceedPhase {
		return core.MakeError("require reconfiguring phase is success!, name: %s, phase: %s", o.diffVersion.Name, diffStatus.Phase)
	}

	for _, tplName := range o.templateNames {
		s1 := findTemplateStatusByName(baseStatus.ReconfiguringStatus, tplName)
		s2 := findTemplateStatusByName(diffStatus.ReconfiguringStatus, tplName)
		if s1 == nil || len(s1.LastAppliedConfiguration) == 0 {
			return core.MakeError("invalid reconfiguring status. CR[%v]", client.ObjectKeyFromObject(o.baseVersion))
		}
		if s2 == nil || len(s2.LastAppliedConfiguration) == 0 {
			return core.MakeError("invalid reconfiguring status. CR[%v]", client.ObjectKeyFromObject(o.diffVersion))
		}
	}
	return nil
}

func (o *configDiffOptions) run() error {
	configDiffs := make(map[string][]core.VisualizedParam, len(o.templateNames))
	baseConfigs := make(map[string]map[string]unstructured.ConfigObject)
	for _, tplName := range o.templateNames {
		diff, baseObj, err := o.diffConfig(tplName)
		if err != nil {
			return err
		}
		configDiffs[tplName] = diff
		baseConfigs[tplName] = baseObj
	}

	printer.PrintTitle("DIFF-CONFIG RESULT")
	for tplName, diff := range configDiffs {
		configObjects := baseConfigs[tplName]
		for _, params := range diff {
			printer.PrintLineWithTabSeparator(
				printer.NewPair("  ConfigFile", printer.BoldYellow(params.Key)),
				printer.NewPair("TemplateName", tplName),
				printer.NewPair("ComponentName", o.componentName),
				printer.NewPair("ClusterName", o.clusterName),
				printer.NewPair("UpdateType", string(params.UpdateType)),
			)
			fmt.Fprintf(o.baseOptions.Out, "\n")
			tbl := printer.NewTablePrinter(o.baseOptions.Out)
			tbl.SetHeader("ParameterName", o.baseVersion.Name, o.diffVersion.Name)
			configObj := configObjects[params.Key]
			for _, v := range params.Parameters {
				baseValue := "null"
				if configObj != nil {
					baseValue = cast.ToString(configObj.Get(v.Key))
				}
				tbl.AddRow(v.Key, baseValue, v.Value)
			}
			tbl.Print()
			fmt.Fprintf(o.baseOptions.Out, "\n\n")
		}
	}
	return nil
}

func (o *configDiffOptions) maybeCompareOps(base *appsv1alpha1.OpsRequest, diff *appsv1alpha1.OpsRequest) bool {
	getClusterName := func(ops client.Object) string {
		labels := ops.GetLabels()
		if len(labels) == 0 {
			return ""
		}
		return labels[constant.AppInstanceLabelKey]
	}
	getComponentName := func(ops appsv1alpha1.OpsRequestSpec) string {
		return ops.Reconfigure.ComponentName
	}
	getTemplateName := func(ops appsv1alpha1.OpsRequestSpec) []string {
		configs := ops.Reconfigure.Configurations
		names := make([]string, len(configs))
		for i, config := range configs {
			names[i] = config.Name
		}
		return names
	}

	clusterName := getClusterName(base)
	if len(clusterName) == 0 || clusterName != getClusterName(diff) {
		return false
	}
	componentName := getComponentName(base.Spec)
	if len(componentName) == 0 || componentName != getComponentName(diff.Spec) {
		return false
	}
	templateNames := getTemplateName(base.Spec)
	if len(templateNames) == 0 || !reflect.DeepEqual(templateNames, getTemplateName(diff.Spec)) {
		return false
	}

	o.clusterName = clusterName
	o.componentName = componentName
	o.templateNames = templateNames
	return true
}

func (o *configDiffOptions) diffConfig(tplName string) ([]core.VisualizedParam, map[string]unstructured.ConfigObject, error) {
	var (
		tpl              *appsv1alpha1.ComponentConfigSpec
		configConstraint = &appsv1alpha1.ConfigConstraint{}
	)

	tplList, err := util.GetConfigTemplateList(o.clusterName, o.baseOptions.namespace, o.baseOptions.dynamic, o.componentName, true)
	if err != nil {
		return nil, nil, err
	}
	if tpl = findTplByName(tplList, tplName); tpl == nil {
		return nil, nil, core.MakeError("not found template: %s", tplName)
	}
	if err := util.GetResourceObjectFromGVR(types.ConfigConstraintGVR(), client.ObjectKey{
		Namespace: "",
		Name:      tpl.ConfigConstraintRef,
	}, o.baseOptions.dynamic, configConstraint); err != nil {
		return nil, nil, err
	}

	formatCfg := configConstraint.Spec.FormatterConfig

	base := findTemplateStatusByName(o.baseVersion.Status.ReconfiguringStatus, tplName)
	diff := findTemplateStatusByName(o.diffVersion.Status.ReconfiguringStatus, tplName)
	patch, _, err := core.CreateConfigPatch(base.LastAppliedConfiguration, diff.LastAppliedConfiguration, formatCfg.Format, tpl.Keys, false)
	if err != nil {
		return nil, nil, err
	}

	baseConfigObj, err := core.LoadRawConfigObject(base.LastAppliedConfiguration, formatCfg, tpl.Keys)
	if err != nil {
		return nil, nil, err
	}
	return core.GenerateVisualizedParamsList(patch, formatCfg, nil), baseConfigObj, nil
}

// NewDiffConfigureCmd shows the difference between two configuration version.
func NewDiffConfigureCmd(f cmdutil.Factory, streams genericiooptions.IOStreams) *cobra.Command {
	o := &configDiffOptions{baseOptions: newDescribeOpsOptions(f, streams)}
	cmd := &cobra.Command{
		Use:               "diff-config",
		Short:             "Show the difference in parameters between the two submitted OpsRequest.",
		Aliases:           []string{"diff"},
		Example:           diffConfigureExample,
		ValidArgsFunction: util.ResourceNameCompletionFunc(f, types.OpsGVR()),
		Run: func(cmd *cobra.Command, args []string) {
			util.CheckErr(o.complete(args))
			util.CheckErr(o.validate())
			util.CheckErr(o.run())
		},
	}
	return cmd
}
