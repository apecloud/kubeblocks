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
	"crypto/sha256"
	"fmt"

	corev1 "k8s.io/api/core/v1"

	"github.com/apecloud/kubeblocks/pkg/controller/graph"
	"github.com/apecloud/kubeblocks/pkg/controller/lifecycle"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

const (
	postProvisionFailedEventReason = "PostProvisionFailed"
	preTerminateFailedEventReason  = "PreTerminateFailed"
	memberJoinFailedEventReason    = "MemberJoinFailed"
	memberLeaveFailedEventReason   = "MemberLeaveFailed"

	postProvisionFailureFingerprintAnnotationKey = "apps.kubeblocks.io/post-provision-failure-fingerprint"
	preTerminateFailureFingerprintAnnotationKey  = "apps.kubeblocks.io/pre-terminate-failure-fingerprint"
)

func reportLifecycleActionFailureEvent(transCtx *componentTransformContext, dag *graph.DAG,
	annotationKey, reason, action string, actionErr error) {
	if !lifecycle.IsActionFailure(actionErr) {
		setLifecycleActionFailureFingerprint(transCtx, dag, annotationKey, "")
		return
	}
	if transCtx.EventRecorder == nil {
		return
	}

	fingerprint := fmt.Sprintf("%x", sha256.Sum256([]byte(actionErr.Error())))
	if transCtx.Component.Annotations[annotationKey] == fingerprint {
		return
	}

	emitLifecycleActionFailureEvent(transCtx, reason, action, actionErr)
	setLifecycleActionFailureFingerprint(transCtx, dag, annotationKey, fingerprint)
}

func setLifecycleActionFailureFingerprint(transCtx *componentTransformContext, dag *graph.DAG,
	annotationKey, fingerprint string) {
	comp := transCtx.Component
	if comp == nil || (fingerprint == "" && (comp.Annotations == nil || comp.Annotations[annotationKey] == "")) {
		return
	}
	graphCli, ok := transCtx.Client.(model.GraphClient)
	if !ok {
		transCtx.Logger.V(1).Info("failed to persist lifecycle action event fingerprint", "component", comp.Name)
		return
	}

	compCopy := comp.DeepCopy()
	if compCopy.Annotations == nil {
		compCopy.Annotations = map[string]string{}
	}
	if fingerprint == "" {
		delete(compCopy.Annotations, annotationKey)
	} else {
		compCopy.Annotations[annotationKey] = fingerprint
	}
	graphCli.Patch(dag, comp, compCopy, &model.ReplaceIfExistingOption{})
}

// emitLifecycleActionFailureEvent reports lifecycle action failures on the
// Component. Waiting and retryable action states are not failures and should
// not produce warning events.
func emitLifecycleActionFailureEvent(transCtx *componentTransformContext, reason, action string, actionErr error) {
	if transCtx.EventRecorder == nil || !lifecycle.IsActionFailure(actionErr) {
		return
	}
	message := fmt.Sprintf("Failed to execute %s lifecycle action for Component %s: %v", action, transCtx.Component.Name, actionErr)
	intctrlutil.SendEvent(transCtx.EventRecorder, transCtx.Component, corev1.EventTypeWarning, reason, message)
}
