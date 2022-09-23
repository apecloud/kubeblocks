/*
Copyright 2022.

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

package list

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/printers"
	"k8s.io/kubectl/pkg/cmd/get"
	"k8s.io/kubectl/pkg/cmd/util"
)

type PrintFlags struct {
	JSONYamlPrintFlags *genericclioptions.JSONYamlPrintFlags
	HumanReadableFlags *get.HumanPrintFlags

	OutputFormat *string
	NoHeaders    *bool
}

func (f *PrintFlags) SetKind(kind schema.GroupKind) {
	f.HumanReadableFlags.SetKind(kind)
}

func (f *PrintFlags) EnsureWithNamespace() {
	_ = f.HumanReadableFlags.EnsureWithNamespace()
}

func (f *PrintFlags) Copy() PrintFlags {
	printFlags := *f
	return printFlags
}

func (f *PrintFlags) AllowedFormats() []string {
	formats := f.JSONYamlPrintFlags.AllowedFormats()
	formats = append(formats, f.HumanReadableFlags.AllowedFormats()...)
	return formats
}

func (f *PrintFlags) ToPrinter() (printers.ResourcePrinter, error) {
	outputFormat := ""
	if f.OutputFormat != nil {
		outputFormat = *f.OutputFormat
	}

	noHeaders := false
	if f.NoHeaders != nil {
		noHeaders = *f.NoHeaders
	}

	f.HumanReadableFlags.NoHeaders = noHeaders

	if p, err := f.JSONYamlPrintFlags.ToPrinter(outputFormat); !genericclioptions.IsNoCompatiblePrinterError(err) {
		return p, err
	}

	if p, err := f.HumanReadableFlags.ToPrinter(outputFormat); !genericclioptions.IsNoCompatiblePrinterError(err) {
		return p, err
	}

	return nil, genericclioptions.NoCompatiblePrinterError{OutputFormat: &outputFormat, AllowedFormats: f.AllowedFormats()}
}

func (f *PrintFlags) AddFlags(cmd *cobra.Command) {
	if f.OutputFormat != nil {
		cmd.Flags().StringVarP(f.OutputFormat, "output", "o", *f.OutputFormat, fmt.Sprintf("Output format. One of: (%s).", strings.Join(f.AllowedFormats(), ", ")))
		util.CheckErr(cmd.RegisterFlagCompletionFunc(
			"output",
			func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
				var comps []string
				for _, format := range f.AllowedFormats() {
					if strings.HasPrefix(format, toComplete) {
						comps = append(comps, format)
					}
				}
				return comps, cobra.ShellCompDirectiveNoFileComp
			},
		))
	}

	if f.NoHeaders != nil {
		cmd.Flags().BoolVar(f.NoHeaders, "no-headers", *f.NoHeaders, "When using the default or custom-column output format, don't print headers (default print headers).")
	}
}

func NewGetPrintFlags() *PrintFlags {
	outputFormat := ""
	noHeaders := false

	return &PrintFlags{
		OutputFormat: &outputFormat,
		NoHeaders:    &noHeaders,

		JSONYamlPrintFlags: genericclioptions.NewJSONYamlPrintFlags(),
		HumanReadableFlags: get.NewHumanPrintFlags(),
	}
}
