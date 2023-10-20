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

package common

import (
	"bytes"
)

// forked from k8s.io/kubernetes/third_party/forked/golang/expansion
const (
	operator        = '$'
	referenceOpener = '('
	referenceCloser = ')'
)

// syntaxWrap returns the input string wrapped by the expansion syntax.
func syntaxWrap(input string) string {
	return string(operator) + string(referenceOpener) + input + string(referenceCloser)
}

// MappingFuncFor returns a mapping function for use with Expand that
// implements the expansion semantics defined in the expansion spec; it
// returns the input string wrapped in the expansion syntax if no mapping
// for the input is found.
func MappingFuncFor(context ...map[string]string) func(string) string {
	return func(input string) string {
		for _, vars := range context {
			val, ok := vars[input]
			if ok {
				return val
			}
		}

		return syntaxWrap(input)
	}
}

// Expand replaces variable references in the input string according to
// the expansion spec using the given mapping function to resolve the
// values of variables.
func Expand(input string, mapping func(string) string) string {
	var buf bytes.Buffer
	checkpoint := 0
	for cursor := 0; cursor < len(input); cursor++ {
		if input[cursor] == operator && cursor+1 < len(input) {
			// Copy the portion of the input string since the last
			// checkpoint into the buffer
			buf.WriteString(input[checkpoint:cursor])

			// Attempt to read the variable name as defined by the
			// syntax from the input string
			read, isVar, advance := tryReadVariableName(input[cursor+1:])

			if isVar {
				// We were able to read a variable name correctly;
				// apply the mapping to the variable name and copy the
				// bytes into the buffer
				buf.WriteString(mapping(read))
			} else {
				// Not a variable name; copy the read bytes into the buffer
				buf.WriteString(read)
			}

			// Advance the cursor in the input string to account for
			// bytes consumed to read the variable name expression
			cursor += advance

			// Advance the checkpoint in the input string
			checkpoint = cursor + 1
		}
	}

	// Return the buffer and any remaining unwritten bytes in the
	// input string.
	return buf.String() + input[checkpoint:]
}

// tryReadVariableName attempts to read a variable name from the input
// string and returns the content read from the input, whether that content
// represents a variable name to perform mapping on, and the number of bytes
// consumed in the input string.
//
// The input string is assumed not to contain the initial operator.
func tryReadVariableName(input string) (string, bool, int) {
	switch input[0] {
	case operator:
		// Escaped operator; return it.
		return input[0:1], false, 1
	case referenceOpener:
		// Scan to expression closer
		for i := 1; i < len(input); i++ {
			if input[i] == referenceCloser {
				return input[1:i], true, i + 1
			}
		}

		// Incomplete reference; return it.
		return string(operator) + string(referenceOpener), false, 1
	default:
		// Not the beginning of an expression, ie, an operator
		// that doesn't begin an expression.  Return the operator
		// and the first rune in the string.
		return (string(operator) + string(input[0])), false, 1
	}
}
