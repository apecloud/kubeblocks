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

package list

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	metav1beta1 "k8s.io/apimachinery/pkg/apis/meta/v1beta1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/printers"
	"k8s.io/cli-runtime/pkg/resource"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	cmdget "k8s.io/kubectl/pkg/cmd/get"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"

	"github.com/apecloud/kubeblocks/internal/cli/printer"
	"github.com/apecloud/kubeblocks/internal/cli/util"
)

type ListOptions struct {
	Factory       cmdutil.Factory
	Namespace     string
	AllNamespaces bool
	LabelSelector string
	FieldSelector string
	ShowLabels    bool
	ToPrinter     func(*meta.RESTMapping, bool) (printers.ResourcePrinterFunc, error)

	// Names are the resource names
	Names  []string
	GVR    schema.GroupVersionResource
	Format printer.Format

	// print the result or not, if true, use default printer to print, otherwise,
	// only return the result to caller.
	Print  bool
	SortBy string
	genericclioptions.IOStreams
}

func NewListOptions(f cmdutil.Factory, streams genericclioptions.IOStreams,
	gvr schema.GroupVersionResource) *ListOptions {
	return &ListOptions{
		Factory:   f,
		IOStreams: streams,
		GVR:       gvr,
		Print:     true,
		SortBy:    ".metadata.name",
	}
}

func (o *ListOptions) AddFlags(cmd *cobra.Command, isClusterScope ...bool) {
	if len(isClusterScope) == 0 || !isClusterScope[0] {
		cmd.Flags().BoolVarP(&o.AllNamespaces, "all-namespace", "A", o.AllNamespaces, "If present, list the requested object(s) across all namespaces. Namespace in current context is ignored even if specified with --namespace.")
	}
	cmd.Flags().StringVarP(&o.LabelSelector, "selector", "l", o.LabelSelector, "Selector (label query) to filter on, supports '=', '==', and '!='.(e.g. -l key1=value1,key2=value2). Matching objects must satisfy all of the specified label constraints.")
	cmd.Flags().BoolVar(&o.ShowLabels, "show-labels", false, "When printing, show all labels as the last column (default hide labels column)")
	//Todo: --sortBy supports custom field sorting, now `list` is to sort using the `.metadata.name` field in default
	printer.AddOutputFlag(cmd, &o.Format)
}

func (o *ListOptions) Complete() error {
	var err error
	o.Namespace, _, err = o.Factory.ToRawKubeConfigLoader().Namespace()
	if err != nil {
		return err
	}

	o.ToPrinter = func(mapping *meta.RESTMapping, withNamespace bool) (printers.ResourcePrinterFunc, error) {
		var p printers.ResourcePrinter
		var kind schema.GroupKind
		if mapping != nil {
			kind = mapping.GroupVersionKind.GroupKind()
		}

		switch o.Format {
		case printer.JSON:
			p = &printers.JSONPrinter{}
		case printer.YAML:
			p = &printers.YAMLPrinter{}
		case printer.Table:
			p = printers.NewTablePrinter(printers.PrintOptions{
				Kind:          kind,
				Wide:          false,
				WithNamespace: o.AllNamespaces,
				ShowLabels:    o.ShowLabels,
			})
		case printer.Wide:
			p = printers.NewTablePrinter(printers.PrintOptions{
				Kind:          kind,
				Wide:          true,
				WithNamespace: o.AllNamespaces,
				ShowLabels:    o.ShowLabels,
			})
		default:
			return nil, genericclioptions.NoCompatiblePrinterError{AllowedFormats: printer.Formats()}
		}

		p, err = printers.NewTypeSetter(scheme.Scheme).WrapToPrinter(p, nil)
		if err != nil {
			return nil, err
		}

		if o.Format.IsHumanReadable() {
			p = &cmdget.SortingPrinter{Delegate: p, SortField: o.SortBy}
			p = &cmdget.TablePrinter{Delegate: p}
		}
		return p.PrintObj, nil
	}

	return nil
}

func (o *ListOptions) Run() (*resource.Result, error) {
	if err := o.Complete(); err != nil {
		return nil, err
	}
	r := o.Factory.NewBuilder().
		Unstructured().
		NamespaceParam(o.Namespace).DefaultNamespace().AllNamespaces(o.AllNamespaces).
		LabelSelectorParam(o.LabelSelector).
		FieldSelectorParam(o.FieldSelector).
		ResourceTypeOrNameArgs(true, append([]string{util.GVRToString(o.GVR)}, o.Names...)...).
		ContinueOnError().
		Latest().
		Flatten().
		TransformRequests(o.transformRequests).
		Do()

	if err := r.Err(); err != nil {
		return nil, err
	}

	// if Print is true, use default printer to print the result, otherwise, only return the result,
	// the caller needs to implement its own printer function to output the result.
	if o.Print {
		return r, o.printResult(r)
	} else {
		return r, nil
	}
}

func (o *ListOptions) transformRequests(req *rest.Request) {
	if !o.Format.IsHumanReadable() || !o.Print {
		return
	}

	req.SetHeader("Accept", strings.Join([]string{
		fmt.Sprintf("application/json;as=Table;v=%s;g=%s", metav1.SchemeGroupVersion.Version, metav1.GroupName),
		fmt.Sprintf("application/json;as=Table;v=%s;g=%s", metav1beta1.SchemeGroupVersion.Version, metav1beta1.GroupName),
		"application/json",
	}, ","))
	if len(o.SortBy) > 0 {
		req.Param("includeObject", "Object")
	}
}

