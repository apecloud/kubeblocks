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

package lifecycle

import (
	"errors"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
)

var (
	ErrActionNotDefined     = errors.New("action is not defined")
	ErrActionNotImplemented = errors.New("action is not implemented")
	ErrPreconditionFailed   = errors.New("action precondition is not met")
	ErrActionInProgress     = errors.New("action is in progress")
	ErrActionBusy           = errors.New("action is busy")
	ErrActionTimedOut       = errors.New("action timed-out")
	ErrActionFailed         = errors.New("action failed")
	ErrActionInternalError  = errors.New("action internal error")
)

type actionResultError struct {
	code      appsv1.ActionResultCode
	retryable *bool
	err       error
}

func (e *actionResultError) Error() string {
	return e.err.Error()
}

func (e *actionResultError) Unwrap() error {
	return e.err
}

func (e *actionResultError) actionResultCode() appsv1.ActionResultCode {
	return e.code
}

func (e *actionResultError) actionResultRetryable() *bool {
	return e.retryable
}

// ActionErrorCode returns a stable semantic Action result only when every aggregated failure agrees.
func ActionErrorCode(err error) (appsv1.ActionResultCode, bool) {
	var coded interface {
		actionResultCode() appsv1.ActionResultCode
	}
	if errors.As(err, &coded) {
		code := coded.actionResultCode()
		return code, code != ""
	}
	return "", false
}

// ActionErrorRetryable returns the retry property declared for a normalized Action result.
func ActionErrorRetryable(err error) *bool {
	var result interface{ actionResultRetryable() *bool }
	if errors.As(err, &result) {
		return result.actionResultRetryable()
	}
	return nil
}

func IgnoreNotDefined(err error) error {
	if errors.Is(err, ErrActionNotDefined) {
		return nil
	}
	return err
}

type actionAggregateError struct {
	errs []error
}

func newActionAggregateError(errs []error) error {
	if len(errs) == 0 {
		return nil
	}
	return &actionAggregateError{errs: append([]error(nil), errs...)}
}

func (e *actionAggregateError) Error() string {
	return errors.Join(e.errs...).Error()
}

func (e *actionAggregateError) Is(target error) bool {
	if target == ErrActionNotDefined {
		for _, err := range e.errs {
			if !errors.Is(err, target) {
				return false
			}
		}
		return true
	}
	for _, err := range e.errs {
		if errors.Is(err, target) {
			return true
		}
	}
	return false
}

func (e *actionAggregateError) actionResultCode() appsv1.ActionResultCode {
	var result appsv1.ActionResultCode
	for _, err := range e.errs {
		code, ok := ActionErrorCode(err)
		if !ok || (result != "" && result != code) {
			return ""
		}
		result = code
	}
	return result
}

func (e *actionAggregateError) actionResultRetryable() *bool {
	var result *bool
	for _, err := range e.errs {
		retryable := ActionErrorRetryable(err)
		if retryable == nil || (result != nil && *result != *retryable) {
			return nil
		}
		value := *retryable
		result = &value
	}
	return result
}
