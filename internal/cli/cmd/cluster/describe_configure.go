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
	"io"
	"sort"
	"strings"

	"github.com/StudioSol/set"

	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	apiext "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/util/templates"
	"sigs.k8s.io/controller-runtime/pkg/client"

	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/cli/printer"
	"github.com/apecloud/kubeblocks/internal/cli/types"
	"github.com/apecloud/kubeblocks/internal/cli/util"
	cfgcore "github.com/apecloud/kubeblocks/internal/configuration"
)

type reconfigureOptions struct {
	*describeOpsOptions

	clusterName   string
	componentName string
	templateNames []string
	isExplain     bool
	truncEnum     bool
	truncDocument bool
	keys          []string
	showDetail    bool
	// for cache
	tpls []dbaasv1alpha1.ConfigTemplate
}

type parameterTemplate struct {
	name        string
	valueType   string
	miniNum     string
	maxiNum     string
	enum        []string
	description string
	scope       string
}

var (
	describeReconfigureExample = templates.Examples(`
		# describe a specified configure
		kbcli cluster describe-configure cluster-name --component-name=component --template-names=tpl1,tpl2`)
	explainReconfigureExample = templates.Examples(`
		# describe a specified configure template
		kbcli cluster explain-configure cluster-name --component-name=component --template-names=tpl1`)
)

func (r *reconfigureOptions) addCommonFlags(cmd *cobra.Command) {
	cmd.Flags().StringVar(&r.componentName, "component-name", "", " Component name to this operations (required)")
	cmd.Flags().StringSliceVar(&r.templateNames, "template-names", nil, "Specifies the name of the configuration template to be describe (options)")
}

func (r *reconfigureOptions) validate() error {
	if r.clusterName == "" {
		return cfgcore.MakeError("missing cluster name")
	}
	if r.componentName == "" {
		return cfgcore.MakeError("missing component name")
	}
	if err := r.syncComponentCfgTpl(); err != nil {
		return err
	}

	if r.isExplain && len(r.templateNames) != 1 {
		return cfgcore.MakeError("explain require one template")
	}

	for _, tplName := range r.templateNames {
		tpl, err := r.findTemplateByName(tplName)
		if err != nil {
			return err
		}
		if r.isExplain && len(tpl.ConfigConstraintRef) == 0 {
			return cfgcore.MakeError("explain command require template has config constraint options")
		}
		// validate config cm
		// cfgName := cfgcore.GetComponentCfgName(r.clusterName, r.componentName, tpl.VolumeName)
		// cmObj := &corev1.ConfigMap{}
		// if err := util.GetResourceObjectFromGVR(types.CMGVR(), client.ObjectKey{
		//	Name:      cfgName,
		//	Namespace: r.namespace,
		// }, r.dynamic, &cmObj); err != nil {
		//	return cfgcore.WrapError(err, "template config instance is not exist, template name: %s, cfg name: %s",
		//		tplName, cfgName)
		// }
	}
	return nil
}

func (r *reconfigureOptions) findTemplateByName(tplName string) (*dbaasv1alpha1.ConfigTemplate, error) {
	if err := r.syncComponentCfgTpl(); err != nil {
		return nil, err
	}

	for i := range r.tpls {
		tpl := &r.tpls[i]
		if tpl.Name == tplName {
			return tpl, nil
		}
	}
	return nil, cfgcore.MakeError("not found template: %s", tplName)
}

