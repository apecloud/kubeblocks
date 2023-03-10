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
