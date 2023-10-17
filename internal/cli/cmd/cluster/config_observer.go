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
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	apiext "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/cli-runtime/pkg/genericiooptions"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/util/templates"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/cli/printer"
	"github.com/apecloud/kubeblocks/internal/cli/types"
	"github.com/apecloud/kubeblocks/internal/cli/util"
	"github.com/apecloud/kubeblocks/internal/cli/util/flags"
	cfgcore "github.com/apecloud/kubeblocks/internal/configuration/core"
	"github.com/apecloud/kubeblocks/internal/configuration/openapi"
	cfgutil "github.com/apecloud/kubeblocks/internal/configuration/util"
	"github.com/apecloud/kubeblocks/internal/constant"
)

type configObserverOptions struct {
	*describeOpsOptions

	clusterName    string
	componentNames []string
	configSpecs    []string

	isExplain     bool
	truncEnum     bool
	truncDocument bool
	paramName     string

	keys       []string
	showDetail bool
}

var (
	describeReconfigureExample = templates.Examples(`
		# describe a cluster, e.g. cluster name is mycluster
		kbcli cluster describe-config mycluster

		# describe a component, e.g. cluster name is mycluster, component name is mysql
		kbcli cluster describe-config mycluster --component=mysql

		# describe all configuration files.
		kbcli cluster describe-config mycluster --component=mysql --show-detail

		# describe a content of configuration file.
		kbcli cluster describe-config mycluster --component=mysql --config-file=my.cnf --show-detail`)
	explainReconfigureExample = templates.Examples(`
		# explain a cluster, e.g. cluster name is mycluster
		kbcli cluster explain-config mycluster

		# explain a specified configure template, e.g. cluster name is mycluster
		kbcli cluster explain-config mycluster --component=mysql --config-specs=mysql-3node-tpl

		# explain a specified configure template, e.g. cluster name is mycluster
		kbcli cluster explain-config mycluster --component=mysql --config-specs=mysql-3node-tpl --trunc-document=false --trunc-enum=false

		# explain a specified parameters, e.g. cluster name is mycluster
		kbcli cluster explain-config mycluster --param=sql_mode`)
)

func (r *configObserverOptions) addCommonFlags(cmd *cobra.Command, f cmdutil.Factory) {
	cmd.Flags().StringSliceVar(&r.configSpecs, "config-specs", nil, "Specify the name of the configuration template to describe. (e.g. for apecloud-mysql: --config-specs=mysql-3node-tpl)")
	flags.AddComponentsFlag(f, cmd, &r.componentNames, "Specify the name of Component to describe (e.g. for apecloud-mysql: --component=mysql). If the cluster has only one component, unset the parameter.\"")
}

func (r *configObserverOptions) complete2(args []string) error {
	if len(args) == 0 {
		return makeMissingClusterNameErr()
	}
	r.clusterName = args[0]
	return r.complete(args)
}

func (r *configObserverOptions) run(printFn func(objects *ConfigRelatedObjects, component string) error) error {
	objects, err := New(r.clusterName, r.namespace, r.dynamic, r.componentNames...).GetObjects()
	if err != nil {
		return err
	}

	components := r.componentNames
	if len(components) == 0 {
		components = getComponentNames(objects.Cluster)
	}

	for _, component := range components {
		fmt.Fprintf(r.Out, "component: %s\n", component)
		if _, ok := objects.ConfigSpecs[component]; !ok {
			fmt.Fprintf(r.Out, "not found component: %s and pass\n\n", component)
		}
		if err := printFn(objects, component); err != nil {
			return err
		}
	}
	return nil
}

func (r *configObserverOptions) printComponentConfigSpecsDescribe(objects *ConfigRelatedObjects, component string) error {
	configSpecs, ok := objects.ConfigSpecs[component]
	if !ok {
		return cfgcore.MakeError("not found component: %s", component)
	}
	configs, err := r.getReconfigureMeta(configSpecs)
	if err != nil {
		return err
	}
	if r.showDetail {
		r.printConfigureContext(configs, component)
	}
	printer.PrintComponentConfigMeta(configs, r.clusterName, component, r.Out)
	return r.printConfigureHistory(component)
}

