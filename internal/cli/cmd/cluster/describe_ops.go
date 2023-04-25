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
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/exp/maps"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/dynamic"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/util/templates"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/cli/cluster"
	"github.com/apecloud/kubeblocks/internal/cli/printer"
	"github.com/apecloud/kubeblocks/internal/cli/types"
	"github.com/apecloud/kubeblocks/internal/cli/util"
)

var (
	describeOpsExample = templates.Examples(`
		# describe a specified OpsRequest
		kbcli cluster describe-ops mysql-restart-82zxv`)
)

type describeOpsOptions struct {
	factory   cmdutil.Factory
	client    clientset.Interface
	dynamic   dynamic.Interface
	namespace string

	// resource type and names
	gvr   schema.GroupVersionResource
	names []string

	genericclioptions.IOStreams
}

type opsObject interface {
	appsv1alpha1.VerticalScaling | appsv1alpha1.HorizontalScaling | appsv1alpha1.OpsRequestVolumeClaimTemplate | appsv1alpha1.VolumeExpansion
}

func newDescribeOpsOptions(f cmdutil.Factory, streams genericclioptions.IOStreams) *describeOpsOptions {
	return &describeOpsOptions{
		factory:   f,
		IOStreams: streams,
		gvr:       types.OpsGVR(),
	}
}

func NewDescribeOpsCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := newDescribeOpsOptions(f, streams)
	cmd := &cobra.Command{
		Use:               "describe-ops",
		Short:             "Show details of a specific OpsRequest.",
		Aliases:           []string{"desc-ops"},
		Example:           describeOpsExample,
		ValidArgsFunction: util.ResourceNameCompletionFunc(f, types.ClusterGVR()),
		Run: func(cmd *cobra.Command, args []string) {
			util.CheckErr(o.complete(args))
			util.CheckErr(o.run())
		},
	}
	return cmd
}

// getCommandFlagsSlice returns the targetName slice by getName function and opsObject slice, their lengths are equal.
func getCommandFlagsSlice[T opsObject](opsSt []T,
	convertObject func(t T) any,
	getName func(t T) string) ([][]string, []any) {
	// returns the index of the first occurrence of v in s,s or -1 if not present.
	indexFromAnySlice := func(s []any, v any) int {
		for i := range s {
			if reflect.DeepEqual(s[i], v) {
				return i
			}
		}
		return -1
	}
	opsObjectSlice := make([]any, 0, len(opsSt))
	targetNameSlice := make([][]string, 0, len(opsSt))
	for _, v := range opsSt {
		index := indexFromAnySlice(opsObjectSlice, convertObject(v))
		if index == -1 {
			opsObjectSlice = append(opsObjectSlice, convertObject(v))
			targetNameSlice = append(targetNameSlice, []string{getName(v)})
			continue
		}
		targetNameSlice[index] = append(targetNameSlice[index], getName(v))
	}
	return targetNameSlice, opsObjectSlice
}

func (o *describeOpsOptions) complete(args []string) error {
	var err error

	if len(args) == 0 {
		return fmt.Errorf("OpsRequest name should be specified")
	}

	o.names = args

	if o.client, err = o.factory.KubernetesClientSet(); err != nil {
		return err
	}

	if o.dynamic, err = o.factory.DynamicClient(); err != nil {
		return err
	}

	if o.namespace, _, err = o.factory.ToRawKubeConfigLoader().Namespace(); err != nil {
		return err
	}
	return nil
}

func (o *describeOpsOptions) run() error {
	for _, name := range o.names {
		if err := o.describeOps(name); err != nil {
			return err
		}
	}
	return nil
}

// describeOps gets the OpsRequest by name and describes it.
func (o *describeOpsOptions) describeOps(name string) error {
	opsRequest := &appsv1alpha1.OpsRequest{}
	if err := cluster.GetK8SClientObject(o.dynamic, opsRequest, o.gvr, o.namespace, name); err != nil {
		return err
	}
	return o.printOpsRequest(opsRequest)
}

