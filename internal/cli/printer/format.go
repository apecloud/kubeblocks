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

package printer

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"k8s.io/klog/v2"
	"k8s.io/kubectl/pkg/cmd/util"
)

// Format is a type for capturing supported output formats
type Format string

const (
	Table Format = "table"
	JSON  Format = "json"
	YAML  Format = "yaml"
	Wide  Format = "wide"
)

var ErrInvalidFormatType = fmt.Errorf("invalid format type")

func Formats() []string {
	return []string{Table.String(), JSON.String(), YAML.String(), Wide.String()}
}

func FormatsWithDesc() map[string]string {
	return map[string]string{
		Table.String(): "Output result in human-readable format",
		JSON.String():  "Output result in JSON format",
		YAML.String():  "Output result in YAML format",
		Wide.String():  "Output result in human-readable format with more information",
	}
}

func (f Format) String() string {
	return string(f)
}

func (f Format) IsHumanReadable() bool {
	return f == Table || f == Wide
}

func ParseFormat(s string) (out Format, err error) {
	switch s {
	case Table.String():
		out, err = Table, nil
	case JSON.String():
		out, err = JSON, nil
	case YAML.String():
		out, err = YAML, nil
	case Wide.String():
		out, err = Wide, nil
	default:
		out, err = "", ErrInvalidFormatType
	}
	return
}

func AddOutputFlag(cmd *cobra.Command, varRef *Format) {
	cmd.Flags().VarP(newOutputValue(Table, varRef), "output", "o",
		fmt.Sprintf("prints the output in the specified format. Allowed values: %s", strings.Join(Formats(), ", ")))
	util.CheckErr(cmd.RegisterFlagCompletionFunc("output",
		func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			var names []string
			for format, desc := range FormatsWithDesc() {
				if strings.HasPrefix(format, toComplete) {
					names = append(names, fmt.Sprintf("%s\t%s", format, desc))
				}
			}
			return names, cobra.ShellCompDirectiveNoFileComp
		}))
}

func AddOutputFlagForCreate(cmd *cobra.Command, varRef *Format, persistent bool) {
	fs := cmd.Flags()
	if persistent {
		fs = cmd.PersistentFlags()
	}
	fs.VarP(newOutputValue(YAML, varRef), "output", "o", "Prints the output in the specified format. Allowed values: JSON and YAML")
}

type outputValue Format

func newOutputValue(defaultValue Format, p *Format) *outputValue {
	*p = defaultValue
	return (*outputValue)(p)
}

func (o *outputValue) String() string {
	return string(*o)
}

func (o *outputValue) Type() string {
	return "format"
}

func (o *outputValue) Set(s string) error {
	outfmt, err := ParseFormat(s)
	if err != nil {
		return err
	}
	*o = outputValue(outfmt)
	return nil
}

// FatalWithRedColor when an error occurs, sets the red color to print it.
func FatalWithRedColor(msg string, code int) {
	if klog.V(99).Enabled() {
		klog.FatalDepth(2, msg)
	}
	if len(msg) > 0 {
		// add newline if needed
		if !strings.HasSuffix(msg, "\n") {
			msg += "\n"
		}
		fmt.Fprint(os.Stderr, BoldRed(msg))
	}
	os.Exit(code)
}
