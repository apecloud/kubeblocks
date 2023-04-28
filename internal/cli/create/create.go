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

package create

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/ast"
	"cuelang.org/go/cue/cuecontext"
	cuejson "cuelang.org/go/encoding/json"
	"github.com/leaanthony/debme"
	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	k8sapitypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/printers"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/scheme"

	"github.com/apecloud/kubeblocks/internal/cli/printer"
	"github.com/apecloud/kubeblocks/internal/cli/types"

	"github.com/apecloud/kubeblocks/internal/cli/util"
)

var (
	//go:embed template/*
	cueTemplate embed.FS
)

type DryRunStrategy int

const (
	// DryRunNone indicates the client will make all mutating calls
	DryRunNone DryRunStrategy = iota
	DryRunClient
	DryRunServer
)

type Inputs struct {
	// Use cobra command use
	Use string

	// Short is the short description shown in the 'help' output.
	Short string

	// Example is examples of how to use the command.
	Example string

	// BaseOptionsObj
	BaseOptionsObj *BaseOptions

	// Options a command options object which extends BaseOptions
	Options interface{}

	// CueTemplateName cue template file name
	CueTemplateName string

	// ResourceName k8s resource name
	ResourceName string

	// Group of API, default is apps
	Group string

	// Group of Version, default is v1alpha1
	Version string

	// Factory
	Factory cmdutil.Factory

	// ValidateFunc optional, custom validate func
	Validate func() error

	// Complete optional, do custom complete options
	Complete func() error

	BuildFlags func(*cobra.Command)

	// PreCreate optional, make changes on yaml before create
	PreCreate func(*unstructured.Unstructured) error

	// CustomOutPut will be executed after creating successfully.
	CustomOutPut func(options *BaseOptions)

	// CleanUpFn will be executed after creating failed.
	CleanUpFn func() error

	// ResourceNameGVRForCompletion resource name for completion.
	ResourceNameGVRForCompletion schema.GroupVersionResource
}

// BaseOptions the options of creation command should inherit baseOptions
type BaseOptions struct {
	// Namespace k8s namespace
	Namespace string `json:"namespace"`

	// Name Resource name of the command line operation
	Name string `json:"name"`

	Dynamic dynamic.Interface `json:"-"`

	Client kubernetes.Interface `json:"-"`

	ToPrinter func(*meta.RESTMapping, bool) (printers.ResourcePrinterFunc, error) `json:"-"`

	Format printer.Format `json:"-"`

	DryRunStrategy string `json:"-"`

	// Quiet minimize unnecessary output
	Quiet bool

	genericclioptions.IOStreams
}

// BuildCommand build create command
func BuildCommand(inputs Inputs) *cobra.Command {
	cmd := &cobra.Command{
		Use:               inputs.Use,
		Short:             inputs.Short,
		Example:           inputs.Example,
		ValidArgsFunction: util.ResourceNameCompletionFunc(inputs.Factory, inputs.ResourceNameGVRForCompletion),
		Run: func(cmd *cobra.Command, args []string) {
			err := inputs.BaseOptionsObj.Complete(inputs, args)
			if err == nil {
				err = inputs.BaseOptionsObj.Validate(inputs)
			}
			if err == nil {
				err = inputs.BaseOptionsObj.Run(inputs)
			}
			if err == nil {
				return
			}

			// if err is not nil, clean up
			if cleanErr := inputs.BaseOptionsObj.CleanUp(inputs); cleanErr != nil {
				fmt.Fprintf(inputs.BaseOptionsObj.ErrOut, "clean up failed: %v\n", cleanErr)
			}
			util.CheckErr(err)
		},
	}
	if inputs.BuildFlags != nil {
		inputs.BuildFlags(cmd)
	}
	return cmd
}

func (o *BaseOptions) Complete(inputs Inputs, args []string) error {
	var err error
	if o.Namespace, _, err = inputs.Factory.ToRawKubeConfigLoader().Namespace(); err != nil {
		return err
	}

	if len(args) > 0 {
		o.Name = args[0]
	}

	if o.Dynamic, err = inputs.Factory.DynamicClient(); err != nil {
		return err
	}

	if o.Client, err = inputs.Factory.KubernetesClientSet(); err != nil {
		return err
	}

	o.ToPrinter = func(mapping *meta.RESTMapping, withNamespace bool) (printers.ResourcePrinterFunc, error) {
		var p printers.ResourcePrinter
		switch o.Format {
		case printer.JSON:
			p = &printers.JSONPrinter{}
		case printer.YAML:
			p = &printers.YAMLPrinter{}
		default:
			return nil, genericclioptions.NoCompatiblePrinterError{AllowedFormats: []string{"JSON", "YAML"}}
		}

		p, err = printers.NewTypeSetter(scheme.Scheme).WrapToPrinter(p, nil)
		if err != nil {
			return nil, err
		}
		return p.PrintObj, nil
	}

	// do custom options complete
	if inputs.Complete != nil {
		if err = inputs.Complete(); err != nil {
			return err
		}
	}
	return nil
}

func (o *BaseOptions) Validate(inputs Inputs) error {
	// do options validate
	if inputs.Validate != nil {
		if err := inputs.Validate(); err != nil {
			return err
		}
	}
	return nil
}