func (r *reconfigureOptions) complete2(args []string) error {
	if len(args) > 0 {
		r.clusterName = args[0]
	}

	if r.componentName == "" {
		return cfgcore.MakeError("missing component name")
	}

	if err := r.complete(args); err != nil {
		return err
	}
	if len(r.templateNames) != 0 {
		return nil
	}
	if err := r.syncComponentCfgTpl(); err != nil {
		return err
	}
	if len(r.tpls) == 0 {
		return cfgcore.MakeError("not any config template, not support describe")
	}

	if !r.isExplain {
		templateNames := make([]string, len(r.tpls))
		for i, tpl := range r.tpls {
			templateNames[i] = tpl.Name
		}
		r.templateNames = templateNames
		return nil
	}

	// for explain
	for _, tpl := range r.tpls {
		if len(tpl.ConfigConstraintRef) == 0 {
			continue
		}
		r.templateNames = []string{tpl.Name}
		break
	}
	return nil
}

func (r *reconfigureOptions) syncComponentCfgTpl() error {
	if r.tpls != nil {
		return nil
	}
	tplList, err := util.GetConfigTemplateList(r.clusterName, r.namespace, r.dynamic, r.componentName)
	if err != nil {
		return err
	}
	r.tpls = tplList
	return nil
}

func (r *reconfigureOptions) printDescribeReconfigure() error {
	configs, err := r.getReconfigureMeta()
	if err != nil {
		return err
	}
	printer.PrintComponentConfigMeta(configs, r.clusterName, r.componentName, r.Out)

	if r.showDetail {
		r.printConfigureContext(configs)
	}
	return r.printConfigureHistory(configs)
}

func (r *reconfigureOptions) printExplainReconfigure(tplName string) error {
	tpl, err := r.findTemplateByName(tplName)
	if err != nil {
		return err
	}

	configConstraint := dbaasv1alpha1.ConfigConstraint{}
	if err := util.GetResourceObjectFromGVR(types.ConfigConstraintGVR(), client.ObjectKey{
		Namespace: "",
		Name:      tpl.ConfigConstraintRef,
	}, r.dynamic, &configConstraint); err != nil {
		return err
	}

	confSpec := configConstraint.Spec
	schema := confSpec.ConfigurationSchema.DeepCopy()
	if schema.Schema == nil {
		apiSchema, err := cfgcore.GenerateOpenAPISchema(schema.CUE, "")
		if err != nil {
			return cfgcore.WrapError(err, "failed to generate open api schema")
		}
		schema.Schema = apiSchema
	}
	return r.printConfigConstraint(schema.Schema, set.NewLinkedHashSetString(confSpec.StaticParameters...), set.NewLinkedHashSetString(confSpec.DynamicParameters...))
}

func (r *reconfigureOptions) getReconfigureMeta() (map[dbaasv1alpha1.ConfigTemplate]*corev1.ConfigMap, error) {
	configs := make(map[dbaasv1alpha1.ConfigTemplate]*corev1.ConfigMap)
	for _, tplName := range r.templateNames {
		// checked by validate
		tpl, _ := r.findTemplateByName(tplName)
		// fetch config configmap
		cmObj := &corev1.ConfigMap{}
		cmName := cfgcore.GetComponentCfgName(r.clusterName, r.componentName, tpl.VolumeName)
		if err := util.GetResourceObjectFromGVR(types.CMGVR(), client.ObjectKey{
			Name:      cmName,
			Namespace: r.namespace,
		}, r.dynamic, cmObj); err != nil {
			return nil, cfgcore.WrapError(err, "template config instance is not exist, template name: %s, cfg name: %s", tplName, cmName)
		}
		configs[*tpl] = cmObj
	}
	return configs, nil
}

func (r *reconfigureOptions) printConfigureContext(configs map[dbaasv1alpha1.ConfigTemplate]*corev1.ConfigMap) {
	printer.PrintTitle("Configures Context")

	keys := set.NewLinkedHashSetString(r.keys...)
	for _, cm := range configs {
		for key, context := range cm.Data {
			if keys.Length() != 0 && !keys.InArray(key) {
				continue
			}
			fmt.Fprintf(r.Out, "%s%s\n",
				printer.BoldYellow(fmt.Sprintf("%s/%s:\n", r.componentName, key)), context)
		}
	}
}

