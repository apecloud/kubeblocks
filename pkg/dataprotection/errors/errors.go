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

package errors

import (
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

// ErrorType for backup
const (
	// ErrorTypeBackupNotSupported this backup type not supported
	ErrorTypeBackupNotSupported intctrlutil.ErrorType = "BackupNotSupported"
	// ErrorTypeBackupPVTemplateNotFound this pv template not found
	ErrorTypeBackupPVTemplateNotFound intctrlutil.ErrorType = "BackupPVTemplateNotFound"
	// ErrorTypeBackupNotCompleted report backup not completed.
	ErrorTypeBackupNotCompleted intctrlutil.ErrorType = "BackupNotCompleted"
	// ErrorTypeBackupPVCNameIsEmpty pvc name for backup is empty
	ErrorTypeBackupPVCNameIsEmpty intctrlutil.ErrorType = "BackupPVCNameIsEmpty"
	// ErrorTypeBackupRepoIsNotReady the backup repository is not ready
	ErrorTypeBackupRepoIsNotReady intctrlutil.ErrorType = "BackupRepoIsNotReady"
	// ErrorTypeToolConfigSecretNameIsEmpty the name of  repository is not ready
	ErrorTypeToolConfigSecretNameIsEmpty intctrlutil.ErrorType = "ToolConfigSecretNameIsEmpty"
	// ErrorTypeBackupJobFailed backup job failed
	ErrorTypeBackupJobFailed intctrlutil.ErrorType = "BackupJobFailed"
	// ErrorTypeStorageNotMatch storage not match
	ErrorTypeStorageNotMatch intctrlutil.ErrorType = "ErrorTypeStorageNotMatch"
	// ErrorTypeReconfigureFailed reconfigure failed
	ErrorTypeReconfigureFailed intctrlutil.ErrorType = "ErrorTypeReconfigureFailed"
	// ErrorTypeInvalidLogfileBackupName invalid logfile backup name
	ErrorTypeInvalidLogfileBackupName intctrlutil.ErrorType = "InvalidLogfileBackupName"
	// ErrorTypeBackupScheduleDisabled backup schedule disabled
	ErrorTypeBackupScheduleDisabled intctrlutil.ErrorType = "BackupScheduleDisabled"
	// ErrorTypeLogfileScheduleDisabled logfile schedule disabled
	ErrorTypeLogfileScheduleDisabled intctrlutil.ErrorType = "LogfileScheduleDisabled"
	// ErrorTypeWaitForExternalHandler wait for external handler to handle the Backup or Restore
	ErrorTypeWaitForExternalHandler intctrlutil.ErrorType = "WaitForExternalHandler"
)

// NewBackupNotSupported returns a new Error with ErrorTypeBackupNotSupported.
func NewBackupNotSupported(backupType, backupPolicyName string) *intctrlutil.Error {
	return intctrlutil.NewErrorf(ErrorTypeBackupNotSupported, `backup type "%s" not supported by backup policy "%s"`, backupType, backupPolicyName)
}

// NewBackupPVTemplateNotFound returns a new Error with ErrorTypeBackupPVTemplateNotFound.
func NewBackupPVTemplateNotFound(cmName, cmNamespace string) *intctrlutil.Error {
	return intctrlutil.NewErrorf(ErrorTypeBackupPVTemplateNotFound, `"the persistentVolume template is empty in the configMap %s/%s", pvConfig.Namespace, pvConfig.Name`, cmNamespace, cmName)
}

// NewBackupRepoIsNotReady returns a new Error with ErrorTypeBackupRepoIsNotReady.
func NewBackupRepoIsNotReady(backupRepo string) *intctrlutil.Error {
	return intctrlutil.NewErrorf(ErrorTypeBackupRepoIsNotReady, `the backup repository %s is not ready`, backupRepo)
}

// NewToolConfigSecretNameIsEmpty returns a new Error with ErrorTypeToolConfigSecretNameIsEmpty.
func NewToolConfigSecretNameIsEmpty(backupRepo string) *intctrlutil.Error {
	return intctrlutil.NewErrorf(ErrorTypeToolConfigSecretNameIsEmpty, `the secret name of tool config from %s is empty`, backupRepo)
}

// NewBackupPVCNameIsEmpty returns a new Error with ErrorTypeBackupPVCNameIsEmpty.
func NewBackupPVCNameIsEmpty(backupRepo, backupPolicyName string) *intctrlutil.Error {
	return intctrlutil.NewErrorf(ErrorTypeBackupPVCNameIsEmpty, `the persistentVolumeClaim name of %s is empty in BackupPolicy "%s"`, backupRepo, backupPolicyName)
}

// NewBackupJobFailed returns a new Error with ErrorTypeBackupJobFailed.
func NewBackupJobFailed(jobName string) *intctrlutil.Error {
	return intctrlutil.NewErrorf(ErrorTypeBackupJobFailed, `backup job "%s" failed`, jobName)
}

// NewInvalidLogfileBackupName returns a new Error with ErrorTypeInvalidLogfileBackupName.
func NewInvalidLogfileBackupName(backupPolicyName string) *intctrlutil.Error {
	return intctrlutil.NewErrorf(ErrorTypeInvalidLogfileBackupName, `backup name is incorrect for logfile, you can create the logfile backup by enabling the schedule in BackupPolicy "%s"`, backupPolicyName)
}

// NewBackupScheduleDisabled returns a new Error with ErrorTypeBackupScheduleDisabled.
func NewBackupScheduleDisabled(backupType, backupPolicyName string) *intctrlutil.Error {
	return intctrlutil.NewErrorf(ErrorTypeBackupScheduleDisabled, `%s schedule is disabled, you can enable spec.schedule.%s in BackupPolicy "%s"`, backupType, backupType, backupPolicyName)
}

// NewBackupLogfileScheduleDisabled returns a new Error with ErrorTypeLogfileScheduleDisabled.
func NewBackupLogfileScheduleDisabled(backupToolName string) *intctrlutil.Error {
	return intctrlutil.NewErrorf(ErrorTypeLogfileScheduleDisabled, `BackupTool "%s" of the backup relies on logfile. Please enable the logfile scheduling firstly`, backupToolName)
}
