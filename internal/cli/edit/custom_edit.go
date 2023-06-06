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

package edit

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/resource"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/cmd/util/editor"

	"github.com/apecloud/kubeblocks/internal/cli/printer"
	"github.com/apecloud/kubeblocks/internal/cli/util"
	"github.com/apecloud/kubeblocks/internal/cli/util/prompt"
)

// CustomEditOptions is used to edit the resource manifest when creating or updating the resource,
// instead of using -o yaml to output the yaml file before editing the manifest.
type CustomEditOptions struct {
	Factory    cmdutil.Factory
	PrintFlags *genericclioptions.PrintFlags
	Method     string

	genericclioptions.IOStreams
}

func NewCustomEditOptions(f cmdutil.Factory, streams genericclioptions.IOStreams, method string) *CustomEditOptions {
	return &CustomEditOptions{
		Factory:    f,
		PrintFlags: genericclioptions.NewPrintFlags("").WithDefaultOutput("yaml"),
		IOStreams:  streams,
		Method:     method,
	}
}

func (o *CustomEditOptions) Run(originalObj runtime.Object, testEnv bool) error {
	buf := &bytes.Buffer{}
	var (
		original []byte
		edited   []byte
		tmpFile  string
		w        io.Writer = buf
	)
	editPrinter, err := o.PrintFlags.ToPrinter()
	if err != nil {
		return fmt.Errorf("failed to create printer: %v", err)
	}
	if err := editPrinter.PrintObj(originalObj, w); err != nil {
		return fmt.Errorf("failed to print object: %v", err)
	}
	original = buf.Bytes()

	if !testEnv {
		edited, tmpFile, err = editObject(original)
		if err != nil {
			return fmt.Errorf("failed to lanch editor: %v", err)
		}
	} else {
		edited = original
	}

	dynamicClient, err := o.Factory.DynamicClient()
	if err != nil {
		return fmt.Errorf("failed to get dynamic client: %v", err)
	}
	// apply validation
	fieldValidationVerifier := resource.NewQueryParamVerifier(dynamicClient, o.Factory.OpenAPIGetter(), resource.QueryParamFieldValidation)
	schemaValidator, err := o.Factory.Validator(metav1.FieldValidationStrict, fieldValidationVerifier)
	if err != nil {
		return fmt.Errorf("failed to get validator: %v", err)
	}
	err = schemaValidator.ValidateBytes(cmdutil.StripComments(edited))
	if err != nil {
		return fmt.Errorf("the edited file failed validation: %v", err)
	}

	// Compare content without comments
	if bytes.Equal(cmdutil.StripComments(original), cmdutil.StripComments(edited)) {
		os.Remove(tmpFile)
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
		os.Remove(tmpFile)
		_, err = fmt.Fprintln(o.ErrOut, "Edit cancelled, saved file was empty.")
		if err != nil {
			return fmt.Errorf("error writing to stderr: %v", err)
		}
	}

	decoder := yaml.NewYAMLOrJSONDecoder(bytes.NewReader(edited), len(edited))
	if err := decoder.Decode(originalObj); err != nil {
		return fmt.Errorf("failed to decode edited object: %v", err)
	}

	if o.Method == "patched" {
		diff, err := util.GetUnifiedDiffString(string(original), string(edited))
		if err != nil {
			return fmt.Errorf("failed to get diff: %v", err)
		}
		util.DisplayDiffWithColor(o.IOStreams.Out, diff)
	} else if o.Method == "create" {
		err := editPrinter.PrintObj(originalObj, o.IOStreams.Out)
		if err != nil {
			return fmt.Errorf("failed to print object: %v", err)
		}
	}
	return confirmToContinue(o.IOStreams)
}

func editObject(original []byte) ([]byte, string, error) {
	err := addHeader(bytes.NewBuffer(original))
	if err != nil {
		return nil, "", err
	}

	edit := editor.NewDefaultEditor([]string{
		"KUBE_EDITOR",
		"EDITOR",
	})
	// launch the editor
	edited, tmpFile, err := edit.LaunchTempFile(fmt.Sprintf("%s-edit-", filepath.Base(os.Args[0])), ".yaml", bytes.NewBuffer(original))
	if err != nil {
		return nil, "", err
	}

	return edited, tmpFile, nil
}

// HasComment returns true if any line in the provided stream is non empty - has non-whitespace
// characters, or the first non-whitespace character is a '#' indicating a comment. Returns
// any errors encountered reading the stream.
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

// AddHeader adds a header to the provided writer
func addHeader(w io.Writer) error {
	_, err := fmt.Fprint(w, `# Please edit the object below. Lines beginning with a '#' will be ignored,
# and an empty file will abort the edit. If an error occurs while saving this file will be
# reopened with the relevant failures.
#
`)
	return err
}

func confirmToContinue(stream genericclioptions.IOStreams) error {
	printer.Warning(stream.Out, "Above resource will be created or changed, do you want to continue to create or change this resource?\n  Only 'yes' will be accepted to confirm.\n\n")
	entered, _ := prompt.NewPrompt("Enter a value:", nil, stream.In).Run()
	if entered != "yes" {
		_, err := fmt.Fprintf(stream.Out, "\nCancel resource creation.\n")
		if err != nil {
			return err
		}
		return cmdutil.ErrExit
	}
	return nil
}
