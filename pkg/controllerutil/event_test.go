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
	"strings"
	"testing"
	"unicode/utf8"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

type capturingEventRecorder struct {
	message string
}

func (r *capturingEventRecorder) Event(_ runtime.Object, _, _, message string) {
	r.message = message
}

func (r *capturingEventRecorder) Eventf(object runtime.Object, eventType, reason, messageFmt string, args ...interface{}) {
}

func (r *capturingEventRecorder) AnnotatedEventf(object runtime.Object, annotations map[string]string, eventType, reason, messageFmt string, args ...interface{}) {
}

func TestSendEventTruncatesMessage(t *testing.T) {
	recorder := &capturingEventRecorder{}
	SendEvent(recorder, &corev1.Pod{}, corev1.EventTypeWarning, "Failed", strings.Repeat("\u754c", 1024))

	if len(recorder.message) > maxEventMessageBytes {
		t.Fatalf("event message has %d bytes, want at most %d", len(recorder.message), maxEventMessageBytes)
	}
	if !utf8.ValidString(recorder.message) {
		t.Fatal("event message is not valid UTF-8")
	}
	if !strings.HasSuffix(recorder.message, eventMessageTruncatedMarker) {
		t.Fatalf("event message %q does not contain the truncation marker", recorder.message)
	}
}
