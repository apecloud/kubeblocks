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

	"github.com/apecloud/kubeblocks/internal/cli/util"
)

type FaultBaseOptions struct {
	Action string `json:"action"`

	Mode string `json:"mode"`

	Value string `json:"value"`

	NamespaceSelector []string `json:"namespaceSelector"`

	Label map[string]string `json:"label"`

	Duration string `json:"duration"`
}

func NewFaultCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "fault",
		Short: "inject faults to pod.",
	}
	cmd.AddCommand(
		NewNetworkChaosCmd(f, streams),
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
