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
	appsutil "github.com/apecloud/kubeblocks/controllers/apps/util"
	"github.com/apecloud/kubeblocks/pkg/controller/graph"
	"github.com/apecloud/kubeblocks/pkg/controller/lifecycle"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
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

func TestReportComponentLifecycleActionFailureEventDeduplicatesByFingerprint(t *testing.T) {
	comp := &appsv1.Component{ObjectMeta: metav1.ObjectMeta{
		Namespace:   "default",
		Name:        "test-cluster-mysql",
		Annotations: map[string]string{},
	}}
	recorder := &capturingEventRecorder{}
	graphCli := model.NewGraphClient(&appsutil.MockReader{})
	newDAG := func(current *appsv1.Component) *graph.DAG {
		dag := graph.NewDAG()
		graphCli.Root(dag, current, current, model.ActionStatusPtr())
		return dag
	}
	transCtx := &componentTransformContext{
		Client:        graphCli,
		EventRecorder: recorder,
		Component:     comp,
	}
	dag := newDAG(comp)
	actionErr := errors.New("post-provision failed")

	reportComponentLifecycleActionFailureEvent(transCtx, dag,
		postProvisionFailureFingerprintAnnotationKey, postProvisionFailedEventReason, "postProvision", actionErr)
	if len(recorder.events) != 1 {
		t.Fatalf("expected one event, got %d", len(recorder.events))
	}
	vertex := graphCli.FindMatchedVertex(dag, comp)
	patchedComp := vertex.(*model.ObjectVertex).Obj.(*appsv1.Component)
	if patchedComp.Annotations[postProvisionFailureFingerprintAnnotationKey] == "" {
		t.Fatal("expected the failure fingerprint to be persisted")
	}

	transCtx.Component = patchedComp
	dag = newDAG(patchedComp)
	reportComponentLifecycleActionFailureEvent(transCtx, dag,
		postProvisionFailureFingerprintAnnotationKey, postProvisionFailedEventReason, "postProvision", actionErr)
	if len(recorder.events) != 1 {
		t.Fatalf("expected the unchanged failure to be suppressed, got %d events", len(recorder.events))
	}

	reportComponentLifecycleActionFailureEvent(transCtx, dag,
		postProvisionFailureFingerprintAnnotationKey, postProvisionFailedEventReason, "postProvision",
		errors.New("post-provision timed out"))
	if len(recorder.events) != 2 {
		t.Fatalf("expected a changed failure to emit again, got %d events", len(recorder.events))
	}
	vertex = graphCli.FindMatchedVertex(dag, patchedComp)
	changedComp := vertex.(*model.ObjectVertex).Obj.(*appsv1.Component)

	transCtx.Component = changedComp
	dag = newDAG(changedComp)
	reportComponentLifecycleActionFailureEvent(transCtx, dag,
		postProvisionFailureFingerprintAnnotationKey, postProvisionFailedEventReason, "postProvision",
		lifecycle.ErrActionInProgress)
	vertex = graphCli.FindMatchedVertex(dag, changedComp)
	clearedComp := vertex.(*model.ObjectVertex).Obj.(*appsv1.Component)
	if _, ok := clearedComp.Annotations[postProvisionFailureFingerprintAnnotationKey]; ok {
		t.Fatal("expected the failure fingerprint to be cleared after recovery")
	}
}
