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

package lifecycle

import (
	"errors"
	"fmt"
	"testing"
)

func TestIsActionFailure(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{name: "nil", err: nil, want: false},
		{name: "not defined", err: ErrActionNotDefined, want: false},
		{name: "precondition", err: ErrPreconditionFailed, want: false},
		{name: "in progress", err: ErrActionInProgress, want: false},
		{name: "busy", err: ErrActionBusy, want: false},
		{name: "wrapped waiting", err: fmt.Errorf("wrapped: %w", ErrActionBusy), want: false},
		{name: "failed", err: ErrActionFailed, want: true},
		{name: "timed out", err: ErrActionTimedOut, want: true},
		{name: "internal", err: ErrActionInternalError, want: true},
		{name: "not implemented", err: ErrActionNotImplemented, want: true},
		{name: "generic", err: errors.New("pod is unavailable"), want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsActionFailure(tt.err); got != tt.want {
				t.Fatalf("IsActionFailure(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}
