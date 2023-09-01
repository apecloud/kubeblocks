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

package controllerutil

import (
	"errors"
	"fmt"
	"strings"
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
	// ErrorWaitCacheRefresh waits for synchronization of the corresponding object cache in client-go from ApiServer.
	ErrorWaitCacheRefresh ErrorType = "WaitCacheRefresh"
	// ErrorTypeNotFound not found any resource.
	ErrorTypeNotFound ErrorType = "NotFound"

	ErrorTypeRequeue ErrorType = "Requeue" // requeue for reconcile.

	// ErrorType for backup
	ErrorTypeBackupNotSupported       ErrorType = "BackupNotSupported"       // this backup type not supported
	ErrorTypeBackupPVTemplateNotFound ErrorType = "BackupPVTemplateNotFound" // this pv template not found
	ErrorTypeBackupNotCompleted       ErrorType = "BackupNotCompleted"       // report backup not completed.
	ErrorTypeBackupPVCNameIsEmpty     ErrorType = "BackupPVCNameIsEmpty"     // pvc name for backup is empty
	ErrorTypeBackupJobFailed          ErrorType = "BackupJobFailed"          // backup job failed
	ErrorTypeStorageNotMatch          ErrorType = "ErrorTypeStorageNotMatch"
	ErrorTypeReconfigureFailed        ErrorType = "ErrorTypeReconfigureFailed"
	ErrorTypeInvalidLogfileBackupName ErrorType = "InvalidLogfileBackupName"
	ErrorTypeBackupScheduleDisabled   ErrorType = "BackupScheduleDisabled"
	ErrorTypeLogfileScheduleDisabled  ErrorType = "LogfileScheduleDisabled"

	// ErrorType for cluster controller
	ErrorTypeBackupFailed ErrorType = "BackupFailed"
	ErrorTypeNeedWaiting  ErrorType = "NeedWaiting" // waiting for next reconcile

	// ErrorType for preflight
	ErrorTypePreflightCommon = "PreflightCommon"
	ErrorTypeSkipPreflight   = "SkipPreflight"
)

var ErrFailedToAddFinalizer = errors.New("failed to add finalizer")

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

// IsTargetError checks if the error is the target error.
func IsTargetError(err error, errorType ErrorType) bool {
	if tmpErr, ok := err.(*Error); ok || errors.As(err, &tmpErr) {
		return tmpErr.Type == errorType
	}
	return false
}

// UnwrapControllerError unwraps the Controller error from target error.
func UnwrapControllerError(err error) *Error {
	if tmpErr, ok := err.(*Error); ok || errors.As(err, &tmpErr) {
		return tmpErr
	}
	return nil
}

// NewNotFound returns a new Error with ErrorTypeNotFound.
func NewNotFound(format string, a ...any) *Error {
	return &Error{
		Type:    ErrorTypeNotFound,
		Message: fmt.Sprintf(format, a...),
	}
}

// IsNotFound returns true if the specified error is the error type of ErrorTypeNotFound.
func IsNotFound(err error) bool {
	return IsTargetError(err, ErrorTypeNotFound)
}

// NewBackupNotSupported returns a new Error with ErrorTypeBackupNotSupported.
func NewBackupNotSupported(backupType, backupPolicyName string) *Error {
	return NewErrorf(ErrorTypeBackupNotSupported, `backup type "%s" not supported by backup policy "%s"`, backupType, backupPolicyName)
}

// NewBackupPVTemplateNotFound returns a new Error with ErrorTypeBackupPVTemplateNotFound.
func NewBackupPVTemplateNotFound(cmName, cmNamespace string) *Error {
	return NewErrorf(ErrorTypeBackupPVTemplateNotFound, `"the persistentVolume template is empty in the configMap %s/%s", pvConfig.Namespace, pvConfig.Name`, cmNamespace, cmName)
}

// NewBackupPVCNameIsEmpty returns a new Error with ErrorTypeBackupPVCNameIsEmpty.
func NewBackupPVCNameIsEmpty(backupType, backupPolicyName string) *Error {
	return NewErrorf(ErrorTypeBackupPVCNameIsEmpty, `the persistentVolumeClaim name of spec.%s is empty in BackupPolicy "%s"`, strings.ToLower(backupType), backupPolicyName)
}

// NewBackupJobFailed returns a new Error with ErrorTypeBackupJobFailed.
func NewBackupJobFailed(jobName string) *Error {
	return NewErrorf(ErrorTypeBackupJobFailed, `backup job "%s" failed`, jobName)
}

// NewInvalidLogfileBackupName returns a new Error with ErrorTypeInvalidLogfileBackupName.
func NewInvalidLogfileBackupName(backupPolicyName string) *Error {
	return NewErrorf(ErrorTypeInvalidLogfileBackupName, `backup name is incorrect for logfile, you can create the logfile backup by enabling the schedule in BackupPolicy "%s"`, backupPolicyName)
}

// NewBackupScheduleDisabled returns a new Error with ErrorTypeBackupScheduleDisabled.
func NewBackupScheduleDisabled(backupType, backupPolicyName string) *Error {
	return NewErrorf(ErrorTypeBackupScheduleDisabled, `%s schedule is disabled, you can enable spec.schedule.%s in BackupPolicy "%s"`, backupType, backupType, backupPolicyName)
}

// NewBackupLogfileScheduleDisabled returns a new Error with ErrorTypeLogfileScheduleDisabled.
func NewBackupLogfileScheduleDisabled(backupToolName string) *Error {
	return NewErrorf(ErrorTypeLogfileScheduleDisabled, `BackupTool "%s" of the backup relies on logfile. Please enable the logfile scheduling firstly`, backupToolName)
}