// printOpsRequest prints the information of OpsRequest for describing command.
func (o *describeOpsOptions) printOpsRequest(ops *appsv1alpha1.OpsRequest) error {
	printer.PrintLine("Spec:")
	printer.PrintLineWithTabSeparator(
		// first pair string
		printer.NewPair("  Name", ops.Name),
		printer.NewPair("NameSpace", ops.Namespace),
		printer.NewPair("Cluster", ops.Spec.ClusterRef),
		printer.NewPair("Type", string(ops.Spec.Type)),
	)

	o.printOpsCommand(ops)

	// print the last configuration of the cluster.
	o.printLastConfiguration(ops.Status.LastConfiguration, ops.Spec.Type)

	// print the OpsRequest.status
	o.printOpsRequestStatus(&ops.Status)

	// print the OpsRequest.status.conditions
	printer.PrintConditions(ops.Status.Conditions, o.Out)

	// get all events about cluster
	events, err := o.client.CoreV1().Events(o.namespace).Search(scheme.Scheme, ops)
	if err != nil {
		return err
	}

	// print the warning events
	printer.PrintAllWarningEvents(events, o.Out)

	return nil
}

// printOpsCommand prints the kbcli command by OpsRequest.spec.
func (o *describeOpsOptions) printOpsCommand(opsRequest *appsv1alpha1.OpsRequest) {
	if opsRequest == nil {
		return
	}
	var commands []string
	switch opsRequest.Spec.Type {
	case appsv1alpha1.RestartType:
		commands = o.getRestartCommand(opsRequest.Spec)
	case appsv1alpha1.UpgradeType:
		commands = o.getUpgradeCommand(opsRequest.Spec)
	case appsv1alpha1.HorizontalScalingType:
		commands = o.getHorizontalScalingCommand(opsRequest.Spec)
	case appsv1alpha1.VerticalScalingType:
		commands = o.getVerticalScalingCommand(opsRequest.Spec)
	case appsv1alpha1.VolumeExpansionType:
		commands = o.getVolumeExpansionCommand(opsRequest.Spec)
	case appsv1alpha1.ReconfiguringType:
		commands = o.getReconfiguringCommand(opsRequest.Spec)
	}
	if len(commands) == 0 {
		printer.PrintLine("\nCommand: " + printer.NoneString)
		return
	}
	printer.PrintTitle("Command")
	for i := range commands {
		command := fmt.Sprintf("%s --namespace=%s", commands[i], opsRequest.Namespace)
		printer.PrintLine("  " + command)
	}
}

// getRestartCommand gets the command of the Restart OpsRequest.
func (o *describeOpsOptions) getRestartCommand(spec appsv1alpha1.OpsRequestSpec) []string {
	if len(spec.RestartList) == 0 {
		return nil
	}
	componentNames := make([]string, len(spec.RestartList))
	for i, v := range spec.RestartList {
		componentNames[i] = v.ComponentName
	}
	return []string{
		fmt.Sprintf("kbcli cluster restart %s --components=%s", spec.ClusterRef,
			strings.Join(componentNames, ",")),
	}
}

// getUpgradeCommand gets the command of the Upgrade OpsRequest.
func (o *describeOpsOptions) getUpgradeCommand(spec appsv1alpha1.OpsRequestSpec) []string {
	return []string{
		fmt.Sprintf("kbcli cluster upgrade %s --cluster-version=%s", spec.ClusterRef,
			spec.Upgrade.ClusterVersionRef),
	}
}

// addResourceFlag adds resource flag for VerticalScaling OpsRequest.
func (o *describeOpsOptions) addResourceFlag(key string, value *resource.Quantity) string {
	if !value.IsZero() {
		return fmt.Sprintf(" --%s=%s", key, value)
	}
	return ""
}

// getVerticalScalingCommand gets the command of the VerticalScaling OpsRequest
func (o *describeOpsOptions) getVerticalScalingCommand(spec appsv1alpha1.OpsRequestSpec) []string {
	if len(spec.VerticalScalingList) == 0 {
		return nil
	}
	convertObject := func(h appsv1alpha1.VerticalScaling) any {
		return h.ResourceRequirements
	}
	getCompName := func(h appsv1alpha1.VerticalScaling) string {
		return h.ComponentName
	}
	componentNameSlice, resourceSlice := getCommandFlagsSlice[appsv1alpha1.VerticalScaling](
		spec.VerticalScalingList, convertObject, getCompName)
	commands := make([]string, len(componentNameSlice))
	for i := range componentNameSlice {
		commands[i] = fmt.Sprintf("kbcli cluster vscale %s --components=%s",
			spec.ClusterRef, strings.Join(componentNameSlice[i], ","))
		clsRef := spec.VerticalScalingList[i].ClassDefRef
		if clsRef != nil {
			class := clsRef.Class
			if clsRef.Name != "" {
				class = fmt.Sprintf("%s:%s", clsRef.Name, class)
			}
			commands[i] += fmt.Sprintf("--class=%s", class)
		} else {
			resource := resourceSlice[i].(corev1.ResourceRequirements)
			commands[i] += o.addResourceFlag("cpu", resource.Limits.Cpu())
			commands[i] += o.addResourceFlag("memory", resource.Limits.Memory())
		}
	}
	return commands
}

