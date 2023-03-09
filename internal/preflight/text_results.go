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

package preflight

import (
	"encoding/json"
	"fmt"

	"github.com/fatih/color"
	"github.com/pkg/errors"
	analyzerunner "github.com/replicatedhq/troubleshoot/pkg/analyze"
	"gopkg.in/yaml.v2"
)

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
	b, err := json.MarshalIndent(showStdoutResultsStructured(preflightName, analyzeResults, verbose), "", "  ")
	if err != nil {
		return errors.Wrap(err, "failed to marshal results as json")
	}
	fmt.Printf("%s\n", b)
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
		passInfo.Println("congratulations, your kubernetes cluster preflight check pass, and begin to enjoy KubeBlocks...")
	}
	if len(data.Pass) > 0 {
		passInfo.Println("pass items")
		if b, err := yaml.Marshal(data.Pass); err != nil {
			return errors.Wrap(err, "failed to marshal results as yaml")
		} else {
			fmt.Printf("%s\n", b)
		}
	}
	if len(data.Warn) > 0 {
		warnInfo.Println("warn items")
		if b, err := yaml.Marshal(data.Warn); err != nil {
			return errors.Wrap(err, "failed to marshal results as yaml")
		} else {
			fmt.Printf("%s\n", b)
		}
	}
	if len(data.Fail) > 0 {
		failInfo.Println("fail items")
		if b, err := yaml.Marshal(data.Fail); err != nil {
			return errors.Wrap(err, "failed to marshal results as yaml")
		} else {
			fmt.Printf("%s\n", b)
		}
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
