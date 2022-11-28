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

package get

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	metav1beta1 "k8s.io/apimachinery/pkg/apis/meta/v1beta1"
	"k8s.io/apimachinery/pkg/runtime"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/printers"
	"k8s.io/cli-runtime/pkg/resource"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	cmdget "k8s.io/kubectl/pkg/cmd/get"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
)

type Options struct {
	PrintFlags             *cmdget.PrintFlags
	ToPrinter              func(*meta.RESTMapping, bool) (printers.ResourcePrinterFunc, error)
	IsHumanReadablePrinter bool

	LabelSelector     string
	FieldSelector     string
	BuildArgs         []string
	AllNamespaces     bool
	Namespace         string
	ExplicitNamespace bool

	ServerPrint    bool
	NoHeaders      bool
	IgnoreNotFound bool

	genericclioptions.IOStreams
}

func NewOptions(streams genericclioptions.IOStreams, args []string) *Options {
	return &Options{
		PrintFlags:  cmdget.NewGetPrintFlags(),
		BuildArgs:   args,
		IOStreams:   streams,
		ServerPrint: true,
	}
}

func (o *Options) Complete(f cmdutil.Factory) error {
	var err error
	o.Namespace, o.ExplicitNamespace, err = f.ToRawKubeConfigLoader().Namespace()
	if err != nil {
		return err
	}

	if o.AllNamespaces {
		o.ExplicitNamespace = false
	}

	o.ToPrinter = func(mapping *meta.RESTMapping, withNamespace bool) (printers.ResourcePrinterFunc, error) {
		printFlags := o.PrintFlags.Copy()

		if mapping != nil {
			printFlags.SetKind(mapping.GroupVersionKind.GroupKind())
		}

		if withNamespace {
			_ = printFlags.EnsureWithNamespace()
		}

		printer, err := printFlags.ToPrinter()
		if err != nil {
			return nil, err
		}

		printer, err = printers.NewTypeSetter(scheme.Scheme).WrapToPrinter(printer, nil)
		if err != nil {
			return nil, err
		}

		if o.ServerPrint {
			printer = &cmdget.TablePrinter{Delegate: printer}
		}
		return printer.PrintObj, nil
	}

	return nil
}

func (o *Options) transformRequests(req *rest.Request) {
	if !o.ServerPrint || !o.IsHumanReadablePrinter {
		return
	}

	req.SetHeader("Accept", strings.Join([]string{
		fmt.Sprintf("application/json;as=Table;v=%s;g=%s", metav1.SchemeGroupVersion.Version, metav1.GroupName),
		fmt.Sprintf("application/json;as=Table;v=%s;g=%s", metav1beta1.SchemeGroupVersion.Version, metav1beta1.GroupName),
		"application/json",
	}, ","))
}

func (o *Options) Run(f cmdutil.Factory) error {
	r := f.NewBuilder().
		Unstructured().
		NamespaceParam(o.Namespace).DefaultNamespace().AllNamespaces(o.AllNamespaces).
		LabelSelectorParam(o.LabelSelector).
		FieldSelectorParam(o.FieldSelector).
		ResourceTypeOrNameArgs(true, o.BuildArgs...).
		ContinueOnError().
		Latest().
		Flatten().
		TransformRequests(o.transformRequests).
		Do()

	if err := r.Err(); err != nil {
		return err
	}

	if !o.IsHumanReadablePrinter {
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

			if lastMapping != nil && !o.NoHeaders && tracingWriter.Written > 0 {
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
	if tracingWriter.Written == 0 && !o.IgnoreNotFound && len(allErrs) == 0 {
		if allResourceNamespaced {
			fmt.Fprintf(o.ErrOut, "No resources found in %s namespace.\n", o.Namespace)
		} else {
			fmt.Fprintln(o.ErrOut, "No resources found")
		}
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
func (o *Options) printGeneric(r *resource.Result) error {
	var errs []error

	singleItemImplied := false
	infos, err := r.IntoSingleItemImplied(&singleItemImplied).Infos()
	if err != nil {
		if singleItemImplied {
			return err
		}
		errs = append(errs, err)
	}

	if len(infos) == 0 && o.IgnoreNotFound {
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
