/*
Copyright (C) 2022-2024 ApeCloud Co., Ltd

# This file is part of KubeBlocks project

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
)

var (
	ErrNotDefined     = errors.New("notDefined")
	ErrNotImplemented = errors.New("notImplemented")
	ErrBadRequest     = errors.New("badRequest")
	ErrInProgress     = errors.New("inProgress")
	ErrBusy           = errors.New("busy")
	ErrTimedOut       = errors.New("timedOut")
	ErrFailed         = errors.New("failed")
	ErrInternalError  = errors.New("internalError")
	ErrUnknown        = errors.New("unknown")
)

func Error2Type(err error) string {
	switch {
	case err == nil:
		return ""
	case errors.Is(err, ErrNotDefined):
		return "notDefined"
	case errors.Is(err, ErrNotImplemented):
		return "notImplemented"
	case errors.Is(err, ErrBadRequest):
		return "badRequest"
	case errors.Is(err, ErrInProgress):
		return "inProgress"
	case errors.Is(err, ErrBusy):
		return "busy"
	case errors.Is(err, ErrTimedOut):
		return "timedOut"
	case errors.Is(err, ErrFailed):
		return "failed"
	case errors.Is(err, ErrInternalError):
		return "internalError"
	default:
		return "unknown"
	}
}

func Type2Error(errType string) error {
	switch errType {
	case "":
		return nil
	case "notDefined":
		return ErrNotDefined
	case "notImplemented":
		return ErrNotImplemented
	case "badRequest":
		return ErrBadRequest
	case "inProgress":
		return ErrInProgress
	case "busy":
		return ErrBusy
	case "timedOut":
		return ErrTimedOut
	case "failed":
		return ErrFailed
	case "internalError":
		return ErrInternalError
	default:
		return ErrUnknown
	}
}
