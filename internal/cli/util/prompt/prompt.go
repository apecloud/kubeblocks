/*
Copyright (C) 2022 ApeCloud Co., Ltd

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
	"io"

	"github.com/manifoldco/promptui"
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
