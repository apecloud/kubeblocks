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
	"encoding/json"
	"fmt"
	"strings"

	jsonpatch "github.com/evanphx/json-patch"
	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	ktypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/resource"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/util/templates"

	"github.com/apecloud/kubeblocks/internal/cli/cluster"
	"github.com/apecloud/kubeblocks/internal/cli/types"
	"github.com/apecloud/kubeblocks/internal/cli/util"
)

var (
	labelExample = templates.Examples(`
		# list label for clusters with specified name
		kbcli cluster label mycluster --list

		# add label 'env' and value 'dev' for clusters with specified name
		kbcli cluster label mycluster env=dev

		# add label 'env' and value 'dev' for all clusters
		kbcli cluster label env=dev --all

		# add label 'env' and value 'dev' for the clusters that match the selector
		kbcli cluster label env=dev -l type=mysql

		# update cluster with the label 'env' with value 'test', overwriting any existing value
		kbcli cluster label mycluster --overwrite env=test

		# delete label env for clusters with specified name
		kbcli cluster label mycluster env-`)
)

type LabelOptions struct {
	Factory cmdutil.Factory
	GVR     schema.GroupVersionResource

	// Common user flags
	overwrite bool
	all       bool
	list      bool
	selector  string

	// results of arg parsing
	resources    []string
	newLabels    map[string]string
	removeLabels []string

	namespace                    string
	enforceNamespace             bool
	dryRunStrategy               cmdutil.DryRunStrategy
	dryRunVerifier               *resource.QueryParamVerifier
	builder                      *resource.Builder
	unstructuredClientForMapping func(mapping *meta.RESTMapping) (resource.RESTClient, error)

	genericclioptions.IOStreams
}

func NewLabelOptions(f cmdutil.Factory, streams genericclioptions.IOStreams, gvr schema.GroupVersionResource) *LabelOptions {
	return &LabelOptions{
		Factory:   f,
		GVR:       gvr,
		IOStreams: streams,
	}
}

func NewLabelCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := NewLabelOptions(f, streams, types.ClusterGVR())
	cmd := &cobra.Command{
		Use:               "label NAME",
		Short:             "Update the labels on cluster",
		Example:           labelExample,
		ValidArgsFunction: util.ResourceNameCompletionFunc(f, o.GVR),
		Run: func(cmd *cobra.Command, args []string) {
			util.CheckErr(o.complete(cmd, args))
			util.CheckErr(o.validate())
			util.CheckErr(o.run())
		},
	}

	cmd.Flags().BoolVar(&o.overwrite, "overwrite", o.overwrite, "If true, allow labels to be overwritten, otherwise reject label updates that overwrite existing labels.")
	cmd.Flags().BoolVar(&o.all, "all", o.all, "Select all cluster")
	cmd.Flags().BoolVar(&o.list, "list", o.list, "If true, display the labels of the clusters")
	cmdutil.AddDryRunFlag(cmd)
	cmdutil.AddLabelSelectorFlagVar(cmd, &o.selector)

	return cmd
}

func (o *LabelOptions) complete(cmd *cobra.Command, args []string) error {
	var err error

	o.dryRunStrategy, err = cmdutil.GetDryRunStrategy(cmd)
	if err != nil {
		return err
	}

	// parse resources and labels
	resources, labelArgs, err := cmdutil.GetResourcesAndPairs(args, "label")
	if err != nil {
		return err
	}
	o.resources = resources
	o.newLabels, o.removeLabels, err = parseLabels(labelArgs)
	if err != nil {
		return err
	}

	o.namespace, o.enforceNamespace, err = o.Factory.ToRawKubeConfigLoader().Namespace()
	if err != nil {
		return nil
	}
	o.builder = o.Factory.NewBuilder()
	o.unstructuredClientForMapping = o.Factory.UnstructuredClientForMapping
	dynamicClient, err := o.Factory.DynamicClient()
	if err != nil {
		return err
	}
	o.dryRunVerifier = resource.NewQueryParamVerifier(dynamicClient, o.Factory.OpenAPIGetter(), resource.QueryParamDryRun)
	return nil
}

func (o *LabelOptions) validate() error {
	if o.all && len(o.selector) > 0 {
		return fmt.Errorf("cannot set --all and --selector at the same time")
	}

	if !o.all && len(o.selector) == 0 && len(o.resources) == 0 {
		return fmt.Errorf("at least one cluster is required")
	}

	if len(o.newLabels) < 1 && len(o.removeLabels) < 1 && !o.list {
		return fmt.Errorf("at least one label update is required")
	}
	return nil
}

