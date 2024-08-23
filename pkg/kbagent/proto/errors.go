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