// getHorizontalScalingCommand gets the command of the HorizontalScaling OpsRequest.
func (o *describeOpsOptions) getHorizontalScalingCommand(spec appsv1alpha1.OpsRequestSpec) []string {
	if len(spec.HorizontalScalingList) == 0 {
		return nil
	}
	convertObject := func(h appsv1alpha1.HorizontalScaling) any {
		return h.Replicas
	}
	getCompName := func(h appsv1alpha1.HorizontalScaling) string {
		return h.ComponentName
	}
	componentNameSlice, replicasSlice := getCommandFlagsSlice[appsv1alpha1.HorizontalScaling](
		spec.HorizontalScalingList, convertObject, getCompName)
	commands := make([]string, len(componentNameSlice))
	for i := range componentNameSlice {
		commands[i] = fmt.Sprintf("kbcli cluster hscale %s --components=%s --replicas=%d",
			spec.ClusterRef, strings.Join(componentNameSlice[i], ","), replicasSlice[i].(int32))
	}
	return commands
}

// getVolumeExpansionCommand gets the command of the VolumeExpansion command.
func (o *describeOpsOptions) getVolumeExpansionCommand(spec appsv1alpha1.OpsRequestSpec) []string {
	convertObject := func(v appsv1alpha1.OpsRequestVolumeClaimTemplate) any {
		return v.Storage
	}
	getVCTName := func(v appsv1alpha1.OpsRequestVolumeClaimTemplate) string {
		return v.Name
	}
	commands := make([]string, 0)
	for _, v := range spec.VolumeExpansionList {
		vctNameSlice, storageSlice := getCommandFlagsSlice[appsv1alpha1.OpsRequestVolumeClaimTemplate](
			v.VolumeClaimTemplates, convertObject, getVCTName)
		for i := range vctNameSlice {
			storage := storageSlice[i].(resource.Quantity)
			commands = append(commands, fmt.Sprintf("kbcli cluster volume-expand %s --components=%s --volume-claim-template-names=%s --storage=%s",
				spec.ClusterRef, v.ComponentName, strings.Join(vctNameSlice[i], ","), storage.String()))
		}
	}
	return commands
}

// getReconfiguringCommand gets the command of the VolumeExpansion command.
func (o *describeOpsOptions) getReconfiguringCommand(spec appsv1alpha1.OpsRequestSpec) []string {
	var (
		updatedParams = spec.Reconfigure
		componentName = updatedParams.ComponentName
	)

	if len(updatedParams.Configurations) == 0 {
		return nil
	}

	configuration := updatedParams.Configurations[0]
	if len(configuration.Keys) == 0 {
		return nil
	}

	commandArgs := make([]string, 0)
	commandArgs = append(commandArgs, "kbcli")
	commandArgs = append(commandArgs, "cluster")
	commandArgs = append(commandArgs, "configure")
	commandArgs = append(commandArgs, spec.ClusterRef)
	commandArgs = append(commandArgs, fmt.Sprintf("--components=%s", componentName))
	commandArgs = append(commandArgs, fmt.Sprintf("--config-spec=%s", configuration.Name))

	config := configuration.Keys[0]
	commandArgs = append(commandArgs, fmt.Sprintf("--config-file=%s", config.Key))
	for _, p := range config.Parameters {
		if p.Value == nil {
			continue
		}
		commandArgs = append(commandArgs, fmt.Sprintf("--set %s=%s", p.Key, *p.Value))
	}
	return []string{strings.Join(commandArgs, " ")}
}

// printOpsRequestStatus prints the OpsRequest status infos.
func (o *describeOpsOptions) printOpsRequestStatus(opsStatus *appsv1alpha1.OpsRequestStatus) {
	printer.PrintTitle("Status")
	startTime := opsStatus.StartTimestamp
	if !startTime.IsZero() {
		printer.PrintPairStringToLine("Start Time", util.TimeFormat(&startTime))
	}
	completeTime := opsStatus.CompletionTimestamp
	if !completeTime.IsZero() {
		printer.PrintPairStringToLine("Completion Time", util.TimeFormat(&completeTime))
	}
	if !startTime.IsZero() {
		printer.PrintPairStringToLine("Duration", util.GetHumanReadableDuration(startTime, completeTime))
	}
	printer.PrintPairStringToLine("Status", string(opsStatus.Phase))
	o.printProgressDetails(opsStatus)
}

