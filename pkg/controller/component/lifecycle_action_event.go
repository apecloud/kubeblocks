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
	"fmt"
	"unicode/utf8"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/record"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
)

const (
	maxEventMessageBytes        = 1024
	eventMessageTruncatedMarker = "...(truncated)"
)

// SendLifecycleActionFailureEvent reports a lifecycle action failure on a Component.
func SendLifecycleActionFailureEvent(recorder record.EventRecorder, comp *appsv1.Component,
	reason, action string, actionErr error) {
	if recorder == nil || comp == nil || actionErr == nil {
		return
	}

	message := truncateEventMessage(fmt.Sprintf("Failed to execute %s lifecycle action for Component %s: %v", action, comp.Name, actionErr))
	recorder.Event(comp, corev1.EventTypeWarning, reason, message)
}

func truncateEventMessage(message string) string {
	if len(message) <= maxEventMessageBytes {
		return message
	}
	limit := maxEventMessageBytes - len(eventMessageTruncatedMarker)
	message = message[:limit]
	for !utf8.ValidString(message) && len(message) > 0 {
		message = message[:len(message)-1]
	}
	return message + eventMessageTruncatedMarker
}
