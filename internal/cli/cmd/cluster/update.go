/*
Copyright ApeCloud Inc.

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

package cluster

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"

	"github.com/apecloud/kubeblocks/internal/cli/builder"
	"github.com/apecloud/kubeblocks/internal/cli/patch"
	"github.com/apecloud/kubeblocks/internal/cli/types"
	"github.com/apecloud/kubeblocks/internal/cli/util"
)

type updateOptions struct {
	*patch.Options

	// update flags
	TerminationPolicy string              `json:"terminationPolicy"`
	PodAntiAffinity   string              `json:"podAntiAffinity"`
	Monitor           bool                `json:"monitor"`
	EnableAllLogs     bool                `json:"enableAllLogs"`
	TopologyKeys      []string            `json:"topologyKeys,omitempty"`
	NodeLabels        map[string]string   `json:"nodeLabels,omitempty"`
	Tolerations       []map[string]string `json:"tolerations,omitempty"`
	TolerationsRaw    []string            `json:"-"`
}

func newUpdateOptions(streams genericclioptions.IOStreams) *updateOptions {
	o := &updateOptions{
		Options: patch.NewOptions(streams),
	}
	return o
}

func NewUpdateCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := newUpdateOptions(streams)
	cmd := builder.NewCmdBuilder().
		Use("update").
		Short("Update an existing cluster").
		Example("").
		Options(o).
		IOStreams(streams).
		Factory(f).
		GVR(types.ClusterGVR()).
		CustomFlags(o.addFlags).
		CustomComplete(o.complete).
		GetCmd()
	return o.Build(cmd)
}

func (o *updateOptions) addFlags(c *builder.Command) {
	cmd := c.Cmd
	f := cmd.Flags()
	f.StringVar(&o.PodAntiAffinity, "pod-anti-affinity", "", "Pod anti-affinity type")
	f.BoolVar(&o.Monitor, "monitor", true, "Set monitor enabled and inject metrics exporter")
	f.BoolVar(&o.EnableAllLogs, "enable-all-logs", true, "Enable advanced application all log extraction, and true will ignore enabledLogs of component level")
	f.StringVar(&o.TerminationPolicy, "termination-policy", "", "Termination policy, one of: (DoNotTerminate, Halt, Delete, WipeOut)")
	f.StringArrayVar(&o.TopologyKeys, "topology-keys", nil, "Topology keys for affinity")
	f.StringToStringVar(&o.NodeLabels, "node-labels", nil, "Node label selector")
	f.StringSliceVar(&o.TolerationsRaw, "tolerations", nil, `Tolerations for cluster, such as '"key=engineType,value=mongo,operator=Equal,effect=NoSchedule"'`)

	util.CheckErr(cmd.RegisterFlagCompletionFunc(
		"termination-policy",
		func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			return []string{"DoNotTerminate", "Halt", "Delete", "WipeOut"}, cobra.ShellCompDirectiveNoFileComp
		}))
}

func (o *updateOptions) complete(c *builder.Command) error {
	if len(c.Args) == 0 {
		return fmt.Errorf("missing updated cluster name")
	}

	// record the flags that been set by user
	var flags []*pflag.Flag
	c.Cmd.Flags().Visit(func(flag *pflag.Flag) {
		flags = append(flags, flag)
	})

	// nothing to do
	if len(flags) == 0 {
		return nil
	}
	return o.buildPatch(flags)
}

func (o *updateOptions) buildPatch(flags []*pflag.Flag) error {
	var err error
	type buildFn func(obj map[string]interface{}, v string, field string) error

	buildFlagObj := func(obj map[string]interface{}, v string, field string) error {
		return unstructured.SetNestedField(obj, v, field)
	}
	buildTolObj := func(obj map[string]interface{}, v string, field string) error {
		tolerations := buildTolerations(o.TolerationsRaw)
		return unstructured.SetNestedField(obj, tolerations, field)
	}

	spec := map[string]interface{}{}
	affinity := map[string]interface{}{}
	type filedObj struct {
		field string
		obj   map[string]interface{}
		fn    buildFn
	}

	flagFieldMapping := map[string]*filedObj{
		"termination-policy": {field: "terminationPolicy", obj: spec, fn: buildFlagObj},
		"pod-anti-affinity":  {field: "podAntiAffinity", obj: affinity, fn: buildFlagObj},
		"topology-keys":      {field: "topologyKeys", obj: affinity, fn: buildFlagObj},
		"node-labels":        {field: "nodeLabels", obj: affinity, fn: buildFlagObj},
		"tolerations":        {field: "tolerations", obj: spec, fn: buildTolObj},
	}

	for _, flag := range flags {
		if f, ok := flagFieldMapping[flag.Name]; ok {
			if err = f.fn(f.obj, flag.Value.String(), f.field); err != nil {
				return err
			}
		}
	}

	if len(affinity) > 0 {
		if err = unstructured.SetNestedField(spec, affinity, "affinity"); err != nil {
			return err
		}
	}

	obj := unstructured.Unstructured{
		Object: map[string]interface{}{
			"spec": spec,
		},
	}
	bytes, err := obj.MarshalJSON()
	if err != nil {
		return err
	}
	o.Patch = string(bytes)
	return nil
}
