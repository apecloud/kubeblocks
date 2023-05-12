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
	"time"
)

type RequeueError interface {
	RequeueAfter() time.Duration
	Reason() string
}

type DelayedRequeueError interface {
	RequeueError
	Delayed()
}

func NewRequeueError(after time.Duration, reason string) error {
	return &requeueError{
		reason:       reason,
		requeueAfter: after,
	}
}

// NewDelayedRequeueError creates a delayed requeue error which only returns in the last step of the DAG.
func NewDelayedRequeueError(after time.Duration, reason string) error {
	return &delayedRequeueError{
		requeueError{
			reason:       reason,
			requeueAfter: after,
		},
	}
}

func IsDelayedRequeueError(err error) bool {
	_, ok := err.(DelayedRequeueError)
	return ok
}

// IsRequeueError checks if the error is the RequeueError.
func IsRequeueError(err error) bool {
	_, ok := err.(RequeueError)
	return ok
}

type requeueError struct {
	reason       string
	requeueAfter time.Duration
}

type delayedRequeueError struct {
	requeueError
}

var _ RequeueError = &requeueError{}
var _ DelayedRequeueError = &delayedRequeueError{}

func (r *requeueError) Error() string {
	return fmt.Sprintf("requeue after: %v as: %s", r.requeueAfter, r.reason)
}

func (r *requeueError) RequeueAfter() time.Duration {
	return r.requeueAfter
}

func (r *requeueError) Reason() string {
	return r.reason
}

func (r *delayedRequeueError) Delayed() {}

// NewValidationError creates a new validation error.
// The reason is used to generate the error message.
// When validation fails, cluster reconciler should prefer `RequeWithError` over `RequeueAfter`.
func NewValidationError(reason string) *validationError {
	return &validationError{reason: reason}
}

// validationError implements error interface BUT-NOT  RequeueError.
type validationError struct {
	reason string
}

var _ error = &validationError{}

func (r *validationError) Error() string {
	return r.reason
}
