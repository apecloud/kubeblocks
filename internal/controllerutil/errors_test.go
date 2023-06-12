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
	"fmt"
	"testing"

	"github.com/pkg/errors"
)

func TestNerError(t *testing.T) {
	err1 := NewError(ErrorTypeBackupNotCompleted, "test c2")
	if err1.Error() != "test c2" {
		t.Error("NewErrorf failed")
	}
}

func TestNerErrorf(t *testing.T) {
	err1 := NewErrorf(ErrorTypeBackupNotCompleted, "test %s %s", "c1", "c2")
	if err1.Error() != "test c1 c2" {
		t.Error("NewErrorf failed")
	}
	testError := fmt.Errorf("test: %w", err1)
	if !errors.Is(testError, err1) {
		t.Error("errors.Is failed")
	}

	var target *Error
	if !errors.As(testError, &target) {
		t.Error("errors.As failed")
	}
}

func TestNewErrors(t *testing.T) {
	backupNotSupported := NewBackupNotSupported("datafile", "policy-test")
	if !IsTargetError(backupNotSupported, ErrorTypeBackupNotSupported) {
		t.Error("should be error of BackupNotSupported")
	}
	pvTemplateNotFound := NewBackupPVTemplateNotFound("configName", "default")
	if !IsTargetError(pvTemplateNotFound, ErrorTypeBackupPVTemplateNotFound) {
		t.Error("should be error of BackupPVTemplateNotFound")
	}
	pvsIsEmpty := NewBackupPVCNameIsEmpty("policy-test1")
	if !IsTargetError(pvsIsEmpty, ErrorTypeBackupPVCNameIsEmpty) {
		t.Error("should be error of BackupPVCNameIsEmpty")
	}
	jobFailed := NewBackupJobFailed("jobName")
	if !IsTargetError(jobFailed, ErrorTypeBackupJobFailed) {
		t.Error("should be error of BackupJobFailed")
	}
}

func TestUnwrapControllerError(t *testing.T) {
	backupNotSupported := NewBackupNotSupported("datafile", "policy-test")
	newErr := UnwrapControllerError(backupNotSupported)
	if newErr == nil {
		t.Error("should unwrap a controller error, but got nil")
	}
	err := errors.New("test error")
	newErr = UnwrapControllerError(err)
	if newErr != nil {
		t.Errorf("should not unwrap a controller error, but got: %v", newErr)
	}
}

func TestIsTargetError(t *testing.T) {
	var err1 error
	if IsTargetError(err1, ErrorWaitCacheRefresh) {
		t.Error("IsTargetError expects a false return, but got false")
	}
	err1 = NewError(ErrorWaitCacheRefresh, "test c2")
	if !IsTargetError(err1, ErrorWaitCacheRefresh) {
		t.Error("IsTargetError expects a true return, but got false")
	}
	err2 := errors.Wrap(err1, "test c1")
	if !IsTargetError(err2, ErrorWaitCacheRefresh) {
		t.Error("IsTargetError expects a true return, but got false")
	}
}