func (o *ListOptions) printResult(r *resource.Result) error {
	if !o.Format.IsHumanReadable() {
		return o.printGeneric(r)
	}

	var allErrs []error
	errs := sets.NewString()
	infos, err := r.Infos()
	if err != nil {
		allErrs = append(allErrs, err)
	}

	objs := make([]runtime.Object, len(infos))
	for ix := range infos {
		objs[ix] = infos[ix].Object
	}

	var printer printers.ResourcePrinter
	var lastMapping *meta.RESTMapping

	tracingWriter := &trackingWriterWrapper{Delegate: o.Out}
	separatorWriter := &separatorWriterWrapper{Delegate: tracingWriter}

	w := printers.GetNewTabWriter(separatorWriter)
	allResourceNamespaced := !o.AllNamespaces
	for ix := range objs {
		info := infos[ix]
		mapping := info.Mapping

		allResourceNamespaced = allResourceNamespaced && info.Namespaced()
		printWithNamespace := o.AllNamespaces

		if mapping != nil && mapping.Scope.Name() == meta.RESTScopeNameRoot {
			printWithNamespace = false
		}

		if shouldGetNewPrinterForMapping(printer, lastMapping, mapping) {
			w.Flush()
			w.SetRememberedWidths(nil)

			if lastMapping != nil && tracingWriter.Written > 0 {
				separatorWriter.SetReady(true)
			}

			printer, err = o.ToPrinter(mapping, printWithNamespace)
			if err != nil {
				if !errs.Has(err.Error()) {
					errs.Insert(err.Error())
					allErrs = append(allErrs, err)
				}
				continue
			}

			lastMapping = mapping
		}

		err = printer.PrintObj(info.Object, w)
		if err != nil {
			if !errs.Has(err.Error()) {
				errs.Insert(err.Error())
				allErrs = append(allErrs, err)
			}
		}
	}

	w.Flush()
	if tracingWriter.Written == 0 && len(allErrs) == 0 {
		o.PrintNotFoundResources()
	}
	return utilerrors.NewAggregate(allErrs)
}

type trackingWriterWrapper struct {
	Delegate io.Writer
	Written  int
}

func (t *trackingWriterWrapper) Write(p []byte) (n int, err error) {
	t.Written += len(p)
	return t.Delegate.Write(p)
}

type separatorWriterWrapper struct {
	Delegate io.Writer
	Ready    bool
}

func (s *separatorWriterWrapper) Write(p []byte) (n int, err error) {
	// If we're about to write non-empty bytes and `s` is ready,
	// we prepend an empty line to `p` and reset `s.Read`.
	if len(p) != 0 && s.Ready {
		fmt.Fprintln(s.Delegate)
		s.Ready = false
	}
	return s.Delegate.Write(p)
}

func (s *separatorWriterWrapper) SetReady(state bool) {
	s.Ready = state
}

func shouldGetNewPrinterForMapping(printer printers.ResourcePrinter, lastMapping, mapping *meta.RESTMapping) bool {
	return printer == nil || lastMapping == nil || mapping == nil || mapping.Resource != lastMapping.Resource
}

// printGeneric copied from kubectl get.go
func (o *ListOptions) printGeneric(r *resource.Result) error {
	var errs []error

	singleItemImplied := false
	infos, err := r.IntoSingleItemImplied(&singleItemImplied).Infos()
	if err != nil {
		if singleItemImplied {
			return err
		}
		errs = append(errs, err)
	}

	if len(infos) == 0 {
		return utilerrors.Reduce(utilerrors.Flatten(utilerrors.NewAggregate(errs)))
	}

	printer, err := o.ToPrinter(nil, false)
	if err != nil {
		return err
	}

	var obj runtime.Object
	if !singleItemImplied || len(infos) != 1 {
		list := corev1.List{
			TypeMeta: metav1.TypeMeta{
				Kind:       "List",
				APIVersion: "v1",
			},
			ListMeta: metav1.ListMeta{},
		}

		for _, info := range infos {
			list.Items = append(list.Items, runtime.RawExtension{Object: info.Object})
		}

		listData, err := json.Marshal(list)
		if err != nil {
			return err
		}

		converted, err := runtime.Decode(unstructured.UnstructuredJSONScheme, listData)
		if err != nil {
			return err
		}

		obj = converted
	} else {
		obj = infos[0].Object
	}

	isList := meta.IsListType(obj)
	if isList {
		items, err := meta.ExtractList(obj)
		if err != nil {
			return err
		}

		list := &unstructured.UnstructuredList{
			Object: map[string]interface{}{
				"kind":       "List",
				"apiVersion": "v1",
				"metadata":   map[string]interface{}{},
			},
		}
		if listMeta, err := meta.ListAccessor(obj); err == nil {
			list.Object["metadata"] = map[string]interface{}{
				"resourceVersion": listMeta.GetResourceVersion(),
			}
		}

		for _, item := range items {
			list.Items = append(list.Items, *item.(*unstructured.Unstructured))
		}
		if err := printer.PrintObj(list, o.Out); err != nil {
			errs = append(errs, err)
		}
		return utilerrors.Reduce(utilerrors.Flatten(utilerrors.NewAggregate(errs)))
	}

	if printErr := printer.PrintObj(obj, o.Out); printErr != nil {
		errs = append(errs, printErr)
	}

	return utilerrors.Reduce(utilerrors.Flatten(utilerrors.NewAggregate(errs)))
}

func (o *ListOptions) PrintNotFoundResources() {
	if !o.AllNamespaces {
		fmt.Fprintf(o.ErrOut, "No %s found in %s namespace.\n", o.GVR.Resource, o.Namespace)
	} else {
		fmt.Fprintf(o.ErrOut, "No %s found\n", o.GVR.Resource)
	}
}