func (r *reconfigureOptions) printConfigureHistory(configs map[dbaasv1alpha1.ConfigTemplate]*corev1.ConfigMap) error {
	printer.PrintTitle("History modifications")

	// filter reconfigure
	// kubernetes not support fieldSelector with CRD: https://github.com/kubernetes/kubernetes/issues/51046
	listOptions := metav1.ListOptions{
		LabelSelector: strings.Join([]string{types.InstanceLabelKey, r.clusterName}, "="),
		// FieldSelector: strings.Join([]string{
		//	strings.Join([]string{"spec.type", string(dbaasv1alpha1.ReconfiguringType)}, "="),
		//	strings.Join([]string{"spec.reconfigure.componentName", r.componentName}, "="),
		// }, ","),
	}

	opsList, err := r.dynamic.Resource(types.OpsGVR()).Namespace(r.namespace).List(context.TODO(), listOptions)
	if err != nil {
		return err
	}
	// sort the unstructured objects with the creationTimestamp in positive order
	sort.Sort(unstructuredList(opsList.Items))
	tbl := printer.NewTablePrinter(r.Out)
	tbl.SetHeader("NAME", "CLUSTER", "COMPONENT", "TEMPLATE", "FILES", "STATUS", "PROGRESS", "CREATED-TIME")
	for _, obj := range opsList.Items {
		ops := &dbaasv1alpha1.OpsRequest{}
		if err = runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, ops); err != nil {
			return err
		}
		if ops.Spec.Type != dbaasv1alpha1.ReconfiguringType {
			continue
		}
		components := getComponentNameFromOps(ops.Spec)
		if !strings.Contains(components, r.componentName) {
			continue
		}
		phase := string(ops.Status.Phase)
		tplNames := getTemplateNameFromOps(ops.Spec)
		keyNames := getKeyNameFromOps(ops.Spec)
		tbl.AddRow(ops.Name, ops.Spec.ClusterRef, components, tplNames, keyNames, phase, ops.Status.Progress, util.TimeFormat(&ops.CreationTimestamp))
	}
	tbl.Print()
	return nil
}

func (r *reconfigureOptions) printConfigConstraint(schema *apiext.JSONSchemaProps, staticParameters *set.LinkedHashSetString, dynamicParameters *set.LinkedHashSetString) error {
	var (
		index             = 0
		maxDocumentLength = 100
		maxEnumLength     = 20
		spec              = schema.Properties["spec"]
		params            = make([]*parameterTemplate, len(spec.Properties))
	)

	for key, property := range spec.Properties {
		if property.Type == "object" {
			continue
		}
		pt, err := generateParameterTemplate(key, property)
		if err != nil {
			return err
		}
		if staticParameters.InArray(pt.name) {
			pt.scope = "static"
		} else if dynamicParameters.InArray(pt.name) {
			pt.scope = "dynamic"
		}
		if r.truncDocument && len(pt.description) > maxDocumentLength {
			pt.description = pt.description[:maxDocumentLength] + "..."
		}
		params[index] = pt
		index++
	}

	if !r.truncEnum {
		maxEnumLength = -1
	}
	printConfigParameterTemplate(params, r.Out, maxEnumLength)
	return nil
}