func (r *configObserverOptions) printComponentExplainConfigure(objects *ConfigRelatedObjects, component string) error {
	configSpecs := r.configSpecs
	if len(configSpecs) == 0 {
		configSpecs = objects.ConfigSpecs[component].listConfigSpecs(true)
	}
	for _, templateName := range configSpecs {
		fmt.Println("template meta:")
		printer.PrintLineWithTabSeparator(
			printer.NewPair("  ConfigSpec", templateName),
			printer.NewPair("ComponentName", component),
			printer.NewPair("ClusterName", r.clusterName),
		)
		if err := r.printExplainConfigure(objects.ConfigSpecs[component], templateName); err != nil {
			return err
		}
	}
	return nil
}

func (r *configObserverOptions) printExplainConfigure(configSpecs configSpecsType, tplName string) error {
	tpl := configSpecs.findByName(tplName)
	if tpl == nil {
		return nil
	}

	confSpec := tpl.ConfigConstraint.Spec
	if confSpec.ConfigurationSchema == nil {
		fmt.Printf("\n%s\n", fmt.Sprintf(notConfigSchemaPrompt, printer.BoldYellow(tplName)))
		return nil
	}

	schema := confSpec.ConfigurationSchema.DeepCopy()
	if schema.Schema == nil {
		if schema.CUE == "" {
			fmt.Printf("\n%s\n", fmt.Sprintf(notConfigSchemaPrompt, printer.BoldYellow(tplName)))
			return nil
		}
		apiSchema, err := openapi.GenerateOpenAPISchema(schema.CUE, confSpec.CfgSchemaTopLevelName)
		if err != nil {
			return cfgcore.WrapError(err, "failed to generate open api schema")
		}
		if apiSchema == nil {
			fmt.Printf("\n%s\n", cue2openAPISchemaFailedPrompt)
			return nil
		}
		schema.Schema = apiSchema
	}
	return r.printConfigConstraint(schema.Schema, cfgutil.NewSet(confSpec.StaticParameters...), cfgutil.NewSet(confSpec.DynamicParameters...))
}

func (r *configObserverOptions) getReconfigureMeta(configSpecs configSpecsType) ([]types.ConfigTemplateInfo, error) {
	configs := make([]types.ConfigTemplateInfo, 0)
	configList := r.configSpecs
	if len(configList) == 0 {
		configList = configSpecs.listConfigSpecs(false)
	}
	for _, tplName := range configList {
		tpl := configSpecs.findByName(tplName)
		if tpl == nil || tpl.ConfigSpec == nil {
			fmt.Fprintf(r.Out, "not found config spec: %s, and pass\n", tplName)
			continue
		}
		if tpl.ConfigSpec == nil {
			fmt.Fprintf(r.Out, "current configSpec[%s] not support reconfiguring and pass\n", tplName)
			continue
		}
		configs = append(configs, types.ConfigTemplateInfo{
			Name:  tplName,
			TPL:   *tpl.ConfigSpec,
			CMObj: tpl.ConfigMap,
		})
	}
	return configs, nil
}

func (r *configObserverOptions) printConfigureContext(configs []types.ConfigTemplateInfo, component string) {
	printer.PrintTitle("Configures Context[${component-name}/${config-spec}/${file-name}]")

	keys := cfgutil.NewSet(r.keys...)
	for _, info := range configs {
		for key, context := range info.CMObj.Data {
			if keys.Length() != 0 && !keys.InArray(key) {
				continue
			}
			fmt.Fprintf(r.Out, "%s%s\n",
				printer.BoldYellow(fmt.Sprintf("%s/%s/%s:\n", component, info.Name, key)), context)
		}
	}
}

