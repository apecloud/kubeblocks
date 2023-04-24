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
)

var (
	//go:embed template/*
	cueTemplate embed.FS
)

// CreateOptions the options of creation command should inherit baseOptions
type CreateOptions struct {
	Factory   cmdutil.Factory
	Namespace string

	// Name Resource name of the command line operation
	Name      string
	Args      []string
	Cmd       *cobra.Command
	Dynamic   dynamic.Interface
	Client    kubernetes.Interface
	Format    printer.Format
	ToPrinter func(*meta.RESTMapping, bool) (printers.ResourcePrinterFunc, error)

	// CueTemplateName cue template file name to render the resource
	CueTemplateName string

	// Options a command options object which extends CreateOptions that will be used
	// to render the cue template
	Options interface{}

	// GVR is the GroupVersionResource of the resource to be created
	GVR schema.GroupVersionResource

	// CustomOutPut will be executed after creating successfully.
	CustomOutPut func(options *CreateOptions)

	// PreCreate optional, make changes on yaml before create
	PreCreate func(*unstructured.Unstructured) error

	// Quiet minimize unnecessary output
	Quiet bool

	genericclioptions.IOStreams
}

func (o *CreateOptions) Complete() error {
	var err error
	if o.Namespace, _, err = o.Factory.ToRawKubeConfigLoader().Namespace(); err != nil {
		return err
	}

	// now we use the first argument as the resource name
	if len(o.Args) > 0 {
		o.Name = o.Args[0]
	}

	if o.Dynamic, err = o.Factory.DynamicClient(); err != nil {
		return err
	}

	if o.Client, err = o.Factory.KubernetesClientSet(); err != nil {
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

	return nil
}

// Run execute command. the options of parameter contain the command flags and args.
func (o *CreateOptions) Run() error {
	var (
		cueValue        cue.Value
		err             error
		unstructuredObj *unstructured.Unstructured
		optionsByte     []byte
	)

	if optionsByte, err = json.Marshal(o.Options); err != nil {
		return err
	}

	// append namespace and name to options and marshal to json
	m := make(map[string]interface{})
	if err = json.Unmarshal(optionsByte, &m); err != nil {
		return err
	}
	m["namespace"] = o.Namespace
	m["name"] = o.Name
	if optionsByte, err = json.Marshal(m); err != nil {
		return err
	}

	if cueValue, err = newCueValue(o.CueTemplateName); err != nil {
		return err
	}

	if cueValue, err = fillOptions(cueValue, optionsByte); err != nil {
		return err
	}

	if unstructuredObj, err = convertContentToUnstructured(cueValue); err != nil {
		return err
	}

	if o.PreCreate != nil {
		if err = o.PreCreate(unstructuredObj); err != nil {
			return err
		}
	}

	previewObj := unstructuredObj
	dryRunStrategy, err := GetDryRunStrategy(o.Cmd)
	if err != nil {
		return err
	}

	if dryRunStrategy != DryRunClient {
		createOptions := metav1.CreateOptions{}

		if dryRunStrategy == DryRunServer {
			createOptions.DryRun = []string{metav1.DryRunAll}
		}
		// create k8s resource
		previewObj, err = o.Dynamic.Resource(o.GVR).Namespace(o.Namespace).Create(context.TODO(), previewObj, createOptions)
		if err != nil {
			return err
		}
		if dryRunStrategy != DryRunServer {
			o.Name = previewObj.GetName()
			if o.Quiet {
				return nil
			}
			if o.CustomOutPut != nil {
				o.CustomOutPut(o)
			} else {
				fmt.Fprintf(o.Out, "%s %s created\n", unstructuredObj.GetKind(), unstructuredObj.GetName())
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

// RunAsApply execute command. the options of parameter contain the command flags and args.
// if the resource exists, run as "kubectl apply".
func (o *CreateOptions) RunAsApply() error {
	var (
		cueValue        cue.Value
		err             error
		unstructuredObj *unstructured.Unstructured
		optionsByte     []byte
	)

	if optionsByte, err = json.Marshal(o.Options); err != nil {
		return err
	}

	if cueValue, err = newCueValue(o.CueTemplateName); err != nil {
		return err
	}

	if cueValue, err = fillOptions(cueValue, optionsByte); err != nil {
		return err
	}

	if unstructuredObj, err = convertContentToUnstructured(cueValue); err != nil {
		return err
	}

	// create k8s resource
	objectName, _, err := unstructured.NestedString(unstructuredObj.Object, "metadata", "name")
	if err != nil {
		return err
	}
	objectByte, err := json.Marshal(unstructuredObj)
	if err != nil {
		return err
	}
	if _, err := o.Dynamic.Resource(o.GVR).Namespace(o.Namespace).Patch(
		context.TODO(), objectName, k8sapitypes.MergePatchType,
		objectByte, metav1.PatchOptions{}); err != nil {

		// create object if not found
		if errors.IsNotFound(err) {
			if _, err = o.Dynamic.Resource(o.GVR).Namespace(o.Namespace).Create(
				context.TODO(), unstructuredObj, metav1.CreateOptions{}); err != nil {
				return err
			}
		} else {
			return err
		}
	}
	return nil
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

type DryRunStrategy int

const (
	// DryRunNone indicates the client will make all mutating calls
	DryRunNone DryRunStrategy = iota
	DryRunClient
	DryRunServer
)

func GetDryRunStrategy(cmd *cobra.Command) (DryRunStrategy, error) {
	if cmd == nil {
		return DryRunNone, nil
	}
	dryRunFlag, err := cmd.Flags().GetString("dry-run")
	if err != nil {
		return DryRunNone, nil
	}
	switch dryRunFlag {
	case cmd.Flag("dry-run").NoOptDefVal:
		return DryRunClient, nil
	case "client":
		return DryRunClient, nil
	case "server":
		return DryRunServer, nil
	case "none":
		return DryRunNone, nil
	default:
		return DryRunNone, fmt.Errorf(`invalid dry-run value (%v). Must be "none", "server", or "client"`, dryRunFlag)
	}
}