// printConfigParameterTemplate prints the conditions of resource.
func printConfigParameterTemplate(paramTemplates []*parameterTemplate, out io.Writer, maxFieldLength int) {
	const (
		r          = "-"
		rangeBegin = "["
		rangeEnd   = "]"
	)

	rangeFormatter := func(pt *parameterTemplate) string {
		if len(pt.maxiNum) == 0 && len(pt.miniNum) == 0 {
			return ""
		}

		v := rangeBegin
		if len(pt.miniNum) != 0 {
			v += pt.miniNum
		}
		if len(pt.maxiNum) != 0 {
			v += r
			v += pt.maxiNum
		} else if len(v) != 0 {
			v += r
		}
		v += rangeEnd
		return v
	}
	enumFormatter := func(pt *parameterTemplate) string {
		if len(pt.enum) == 0 {
			return ""
		}
		v := strings.Join(pt.enum, ",")
		if maxFieldLength > 0 && len(v) > maxFieldLength {
			v = v[:maxFieldLength] + "..."
		}
		return v
	}

	if len(paramTemplates) == 0 {
		return
	}
	tbl := printer.NewTablePrinter(out)
	tbl.SetStyle(printer.TerminalStyle)
	printer.PrintTitle("Configure Constraint")
	tbl.SetHeader("PARAMETER NAME", "RANGE", "ENUM", "SCOPE", "TYPE", "description")
	for _, pt := range paramTemplates {
		tbl.AddRow(pt.name, rangeFormatter(pt), enumFormatter(pt), pt.scope, pt.valueType, pt.description)
	}
	tbl.Print()
}

func generateParameterTemplate(paramName string, property apiext.JSONSchemaProps) (*parameterTemplate, error) {
	toString := func(v interface{}) (string, error) {
		b, err := json.Marshal(v)
		if err != nil {
			return "", err
		}
		return string(b), nil
	}
	pt := &parameterTemplate{
		name:        paramName,
		valueType:   property.Type,
		description: strings.TrimSpace(property.Description),
	}
	if property.Minimum != nil {
		b, err := toString(property.Minimum)
		if err != nil {
			return nil, err
		}
		pt.miniNum = b
	}
	if property.Maximum != nil {
		b, err := toString(property.Maximum)
		if err != nil {
			return nil, err
		}
		pt.maxiNum = b
	}
	if property.Enum != nil {
		pt.enum = make([]string, len(property.Enum))
		for i, v := range property.Enum {
			b, err := toString(v)
			if err != nil {
				return nil, err
			}
			pt.enum[i] = b
		}
	}
	return pt, nil
}

func NewDescribeReconfigureCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := &reconfigureOptions{
		isExplain:          false,
		showDetail:         false,
		describeOpsOptions: newDescribeOpsOptions(f, streams),
	}
	cmd := &cobra.Command{
		Use:               "describe-configure",
		Short:             "Show details of a specific reconfiguring",
		Example:           describeReconfigureExample,
		ValidArgsFunction: util.ResourceNameCompletionFunc(f, types.ClusterGVR()),
		Run: func(cmd *cobra.Command, args []string) {
			util.CheckErr(o.complete2(args))
			util.CheckErr(o.validate())
			util.CheckErr(o.printDescribeReconfigure())
		},
	}
	o.addCommonFlags(cmd)
	cmd.Flags().BoolVar(&o.showDetail, "show-detail", o.showDetail, " trunc enum string (options)")
	cmd.Flags().StringSliceVar(&o.keys, "keys", nil, " display keys context (options)")
	return cmd
}

func NewExplainReconfigureCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := &reconfigureOptions{
		isExplain:          true,
		truncEnum:          true,
		truncDocument:      false,
		describeOpsOptions: newDescribeOpsOptions(f, streams),
	}
	cmd := &cobra.Command{
		Use:               "explain-configure",
		Short:             "List the constraint for supported configuration params",
		Example:           explainReconfigureExample,
		ValidArgsFunction: util.ResourceNameCompletionFunc(f, types.ClusterGVR()),
		Run: func(cmd *cobra.Command, args []string) {
			util.CheckErr(o.complete2(args))
			util.CheckErr(o.validate())
			util.CheckErr(o.printExplainReconfigure(o.templateNames[0]))
		},
	}
	o.addCommonFlags(cmd)
	cmd.Flags().BoolVar(&o.truncEnum, "trunc-enum", o.truncEnum, " trunc enum string (options)")
	cmd.Flags().BoolVar(&o.truncDocument, "trunc-document", o.truncDocument, " trunc document string (options)")
	return cmd
}
