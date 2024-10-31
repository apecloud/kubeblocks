/*
Copyright (C) 2022-2024 ApeCloud Co., Ltd

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

package component

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	"github.com/apecloud/kubeblocks/pkg/kbagent/proto"
)

const (
	availableProbe         = "availableProbe"
	availableProbeEventKey = "apps.kubeblocks.io/available-probe-event"
)

type AvailableEventHandler struct{}

func (h *AvailableEventHandler) Handle(cli client.Client, reqCtx intctrlutil.RequestCtx, recorder record.EventRecorder, event *corev1.Event) error {
	if !h.isAvailableEvent(event) {
		return nil
	}

	ppEvent := &proto.ProbeEvent{}
	if err := json.Unmarshal([]byte(event.Message), ppEvent); err != nil {
		return err
	}

	compKey := types.NamespacedName{
		Namespace: event.InvolvedObject.Namespace,
		Name:      ppEvent.Instance,
	}
	comp := &appsv1.Component{}
	if err := cli.Get(reqCtx.Ctx, compKey, comp); err != nil {
		return err
	}
	compCopy := comp.DeepCopy()

	compDef, err := h.getNCheckCompDefinition(reqCtx.Ctx, cli, comp.Spec.CompDef)
	if err != nil {
		return err
	}

	available, message, err := h.handleEvent(newProbeEvent(event, ppEvent), comp, compDef)
	if err != nil {
		return err
	}
	if available == nil {
		return nil // w/o available probe
	}
	if *available {
		return h.available(reqCtx.Ctx, cli, recorder, compCopy, comp, message)
	}
	return h.unavailable(reqCtx.Ctx, cli, recorder, compCopy, comp, message)
}

func (h *AvailableEventHandler) isAvailableEvent(event *corev1.Event) bool {
	return event.ReportingController == proto.ProbeEventReportingController &&
		event.Reason == availableProbe && event.InvolvedObject.FieldPath == proto.ProbeEventFieldPath
}

func (h *AvailableEventHandler) available(ctx context.Context, cli client.Client,
	recorder record.EventRecorder, compCopy, comp *appsv1.Component, message string) error {
	return h.status(ctx, cli, recorder, compCopy, comp, metav1.ConditionTrue, "Available", message)
}

func (h *AvailableEventHandler) unavailable(ctx context.Context, cli client.Client,
	recorder record.EventRecorder, compCopy, comp *appsv1.Component, message string) error {
	return h.status(ctx, cli, recorder, compCopy, comp, metav1.ConditionFalse, "Unavailable", message)
}

func (h *AvailableEventHandler) status(ctx context.Context, cli client.Client, recorder record.EventRecorder,
	compCopy, comp *appsv1.Component, status metav1.ConditionStatus, reason, message string) error {
	var (
		cond = metav1.Condition{
			Type:               appsv1.ConditionTypeAvailable,
			Status:             status,
			ObservedGeneration: comp.Generation, // TODO: ???
			LastTransitionTime: metav1.Now(),
			Reason:             reason,
			Message:            message,
		}
		// backup the available probe events
		probeEvents, ok = comp.Annotations[availableProbeEventKey]
	)
	if meta.SetStatusCondition(&comp.Status.Conditions, cond) {
		recorder.Event(comp, corev1.EventTypeNormal, reason, message)
		if err := cli.Status().Patch(ctx, comp, client.MergeFrom(compCopy)); err != nil {
			return err
		}
		compCopy = comp.DeepCopy() // update the compCopy since the comp is updated
	}

	if ok {
		if comp.Annotations == nil {
			comp.Annotations = make(map[string]string)
		}
		comp.Annotations[availableProbeEventKey] = probeEvents
	}
	if !reflect.DeepEqual(comp.Annotations, compCopy.Annotations) {
		return cli.Patch(ctx, comp, client.MergeFrom(compCopy))
	}
	return nil
}

func (h *AvailableEventHandler) handleEvent(event probeEvent, comp *appsv1.Component, compDef *appsv1.ComponentDefinition) (*bool, string, error) {
	policy := GetComponentAvailablePolicy(compDef)
	if policy.WithProbe == nil || policy.WithProbe.Condition == nil {
		if policy.WithPhases != nil {
			return nil, "", nil
		}
		return nil, "", fmt.Errorf("the referenced ComponentDefinition does not have available probe defined, but we got a probe event? %s", compDef.Name)
	}

	events, err := h.pickupProbeEvents(event, *policy.WithProbe.TimeWindowSeconds, comp)
	if err != nil {
		return nil, "", err
	}
	available, message := h.evalCond(*policy.WithProbe.Condition, comp.Spec.Replicas, events)
	if available {
		message = "the available conditions are met"
		if len(policy.WithProbe.Description) > 0 {
			message = policy.WithProbe.Description
		}
	}
	return &available, message, nil
}

func (h *AvailableEventHandler) getNCheckCompDefinition(ctx context.Context, cli client.Reader, name string) (*appsv1.ComponentDefinition, error) {
	compKey := types.NamespacedName{
		Name: name,
	}
	compDef := &appsv1.ComponentDefinition{}
	if err := cli.Get(ctx, compKey, compDef); err != nil {
		return nil, err
	}
	if compDef.Generation != compDef.Status.ObservedGeneration {
		return nil, fmt.Errorf("the referenced ComponentDefinition is not up to date: %s", compDef.Name)
	}
	if compDef.Status.Phase != appsv1.AvailablePhase {
		return nil, fmt.Errorf("the referenced ComponentDefinition is unavailable: %s", compDef.Name)
	}
	return compDef, nil
}

type probeEvent struct {
	PodName   string    `json:"podName"`
	PodUID    types.UID `json:"podUID"`
	Timestamp time.Time `json:"timestamp"`
	Code      int32     `json:"code"`
	Stdout    []byte    `json:"stdout,omitempty"`
	Stderr    []byte    `json:"stderr,omitempty"`
}

func newProbeEvent(event *corev1.Event, ppEvent *proto.ProbeEvent) probeEvent {
	return probeEvent{
		PodName:   event.InvolvedObject.Name,
		PodUID:    event.InvolvedObject.UID,
		Timestamp: event.LastTimestamp.Time,
		Code:      ppEvent.Code,
		Stdout:    ppEvent.Output,
		Stderr:    []byte(ppEvent.Message),
	}
}

func (h *AvailableEventHandler) pickupProbeEvents(event probeEvent, timeWindow int32, comp *appsv1.Component) ([]probeEvent, error) {
	events, err := h.getCachedEvents(comp)
	if err != nil {
		return nil, err
	}
	events = append(events, event)

	podNames, err := GenerateAllPodNames(comp.Spec.Replicas, comp.Spec.Instances, comp.Spec.OfflineInstances, comp.Name)
	if err != nil {
		return nil, err
	}

	timestamp := time.Now().Add(time.Duration(timeWindow*-1) * time.Second)

	filterByTimeWindow := func(events []probeEvent) []probeEvent {
		result := make([]probeEvent, 0)
		for i, evt := range events {
			if evt.Timestamp.After(timestamp) {
				result = append(result, events[i])
			}
		}
		return result
	}

	filterByPodNames := func(events map[string][]probeEvent) map[string][]probeEvent {
		names := sets.New[string](podNames...)
		result := make(map[string][]probeEvent)
		for name := range events {
			if names.Has(name) {
				result[name] = events[name]
			}
		}
		return result
	}

	groupByPod := func(events []probeEvent) map[string][]probeEvent {
		result := make(map[string][]probeEvent)
		for i, evt := range events {
			podEvents, ok := result[evt.PodName]
			if ok {
				result[evt.PodName] = append(podEvents, events[i])
			} else {
				result[evt.PodName] = []probeEvent{events[i]}
			}
		}
		return result
	}

	latest := func(events map[string][]probeEvent) []probeEvent {
		result := make([]probeEvent, 0)
		for k := range events {
			podEvents := events[k]
			evt := podEvents[0]
			for i := 1; i < len(podEvents); i++ {
				if podEvents[i].Timestamp.After(evt.Timestamp) {
					evt = podEvents[i]
				}
			}
			result = append(result, evt)
		}
		return result
	}

	pickedEvents := latest(filterByPodNames(groupByPod(filterByTimeWindow(events))))
	if err = h.updateCachedEvents(comp, pickedEvents); err != nil {
		return nil, err
	}
	return pickedEvents, nil
}

func (h *AvailableEventHandler) getCachedEvents(comp *appsv1.Component) ([]probeEvent, error) {
	if comp.Annotations == nil {
		return nil, nil
	}
	message, ok := comp.Annotations[availableProbeEventKey]
	if !ok {
		return nil, nil
	}
	events := make([]probeEvent, 0)
	err := json.Unmarshal([]byte(message), &events)
	if err != nil {
		return nil, err
	}
	return events, nil
}

func (h *AvailableEventHandler) updateCachedEvents(comp *appsv1.Component, events []probeEvent) error {
	if comp.Annotations == nil && len(events) == 0 {
		return nil
	}

	out, err := json.Marshal(&events)
	if err != nil {
		return err
	}

	if comp.Annotations == nil {
		comp.Annotations = make(map[string]string)
	}
	comp.Annotations[availableProbeEventKey] = string(out)

	return nil
}

func (h *AvailableEventHandler) evalCond(cond appsv1.ComponentAvailableCondition, replicas int32, events []probeEvent) (bool, string) {
	if len(cond.And) > 0 {
		return h.evalCondAnd(cond.And, replicas, events)
	}
	if len(cond.Or) > 0 {
		return h.evalCondOr(cond.Or, replicas, events)
	}
	if cond.Not != nil {
		return h.evalCondNot(*cond.Not, replicas, events)
	}
	return h.evalExpr(cond.ComponentAvailableExpression, replicas, events)
}

func (h *AvailableEventHandler) evalCondAnd(expressions []appsv1.ComponentAvailableExpression, replicas int32, events []probeEvent) (bool, string) {
	for _, expr := range expressions {
		ok, msg := h.evalExpr(expr, replicas, events)
		if !ok {
			return false, msg
		}
	}
	return true, ""
}

func (h *AvailableEventHandler) evalCondOr(expressions []appsv1.ComponentAvailableExpression, replicas int32, events []probeEvent) (bool, string) {
	msgs := make([]string, 0)
	for _, expr := range expressions {
		ok, msg := h.evalExpr(expr, replicas, events)
		if ok {
			return true, ""
		}
		if len(msg) > 0 {
			msgs = append(msgs, msg)
		}
	}
	return false, strings.Join(h.distinct(msgs), ",")
}

func (h *AvailableEventHandler) evalCondNot(expr appsv1.ComponentAvailableExpression, replicas int32, events []probeEvent) (bool, string) {
	ok, msg := h.evalExpr(expr, replicas, events)
	if ok {
		return false, msg
	}
	return true, ""
}

func (h *AvailableEventHandler) evalExpr(expr appsv1.ComponentAvailableExpression, replicas int32, events []probeEvent) (bool, string) {
	if expr.All != nil {
		return h.evalAssertionAll(*expr.All, replicas, events)
	}
	if expr.Any != nil {
		return h.evalAssertionAny(*expr.Any, replicas, events)
	}
	if expr.None != nil {
		return h.evalAssertionNone(*expr.None, replicas, events)
	}
	if expr.Majority != nil {
		return h.evalAssertionMajority(*expr.Majority, replicas, events)
	}
	return true, ""
}

func (h *AvailableEventHandler) evalAssertionAll(assertion appsv1.ComponentAvailableProbeAssertion, replicas int32, events []probeEvent) (bool, string) {
	if !h.strictCheck(assertion, replicas, events) {
		return false, fmt.Sprintf("not all replicas are available: %d/%d", len(events), replicas)
	}
	for _, event := range events {
		ok, msg := h.evalAssertion(assertion, []probeEvent{event})
		if !ok {
			return false, msg
		}
	}
	return true, ""
}

func (h *AvailableEventHandler) evalAssertionAny(assertion appsv1.ComponentAvailableProbeAssertion, replicas int32, events []probeEvent) (bool, string) {
	if !h.strictCheck(assertion, replicas, events) {
		return false, fmt.Sprintf("not all replicas are available: %d/%d", len(events), replicas)
	}
	msgs := make([]string, 0)
	for _, event := range events {
		ok, msg := h.evalAssertion(assertion, []probeEvent{event})
		if ok {
			return true, ""
		}
		if len(msg) > 0 {
			msgs = append(msgs, msg)
		}
	}
	return false, strings.Join(h.distinct(msgs), ",")
}

func (h *AvailableEventHandler) evalAssertionNone(assertion appsv1.ComponentAvailableProbeAssertion, replicas int32, events []probeEvent) (bool, string) {
	if !h.strictCheck(assertion, replicas, events) {
		return false, fmt.Sprintf("not all replicas are available: %d/%d", len(events), replicas)
	}
	for _, event := range events {
		ok, msg := h.evalAssertion(assertion, []probeEvent{event})
		if ok {
			return false, msg
		}
	}
	return true, ""
}

func (h *AvailableEventHandler) evalAssertionMajority(assertion appsv1.ComponentAvailableProbeAssertion, replicas int32, events []probeEvent) (bool, string) {
	count := 0
	msgs := make([]string, 0)
	for _, event := range events {
		ok, msg := h.evalAssertion(assertion, []probeEvent{event})
		if ok {
			count++
		} else if len(msg) > 0 {
			msgs = append(msgs, msg)
		}
	}
	ok := int32(count) > replicas/2
	if ok {
		return true, ""
	}
	return false, strings.Join(h.distinct(msgs), ",")
}

func (h *AvailableEventHandler) strictCheck(assertion appsv1.ComponentAvailableProbeAssertion, replicas int32, events []probeEvent) bool {
	if assertion.Strict != nil && *assertion.Strict {
		if replicas != int32(len(events)) {
			return false
		}
	}
	return true
}

func (h *AvailableEventHandler) evalAssertion(assertion appsv1.ComponentAvailableProbeAssertion, events []probeEvent) (bool, string) {
	if assertion.ActionAssertion != (appsv1.ActionAssertion{}) {
		return h.evalAction(assertion.ActionAssertion, events)
	}
	if len(assertion.And) > 0 {
		return h.evalActionAnd(assertion.And, events)
	}
	if assertion.Or != nil {
		return h.evalActionOr(assertion.Or, events)
	}
	if assertion.Not != nil {
		return h.evalActionNot(*assertion.Not, events)
	}
	return true, ""
}

func (h *AvailableEventHandler) evalActionAnd(assertions []appsv1.ActionAssertion, events []probeEvent) (bool, string) {
	for _, assertion := range assertions {
		ok, msg := h.evalAction(assertion, events)
		if !ok {
			return false, msg
		}
	}
	return true, ""
}

func (h *AvailableEventHandler) evalActionOr(assertions []appsv1.ActionAssertion, events []probeEvent) (bool, string) {
	msgs := make([]string, 0)
	for _, assertion := range assertions {
		ok, msg := h.evalAction(assertion, events)
		if ok {
			return true, ""
		}
		if len(msg) > 0 {
			msgs = append(msgs, msg)
		}
	}
	return false, strings.Join(h.distinct(msgs), ",")
}

func (h *AvailableEventHandler) evalActionNot(assertion appsv1.ActionAssertion, events []probeEvent) (bool, string) {
	ok, msg := h.evalAction(assertion, events)
	return !ok, msg
}

func (h *AvailableEventHandler) evalAction(assertion appsv1.ActionAssertion, events []probeEvent) (bool, string) {
	msgs := make([]string, 0)
	for _, event := range events {
		ok, msg := h.evalActionEvent(assertion, event)
		if ok {
			return true, ""
		}
		if len(msg) > 0 {
			msgs = append(msgs, msg)
		}
	}
	return false, strings.Join(h.distinct(msgs), ",")

}

func (h *AvailableEventHandler) evalActionEvent(assertion appsv1.ActionAssertion, event probeEvent) (bool, string) {
	if assertion.Succeed != nil && *assertion.Succeed != (event.Code == 0) {
		return false, fmt.Sprintf("probe code is not 0: %d", event.Code)
	}
	prefix16 := func(out string) string {
		if len(out) <= 16 {
			return out
		}
		return out[:16] + "..."
	}
	if assertion.Stdout != nil {
		if assertion.Stdout.EqualTo != nil && !bytes.Equal(event.Stdout, []byte(*assertion.Stdout.EqualTo)) {
			return false, fmt.Sprintf("probe stdout is not match: %s", prefix16(*assertion.Stdout.EqualTo))
		}
		if assertion.Stdout.Contains != nil && !bytes.Contains(event.Stdout, []byte(*assertion.Stdout.Contains)) {
			return false, fmt.Sprintf("probe stdout does not contain: %s", prefix16(*assertion.Stdout.Contains))
		}
	}
	if assertion.Stderr != nil {
		if assertion.Stderr.EqualTo != nil && !bytes.Equal(event.Stderr, []byte(*assertion.Stderr.EqualTo)) {
			return false, fmt.Sprintf("probe stderr is not match: %s", prefix16(*assertion.Stderr.EqualTo))
		}
		if assertion.Stderr.Contains != nil && !bytes.Contains(event.Stderr, []byte(*assertion.Stderr.Contains)) {
			return false, fmt.Sprintf("probe stderr does not contain: %s", prefix16(*assertion.Stderr.Contains))
		}
	}
	return true, ""
}

func (h *AvailableEventHandler) distinct(msgs []string) []string {
	seen := make(map[string]bool)
	result := make([]string, 0)
	for _, msg := range msgs {
		if !seen[msg] {
			seen[msg] = true
			result = append(result, msg)
		}
	}
	return result
}

func GetComponentAvailablePolicy(compDef *appsv1.ComponentDefinition) appsv1.ComponentAvailable {
	timeWindowSeconds := func() *int32 {
		periodSeconds := int32(0)
		if compDef.Spec.LifecycleActions != nil && compDef.Spec.LifecycleActions.AvailableProbe != nil {
			periodSeconds = compDef.Spec.LifecycleActions.AvailableProbe.PeriodSeconds
		}
		return pointer.Int32(probeReportPeriodSeconds(periodSeconds) * 2)
	}

	// has available policy defined
	if compDef.Spec.Available != nil {
		policy := *compDef.Spec.Available
		if policy.WithProbe != nil && policy.WithProbe.TimeWindowSeconds == nil {
			policy.WithProbe.TimeWindowSeconds = timeWindowSeconds()
		}
		return policy
	}

	// has available probe defined
	if compDef.Spec.LifecycleActions != nil && compDef.Spec.LifecycleActions.AvailableProbe != nil {
		return appsv1.ComponentAvailable{
			WithProbe: &appsv1.ComponentAvailableWithProbe{
				TimeWindowSeconds: timeWindowSeconds(),
				Condition: &appsv1.ComponentAvailableCondition{
					ComponentAvailableExpression: appsv1.ComponentAvailableExpression{
						All: &appsv1.ComponentAvailableProbeAssertion{
							ActionAssertion: appsv1.ActionAssertion{
								Succeed: pointer.Bool(true),
							},
							Strict: pointer.Bool(true),
						},
					},
				},
				Description: "all replicas are available",
			},
		}
	}

	// use phases as default policy
	return appsv1.ComponentAvailable{
		// TODO: replicas == 0, stopped, updating, abnormal?
		WithPhases: pointer.String(string(appsv1.RunningClusterCompPhase)),
	}
}
