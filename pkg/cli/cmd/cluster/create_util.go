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
	"strings"

	"github.com/spf13/cobra"
	flag "github.com/spf13/pflag"
	"github.com/stoewer/go-strcase"
	"golang.org/x/exp/maps"
	"golang.org/x/exp/slices"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	utilcomp "k8s.io/kubectl/pkg/util/completion"
	"k8s.io/kubectl/pkg/util/templates"
	"sigs.k8s.io/yaml"

	"github.com/apecloud/kubeblocks/pkg/cli/cluster"
	"github.com/apecloud/kubeblocks/pkg/cli/types"
	"github.com/apecloud/kubeblocks/pkg/cli/util"
	"github.com/apecloud/kubeblocks/pkg/cli/util/flags"
	"github.com/apecloud/kubeblocks/pkg/constant"
)

var (
	resetValFlagNames = []string{
		cluster.VersionSchemaProp.String(),
	}
)

// addCreateFlags adds the flags for creating a cluster, these flags are built by the cluster schema.
func addCreateFlags(cmd *cobra.Command, f cmdutil.Factory, c *cluster.ChartInfo) error {
	if c == nil {
		return nil
	}

	// add the flags for the cluster schema
	if err := flags.BuildFlagsBySchema(cmd, c.Schema); err != nil {
		return err
	}

	// add the flags for sub helm chart
	if err := flags.BuildFlagsBySchema(cmd, c.SubSchema); err != nil {
		return err
	}

	// reset some flags default value, such as version, a suitable version will be chosen
	// by cli if user doesn't specify the version
	resetFlagsValue(cmd.Flags())

	// register completion function for some generic flag
	registerFlagCompFunc(cmd, f, c)
	return nil
}

// getValuesFromFlags gets the values from the flags, these values are used to render a cluster.
func getValuesFromFlags(fs *flag.FlagSet) map[string]interface{} {
	values := make(map[string]interface{}, 0)
	fs.VisitAll(func(f *flag.Flag) {
		if f.Name == "help" {
			return
		}
		var val interface{}
		switch f.Value.Type() {
		case flags.CobraBool:
			val, _ = fs.GetBool(f.Name)
		case flags.CobraInt:
			val, _ = fs.GetInt(f.Name)
		case flags.CobraFloat64:
			val, _ = fs.GetFloat64(f.Name)
		case flags.CobraStringArray:
			val, _ = fs.GetStringArray(f.Name)
		case flags.CobraIntSlice:
			val, _ = fs.GetIntSlice(f.Name)
		case flags.CobraFloat64Slice:
			val, _ = fs.GetFloat64Slice(f.Name)
		case flags.CobraBoolSlice:
			val, _ = fs.GetBoolSlice(f.Name)
		default:
			val, _ = fs.GetString(f.Name)
		}
		values[strcase.LowerCamelCase(f.Name)] = val
	})
	return values
}

// resetFlagsValue reset the default value of some flags
func resetFlagsValue(fs *flag.FlagSet) {
	fs.VisitAll(func(f *flag.Flag) {
		for _, n := range resetValFlagNames {
			if n == f.Name {
				f.DefValue = ""
				_ = f.Value.Set("")
			}
		}
	})
}

func registerFlagCompFunc(cmd *cobra.Command, f cmdutil.Factory, c *cluster.ChartInfo) {
	_ = cmd.RegisterFlagCompletionFunc(string(cluster.VersionSchemaProp),
		func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			var versions []string
			if c != nil && c.ClusterDef != "" {
				label := fmt.Sprintf("%s=%s", constant.ClusterDefLabelKey, c.ClusterDef)
				versions = util.CompGetResourceWithLabels(f, cmd, util.GVRToString(types.ClusterVersionGVR()), []string{label}, toComplete)
			} else {
				versions = utilcomp.CompGetResource(f, util.GVRToString(types.ClusterVersionGVR()), toComplete)
			}
			return versions, cobra.ShellCompDirectiveNoFileComp
		})
}

// buildCreateSubCmdsExamples builds the creation examples for the specified clusterType type.
func buildCreateSubCmdsExamples(t cluster.ClusterType) string {
	exampleTpl := `
	# Create a cluster with the default values
	kbcli cluster create {{ .ClusterType }}

	# Create a cluster with the specified cpu, memory and storage
	kbcli cluster create {{ .ClusterType }} --cpu 1 --memory 2 --storage 10
`

	var builder strings.Builder
	_ = util.PrintGoTemplate(&builder, exampleTpl, map[string]interface{}{
		"ClusterType": t.String(),
	})
	return templates.Examples(builder.String())
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
func buildHelmValues(c *cluster.ChartInfo, values map[string]interface{}) map[string]interface{} {
	if c.SubSchema == nil {
		return values
	}
	// todo: for key like `etcd.cluster` should adjust it to a map like
	subSchemaKeys := maps.Keys(c.SubSchema.Properties)
	newValues := map[string]interface{}{
		c.SubChartName: map[string]interface{}{},
	}
	var build func(key []string, v interface{}, values *map[string]interface{})
	build = func(key []string, v interface{}, values *map[string]interface{}) {
		if len(key) == 1 {
			(*values)[key[0]] = v
			return
		}
		if (*values)[key[0]] == nil {
			(*values)[key[0]] = make(map[string]interface{})
		}
		nextMap := (*values)[key[0]].(map[string]interface{})
		build(key[1:], v, &nextMap)
	}

	for k, v := range values {
		if slices.Contains(subSchemaKeys, k) {
			newValues[c.SubChartName].(map[string]interface{})[k] = v
		} else {
			// todo: fix "."
			build(strings.Split(k, "."), v, &newValues)
		}
	}

	return newValues
}
