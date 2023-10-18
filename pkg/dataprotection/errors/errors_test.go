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
	"fmt"
	"reflect"
	"testing"

	"github.com/pkg/errors"

	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

func TestNerError(t *testing.T) {
	err1 := intctrlutil.NewError(ErrorTypeBackupNotCompleted, "test c2")
	if err1.Error() != "test c2" {
		t.Error("NewErrorf failed")
	}
}

func TestNerErrorf(t *testing.T) {
	err1 := intctrlutil.NewErrorf(ErrorTypeBackupNotCompleted, "test %s %s", "c1", "c2")
	if err1.Error() != "test c1 c2" {
		t.Error("NewErrorf failed")
	}
	testError := fmt.Errorf("test: %w", err1)
	if !errors.Is(testError, err1) {
		t.Error("errors.Is failed")
	}

	var target *intctrlutil.Error
	if !errors.As(testError, &target) {
		t.Error("errors.As failed")
	}
}

func TestNewErrors(t *testing.T) {
	backupNotSupported := NewBackupNotSupported("datafile", "policy-test")
	if !intctrlutil.IsTargetError(backupNotSupported, ErrorTypeBackupNotSupported) {
		t.Error("should be error of BackupNotSupported")
	}
	pvTemplateNotFound := NewBackupPVTemplateNotFound("configName", "default")
	if !intctrlutil.IsTargetError(pvTemplateNotFound, ErrorTypeBackupPVTemplateNotFound) {
		t.Error("should be error of BackupPVTemplateNotFound")
	}
	pvcIsEmpty := NewBackupPVCNameIsEmpty("datafile", "policy-test1")
	if !intctrlutil.IsTargetError(pvcIsEmpty, ErrorTypeBackupPVCNameIsEmpty) {
		t.Error("should be error of BackupPVCNameIsEmpty")
	}
	repoIsNotReady := NewBackupRepoIsNotReady("repo")
	if !intctrlutil.IsTargetError(repoIsNotReady, ErrorTypeBackupRepoIsNotReady) {
		t.Error("should be error of BackupRepoIsNotReady")
	}
	toolConfigSecretNameIsEmpty := NewToolConfigSecretNameIsEmpty("repo")
	if !intctrlutil.IsTargetError(toolConfigSecretNameIsEmpty, ErrorTypeToolConfigSecretNameIsEmpty) {
		t.Error("should be error of ToolConfigSecretNameIsEmpty")
	}
	jobFailed := NewBackupJobFailed("jobName")
	if !intctrlutil.IsTargetError(jobFailed, ErrorTypeBackupJobFailed) {
		t.Error("should be error of BackupJobFailed")
	}
}

func TestUnwrapControllerError(t *testing.T) {
	backupNotSupported := NewBackupNotSupported("datafile", "policy-test")
	newErr := intctrlutil.UnwrapControllerError(backupNotSupported)
	if newErr == nil {
		t.Error("should unwrap a controller error, but got nil")
	}
	err := errors.New("test error")
	newErr = intctrlutil.UnwrapControllerError(err)
	if newErr != nil {
		t.Errorf("should not unwrap a controller error, but got: %v", newErr)
	}
}

func TestNewBackupJobFailed(t *testing.T) {
	type args struct {
		jobName string
	}
	tests := []struct {
		name string
		args args
		want *intctrlutil.Error
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NewBackupJobFailed(tt.args.jobName); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NewBackupJobFailed() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNewBackupLogfileScheduleDisabled(t *testing.T) {
	type args struct {
		backupToolName string
	}
	tests := []struct {
		name string
		args args
		want *intctrlutil.Error
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NewBackupLogfileScheduleDisabled(tt.args.backupToolName); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NewBackupLogfileScheduleDisabled() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNewBackupNotSupported(t *testing.T) {
	type args struct {
		backupType       string
		backupPolicyName string
	}
	tests := []struct {
		name string
		args args
		want *intctrlutil.Error
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NewBackupNotSupported(tt.args.backupType, tt.args.backupPolicyName); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NewBackupNotSupported() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNewBackupPVCNameIsEmpty(t *testing.T) {
	type args struct {
		backupRepo       string
		backupPolicyName string
	}
	tests := []struct {
		name string
		args args
		want *intctrlutil.Error
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NewBackupPVCNameIsEmpty(tt.args.backupRepo, tt.args.backupPolicyName); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NewBackupPVCNameIsEmpty() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNewBackupPVTemplateNotFound(t *testing.T) {
	type args struct {
		cmName      string
		cmNamespace string
	}
	tests := []struct {
		name string
		args args
		want *intctrlutil.Error
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NewBackupPVTemplateNotFound(tt.args.cmName, tt.args.cmNamespace); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NewBackupPVTemplateNotFound() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNewBackupScheduleDisabled(t *testing.T) {
	type args struct {
		backupType       string
		backupPolicyName string
	}
	tests := []struct {
		name string
		args args
		want *intctrlutil.Error
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NewBackupScheduleDisabled(tt.args.backupType, tt.args.backupPolicyName); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NewBackupScheduleDisabled() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNewInvalidLogfileBackupName(t *testing.T) {
	type args struct {
		backupPolicyName string
	}
	tests := []struct {
		name string
		args args
		want *intctrlutil.Error
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NewInvalidLogfileBackupName(tt.args.backupPolicyName); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NewInvalidLogfileBackupName() = %v, want %v", got, tt.want)
			}
		})
	}
}
