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

package flags

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/stoewer/go-strcase"
	"k8s.io/kube-openapi/pkg/validation/spec"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	utilcomp "k8s.io/kubectl/pkg/util/completion"

	"github.com/apecloud/kubeblocks/internal/cli/types"
	"github.com/apecloud/kubeblocks/internal/cli/util"
)

// AddClusterDefinitionFlag adds a flag "cluster-definition" for the cmd and stores the value of the flag
// in string p
func AddClusterDefinitionFlag(f cmdutil.Factory, cmd *cobra.Command, p *string) {
	cmd.Flags().StringVar(p, "cluster-definition", *p, "Specify cluster definition, run \"kbcli clusterdefinition list\" to show all available cluster definition")
	util.CheckErr(cmd.RegisterFlagCompletionFunc("cluster-definition",
		func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			return utilcomp.CompGetResource(f, cmd, util.GVRToString(types.ClusterDefGVR()), toComplete), cobra.ShellCompDirectiveNoFileComp
		}))
}

// BuildFlagsBySchema builds a flag.FlagSet by the given schema, convert the schema key
// to flag name, and convert the schema type to flag type.
func BuildFlagsBySchema(cmd *cobra.Command, schema *spec.Schema) error {
	if schema == nil {
		return nil
	}

	for name, prop := range schema.Properties {
		if err := buildOneFlag(cmd, name, &prop); err != nil {
			return err
		}
	}

	for _, name := range schema.Required {
		flagName := strcase.KebabCase(name)
		if err := cmd.MarkFlagRequired(flagName); err != nil {
			return err
		}
	}

	return nil
}

// castOrZero is a helper function to cast a value to a specific type.
// If the cast fails, the zero value will be returned.
func castOrZero[T any](v any) T {
	cv, _ := v.(T)
	return cv
}

func buildOneFlag(cmd *cobra.Command, k string, s *spec.Schema) error {
	name := strcase.KebabCase(k)
	tpe := "string"
	if len(s.Type) > 0 {
		tpe = s.Type[0]
	}
	// flag not support array type
	switch tpe {
	case "string":
		cmd.Flags().String(name, castOrZero[string](s.Default), buildFlagDescription(s))
	case "integer":
		cmd.Flags().Int(name, int(castOrZero[float64](s.Default)), buildFlagDescription(s))
	case "number":
		cmd.Flags().Float64(name, castOrZero[float64](s.Default), buildFlagDescription(s))
	case "boolean":
		cmd.Flags().Bool(name, castOrZero[bool](s.Default), buildFlagDescription(s))
	case "object":
		for subName, prop := range s.Properties {
			if err := buildOneFlag(cmd, fmt.Sprintf("%s.%s", name, subName), &prop); err != nil {
				return err
			}
		}
	default:
		return fmt.Errorf("unsupported json schema type %s", s.Type)
	}

	registerFlagCompFunc(cmd, name, s)
	return nil
}

func buildFlagDescription(s *spec.Schema) string {
	desc := strings.Builder{}
	desc.WriteString(s.Description)

	var legalVals []string
	for _, e := range s.Enum {
		legalVals = append(legalVals, fmt.Sprintf("%v", e))
	}
	if len(legalVals) > 0 {
		desc.WriteString(fmt.Sprintf(" Legal values [%s].", strings.Join(legalVals, ", ")))
	}

	var valueRange []string
	if s.Minimum != nil {
		valueRange = append(valueRange, fmt.Sprintf("%v", *s.Minimum))
	}
	if s.Maximum != nil {
		valueRange = append(valueRange, fmt.Sprintf("%v", *s.Maximum))
	}
	if len(valueRange) > 0 {
		desc.WriteString(fmt.Sprintf(" Value range [%s].", strings.Join(valueRange, ", ")))
	}
	return desc.String()
}

func registerFlagCompFunc(cmd *cobra.Command, name string, s *spec.Schema) {
	// register the enum entry for autocompletion
	if len(s.Enum) > 0 {
		var entries []string
		for _, e := range s.Enum {
			entries = append(entries, fmt.Sprintf("%s\t", e))
		}
		_ = cmd.RegisterFlagCompletionFunc(name,
			func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
				return entries, cobra.ShellCompDirectiveNoFileComp
			})
		return
	}
}
