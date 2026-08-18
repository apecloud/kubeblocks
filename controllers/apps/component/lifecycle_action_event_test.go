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
	"errors"
	"fmt"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	"github.com/apecloud/kubeblocks/pkg/controller/lifecycle"
)

type capturedEvent struct {
	object    runtime.Object
	eventType string
	reason    string
	message   string
}

type capturingEventRecorder struct {
	events []capturedEvent
}

func (r *capturingEventRecorder) Event(object runtime.Object, eventType, reason, message string) {
	r.events = append(r.events, capturedEvent{object: object, eventType: eventType, reason: reason, message: message})
}

func (r *capturingEventRecorder) Eventf(object runtime.Object, eventType, reason, messageFmt string, args ...interface{}) {
	r.Event(object, eventType, reason, fmt.Sprintf(messageFmt, args...))
}

func (r *capturingEventRecorder) AnnotatedEventf(object runtime.Object, _ map[string]string, eventType, reason, messageFmt string, args ...interface{}) {
	r.Eventf(object, eventType, reason, messageFmt, args...)
}

func TestEmitLifecycleActionFailureEvent(t *testing.T) {
	const (
		namespace = "default"
		compName  = "test-cluster-mysql"
	)
	comp := &appsv1.Component{ObjectMeta: metav1.ObjectMeta{
		Namespace: namespace,
		Name:      compName,
	}}
	recorder := &capturingEventRecorder{}
	transCtx := &componentTransformContext{
		EventRecorder: recorder,
		Component:     comp,
	}

	emitLifecycleActionFailureEvent(transCtx, postProvisionFailedEventReason, "postProvision", lifecycle.ErrActionTimedOut)

	if len(recorder.events) != 1 {
		t.Fatalf("expected one event on Component, got %d", len(recorder.events))
	}
	if recorder.events[0].object != comp {
		t.Fatalf("expected event on Component, got %T", recorder.events[0].object)
	}
	for _, event := range recorder.events {
		if event.eventType != corev1.EventTypeWarning || event.reason != postProvisionFailedEventReason {
			t.Fatalf("unexpected event type or reason: %#v", event)
		}
		if want := "Failed to execute postProvision lifecycle action for Component " + compName; !strings.Contains(event.message, want) {
			t.Fatalf("expected event message to contain %q, got %q", want, event.message)
		}
	}
}

func TestEmitLifecycleActionFailureEventSuppressesWaitingStates(t *testing.T) {
	waitingErrors := []error{
		lifecycle.ErrActionNotDefined,
		lifecycle.ErrPreconditionFailed,
		lifecycle.ErrActionInProgress,
		lifecycle.ErrActionBusy,
		fmt.Errorf("wrapped: %w", lifecycle.ErrActionBusy),
	}
	for _, actionErr := range waitingErrors {
		t.Run(actionErr.Error(), func(t *testing.T) {
			recorder := &capturingEventRecorder{}
			transCtx := &componentTransformContext{EventRecorder: recorder}
			emitLifecycleActionFailureEvent(transCtx, postProvisionFailedEventReason, "postProvision", actionErr)
			if len(recorder.events) != 0 {
				t.Fatalf("expected no event for waiting state %q", actionErr)
			}
		})
	}

	if !lifecycle.IsActionFailure(errors.New("pod is unavailable")) {
		t.Fatal("expected an execution error to be classified as a lifecycle action failure")
	}
}
