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

	"github.com/spf13/cobra"
	flag "github.com/spf13/pflag"
	"k8s.io/kube-openapi/pkg/validation/spec"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	utilcomp "k8s.io/kubectl/pkg/util/completion"

	"github.com/apecloud/kubeblocks/internal/cli/cluster"
	"github.com/apecloud/kubeblocks/internal/cli/create"
	"github.com/apecloud/kubeblocks/internal/cli/types"
	"github.com/apecloud/kubeblocks/internal/cli/util"
)

// addCreateFlags adds the flags for creating a cluster, these flags are built by the cluster schema.
func addCreateFlags(cmd *cobra.Command, f cmdutil.Factory, e cluster.EngineType) error {
	schema, err := cluster.GetSchema(e)
	if err != nil {
		return err
	}

	if schema == nil {
		return fmt.Errorf("failed to find the schema for cluster type %s", e)
	}

	for k, s := range schema.Properties {
		if err = buildOneFlag(cmd, f, k, &s); err != nil {
			return err
		}
	}

	return nil
}

// getValuesFromFlags gets the values from the flags, these values are used to template a cluster.
func getValuesFromFlags(fs *flag.FlagSet) map[string]interface{} {
	values := make(map[string]interface{}, 0)
	fs.VisitAll(func(f *flag.Flag) {
		if f.Name == "help" {
			return
		}
		var val interface{}
		switch f.Value.Type() {
		case "bool":
			val, _ = fs.GetBool(f.Name)
		case "int":
			val, _ = fs.GetInt(f.Name)
		case "float64":
			val, _ = fs.GetFloat64(f.Name)
		default:
			val, _ = fs.GetString(f.Name)
		}
		values[util.ToLowerCamelCase(f.Name)] = val
	})
	return values
}

func buildOneFlag(cmd *cobra.Command, f cmdutil.Factory, k string, s *spec.Schema) error {
	name := util.ToKebabCase(k)
	tpe := "string"
	if len(s.Type) > 0 {
		tpe = s.Type[0]
	}

	switch tpe {
	case "string":
		cmd.Flags().String(name, s.Default.(string), s.Description)
	case "integer":
		cmd.Flags().Int(name, int(s.Default.(float64)), s.Description)
	case "number":
		cmd.Flags().Float64(name, s.Default.(float64), s.Description)
	case "boolean":
		cmd.Flags().Bool(name, s.Default.(bool), s.Description)
	default:
		return fmt.Errorf("unsupported json schema type %s", s.Type)
	}

	registerFlagCompFunc(cmd, f, name, s)
	return nil
}

func registerFlagCompFunc(cmd *cobra.Command, f cmdutil.Factory, name string, s *spec.Schema) {
	// register the enum entry for autocompletion
	if len(s.Enum) > 0 {
		var entries []string
		for _, e := range s.Enum {
			entries = append(entries, fmt.Sprintf("%s\t", e))
		}
		_ = cmd.RegisterFlagCompletionFunc(name, func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			return entries, cobra.ShellCompDirectiveNoFileComp
		})
		return
	}

	// for general property, register the completion function
	switch cluster.SchemaPropName(name) {
	case cluster.VersionProp:
		// TODO(ldm): gets cluster versions based on the cluster engine type, do not get all
		// but, now the cluster version does not has any engine type label, we can not get them by label
		_ = cmd.RegisterFlagCompletionFunc(name,
			func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
				return utilcomp.CompGetResource(f, cmd, util.GVRToString(types.ClusterVersionGVR()), toComplete), cobra.ShellCompDirectiveNoFileComp
			})
	}
}

func getDryRunStrategy(dryRunOpt string) (create.DryRunStrategy, error) {
	if dryRunOpt == "" {
		return create.DryRunNone, nil
	}
	switch dryRunOpt {
	case "client":
		return create.DryRunClient, nil
	case "server":
		return create.DryRunServer, nil
	case "unchanged":
		return create.DryRunClient, nil
	case "none":
		return create.DryRunNone, nil
	default:
		return create.DryRunNone, fmt.Errorf(`invalid dry-run value (%v). Must be "none", "server", or "client"`, dryRunOpt)
	}
}
