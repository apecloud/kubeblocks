/*
Copyright (C) 2022-2026 ApeCloud Co., Ltd

This file is part of KubeBlocks project

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program.  If not, see <http://www.gnu.org/licenses/>.
*/

package util

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestTruncateEventMessage(t *testing.T) {
	tests := []struct {
		name    string
		message string
	}{
		{name: "short", message: "action failed"},
		{name: "exact limit", message: strings.Repeat("a", maxEventMessageBytes)},
		{name: "long ASCII", message: strings.Repeat("a", maxEventMessageBytes+1)},
		{name: "long UTF-8", message: strings.Repeat("失", maxEventMessageBytes)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := truncateEventMessage(tt.message)
			if len(got) > maxEventMessageBytes {
				t.Fatalf("message length = %d, want <= %d", len(got), maxEventMessageBytes)
			}
			if !utf8.ValidString(got) {
				t.Fatal("message is not valid UTF-8")
			}
			if len(tt.message) <= maxEventMessageBytes && got != tt.message {
				t.Fatalf("message within limit changed: got %q", got)
			}
			if len(tt.message) > maxEventMessageBytes && !strings.HasSuffix(got, eventMessageTruncatedMarker) {
				t.Fatalf("truncated message does not end with %q", eventMessageTruncatedMarker)
			}
		})
	}
}