func (o *LabelOptions) run() error {
	r := o.builder.
		Unstructured().
		NamespaceParam(o.namespace).DefaultNamespace().
		LabelSelector(o.selector).
		ResourceTypeOrNameArgs(o.all, append([]string{util.GVRToString(o.GVR)}, o.resources...)...).
		ContinueOnError().
		Latest().
		Flatten().
		Do()

	if err := r.Err(); err != nil {
		return err
	}

	infos, err := r.Infos()
	if err != nil {
		return err
	}

	if len(infos) == 0 {
		return fmt.Errorf("no clusters found")
	}

	for _, info := range infos {
		obj := info.Object
		oldData, err := json.Marshal(obj)
		if err != nil {
			return err
		}

		if o.dryRunStrategy == cmdutil.DryRunClient || o.list {
			err = labelFunc(obj, o.overwrite, o.newLabels, o.removeLabels)
			if err != nil {
				return err
			}
		} else {
			name, namespace := info.Name, info.Namespace
			if err != nil {
				return err
			}
			accessor, err := meta.Accessor(obj)
			if err != nil {
				return err
			}
			for _, label := range o.removeLabels {
				if _, ok := accessor.GetLabels()[label]; !ok {
					fmt.Fprintf(o.Out, "label %q not found.\n", label)
				}
			}

			if err := labelFunc(obj, o.overwrite, o.newLabels, o.removeLabels); err != nil {
				return err
			}

			newObj, err := json.Marshal(obj)
			if err != nil {
				return err
			}
			patchBytes, err := jsonpatch.CreateMergePatch(oldData, newObj)
			createPatch := err == nil
			mapping := info.ResourceMapping()
			client, err := o.unstructuredClientForMapping(mapping)
			if err != nil {
				return err
			}
			helper := resource.NewHelper(client, mapping).
				DryRun(o.dryRunStrategy == cmdutil.DryRunServer)
			if createPatch {
				_, err = helper.Patch(namespace, name, ktypes.MergePatchType, patchBytes, nil)
			} else {
				_, err = helper.Replace(namespace, name, false, obj)
			}
			if err != nil {
				return err
			}
		}
	}

	if o.list {
		dynamic, err := o.Factory.DynamicClient()
		if err != nil {
			return err
		}

		client, err := o.Factory.KubernetesClientSet()
		if err != nil {
			return err
		}

		opt := &cluster.PrinterOptions{
			ShowLabels: true,
		}

		p := cluster.NewPrinter(o.IOStreams.Out, cluster.PrintLabels, opt)
		for _, info := range infos {
			if err = addRow(dynamic, client, info.Namespace, info.Name, p); err != nil {
				return err
			}
		}
		p.Print()
	}

	return nil
}

func parseLabels(spec []string) (map[string]string, []string, error) {
	labels := map[string]string{}
	var remove []string
	for _, labelSpec := range spec {
		switch {
		case strings.Contains(labelSpec, "="):
			parts := strings.Split(labelSpec, "=")
			if len(parts) != 2 {
				return nil, nil, fmt.Errorf("invalid label spec: %s", labelSpec)
			}
			labels[parts[0]] = parts[1]
		case strings.HasSuffix(labelSpec, "-"):
			remove = append(remove, labelSpec[:len(labelSpec)-1])
		default:
			return nil, nil, fmt.Errorf("unknown label spec: %s", labelSpec)
		}
	}
	for _, removeLabel := range remove {
		if _, found := labels[removeLabel]; found {
			return nil, nil, fmt.Errorf("can not both modify and remove label in the same command")
		}
	}
	return labels, remove, nil
}

func validateNoOverwrites(obj runtime.Object, labels map[string]string) error {
	accessor, err := meta.Accessor(obj)
	if err != nil {
		return err
	}

	objLabels := accessor.GetLabels()
	if objLabels == nil {
		return nil
	}

	for key := range labels {
		if _, found := objLabels[key]; found {
			return fmt.Errorf("'%s' already has a value (%s), and --overwrite is false", key, objLabels[key])
		}
	}
	return nil
}

func labelFunc(obj runtime.Object, overwrite bool, labels map[string]string, remove []string) error {
	accessor, err := meta.Accessor(obj)
	if err != nil {
		return err
	}
	if !overwrite {
		if err := validateNoOverwrites(obj, labels); err != nil {
			return err
		}
	}

	objLabels := accessor.GetLabels()
	if objLabels == nil {
		objLabels = make(map[string]string)
	}

	for key, value := range labels {
		objLabels[key] = value
	}
	for _, label := range remove {
		delete(objLabels, label)
	}
	accessor.SetLabels(objLabels)

	return nil
}