func (r *configObserverOptions) printConfigureHistory(component string) error {
	printer.PrintTitle("History modifications")

	// filter reconfigure
	// kubernetes not support fieldSelector with CRD: https://github.com/kubernetes/kubernetes/issues/51046
	listOptions := metav1.ListOptions{
		LabelSelector: strings.Join([]string{constant.AppInstanceLabelKey, r.clusterName}, "="),
	}

	opsList, err := r.dynamic.Resource(types.OpsGVR()).Namespace(r.namespace).List(context.TODO(), listOptions)
	if err != nil {
		return err
	}
	// sort the unstructured objects with the creationTimestamp in positive order
	sort.Sort(unstructuredList(opsList.Items))
	tbl := printer.NewTablePrinter(r.Out)
	tbl.SetHeader("OPS-NAME", "CLUSTER", "COMPONENT", "CONFIG-SPEC-NAME", "FILE", "STATUS", "POLICY", "PROGRESS", "CREATED-TIME", "VALID-UPDATED")
	for _, obj := range opsList.Items {
		ops := &appsv1alpha1.OpsRequest{}
		if err = runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, ops); err != nil {
			return err
		}
		if ops.Spec.Type != appsv1alpha1.ReconfiguringType {
			continue
		}
		components := getComponentNameFromOps(ops)
		if !strings.Contains(components, component) {
			continue
		}
		phase := string(ops.Status.Phase)
		tplNames := getTemplateNameFromOps(ops.Spec)
		keyNames := getKeyNameFromOps(ops.Spec)
		tbl.AddRow(ops.Name,
			ops.Spec.ClusterRef,
			components,
			tplNames,
			keyNames,
			phase,
			getReconfigurePolicy(ops.Status),
			ops.Status.Progress,
			util.TimeFormat(&ops.CreationTimestamp),
			getValidUpdatedParams(ops.Status))
	}
	tbl.Print()
	return nil
}

func (r *configObserverOptions) hasSpecificParam() bool {
	return len(r.paramName) != 0
}

func (r *configObserverOptions) isSpecificParam(paramName string) bool {
	return r.paramName == paramName
}

func (r *configObserverOptions) printConfigConstraint(schema *apiext.JSONSchemaProps,
	staticParameters, dynamicParameters *cfgutil.Sets) error {
	var (
		maxDocumentLength = 100
		maxEnumLength     = 20
		spec              = schema.Properties[openapi.DefaultSchemaName]
		params            = make([]*parameterSchema, 0)
	)

	for key, property := range openapi.FlattenSchema(spec).Properties {
		if property.Type == openapi.SchemaStructType {
			continue
		}
		if r.hasSpecificParam() && !r.isSpecificParam(key) {
			continue
		}

		pt, err := generateParameterSchema(key, property)
		if err != nil {
			return err
		}
		pt.scope = "Global"
		pt.dynamic = isDynamicType(pt, staticParameters, dynamicParameters)

		if r.hasSpecificParam() {
			printSingleParameterSchema(pt)
			return nil
		}
		if !r.hasSpecificParam() && r.truncDocument && len(pt.description) > maxDocumentLength {
			pt.description = pt.description[:maxDocumentLength] + "..."
		}
		params = append(params, pt)
	}

	if !r.truncEnum {
		maxEnumLength = -1
	}
	printConfigParameterSchema(params, r.Out, maxEnumLength)
	return nil
}

func getReconfigurePolicy(status appsv1alpha1.OpsRequestStatus) string {
	if status.ReconfiguringStatus == nil || len(status.ReconfiguringStatus.ConfigurationStatus) == 0 {
		return ""
	}

	var policy string
	reStatus := status.ReconfiguringStatus.ConfigurationStatus[0]
	switch reStatus.UpdatePolicy {
	case appsv1alpha1.AutoReload:
		policy = "reload"
	case appsv1alpha1.NormalPolicy, appsv1alpha1.RestartPolicy, appsv1alpha1.RollingPolicy:
		policy = "restart"
	default:
		return ""
	}
	return printer.BoldYellow(policy)
}

