/*
Copyright (C) 2022-2026 ApeCloud Co., Ltd

This file is part of KubeBlocks project

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program.  If not, see <http://www.gnu.org/licenses/>.
*/

package component

import (
	appsutil "github.com/apecloud/kubeblocks/controllers/apps/util"
	"github.com/apecloud/kubeblocks/pkg/controller/lifecycle"
)

const (
	postProvisionFailedEventReason = "PostProvisionFailed"
	preTerminateFailedEventReason  = "PreTerminateFailed"
	memberJoinFailedEventReason    = "MemberJoinFailed"
	memberLeaveFailedEventReason   = "MemberLeaveFailed"
)

// emitLifecycleActionFailureEvent reports lifecycle action failures on the
// Component. Waiting and retryable action states are not failures and should
// not produce warning events.
func emitLifecycleActionFailureEvent(transCtx *componentTransformContext, reason, action string, actionErr error) {
	if transCtx.EventRecorder == nil || !lifecycle.IsActionFailure(actionErr) {
		return
	}
	appsutil.SendLifecycleActionFailureEvent(transCtx.EventRecorder, transCtx.Component, reason, action, actionErr)
}
