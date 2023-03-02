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

	"github.com/pkg/errors"
	analyzerunner "github.com/replicatedhq/troubleshoot/pkg/analyze"
	"gopkg.in/yaml.v2"
)

// ShowStdoutResults shadows interactive mode, and exports results by customized format
func ShowStdoutResults(preflightName string, analyzeResults []*analyzerunner.AnalyzeResult, format string) error {
	switch format {
	case "human":
		return showStdoutResultsHuman(preflightName, analyzeResults)
	case "json":
		return showStdoutResultsJSON(preflightName, analyzeResults)
	case "yaml":
		return showStdoutResultsYAML(preflightName, analyzeResults)
	default:
		return errors.Errorf("unknown output format: %q", format)
	}
}

func showStdoutResultsHuman(preflightName string, analyzeResults []*analyzerunner.AnalyzeResult) error {
	fmt.Println("")
	var failed bool
	for _, analyzeResult := range analyzeResults {
		testResultFailed := outputResult(analyzeResult)
		if testResultFailed {
			failed = true
		}
	}
	if failed {
		fmt.Printf("--- FAIL   %s\n", preflightName)
		fmt.Println("FAILED")
	} else {
		fmt.Printf("--- PASS   %s\n", preflightName)
		fmt.Println("PASS")
	}
	return nil
}

func showStdoutResultsJSON(preflightName string, analyzeResults []*analyzerunner.AnalyzeResult) error {
	output := showStdoutResultsStructured(preflightName, analyzeResults)

	b, err := json.MarshalIndent(*output, "", "  ")
	if err != nil {
		return errors.Wrap(err, "failed to marshal results as json")
	}

	fmt.Printf("%s\n", b)

	return nil
}

func showStdoutResultsYAML(preflightName string, analyzeResults []*analyzerunner.AnalyzeResult) error {
	output := showStdoutResultsStructured(preflightName, analyzeResults)

	b, err := yaml.Marshal(*output)
	if err != nil {
		return errors.Wrap(err, "failed to marshal results as yaml")
	}

	fmt.Printf("%s\n", b)

	return nil
}

type stdoutResultOutput struct {
	Title   string `json:"title" yaml:"title"`
	Message string `json:"message" yaml:"message"`
	URI     string `json:"uri,omitempty" yaml:"uri,omitempty"`
	Strict  bool   `json:"strict,omitempty" yaml:"strict,omitempty"`
}

type stdoutOutput struct {
	Pass []stdoutResultOutput `json:"pass,omitempty" yaml:"pass,omitempty"`
	Warn []stdoutResultOutput `json:"warn,omitempty" yaml:"warn,omitempty"`
	Fail []stdoutResultOutput `json:"fail,omitempty" yaml:"fail,omitempty"`
}

func showStdoutResultsStructured(preflightName string, analyzeResults []*analyzerunner.AnalyzeResult) *stdoutOutput {
	output := stdoutOutput{
		Pass: []stdoutResultOutput{},
		Warn: []stdoutResultOutput{},
		Fail: []stdoutResultOutput{},
	}

	for _, analyzeResult := range analyzeResults {
		resultOutput := stdoutResultOutput{
			Title:   analyzeResult.Title,
			Message: analyzeResult.Message,
			URI:     analyzeResult.URI,
		}

		if analyzeResult.Strict {
			resultOutput.Strict = analyzeResult.Strict
		}
		switch {
		case analyzeResult.IsPass:
			output.Pass = append(output.Pass, resultOutput)
		case analyzeResult.IsWarn:
			output.Warn = append(output.Warn, resultOutput)
		case analyzeResult.IsFail:
			output.Fail = append(output.Fail, resultOutput)
		}
	}
	return &output
}

func outputResult(analyzeResult *analyzerunner.AnalyzeResult) bool {
	switch {
	case analyzeResult.IsPass:
		fmt.Printf("   --- PASS %s\n", analyzeResult.Title)
		fmt.Printf("      --- %s\n", analyzeResult.Message)
	case analyzeResult.IsWarn:
		fmt.Printf("   --- WARN: %s\n", analyzeResult.Title)
		fmt.Printf("      --- %s\n", analyzeResult.Message)
	case analyzeResult.IsFail:
		fmt.Printf("   --- FAIL: %s\n", analyzeResult.Title)
		fmt.Printf("      --- %s\n", analyzeResult.Message)
	}

	if analyzeResult.Strict {
		fmt.Printf("      --- Strict: %t\n", analyzeResult.Strict)
	}

	if analyzeResult.IsFail {
		return true
	}
	return false
}
