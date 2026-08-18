/*
Copyright (C) 2022-2026 ApeCloud Co., Ltd

This file is part of KubeBlocks project

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <http://www.gnu.org/licenses/>.
*/

package controllerutil

import (
	"unicode/utf8"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
)

const (
	maxEventMessageBytes        = 1024
	eventMessageTruncatedMarker = "...(truncated)"
)

// SendEvent sends an Event after truncating its message to Kubernetes' limit.
func SendEvent(recorder record.EventRecorder, object runtime.Object, eventType, reason, message string) {
	if recorder == nil || object == nil {
		return
	}
	recorder.Event(object, eventType, reason, truncateEventMessage(message))
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
