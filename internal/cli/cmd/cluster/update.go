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

package cluster

import (
	"encoding/csv"
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/dynamic"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/util/templates"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/cli/cluster"
	"github.com/apecloud/kubeblocks/internal/cli/patch"
	"github.com/apecloud/kubeblocks/internal/cli/types"
	"github.com/apecloud/kubeblocks/internal/cli/util"
)

var clusterUpdateExample = templates.Examples(`
	# update cluster mycluster termination policy to Delete
	kbcli cluster update mycluster --termination-policy=Delete

	# enable cluster monitor
	kbcli cluster update mycluster --monitor=true

    # enable all logs
	kbcli cluster update mycluster --enable-all-logs=true

    # update cluster topology keys and affinity
	kbcli cluster update mycluster --topology-keys=kubernetes.io/hostname --pod-anti-affinity=Required

	# update cluster tolerations
	kbcli cluster update mycluster --tolerations='"key=engineType,value=mongo,operator=Equal,effect=NoSchedule","key=diskType,value=ssd,operator=Equal,effect=NoSchedule"'
`)

type updateOptions struct {
	namespace string
	dynamic   dynamic.Interface
	cluster   *appsv1alpha1.Cluster

	UpdatableFlags
	*patch.Options
}

func NewUpdateCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := &updateOptions{Options: patch.NewOptions(f, streams, types.ClusterGVR())}
	o.Options.OutputOperation = func(didPatch bool) string {
		if didPatch {
			return "updated"
		}
		return "updated (no change)"
	}

	cmd := &cobra.Command{
		Use:               "update NAME",
		Short:             "Update the cluster settings, such as enable or disable monitor or log.",
		Example:           clusterUpdateExample,
		ValidArgsFunction: util.ResourceNameCompletionFunc(f, o.GVR),
		Run: func(cmd *cobra.Command, args []string) {
			util.CheckErr(o.complete(cmd, args))
			util.CheckErr(o.Run(cmd))
		},
	}
	o.UpdatableFlags.addFlags(cmd)
	o.Options.AddFlags(cmd)
	return cmd
}

func (o *updateOptions) complete(cmd *cobra.Command, args []string) error {
	var err error
	if len(args) == 0 {
		return makeMissingClusterNameErr()
	}
	if len(args) > 1 {
		return fmt.Errorf("only support to update one cluster")
	}
	o.Names = args

	// record the flags that been set by user
	var flags []*pflag.Flag
	cmd.Flags().Visit(func(flag *pflag.Flag) {
		flags = append(flags, flag)
	})

	// nothing to do
	if len(flags) == 0 {
		return nil
	}

	if o.namespace, _, err = o.Factory.ToRawKubeConfigLoader().Namespace(); err != nil {
		return err
	}
	if o.dynamic, err = o.Factory.DynamicClient(); err != nil {
		return err
	}
	return o.buildPatch(flags)
}

func (o *updateOptions) buildPatch(flags []*pflag.Flag) error {
	var err error
	type buildFn func(obj map[string]interface{}, v pflag.Value, field string) error

	buildFlagObj := func(obj map[string]interface{}, v pflag.Value, field string) error {
		var val interface{}
		switch v.Type() {
		case "string":
			val = v.String()
		case "stringArray", "stringSlice":
			val = v.(pflag.SliceValue).GetSlice()
		case "stringToString":
			valMap := make(map[string]interface{}, 0)
			vStr := strings.Trim(v.String(), "[]")
			if len(vStr) > 0 {
				r := csv.NewReader(strings.NewReader(vStr))
				ss, err := r.Read()
				if err != nil {
					return err
				}
				for _, pair := range ss {
					kv := strings.SplitN(pair, "=", 2)
					if len(kv) != 2 {
						return fmt.Errorf("%s must be formatted as key=value", pair)
					}
					valMap[kv[0]] = kv[1]
				}
			}
			val = valMap
		}
		return unstructured.SetNestedField(obj, val, field)
	}

	buildTolObj := func(obj map[string]interface{}, v pflag.Value, field string) error {
		tolerations := buildTolerations(o.TolerationsRaw)
		return unstructured.SetNestedField(obj, tolerations, field)
	}

	buildComps := func(obj map[string]interface{}, v pflag.Value, field string) error {
		return o.buildComponents(field, v.String())
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
		"tenancy":            {field: "tenancy", obj: affinity, fn: buildFlagObj},

		// tolerations
		"tolerations": {field: "tolerations", obj: spec, fn: buildTolObj},

		// monitor and logs
		"monitor":         {field: "monitor", obj: nil, fn: buildComps},
		"enable-all-logs": {field: "enable-all-logs", obj: nil, fn: buildComps},
	}

	for _, flag := range flags {
		if f, ok := flagFieldMapping[flag.Name]; ok {
			if err = f.fn(f.obj, flag.Value, f.field); err != nil {
				return err
			}
		}
	}

	if len(affinity) > 0 {
		if err = unstructured.SetNestedField(spec, affinity, "affinity"); err != nil {
			return err
		}
	}

	if o.cluster != nil {
		data, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&o.cluster.Spec)
		if err != nil {
			return err
		}

		if err = unstructured.SetNestedField(spec, data["componentSpecs"], "componentSpecs"); err != nil {
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

func (o *updateOptions) buildComponents(field string, val string) error {
	if o.cluster == nil {
		c, err := cluster.GetClusterByName(o.dynamic, o.Names[0], o.namespace)
		if err != nil {
			return err
		}
		o.cluster = c
	}

	switch field {
	case "monitor":
		return o.setMonitor(val)
	case "enable-all-logs":
		return o.setEnabledLog(val)
	default:
		return nil
	}
}

func (o *updateOptions) setEnabledLog(val string) error {
	boolVal, err := strconv.ParseBool(val)
	if err != nil {
		return err
	}

	// disable all monitor
	if !boolVal {
		for _, c := range o.cluster.Spec.ComponentSpecs {
			c.EnabledLogs = nil
		}
		return nil
	}

	// enable all monitor
	cd, err := cluster.GetClusterDefByName(o.dynamic, o.cluster.Spec.ClusterDefRef)
	if err != nil {
		return err
	}
	setEnableAllLogs(o.cluster, cd)
	return nil
}

func (o *updateOptions) setMonitor(val string) error {
	boolVal, err := strconv.ParseBool(val)
	if err != nil {
		return err
	}

	for i := range o.cluster.Spec.ComponentSpecs {
		o.cluster.Spec.ComponentSpecs[i].Monitor = boolVal
	}
	return nil
}
