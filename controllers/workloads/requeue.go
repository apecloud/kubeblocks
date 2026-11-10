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
	"time"

	ctrl "sigs.k8s.io/controller-runtime"
)

// withStatusRetry retains the status observation's timer after the remaining work commits.
func withStatusRetry(result ctrl.Result, err error, after time.Duration) (ctrl.Result, error) {
	if err != nil || after <= 0 || (result.Requeue && result.RequeueAfter <= 0) {
		return result, err
	}
	if result.RequeueAfter <= 0 || after < result.RequeueAfter {
		result.Requeue = true
		result.RequeueAfter = after
	}
	return result, nil
}
