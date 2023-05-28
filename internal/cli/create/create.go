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
	"bufio"
	"bytes"
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/ast"
	"cuelang.org/go/cue/cuecontext"
	cuejson "cuelang.org/go/encoding/json"
	"github.com/leaanthony/debme"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/printers"
	"k8s.io/cli-runtime/pkg/resource"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/cmd/util/editor"
	"k8s.io/kubectl/pkg/scheme"

	"github.com/apecloud/kubeblocks/internal/cli/printer"
	"github.com/apecloud/kubeblocks/internal/cli/util/prompt"
)

var (
	//go:embed template/*
	cueTemplate embed.FS
)

type CreateDependency func(dryRun []string) error

type DryRunStrategy int

const (
	// DryRunNone indicates the client will make all mutating calls
	DryRunNone DryRunStrategy = iota
	DryRunClient
	DryRunServer
)

// CreateOptions the options of creation command should inherit baseOptions
type CreateOptions struct {
	Factory   cmdutil.Factory
	Namespace string

	// Name Resource name of the command line operation
	Name             string
	Args             []string
	Dynamic          dynamic.Interface
	Client           kubernetes.Interface
	Format           printer.Format
	ToPrinter        func(*meta.RESTMapping, bool) (printers.ResourcePrinterFunc, error)
	DryRun           string
	EditBeforeCreate bool

	// CueTemplateName cue template file name to render the resource
	CueTemplateName string

	AutoApprove bool

	// Options a command options object which extends CreateOptions that will be used
	// to render the cue template
	Options interface{}

	// GVR is the GroupVersionResource of the resource to be created
	GVR schema.GroupVersionResource

	// CustomOutPut will be executed after creating successfully.
	CustomOutPut func(options *CreateOptions)

	// PreCreate optional, make changes on yaml before create
	PreCreate func(*unstructured.Unstructured) error

	// CleanUpFn will be executed after creating failed.
	CleanUpFn func() error

	// CreateDependencies will be executed before creating.
	CreateDependencies CreateDependency

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
	resObj, err := o.buildResourceObj()
	if err != nil {
		return err
	}

	if o.PreCreate != nil {
		if err = o.PreCreate(resObj); err != nil {
			return err
		}
	}

	if o.EditBeforeCreate {
		if err = o.RunEditOnCreate(resObj); err != nil {
			return err
		}
	}

	dryRunStrategy, err := o.GetDryRunStrategy()
	if err != nil {
		return err
	}

	if dryRunStrategy != DryRunClient {
		createOptions := metav1.CreateOptions{}

		if dryRunStrategy == DryRunServer {
			createOptions.DryRun = []string{metav1.DryRunAll}
		}

		// create dependencies
		if o.CreateDependencies != nil {
			if err = o.CreateDependencies(createOptions.DryRun); err != nil {
				return err
			}
		}

		// create kubernetes resource
		resObj, err = o.Dynamic.Resource(o.GVR).Namespace(o.Namespace).Create(context.TODO(), resObj, createOptions)
		if err != nil {
			if apierrors.IsAlreadyExists(err) {
				return err
			}

			// for other errors, clean up dependencies
			if cleanErr := o.CleanUp(); cleanErr != nil {
				fmt.Fprintf(o.ErrOut, "Failed to clean up denpendencies: %v\n", cleanErr)
			}
			return err
		}

		if dryRunStrategy != DryRunServer {
			o.Name = resObj.GetName()
			if o.Quiet {
				return nil
			}
			if o.CustomOutPut != nil {
				o.CustomOutPut(o)
			} else {
				fmt.Fprintf(o.Out, "%s %s created\n", resObj.GetKind(), resObj.GetName())
			}
			return nil
		}
	}
	printer, err := o.ToPrinter(nil, false)
	if err != nil {
		return err
	}
	return printer.PrintObj(resObj, o.Out)
}

