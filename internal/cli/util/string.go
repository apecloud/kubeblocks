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

package util

import (
	"regexp"
	"strings"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

// ToKebabCase converts a string to kebab case.
func ToKebabCase(s string) string {
	pattern := regexp.MustCompile("[A-Z]")
	kebab := pattern.ReplaceAllString(s, "-$0")
	if len(kebab) > 0 && kebab[0] == '-' {
		return strings.ToLower(kebab[1:])
	}
	return strings.ToLower(kebab)
}

// ToLowerCamelCase converts a string to lower camel case.
func ToLowerCamelCase(s string) string {
	parts := strings.Split(s, "-")
	for i := range parts {
		parts[i] = cases.Title(language.English).String(parts[i])
	}
	parts[0] = strings.ToLower(parts[0])
	return strings.Join(parts, "")
}
