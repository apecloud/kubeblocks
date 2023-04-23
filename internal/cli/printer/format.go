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

package printer

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
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

func AddOutputFlagForCreate(cmd *cobra.Command, varRef *Format) {
	cmd.Flags().VarP(newOutputValue(YAML, varRef), "output", "o", "prints the output in the specified format. Allowed values: JSON and YAML")
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
