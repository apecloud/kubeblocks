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

package redis

import (
	"strings"
)

const (
	eof          = -1
	escape       = '\\'
	quotes       = '"'
	singleQuotes = '\''

	escapeChars = "\a\b\f\n\r\t\v \""
)

// isSplitCharacter reports whether the rune is a split character
func isSplitCharacter(r rune) bool {
	return strings.ContainsRune(trimChars, r)
}

// isEscape reports whether the rune is the escape character which
// prefixes unicode literals and other escaped characters.
func isEscape(r rune) bool {
	return r == escape
}

func isEOF(r rune) bool {
	return r == eof
}

func isQuotes(r rune) bool {
	return r == quotes
}

func isSingleQuotes(r rune) bool {
	return r == singleQuotes
}

func ContainerEscapeString(v string) bool {
	for _, c := range v {
		if strings.ContainsRune(escapeChars, c) {
			return true
		}
	}
	return false
}
