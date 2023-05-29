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

package types

import (
	"fmt"
	"time"
)

// default reconcile requeue after duration
var RequeueDuration = time.Millisecond * 100

type RequeueError interface {
	RequeueAfter() time.Duration
	Reason() string
}

type realRequeueError struct {
	reason       string
	requeueAfter time.Duration
}

func (r *realRequeueError) Error() string {
	return fmt.Sprintf("requeue after: %v as: %s", r.requeueAfter, r.reason)
}

func (r *realRequeueError) RequeueAfter() time.Duration {
	return r.requeueAfter
}

func (r *realRequeueError) Reason() string {
	return r.reason
}

var _ RequeueError = &realRequeueError{}

func NewRequeueError(after time.Duration, reason string) error {
	return &realRequeueError{
		reason:       reason,
		requeueAfter: after,
	}
}
