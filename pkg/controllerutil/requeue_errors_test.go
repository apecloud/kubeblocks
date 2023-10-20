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
	"time"
)

func TestIsDelayedRequeueError(t *testing.T) {
	err := NewDelayedRequeueError(time.Second, "requeue for test")
	if !IsDelayedRequeueError(err) {
		t.Error("should return ture when error is a delayed requeue error")
	}
	if !IsRequeueError(err) {
		t.Error("should return ture when error is a requeue error")
	}
}

func TestIsRequeueError(t *testing.T) {
	err := NewRequeueError(time.Second, "requeue for test")
	if !IsRequeueError(err) {
		t.Error("should return ture when error is a requeue error")
	}
}

func TestOtherFunctions(t *testing.T) {
	reason := "requeue for test"
	err := NewRequeueError(time.Second, reason)
	requeueErr := err.(RequeueError)
	if requeueErr.RequeueAfter() != time.Second {
		t.Errorf("requeue after should equals %d", time.Second)
	}

	if requeueErr.Reason() != reason {
		t.Errorf("reason should equals %s", reason)
	}

	if err.Error() != fmt.Sprintf("requeue after: %v as: %s", requeueErr.RequeueAfter(), requeueErr.Reason()) {
		t.Error("error message is incorrect")
	}
}
