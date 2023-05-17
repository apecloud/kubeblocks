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

package fault

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"

	"github.com/apecloud/kubeblocks/internal/cli/create"
	"github.com/apecloud/kubeblocks/internal/cli/printer"
	"github.com/apecloud/kubeblocks/internal/cli/util"
)

type Selector struct {
	PodNameSelectors map[string][]string `json:"pods"`

	NamespaceSelectors []string `json:"namespaces"`

	LabelSelectors map[string]string `json:"labelSelectors"`

	PodPhaseSelectors []string `json:"podPhaseSelectors"`

	NodeLabelSelectors map[string]string `json:"nodeSelectors"`

	AnnotationSelectors map[string]string `json:"annotationSelectors"`

	NodeNameSelectors []string `json:"nodes"`
}

type FaultBaseOptions struct {
	Action string `json:"action"`

	Mode string `json:"mode"`

	Value string `json:"value"`

	Duration string `json:"duration"`

	Selector `json:"selector"`

	create.CreateOptions `json:"-"`
}

func NewFaultCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "fault",
		Short: "Inject faults to pod.",
	}
	cmd.AddCommand(
		NewPodChaosCmd(f, streams),
		NewNetworkChaosCmd(f, streams),
		NewNodeChaosCmd(f, streams),
	)
	return cmd
}

func registerFlagCompletionFunc(cmd *cobra.Command, f cmdutil.Factory) {
	var formatsWithDesc = map[string]string{
		"JSON": "Output result in JSON format",
		"YAML": "Output result in YAML format",
	}
	util.CheckErr(cmd.RegisterFlagCompletionFunc("output",
		func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			var names []string
			for format, desc := range formatsWithDesc {
				if strings.HasPrefix(format, toComplete) {
					names = append(names, fmt.Sprintf("%s\t%s", format, desc))
				}
			}
			return names, cobra.ShellCompDirectiveNoFileComp
		}))
}

func (o *FaultBaseOptions) AddCommonFlag(cmd *cobra.Command) {
	cmd.Flags().StringVar(&o.Mode, "mode", "all", `You can select "one", "all", "fixed", "fixed-percent", "random-max-percent", Specify the experimental mode, that is, which Pods to experiment with.`)
	cmd.Flags().StringVar(&o.Value, "value", "", `If you choose mode=fixed or fixed-percent or random-max-percent, you can enter a value to specify the number or percentage of pods you want to inject.`)
	cmd.Flags().StringVar(&o.Duration, "duration", "10s", "Supported formats of the duration are: ms / s / m / h.")
	cmd.Flags().StringToStringVar(&o.LabelSelectors, "label", map[string]string{}, `label for pod, such as '"app.kubernetes.io/component=mysql, statefulset.kubernetes.io/pod-name=mycluster-mysql-0.`)
	cmd.Flags().StringArrayVar(&o.NamespaceSelectors, "ns-fault", []string{"default"}, `Specifies the namespace into which you want to inject faults.`)
	cmd.Flags().StringArrayVar(&o.PodPhaseSelectors, "phase", []string{}, `Specify the pod that injects the fault by the state of the pod.`)
	cmd.Flags().StringToStringVar(&o.NodeLabelSelectors, "node-label", map[string]string{}, `label for node, such as '"kubernetes.io/arch=arm64,kubernetes.io/hostname=minikube-m03,kubernetes.io/os=linux.`)
	cmd.Flags().StringArrayVar(&o.NodeNameSelectors, "node", []string{}, `Inject faults into pods in the specified node.`)
	cmd.Flags().StringToStringVar(&o.AnnotationSelectors, "annotation", map[string]string{}, `Select the pod to inject the fault according to Annotation.`)

	cmd.Flags().StringVar(&o.DryRun, "dry-run", "none", `Must be "client", or "server". If client strategy, only print the object that would be sent, without sending it. If server strategy, submit server-side request without persisting the resource.`)
	cmd.Flags().Lookup("dry-run").NoOptDefVal = Unchanged

	printer.AddOutputFlagForCreate(cmd, &o.Format)
}

func (o *FaultBaseOptions) BaseValidate() error {
	if ok, err := IsRegularMatch(o.Duration); !ok {
		return err
	}

	if o.Value == "" && (o.Mode == "fixed" || o.Mode == "fixed-percent" || o.Mode == "random-max-percent") {
		return fmt.Errorf("you must use --value to specify an integer")
	}

	if ok, err := IsInteger(o.Value); !ok {
		return err
	}

	return nil
}

func (o *FaultBaseOptions) BaseComplete() error {
	if len(o.Args) > 0 {
		o.PodNameSelectors = make(map[string][]string, len(o.NamespaceSelectors))
		for _, ns := range o.NamespaceSelectors {
			o.PodNameSelectors[ns] = o.Args
		}
	}
	return nil
}

func IsRegularMatch(str string) (bool, error) {
	pattern := regexp.MustCompile(`^\d+(ms|s|m|h)$`)
	if str != "" && !pattern.MatchString(str) {
		return false, fmt.Errorf("invalid duration:%s; input format must be in the form of number + time unit, like 10s, 10m", str)
	} else {
		return true, nil
	}
}

func IsInteger(str string) (bool, error) {
	if _, err := strconv.Atoi(str); str != "" && err != nil {
		return false, fmt.Errorf("invalid value:%s; must be an integer", str)
	} else {
		return true, nil
	}
}

func GetGVR(group, version, resourceName string) schema.GroupVersionResource {
	return schema.GroupVersionResource{Group: group, Version: version, Resource: resourceName}
}
