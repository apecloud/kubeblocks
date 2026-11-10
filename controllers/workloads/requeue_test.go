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

package workloads

import (
	"errors"
	"testing"
	"time"

	ctrl "sigs.k8s.io/controller-runtime"
)

func TestWithStatusRetry(t *testing.T) {
	failure := errors.New("commit failed")
	for _, tc := range []struct {
		name   string
		result ctrl.Result
		err    error
		after  time.Duration
		want   ctrl.Result
	}{
		{name: "availability wait after commit", after: time.Second, want: ctrl.Result{Requeue: true, RequeueAfter: time.Second}},
		{name: "settled observation", result: ctrl.Result{Requeue: true, RequeueAfter: time.Second}, want: ctrl.Result{Requeue: true, RequeueAfter: time.Second}},
		{name: "commit error", err: failure, after: time.Second},
		{name: "conflict or immediate retry", result: ctrl.Result{Requeue: true}, after: time.Second, want: ctrl.Result{Requeue: true}},
		{name: "shorter publication retry", result: ctrl.Result{Requeue: true, RequeueAfter: time.Millisecond}, after: time.Second, want: ctrl.Result{Requeue: true, RequeueAfter: time.Millisecond}},
		{name: "longer update retry", result: ctrl.Result{Requeue: true, RequeueAfter: time.Hour}, after: time.Second, want: ctrl.Result{Requeue: true, RequeueAfter: time.Second}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result, err := withStatusRetry(tc.result, tc.err, tc.after)
			if result != tc.want || err != tc.err {
				t.Fatalf("got (%#v, %v), want (%#v, %v)", result, err, tc.want, tc.err)
			}
		})
	}
}
