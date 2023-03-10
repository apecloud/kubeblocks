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
