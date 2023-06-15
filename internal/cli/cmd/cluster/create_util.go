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

	"github.com/apecloud/kubeblocks/internal/cli/cluster"
	"github.com/apecloud/kubeblocks/internal/cli/util"
)

// addCreateFlags adds the flags for creating a cluster, these flags are built by the cluster schema.
func addCreateFlags(cmd *cobra.Command, e cluster.EngineType) error {
	schema, err := cluster.GetSchema(e)
	if err != nil {
		return err
	}

	if schema == nil {
		return fmt.Errorf("failed to find the schema for cluster type %s", e)
	}

	for k, s := range schema.Properties {
		if err = buildOneFlag(cmd, k, &s); err != nil {
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
		case "int32":
			val, _ = fs.GetInt32(f.Name)
		case "float64":
			val, _ = fs.GetFloat64(f.Name)
		default:
			val, _ = fs.GetString(f.Name)
		}
		values[util.ToLowerCamelCase(f.Name)] = val
	})
	return values
}

func buildOneFlag(cmd *cobra.Command, k string, s *spec.Schema) error {
	name := util.ToKebabCase(k)
	tpe := "string"
	if len(s.Type) > 0 {
		tpe = s.Type[0]
	}

	switch tpe {
	case "string":
		cmd.Flags().String(name, s.Default.(string), s.Description)
	case "integer":
		cmd.Flags().Int32(name, int32(s.Default.(float64)), s.Description)
	case "number":
		cmd.Flags().Float64(name, s.Default.(float64), s.Description)
	case "boolean":
		cmd.Flags().Bool(name, s.Default.(bool), s.Description)
	default:
		return fmt.Errorf("unsupported json schema type %s", s.Type)
	}
	return nil
}