// printLastConfiguration prints the last configuration of the cluster before doing the OpsRequest.
func (o *describeOpsOptions) printLastConfiguration(configuration appsv1alpha1.LastConfiguration, opsType appsv1alpha1.OpsType) {
	if reflect.DeepEqual(configuration, appsv1alpha1.LastConfiguration{}) {
		return
	}
	printer.PrintTitle("Last Configuration")
	switch opsType {
	case appsv1alpha1.UpgradeType:
		printer.PrintPairStringToLine("Cluster Version", configuration.ClusterVersionRef)
	case appsv1alpha1.VerticalScalingType:
		handleVScale := func(tbl *printer.TablePrinter, cName string, compConf appsv1alpha1.LastComponentConfiguration) {
			tbl.AddRow(cName, compConf.Requests.Cpu(), compConf.Requests.Memory(), compConf.Limits.Cpu(), compConf.Limits.Memory())
		}
		headers := []interface{}{"COMPONENT", "REQUEST-CPU", "REQUEST-MEMORY", "LIMIT-CPU", "LIMIT-MEMORY"}
		o.printLastConfigurationByOpsType(configuration, headers, handleVScale)
	case appsv1alpha1.HorizontalScalingType:
		handleHScale := func(tbl *printer.TablePrinter, cName string, compConf appsv1alpha1.LastComponentConfiguration) {
			tbl.AddRow(cName, *compConf.Replicas)
		}
		headers := []interface{}{"COMPONENT", "REPLICAS"}
		o.printLastConfigurationByOpsType(configuration, headers, handleHScale)
	case appsv1alpha1.VolumeExpansionType:
		handleVolumeExpansion := func(tbl *printer.TablePrinter, cName string, compConf appsv1alpha1.LastComponentConfiguration) {
			vcts := compConf.VolumeClaimTemplates
			for _, v := range vcts {
				tbl.AddRow(cName, v.Name, v.Storage)
			}
		}
		headers := []interface{}{"COMPONENT", "VOLUME-CLAIM-TEMPLATE", "STORAGE"}
		o.printLastConfigurationByOpsType(configuration, headers, handleVolumeExpansion)
	}
}

// printLastConfigurationByOpsType prints the last configuration by ops type.
func (o *describeOpsOptions) printLastConfigurationByOpsType(configuration appsv1alpha1.LastConfiguration,
	headers []interface{},
	handleOpsObject func(tbl *printer.TablePrinter, cName string, compConf appsv1alpha1.LastComponentConfiguration),
) {
	tbl := printer.NewTablePrinter(o.Out)
	tbl.SetHeader(headers...)
	keys := maps.Keys(configuration.Components)
	sort.Strings(keys)
	for _, cName := range keys {
		handleOpsObject(tbl, cName, configuration.Components[cName])
	}
	tbl.Print()
}

// printProgressDetails prints the progressDetails of all components in this OpsRequest.
func (o *describeOpsOptions) printProgressDetails(opsStatus *appsv1alpha1.OpsRequestStatus) {
	printer.PrintPairStringToLine("Progress", opsStatus.Progress)
	keys := maps.Keys(opsStatus.Components)
	sort.Strings(keys)
	tbl := printer.NewTablePrinter(o.Out)
	tbl.SetHeader(fmt.Sprintf("%-22s%s", "", "OBJECT-KEY"), "STATUS", "DURATION", "MESSAGE")
	for _, cName := range keys {
		progressDetails := opsStatus.Components[cName].ProgressDetails
		for _, v := range progressDetails {
			var groupStr string
			if len(v.Group) > 0 {
				groupStr = fmt.Sprintf("(%s)", v.Group)
			}
			tbl.AddRow(fmt.Sprintf("%-22s%s%s", "", v.ObjectKey, groupStr),
				v.Status, util.GetHumanReadableDuration(v.StartTime, v.EndTime), v.Message)
		}
	}
	//  "-/-" is the progress default value.
	if opsStatus.Progress != "-/-" {
		tbl.Print()
	}
}
