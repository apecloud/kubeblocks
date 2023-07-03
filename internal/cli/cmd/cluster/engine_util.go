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
	"github.com/spf13/cobra"
	flag "github.com/spf13/pflag"
	"github.com/stoewer/go-strcase"
	"golang.org/x/exp/maps"
	"golang.org/x/exp/slices"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/kube-openapi/pkg/validation/spec"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	utilcomp "k8s.io/kubectl/pkg/util/completion"
	"sigs.k8s.io/yaml"

	"github.com/apecloud/kubeblocks/internal/cli/cluster"
	"github.com/apecloud/kubeblocks/internal/cli/types"
	"github.com/apecloud/kubeblocks/internal/cli/util"
	"github.com/apecloud/kubeblocks/internal/cli/util/flags"
)

// addEngineFlags adds the flags for creating a cluster, these flags are built by the cluster schema.
func addEngineFlags(cmd *cobra.Command, f cmdutil.Factory, schema *cluster.EngineSchema) error {
	if schema == nil {
		return nil
	}

	// add the flags for the cluster schema
	if err := flags.BuildFlagsBySchema(cmd, f, schema.Schema); err != nil {
		return err
	}

	// add the flags for sub helm chart
	if err := flags.BuildFlagsBySchema(cmd, f, schema.SubSchema); err != nil {
		return err
	}

	registerFlagCompFunc(cmd, f)
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
		values[strcase.LowerCamelCase(f.Name)] = val
	})
	return values
}

func registerFlagCompFunc(cmd *cobra.Command, f cmdutil.Factory) {
	// TODO(ldm): gets cluster versions based on the cluster engine type, do not get all
	// but, now the cluster version does not has any engine type label, we can not get them by label
	_ = cmd.RegisterFlagCompletionFunc(string(cluster.VersionSchemaProp),
		func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			return utilcomp.CompGetResource(f, cmd, util.GVRToString(types.ClusterVersionGVR()), toComplete), cobra.ShellCompDirectiveNoFileComp
		})
}

// buildEngineCreateExamples builds the creation examples for the specified engine type.
func buildEngineCreateExamples(e cluster.EngineType, schema *spec.SchemaProps) string {
	return ""
}

// getObjectsInfo gets the objects info from the manifests.
func getObjectsInfo(f cmdutil.Factory, manifests map[string]string) ([]*objectInfo, error) {
	mapper, err := f.ToRESTMapper()
	if err != nil {
		return nil, err
	}

	var objects []*objectInfo
	for _, manifest := range manifests {
		objInfo := &objectInfo{}

		// convert yaml to json
		jsonData, err := yaml.YAMLToJSON([]byte(manifest))
		if err != nil {
			return nil, err
		}

		// get resource gvk
		obj, gvk, err := unstructured.UnstructuredJSONScheme.Decode(jsonData, nil, nil)
		if err != nil {
			return nil, err
		}

		// convert gvk to gvr
		m, err := mapper.RESTMapping(gvk.GroupKind(), gvk.Version)
		if err != nil {
			return nil, err
		}

		objInfo.obj = obj.(*unstructured.Unstructured)
		objInfo.gvr = m.Resource
		objects = append(objects, objInfo)
	}
	return objects, nil
}

// buildHelmValues builds the helm values from the cluster schema and the values from the flags.
// For helm, the sub chart values should be in the sub map of the values.
func buildHelmValues(schema *cluster.EngineSchema, values map[string]interface{}) map[string]interface{} {
	if schema == nil || schema.SubSchema == nil {
		return values
	}

	subSchemaKeys := maps.Keys(schema.SubSchema.Properties)
	newValues := map[string]interface{}{
		schema.SubChartName: map[string]interface{}{},
	}

	for k, v := range values {
		if slices.Contains(subSchemaKeys, k) {
			subValues := newValues[schema.SubChartName]
			subValues.(map[string]interface{})[k] = v
		} else {
			newValues[k] = v
		}
	}
	return newValues
}