func getValidUpdatedParams(status appsv1alpha1.OpsRequestStatus) string {
	if status.ReconfiguringStatus == nil || len(status.ReconfiguringStatus.ConfigurationStatus) == 0 {
		return ""
	}

	reStatus := status.ReconfiguringStatus.ConfigurationStatus[0]
	if len(reStatus.UpdatedParameters.UpdatedKeys) == 0 {
		return ""
	}
	b, err := json.Marshal(reStatus.UpdatedParameters.UpdatedKeys)
	if err != nil {
		return err.Error()
	}
	return string(b)
}

func isDynamicType(pt *parameterSchema, staticParameters, dynamicParameters *cfgutil.Sets) bool {
	switch {
	case staticParameters.InArray(pt.name):
		return false
	case dynamicParameters.InArray(pt.name):
		return true
	case dynamicParameters.Length() == 0 && staticParameters.Length() != 0:
		return true
	case dynamicParameters.Length() != 0 && staticParameters.Length() == 0:
		return false
	default:
		return false
	}
}

// NewDescribeReconfigureCmd shows details of history modifications or configuration file of reconfiguring operations
func NewDescribeReconfigureCmd(f cmdutil.Factory, streams genericiooptions.IOStreams) *cobra.Command {
	o := &configObserverOptions{
		isExplain:          false,
		showDetail:         false,
		describeOpsOptions: newDescribeOpsOptions(f, streams),
	}
	cmd := &cobra.Command{
		Use:               "describe-config",
		Short:             "Show details of a specific reconfiguring.",
		Aliases:           []string{"desc-config"},
		Example:           describeReconfigureExample,
		ValidArgsFunction: util.ResourceNameCompletionFunc(f, types.ClusterGVR()),
		Run: func(cmd *cobra.Command, args []string) {
			util.CheckErr(o.complete2(args))
			util.CheckErr(o.run(o.printComponentConfigSpecsDescribe))
		},
	}
	o.addCommonFlags(cmd, f)
	cmd.Flags().BoolVar(&o.showDetail, "show-detail", o.showDetail, "If true, the content of the files specified by config-file will be printed.")
	cmd.Flags().StringSliceVar(&o.keys, "config-file", nil, "Specify the name of the configuration file to be describe (e.g. for mysql: --config-file=my.cnf). If unset, all files.")
	return cmd
}

// NewExplainReconfigureCmd shows details of modifiable parameters.
func NewExplainReconfigureCmd(f cmdutil.Factory, streams genericiooptions.IOStreams) *cobra.Command {
	o := &configObserverOptions{
		isExplain:          true,
		truncEnum:          true,
		truncDocument:      false,
		describeOpsOptions: newDescribeOpsOptions(f, streams),
	}
	cmd := &cobra.Command{
		Use:               "explain-config",
		Short:             "List the constraint for supported configuration params.",
		Aliases:           []string{"ex-config"},
		Example:           explainReconfigureExample,
		ValidArgsFunction: util.ResourceNameCompletionFunc(f, types.ClusterGVR()),
		Run: func(cmd *cobra.Command, args []string) {
			util.CheckErr(o.complete2(args))
			util.CheckErr(o.run(o.printComponentExplainConfigure))
		},
	}
	o.addCommonFlags(cmd, f)
	cmd.Flags().BoolVar(&o.truncEnum, "trunc-enum", o.truncEnum, "If the value list length of the parameter is greater than 20, it will be truncated.")
	cmd.Flags().BoolVar(&o.truncDocument, "trunc-document", o.truncDocument, "If the document length of the parameter is greater than 100, it will be truncated.")
	cmd.Flags().StringVar(&o.paramName, "param", o.paramName, "Specify the name of parameter to be query. It clearly display the details of the parameter.")
	return cmd
}
