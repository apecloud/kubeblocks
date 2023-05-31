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

package patch

import (
	"fmt"
	"reflect"
	"strings"

	jsonpatch "github.com/evanphx/json-patch"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/printers"
	"k8s.io/cli-runtime/pkg/resource"
	"k8s.io/client-go/kubernetes/scheme"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"

	"github.com/apecloud/kubeblocks/internal/cli/util"
)

type OutputOperation func(bool) string

type Options struct {
	Factory cmdutil.Factory

	// resource names
	Names           []string
	GVR             schema.GroupVersionResource
	OutputOperation OutputOperation

	// following fields are similar to kubectl patch
	PrintFlags  *genericclioptions.PrintFlags
	ToPrinter   func(string) (printers.ResourcePrinter, error)
	Patch       string
	Subresource string

	namespace                    string
	enforceNamespace             bool
	dryRunStrategy               cmdutil.DryRunStrategy
	dryRunVerifier               *resource.QueryParamVerifier
	args                         []string
	builder                      *resource.Builder
	unstructuredClientForMapping func(mapping *meta.RESTMapping) (resource.RESTClient, error)
	fieldManager                 string

	genericclioptions.IOStreams
}

func NewOptions(f cmdutil.Factory, streams genericclioptions.IOStreams, gvr schema.GroupVersionResource) *Options {
	return &Options{
		Factory:         f,
		GVR:             gvr,
		PrintFlags:      genericclioptions.NewPrintFlags("").WithTypeSetter(scheme.Scheme),
		IOStreams:       streams,
		OutputOperation: patchOperation,
	}
}

func (o *Options) AddFlags(cmd *cobra.Command) {
	o.PrintFlags.AddFlags(cmd)
	cmdutil.AddDryRunFlag(cmd)
}

func (o *Options) complete(cmd *cobra.Command) error {
	var err error
	if len(o.Names) == 0 {
		return fmt.Errorf("missing %s name", o.GVR.Resource)
	}

	o.dryRunStrategy, err = cmdutil.GetDryRunStrategy(cmd)
	if err != nil {
		return err
	}

	cmdutil.PrintFlagsWithDryRunStrategy(o.PrintFlags, o.dryRunStrategy)
	o.ToPrinter = func(operation string) (printers.ResourcePrinter, error) {
		o.PrintFlags.NamePrintFlags.Operation = operation
		return o.PrintFlags.ToPrinter()
	}

	o.namespace, o.enforceNamespace, err = o.Factory.ToRawKubeConfigLoader().Namespace()
	if err != nil {
		return err
	}
	o.args = append([]string{util.GVRToString(o.GVR)}, o.Names...)
	o.builder = o.Factory.NewBuilder()
	o.unstructuredClientForMapping = o.Factory.UnstructuredClientForMapping
	dynamicClient, err := o.Factory.DynamicClient()
	if err != nil {
		return err
	}
	o.dryRunVerifier = resource.NewQueryParamVerifier(dynamicClient, o.Factory.OpenAPIGetter(), resource.QueryParamDryRun)
	return nil
}

func (o *Options) Run(cmd *cobra.Command) error {
	if err := o.complete(cmd); err != nil {
		return err
	}

	if len(o.Patch) == 0 {
		return fmt.Errorf("the contents of the patch is empty")
	}

	// for CRD, we always use Merge patch type
	patchType := types.MergePatchType
	patchBytes, err := yaml.ToJSON([]byte(o.Patch))
	if err != nil {
		return fmt.Errorf("unable to parse %q: %v", o.Patch, err)
	}

	r := o.builder.
		Unstructured().
		ContinueOnError().
		NamespaceParam(o.namespace).DefaultNamespace().
		Subresource(o.Subresource).
		ResourceTypeOrNameArgs(false, o.args...).
		Flatten().
		Do()
	err = r.Err()
	if err != nil {
		return err
	}
	count := 0
	err = r.Visit(func(info *resource.Info, err error) error {
		if err != nil {
			return err
		}
		count++
		name, namespace := info.Name, info.Namespace
		if o.dryRunStrategy != cmdutil.DryRunClient {
			mapping := info.ResourceMapping()
			if o.dryRunStrategy == cmdutil.DryRunServer {
				if err := o.dryRunVerifier.HasSupport(mapping.GroupVersionKind); err != nil {
					return err
				}
			}
			client, err := o.unstructuredClientForMapping(mapping)
			if err != nil {
				return err
			}

			helper := resource.
				NewHelper(client, mapping).
				DryRun(o.dryRunStrategy == cmdutil.DryRunServer).
				WithFieldManager(o.fieldManager).
				WithSubresource(o.Subresource)
			patchedObj, err := helper.Patch(namespace, name, patchType, patchBytes, nil)
			if err != nil {
				if apierrors.IsUnsupportedMediaType(err) {
					return errors.Wrap(err, fmt.Sprintf("%s is not supported by %s", patchType, mapping.GroupVersionKind))
				}
				return err
			}

			didPatch := !reflect.DeepEqual(info.Object, patchedObj)
			printer, err := o.ToPrinter(o.OutputOperation(didPatch))
			if err != nil {
				return err
			}
			return printer.PrintObj(patchedObj, o.Out)
		}

		originalObjJS, err := runtime.Encode(unstructured.UnstructuredJSONScheme, info.Object)
		if err != nil {
			return err
		}

		originalPatchedObjJS, err := getPatchedJSON(patchType, originalObjJS, patchBytes, info.Object.GetObjectKind().GroupVersionKind(), scheme.Scheme)
		if err != nil {
			return err
		}

		targetObj, err := runtime.Decode(unstructured.UnstructuredJSONScheme, originalPatchedObjJS)
		if err != nil {
			return err
		}

		didPatch := !reflect.DeepEqual(info.Object, targetObj)
		printer, err := o.ToPrinter(o.OutputOperation(didPatch))
		if err != nil {
			return err
		}
		return printer.PrintObj(targetObj, o.Out)
	})
	if err != nil {
		return err
	}
	if count == 0 {
		return fmt.Errorf("no objects passed to patch")
	}
	return nil
}

func getPatchedJSON(patchType types.PatchType, originalJS, patchJS []byte, gvk schema.GroupVersionKind, oc runtime.ObjectCreater) ([]byte, error) {
	switch patchType {
	case types.JSONPatchType:
		patchObj, err := jsonpatch.DecodePatch(patchJS)
		if err != nil {
			return nil, err
		}
		bytes, err := patchObj.Apply(originalJS)
		// TODO: This is pretty hacky, we need a better structured error from the json-patch
		if err != nil && strings.Contains(err.Error(), "doc is missing key") {
			msg := err.Error()
			ix := strings.Index(msg, "key:")
			key := msg[ix+5:]
			return bytes, fmt.Errorf("object to be patched is missing field (%s)", key)
		}
		return bytes, err

	case types.MergePatchType:
		return jsonpatch.MergePatch(originalJS, patchJS)

	case types.StrategicMergePatchType:
		// get a typed object for this GVK if we need to apply a strategic merge patch
		obj, err := oc.New(gvk)
		if err != nil {
			return nil, fmt.Errorf("strategic merge patch is not supported for %s locally, try --type merge", gvk.String())
		}
		return strategicpatch.StrategicMergePatch(originalJS, patchJS, obj)

	default:
		// only here as a safety net - go-restful filters content-type
		return nil, fmt.Errorf("unknown Content-Type header for patch: %v", patchType)
	}
}

func patchOperation(didPatch bool) string {
	if didPatch {
		return "patched"
	}
	return "patched (no change)"
}
