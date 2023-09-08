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

package prompt

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/manifoldco/promptui"
	"golang.org/x/exp/slices"
)

func NewPrompt(label string, validate promptui.ValidateFunc, in io.Reader) *promptui.Prompt {
	template := &promptui.PromptTemplates{
		Prompt:  "{{ . }} ",
		Valid:   "{{ . | green }} ",
		Invalid: "{{ . | red }} ",
		Success: "{{ . | bold }} ",
	}

	if validate == nil {
		template = &promptui.PromptTemplates{
			Prompt:  "{{ . }} ",
			Valid:   "{{ . | red }} ",
			Invalid: "{{ . | red }} ",
			Success: "{{ . | bold }} ",
		}
	}
	p := promptui.Prompt{
		Label:     label,
		Stdin:     io.NopCloser(in),
		Templates: template,
		Validate:  validate,
	}
	return &p
}

// Confirm let user double-check for the cluster ops
// use customMessage to display more information
// when names are empty, require validation for 'yes'.
func Confirm(names []string, in io.Reader, customMessage string, prompt string) error {
	if len(names) == 0 {
		names = []string{"yes"}
	}
	if len(customMessage) != 0 {
		fmt.Println(customMessage)
	}
	if prompt == "" {
		prompt = "Please type the name again(separate with white space when more than one):"
	}
	_, err := NewPrompt(prompt,
		func(entered string) error {
			if len(names) == 1 && names[0] == "yes" {
				entered = strings.ToLower(entered)
			}
			enteredNames := strings.Split(entered, " ")
			sort.Strings(names)
			sort.Strings(enteredNames)
			if !slices.Equal(names, enteredNames) {
				return fmt.Errorf("typed \"%s\" does not match \"%s\"", entered, strings.Join(names, " "))
			}
			return nil
		}, in).Run()
	return err
}
