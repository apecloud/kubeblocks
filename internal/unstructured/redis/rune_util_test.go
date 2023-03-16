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
	"testing"
)

func TestContainerEscapeString(t *testing.T) {
	tests := []struct {
		args string
		want bool
	}{{
		args: "",
		want: false,
	}, {
		args: "abcd\"",
		want: true,
	}, {
		args: "ab cd",
		want: true,
	}, {
		args: "abcd",
		want: false,
	}, {
		args: "\xff",
		want: false,
	}, {
		args: "\075",
		want: false,
	}}
	for _, tt := range tests {
		t.Run("escapeStringTest", func(t *testing.T) {
			if got := ContainerEscapeString(tt.args); got != tt.want {
				t.Errorf("ContainerEscapeString() = %v, want %v", got, tt.want)
			}
		})
	}
}
