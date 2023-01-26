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
	"sort"
	"strings"

	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
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
	clusterName   string
	componentName string
	templateNames []string

	printFn func()

	tpls []dbaasv1alpha1.ConfigTemplate

	*describeOpsOptions
}

var (
	describeReconfigureExample = templates.Examples(`
		# describe a specified configure
		kbcli cluster describe-configure cluster-name mysql-restart-82zxv --component-name=component --template-names=tpl1`)
	explainReconfigureExample = templates.Examples(`
		# describe a specified OpsRequest
		kbcli cluster describe-ops mysql-restart-82zxv`)
)

func (r *reconfigureOptions) addCommonFlags(cmd *cobra.Command) {
	cmd.Flags().StringVar(&r.componentName, "component-name", "", " Component name to this operations (required)")
	cmd.Flags().StringVar(&r.clusterName, "cluster-name", "", " cluster name to this operations (required)")
	cmd.Flags().StringSliceVar(&r.templateNames, "template-names", nil, "Specifies the name of the configuration template to be describe")
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

	for _, tplName := range r.templateNames {
		_, err := r.findTemplateByName(tplName)
		if err != nil {
			return err
		}
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

	r.templateNames = make([]string, len(r.tpls))
	for i, tpl := range r.tpls {
		r.templateNames[i] = tpl.Name
	}
	return nil
}

func (r *reconfigureOptions) run() error {
	r.printFn()
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

	r.printConfigureContext(configs)
	return r.printConfigureHistory(configs)
}

func (r *reconfigureOptions) getReconfigureMeta() (map[dbaasv1alpha1.ConfigTemplate]*corev1.ConfigMap, error) {
	configs := make(map[dbaasv1alpha1.ConfigTemplate]*corev1.ConfigMap)
	for _, tplName := range r.templateNames {
		tpl, err := r.findTemplateByName(tplName)
		if err != nil {
			return nil, err
		}

		// fetch config configmap
		cmObj := &corev1.ConfigMap{}
		cmName := cfgcore.GetComponentCfgName(r.clusterName, r.componentName, tpl.VolumeName)
		if err := util.GetResourceObjectFromGVR(types.CMGVR(), client.ObjectKey{
			Name:      cmName,
			Namespace: r.namespace,
		}, r.dynamic, &cmObj); err != nil {
			return nil, err
		}
		configs[*tpl] = cmObj
	}
	return configs, nil
}

func (r *reconfigureOptions) printConfigureContext(configs map[dbaasv1alpha1.ConfigTemplate]*corev1.ConfigMap) {
	printer.PrintTitle("Configures Context")

	for _, cm := range configs {
		for key, context := range cm.Data {
			fmt.Fprintf(r.Out, "%s%s\n\n",
				printer.BoldYellow(fmt.Sprintf("%s/%s:\n", r.componentName, key)), context)
		}
	}
}

func (r *reconfigureOptions) printConfigureHistory(configs map[dbaasv1alpha1.ConfigTemplate]*corev1.ConfigMap) error {
	printer.PrintTitle("History modifications")

	// filter reconfigure
	listOptions := metav1.ListOptions{
		LabelSelector: strings.Join([]string{types.InstanceLabelKey, r.clusterName}, "="),
		FieldSelector: strings.Join([]string{"spec.type", string(dbaasv1alpha1.ReconfiguringType)}, "="),
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

func NewDescribeReconfigureCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := &reconfigureOptions{
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
	return cmd
}

func NewExplainReconfigureCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := &reconfigureOptions{
		describeOpsOptions: newDescribeOpsOptions(f, streams),
	}
	cmd := &cobra.Command{
		Use:               "explain-configure",
		Short:             "List the constraint for supported configuration params",
		Example:           explainReconfigureExample,
		ValidArgsFunction: util.ResourceNameCompletionFunc(f, types.ClusterGVR()),
		Run: func(cmd *cobra.Command, args []string) {
			util.CheckErr(o.complete(args))
			util.CheckErr(o.run())
		},
	}
	o.addCommonFlags(cmd)
	return cmd
}
