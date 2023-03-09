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
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/exp/slices"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/util/templates"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/cli/list"
	"github.com/apecloud/kubeblocks/internal/cli/printer"
	"github.com/apecloud/kubeblocks/internal/cli/types"
	"github.com/apecloud/kubeblocks/internal/cli/util"
)

var listOpsExample = templates.Examples(`
		# list all opsRequests
		kbcli cluster list-ops

		# list all opsRequests of specified cluster
		kbcli cluster list-ops mycluster`)

type opsListOptions struct {
	*list.ListOptions
	status  []string
	opsType []string
}

func NewListOpsCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := &opsListOptions{
		ListOptions: list.NewListOptions(f, streams, types.OpsGVR()),
	}
	cmd := &cobra.Command{
		Use:               "list-ops",
		Short:             "List all opsRequests",
		Aliases:           []string{"ls-ops"},
		Example:           listOpsExample,
		ValidArgsFunction: util.ResourceNameCompletionFunc(f, types.ClusterGVR()),
		Run: func(cmd *cobra.Command, args []string) {
			// build label selector for listing ops
			o.LabelSelector = util.BuildLabelSelectorByNames(o.LabelSelector, args)
			// args are the cluster names. we only use the label selector to get ops, so resources names
			// are not needed.
			o.Names = nil
			util.CheckErr(o.Complete())
			util.CheckErr(o.printOpsList())
		},
	}
	o.AddFlags(cmd)
	cmd.Flags().StringSliceVar(&o.opsType, "type", nil, "The OpsRequest type")
	cmd.Flags().StringSliceVar(&o.status, "status", []string{"running", "pending", "failed"}, "Options include all, pending, running, succeeded, failed. by default, outputs the pending/running/failed OpsRequest.")
	return cmd
}

func (o *opsListOptions) printOpsList() error {
	dynamic, err := o.Factory.DynamicClient()
	if err != nil {
		return err
	}

	listOptions := metav1.ListOptions{
		LabelSelector: o.LabelSelector,
		FieldSelector: o.FieldSelector,
	}

	opsList, err := dynamic.Resource(types.OpsGVR()).Namespace(o.Namespace).List(context.TODO(), listOptions)
	if err != nil {
		return err
	}
	if len(opsList.Items) == 0 {
		o.PrintNotFoundResources()
		return nil
	}
	// sort the unstructured objects with the creationTimestamp in positive order
	sort.Sort(unstructuredList(opsList.Items))

	// check if specific the "all" keyword for status.
	isAllStatus := o.isAllStatus()
	tbl := printer.NewTablePrinter(o.Out)
	tbl.SetHeader("NAME", "TYPE", "CLUSTER", "COMPONENT", "STATUS", "PROGRESS", "CREATED-TIME")
	for _, obj := range opsList.Items {
		ops := &appsv1alpha1.OpsRequest{}
		if err = runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, ops); err != nil {
			return err
		}
		// if the OpsRequest phase is not in the expected phases, continue
		phase := string(ops.Status.Phase)
		if !isAllStatus && !o.containsIgnoreCase(o.status, phase) {
			continue
		}

		opsType := string(ops.Spec.Type)
		if len(o.opsType) != 0 && !o.containsIgnoreCase(o.opsType, opsType) {
			continue
		}
		tbl.AddRow(ops.Name, opsType, ops.Spec.ClusterRef, getComponentNameFromOps(ops), phase, ops.Status.Progress, util.TimeFormat(&ops.CreationTimestamp))
	}
	tbl.Print()
	return nil
}

func getComponentNameFromOps(ops *appsv1alpha1.OpsRequest) string {
	components := make([]string, 0)
	opsSpec := ops.Spec
	switch opsSpec.Type {
	case appsv1alpha1.ReconfiguringType:
		components = append(components, opsSpec.Reconfigure.ComponentName)
	case appsv1alpha1.HorizontalScalingType:
		for _, item := range opsSpec.HorizontalScalingList {
			components = append(components, item.ComponentName)
		}
	case appsv1alpha1.VolumeExpansionType:
		for _, item := range opsSpec.VolumeExpansionList {
			components = append(components, item.ComponentName)
		}
	case appsv1alpha1.RestartType:
		for _, item := range opsSpec.RestartList {
			components = append(components, item.ComponentName)
		}
	case appsv1alpha1.VerticalScalingType:
		for _, item := range opsSpec.VerticalScalingList {
			components = append(components, item.ComponentName)
		}
	case appsv1alpha1.UpgradeType:
		for k := range ops.Status.Components {
			components = append(components, k)
		}
		slices.Sort(components)
	}
	return strings.Join(components, ",")
}

func getTemplateNameFromOps(ops appsv1alpha1.OpsRequestSpec) string {
	if ops.Type != appsv1alpha1.ReconfiguringType {
		return ""
	}

	tpls := make([]string, 0)
	for _, config := range ops.Reconfigure.Configurations {
		tpls = append(tpls, config.Name)
	}
	return strings.Join(tpls, ",")
}

func getKeyNameFromOps(ops appsv1alpha1.OpsRequestSpec) string {
	if ops.Type != appsv1alpha1.ReconfiguringType {
		return ""
	}

	keys := make([]string, 0)
	for _, config := range ops.Reconfigure.Configurations {
		for _, key := range config.Keys {
			keys = append(keys, key.Key)
		}
	}
	return strings.Join(keys, ",")
}

func (o *opsListOptions) containsIgnoreCase(s []string, e string) bool {
	for i := range s {
		if strings.EqualFold(s[i], e) {
			return true
		}
	}
	return false
}

// isAllStatus checks if the status flag contains "all" keyword.
func (o *opsListOptions) isAllStatus() bool {
	return slices.Contains(o.status, "all")
}

type unstructuredList []unstructured.Unstructured

func (us unstructuredList) Len() int {
	return len(us)
}
func (us unstructuredList) Swap(i, j int) {
	us[i], us[j] = us[j], us[i]
}
func (us unstructuredList) Less(i, j int) bool {
	createTimeForJ := us[j].GetCreationTimestamp()
	createTimeForI := us[i].GetCreationTimestamp()
	return createTimeForI.Before(&createTimeForJ)
}