// Run execute command. the options of parameter contain the command flags and args.
func (o *BaseOptions) Run(inputs Inputs) error {
	var (
		cueValue        cue.Value
		err             error
		unstructuredObj *unstructured.Unstructured
		optionsByte     []byte
	)

	if optionsByte, err = json.Marshal(inputs.Options); err != nil {
		return err
	}

	if cueValue, err = newCueValue(inputs.CueTemplateName); err != nil {
		return err
	}

	if cueValue, err = fillOptions(cueValue, optionsByte); err != nil {
		return err
	}

	if unstructuredObj, err = convertContentToUnstructured(cueValue); err != nil {
		return err
	}

	if inputs.PreCreate != nil {
		if err = inputs.PreCreate(unstructuredObj); err != nil {
			return err
		}
	}
	group := inputs.Group
	if len(group) == 0 {
		group = types.AppsAPIGroup
	}

	version := inputs.Version
	if len(version) == 0 {
		version = types.AppsAPIVersion
	}

	previewObj := unstructuredObj
	dryRunStrategy, err := o.GetDryRunStrategy()
	if err != nil {
		return err
	}

	if dryRunStrategy != DryRunClient {
		gvr := schema.GroupVersionResource{Group: group, Version: version, Resource: inputs.ResourceName}
		createOptions := metav1.CreateOptions{}

		if dryRunStrategy == DryRunServer {
			createOptions.DryRun = []string{metav1.DryRunAll}
		}
		// create k8s resource
		previewObj, err = o.Dynamic.Resource(gvr).Namespace(o.Namespace).Create(context.TODO(), previewObj, createOptions)
		if err != nil {
			return err
		}
		if dryRunStrategy != DryRunServer {
			o.Name = previewObj.GetName()
			if o.Quiet {
				return nil
			}
			if inputs.CustomOutPut != nil {
				inputs.CustomOutPut(o)
			} else {
				fmt.Fprintf(o.Out, "%s %s created\n", previewObj.GetKind(), previewObj.GetName())
			}
			return nil
		}
	}
	printer, err := o.ToPrinter(nil, false)
	if err != nil {
		return err
	}
	return printer.PrintObj(previewObj, o.Out)
}

func (o *BaseOptions) CleanUp(inputs Inputs) error {
	if inputs.CleanUpFn != nil {
		return inputs.CleanUpFn()
	}
	return nil
}

// RunAsApply execute command. the options of parameter contain the command flags and args.
// if the resource exists, run as "kubectl apply".
func (o *BaseOptions) RunAsApply(inputs Inputs) error {
	var (
		cueValue        cue.Value
		err             error
		unstructuredObj *unstructured.Unstructured
		optionsByte     []byte
	)

	if optionsByte, err = json.Marshal(inputs.Options); err != nil {
		return err
	}

	if cueValue, err = newCueValue(inputs.CueTemplateName); err != nil {
		return err
	}

	if cueValue, err = fillOptions(cueValue, optionsByte); err != nil {
		return err
	}

	if unstructuredObj, err = convertContentToUnstructured(cueValue); err != nil {
		return err
	}

	group := inputs.Group
	if len(group) == 0 {
		group = types.AppsAPIGroup
	}

	version := inputs.Version
	if len(version) == 0 {
		version = types.AppsAPIVersion
	}
	// create k8s resource
	gvr := schema.GroupVersionResource{Group: group, Version: version, Resource: inputs.ResourceName}
	objectName, _, err := unstructured.NestedString(unstructuredObj.Object, "metadata", "name")
	if err != nil {
		return err
	}
	objectByte, err := json.Marshal(unstructuredObj)
	if err != nil {
		return err
	}
	if _, err := o.Dynamic.Resource(gvr).Namespace(o.Namespace).Patch(
		context.TODO(), objectName, k8sapitypes.MergePatchType,
		objectByte, metav1.PatchOptions{}); err != nil {

		// create object if not found
		if errors.IsNotFound(err) {
			if _, err = o.Dynamic.Resource(gvr).Namespace(o.Namespace).Create(
				context.TODO(), unstructuredObj, metav1.CreateOptions{}); err != nil {
				return err
			}
		} else {
			return err
		}
	}
	return nil
}

func (o *BaseOptions) GetDryRunStrategy() (DryRunStrategy, error) {
	if o.DryRunStrategy == "" {
		return DryRunNone, nil
	}
	switch o.DryRunStrategy {
	case "client":
		return DryRunClient, nil
	case "server":
		return DryRunServer, nil
	case "unchanged":
		return DryRunClient, nil
	case "none":
		return DryRunNone, nil
	default:
		return DryRunNone, fmt.Errorf(`invalid dry-run value (%v). Must be "none", "server", or "client"`, o.DryRunStrategy)
	}
}

// NewCueValue convert cue template  to cue Value which holds any value like Boolean,Struct,String and more cue type.
func newCueValue(cueTemplateName string) (cue.Value, error) {
	tmplFs, _ := debme.FS(cueTemplate, "template")
	if tmlBytes, err := tmplFs.ReadFile(cueTemplateName); err != nil {
		return cue.Value{}, err
	} else {
		return cuecontext.New().CompileString(string(tmlBytes)), nil
	}
}

// fillOptions fill options object in cue template file
func fillOptions(cueValue cue.Value, optionsByte []byte) (cue.Value, error) {
	var (
		expr ast.Expr
		err  error
	)
	if expr, err = cuejson.Extract("", optionsByte); err != nil {
		return cue.Value{}, err
	}
	optionsValue := cueValue.Context().BuildExpr(expr)
	cueValue = cueValue.FillPath(cue.ParsePath("options"), optionsValue)
	return cueValue, nil
}

// convertContentToUnstructured get content object in cue template file and convert it to Unstructured
func convertContentToUnstructured(cueValue cue.Value) (*unstructured.Unstructured, error) {
	var (
		contentByte     []byte
		err             error
		unstructuredObj = &unstructured.Unstructured{}
	)
	if contentByte, err = cueValue.LookupPath(cue.ParsePath("content")).MarshalJSON(); err != nil {
		return nil, err
	}
	if err = json.Unmarshal(contentByte, &unstructuredObj); err != nil {
		return nil, err
	}
	return unstructuredObj, nil
}
