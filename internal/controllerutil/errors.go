package controllerutil

import (
	"fmt"
)

type Error struct {
	Type    ErrorType
	Message string
}

var _ error = &Error{}

// Error implements the error interface.
func (v *Error) Error() string {
	return v.Message
}

// ErrorType is explicit error type.
type ErrorType string

const (
	// ErrorTypeBackupNotCompleted is used to report backup not completed.
	ErrorTypeBackupNotCompleted ErrorType = "BackupNotCompleted"
)

func NewError(errorType ErrorType, message string) *Error {
	return &Error{
		Type:    errorType,
		Message: message,
	}
}

func NewErrorf(errorType ErrorType, format string, a ...any) *Error {
	return &Error{
		Type:    errorType,
		Message: fmt.Sprintf(format, a...),
	}
}
