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

package preflight

import (
	"encoding/json"
	"fmt"

	"github.com/fatih/color"
	"github.com/pkg/errors"
	analyzerunner "github.com/replicatedhq/troubleshoot/pkg/analyze"
	"gopkg.in/yaml.v2"
)

const FailMessage = "Fail items were found. Please resolve the fail items and try again."

type TextResultOutput struct {
	Title   string `json:"title" yaml:"title"`
	Message string `json:"message" yaml:"message"`
	URI     string `json:"uri,omitempty" yaml:"uri,omitempty"`
	Strict  bool   `json:"strict,omitempty" yaml:"strict,omitempty"`
}

func NewTextResultOutput(title, message, uri string) TextResultOutput {
	return TextResultOutput{
		Title:   title,
		Message: message,
		URI:     uri,
	}
}

type TextOutput struct {
	Pass []TextResultOutput `json:"pass,omitempty" yaml:"pass,omitempty"`
	Warn []TextResultOutput `json:"warn,omitempty" yaml:"warn,omitempty"`
	Fail []TextResultOutput `json:"fail,omitempty" yaml:"fail,omitempty"`
}

func NewTextOutput() TextOutput {
	return TextOutput{
		Pass: []TextResultOutput{},
		Warn: []TextResultOutput{},
		Fail: []TextResultOutput{},
	}
}

// ShowTextResults shadows interactive mode, and exports results by customized format
func ShowTextResults(preflightName string, analyzeResults []*analyzerunner.AnalyzeResult, format string, verbose bool) error {
	switch format {
	case "json":
		return showTextResultsJSON(preflightName, analyzeResults, verbose)
	case "yaml":
		return showStdoutResultsYAML(preflightName, analyzeResults, verbose)
	default:
		return errors.Errorf("unknown output format: %q", format)
	}
}

func showTextResultsJSON(preflightName string, analyzeResults []*analyzerunner.AnalyzeResult, verbose bool) error {
	output := showStdoutResultsStructured(preflightName, analyzeResults, verbose)
	b, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		return errors.Wrap(err, "failed to marshal results as json")
	}
	fmt.Printf("%s\n", b)
	if len(output.Fail) > 0 {
		return errors.New(FailMessage)
	}
	return nil
}

func showStdoutResultsYAML(preflightName string, analyzeResults []*analyzerunner.AnalyzeResult, verbose bool) error {
	data := showStdoutResultsStructured(preflightName, analyzeResults, verbose)
	var (
		passInfo = color.New(color.FgGreen)
		warnInfo = color.New(color.FgYellow)
		failInfo = color.New(color.FgRed)
	)
	if len(data.Warn) == 0 && len(data.Fail) == 0 {
		fmt.Println("The kubernetes cluster preflight check pass, and you can enjoy KubeBlocks now.")
	}
	if len(data.Pass) > 0 {
		passInfo.Println("Pass items")
		if b, err := yaml.Marshal(data.Pass); err != nil {
			return errors.Wrap(err, "failed to marshal results as yaml")
		} else {
			fmt.Printf("%s\n", b)
		}
	}
	if len(data.Warn) > 0 {
		warnInfo.Println("Warn items")
		if b, err := yaml.Marshal(data.Warn); err != nil {
			return errors.Wrap(err, "failed to marshal results as yaml")
		} else {
			fmt.Printf("%s\n", b)
		}
	}
	if len(data.Fail) > 0 {
		failInfo.Println("Fail items")
		if b, err := yaml.Marshal(data.Fail); err != nil {
			return errors.Wrap(err, "failed to marshal results as yaml")
		} else {
			fmt.Printf("%s\n", b)
		}
		return errors.New(FailMessage)
	}
	return nil
}

// showStdoutResultsStructured is Used by both JSON and YAML outputs
func showStdoutResultsStructured(preflightName string, analyzeResults []*analyzerunner.AnalyzeResult, verbose bool) TextOutput {
	output := NewTextOutput()
	for _, analyzeResult := range analyzeResults {
		resultOutput := NewTextResultOutput(analyzeResult.Title, analyzeResult.Message, analyzeResult.URI)
		if analyzeResult.Strict {
			resultOutput.Strict = analyzeResult.Strict
		}
		switch {
		case analyzeResult.IsPass:
			if verbose {
				output.Pass = append(output.Pass, resultOutput)
			}
		case analyzeResult.IsWarn:
			output.Warn = append(output.Warn, resultOutput)
		case analyzeResult.IsFail:
			output.Fail = append(output.Fail, resultOutput)
		}
	}
	return output
}
