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

package prompt

import (
	"io"

	"github.com/manifoldco/promptui"
	"github.com/pkg/errors"
)

type prompt struct {
	errorMsg string
	label    string
	in       io.ReadCloser
}

func NewPrompt(errMsg string, label string, in io.Reader) *prompt {
	return &prompt{
		errorMsg: errMsg,
		label:    label,
		in:       io.NopCloser(in),
	}
}

func (p *prompt) GetInput() (string, error) {
	validate := func(input string) error {
		if len(input) == 0 {
			return errors.New(p.errorMsg)
		}
		return nil
	}

	templates := &promptui.PromptTemplates{
		Prompt:  "{{ . }} ",
		Valid:   "{{ . | green }} ",
		Invalid: "{{ . | red }} ",
		Success: "{{ . | bold }} ",
	}

	prompt := promptui.Prompt{
		Label:     p.label,
		Templates: templates,
		Validate:  validate,
		Stdin:     p.in,
	}

	return prompt.Run()
}
