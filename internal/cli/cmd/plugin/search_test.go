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

package plugin

import (
	"testing"
)

func TestSearchByNameAndDesc(t *testing.T) {
	testPlugins := []struct {
		keyword     string
		name        string
		description string
		expected    bool
	}{
		{
			keyword:     "foo",
			name:        "foo", // name matched
			description: "this description don't have keyword",
			expected:    true,
		},
		{
			keyword:     "foo",
			name:        "foobar", // name matched
			description: "this description don't have keyword",
			expected:    true,
		},
		{
			keyword:     "foo",
			name:        "test",
			description: "this description have keyword foo", // description matched but score < 0
			expected:    false,
		},
		{
			keyword:     "foo",
			name:        "test",
			description: "this description don't have keyword",
			expected:    false,
		},
		{
			keyword:     "foo",
			name:        "test",
			description: "this description have foo ", // description matched and score > 0
			expected:    true,
		},
	}

	for _, tp := range testPlugins {
		t.Run(tp.keyword, func(t *testing.T) {
			result := fuzzySearchByNameAndDesc(tp.keyword, tp.name, tp.description)
			if result != tp.expected {
				t.Fatalf("expected %v, got %v", tp.expected, result)
			}
		})
	}
}