func (o *CreateOptions) RunEditOnCreate(unstructuredObj *unstructured.Unstructured) error {
	edit := editor.NewDefaultEditor([]string{
		"KUBE_EDITOR",
		"EDITOR",
	})

	var (
		original []byte
		edited   []byte
		tmpFile  string
	)

	editPrinter, err := o.ToPrinter(nil, false)
	if err != nil {
		return err
	}

	buf := &bytes.Buffer{}
	var w io.Writer = buf

	// add header
	if err := addHeader(w); err != nil {
		return err
	}

	originalObj := unstructuredObj.DeepCopyObject()
	if err := editPrinter.PrintObj(originalObj, w); err != nil {
		return err
	}
	original = buf.Bytes()

	// launch the editor
	edited, tmpFile, err = edit.LaunchTempFile(fmt.Sprintf("%s-edit-", filepath.Base(os.Args[0])), ".yaml", buf)
	if err != nil {
		return fmt.Errorf("error executing editor: %v", err)
	}

	// apply validation
	fieldValidationVerifier := resource.NewQueryParamVerifier(o.Dynamic, o.Factory.OpenAPIGetter(), resource.QueryParamFieldValidation)
	schema, err := o.Factory.Validator(metav1.FieldValidationStrict, fieldValidationVerifier)
	if err != nil {
		return err
	}
	err = schema.ValidateBytes(cmdutil.StripComments(edited))
	if err != nil {
		return fmt.Errorf("the edited file failed validation: %v", err)
	}

	// Compare content without comments
	if bytes.Equal(cmdutil.StripComments(original), cmdutil.StripComments(edited)) {
		err := os.Remove(tmpFile)
		if err != nil {
			return fmt.Errorf("error removing file: %v", err)
		}
		_, err = fmt.Fprintln(o.ErrOut, "Edit cancelled, no changes made.")
		if err != nil {
			return fmt.Errorf("error writing to stderr: %v", err)
		}
		return nil
	}

	// Returns an error if comments are included.
	lines, err := hasComment(bytes.NewBuffer(edited))
	if err != nil {
		return fmt.Errorf("error checking for comment: %v", err)
	}
	if !lines {
		err := os.Remove(tmpFile)
		if err != nil {
			return err
		}
		_, err = fmt.Fprintln(o.ErrOut, "Edit cancelled, saved file was empty.")
		if err != nil {
			return err
		}
	}

	decoder := yaml.NewYAMLOrJSONDecoder(bytes.NewReader(edited), len(edited))
	if err := decoder.Decode(unstructuredObj); err != nil {
		return err
	}

	if err := editPrinter.PrintObj(unstructuredObj, o.Out); err != nil {
		return err
	}
	return o.confirmToContinue()
}

func (o *CreateOptions) confirmToContinue() error {
	if !o.AutoApprove {
		printer.Warning(o.Out, "Above resource will be created, do you want to continue to create this resource: %s  ?\n  Only 'yes' will be accepted to confirm.\n\n", o.Name)
		entered, _ := prompt.NewPrompt("Enter a value:", nil, o.In).Run()
		if entered != "yes" {
			_, err := fmt.Fprintf(o.Out, "\nCancel resource creation.\n")
			if err != nil {
				return err
			}
			return cmdutil.ErrExit
		}
	}
	_, err := fmt.Fprintf(o.Out, "Continue to create resource: %s\n", o.Name)
	if err != nil {
		return err
	}
	return nil
}

func (o *CreateOptions) CleanUp() error {
	if o.CreateDependencies == nil {
		return nil
	}

	if o.CleanUpFn != nil {
		return o.CleanUpFn()
	}
	return nil
}

func (o *CreateOptions) buildResourceObj() (*unstructured.Unstructured, error) {
	var (
		cueValue    cue.Value
		err         error
		optionsByte []byte
	)

	if optionsByte, err = json.Marshal(o.Options); err != nil {
		return nil, err
	}

	// append namespace and name to options and marshal to json
	m := make(map[string]interface{})
	if err = json.Unmarshal(optionsByte, &m); err != nil {
		return nil, err
	}
	m["namespace"] = o.Namespace
	m["name"] = o.Name
	if optionsByte, err = json.Marshal(m); err != nil {
		return nil, err
	}

	if cueValue, err = newCueValue(o.CueTemplateName); err != nil {
		return nil, err
	}

	if cueValue, err = fillOptions(cueValue, optionsByte); err != nil {
		return nil, err
	}
	return convertContentToUnstructured(cueValue)
}

func (o *CreateOptions) GetDryRunStrategy() (DryRunStrategy, error) {
	if o.DryRun == "" {
		return DryRunNone, nil
	}
	switch o.DryRun {
	case "client":
		return DryRunClient, nil
	case "server":
		return DryRunServer, nil
	case "unchanged":
		return DryRunClient, nil
	case "none":
		return DryRunNone, nil
	default:
		return DryRunNone, fmt.Errorf(`invalid dry-run value (%v). Must be "none", "server", or "client"`, o.DryRun)
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

func hasComment(r io.Reader) (bool, error) {
	s := bufio.NewScanner(r)
	for s.Scan() {
		if line := strings.TrimSpace(s.Text()); len(line) > 0 && line[0] != '#' {
			return true, nil
		}
	}
	if err := s.Err(); err != nil && err != io.EOF {
		return false, err
	}
	return false, nil
}

func addHeader(w io.Writer) error {
	_, err := fmt.Fprint(w, `# Please edit the object below. Lines beginning with a '#' will be ignored,
# and an empty file will abort the edit. If an error occurs while saving this file will be
# reopened with the relevant failures.
#
`)
	return err
}
