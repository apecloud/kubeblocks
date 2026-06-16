/*
Copyright (C) 2022-2026 ApeCloud Co., Ltd

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

package proto

import (
	"errors"
	"testing"
)

func TestError2Type(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want string
	}{
		{name: "nil", want: ""},
		{name: "not defined", err: ErrNotDefined, want: "notDefined"},
		{name: "not implemented", err: ErrNotImplemented, want: "notImplemented"},
		{name: "precondition failed", err: ErrPreconditionFailed, want: "preconditionFailed"},
		{name: "bad request", err: ErrBadRequest, want: "badRequest"},
		{name: "in progress", err: ErrInProgress, want: "inProgress"},
		{name: "busy", err: ErrBusy, want: "busy"},
		{name: "timed out", err: ErrTimedOut, want: "timedOut"},
		{name: "failed", err: ErrFailed, want: "failed"},
		{name: "internal error", err: ErrInternalError, want: "internalError"},
		{name: "wrapped", err: errors.Join(ErrBusy), want: "busy"},
		{name: "unknown", err: errors.New("other"), want: "unknown"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Error2Type(tt.err); got != tt.want {
				t.Fatalf("Error2Type() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestType2Error(t *testing.T) {
	tests := []struct {
		errType string
		want    error
	}{
		{errType: "", want: nil},
		{errType: "notDefined", want: ErrNotDefined},
		{errType: "notImplemented", want: ErrNotImplemented},
		{errType: "preconditionFailed", want: ErrPreconditionFailed},
		{errType: "badRequest", want: ErrBadRequest},
		{errType: "inProgress", want: ErrInProgress},
		{errType: "busy", want: ErrBusy},
		{errType: "timedOut", want: ErrTimedOut},
		{errType: "failed", want: ErrFailed},
		{errType: "internalError", want: ErrInternalError},
		{errType: "unknown-type", want: ErrUnknown},
	}
	for _, tt := range tests {
		t.Run(tt.errType, func(t *testing.T) {
			if got := Type2Error(tt.errType); !errors.Is(got, tt.want) {
				t.Fatalf("Type2Error() = %v, want %v", got, tt.want)
			}
		})
	}
}
