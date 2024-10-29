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
	availableProbe             = "availableProbe"
	defaultTimeWindow    int32 = 10
	availableProbeEvents       = "availableProbeEvents"
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
		return h.available(reqCtx.Ctx, cli, recorder, compCopy, comp)
	}
	return h.unavailable(reqCtx.Ctx, cli, recorder, compCopy, comp, message)
}

func (h *AvailableEventHandler) isAvailableEvent(event *corev1.Event) bool {
	return event.ReportingController == proto.ProbeEventReportingController &&
		event.Reason == availableProbe && event.InvolvedObject.FieldPath == proto.ProbeEventFieldPath
}

func (h *AvailableEventHandler) available(ctx context.Context, cli client.Client,
	recorder record.EventRecorder, compCopy, comp *appsv1.Component) error {
	return h.status(ctx, cli, recorder, compCopy, comp, metav1.ConditionTrue, "Available", "Component is available")
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
	)
	if meta.SetStatusCondition(&comp.Status.Conditions, cond) {
		recorder.Event(comp, corev1.EventTypeNormal, reason, message)
		return cli.Status().Patch(ctx, comp, client.MergeFrom(compCopy))
	}
	if !reflect.DeepEqual(comp.Status.Message, compCopy.Status.Message) {
		return cli.Status().Patch(ctx, comp, client.MergeFrom(compCopy))
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
	available, message := h.evaluateCondition(*policy.WithProbe.Condition, comp.Spec.Replicas, events)
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

	events = latest(filterByPodNames(groupByPod(filterByTimeWindow(events))))
	if err = h.updateCachedEvents(comp, events); err != nil {
		return nil, err
	}
	return events, nil
}

func (h *AvailableEventHandler) getCachedEvents(comp *appsv1.Component) ([]probeEvent, error) {
	if comp.Status.Message == nil {
		return nil, nil
	}
	// TODO: fix me
	message, ok := comp.Status.Message[availableProbeEvents]
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
	if comp.Status.Message == nil && len(events) == 0 {
		return nil
	}

	out, err := json.Marshal(&events)
	if err != nil {
		return err
	}

	if comp.Status.Message == nil {
		comp.Status.Message = make(map[string]string)
	}
	// TODO: fix me
	comp.Status.Message[availableProbeEvents] = string(out)

	return nil
}

func (h *AvailableEventHandler) evaluateCondition(cond appsv1.ComponentAvailableCondition, replicas int32, events []probeEvent) (bool, string) {
	if len(cond.And) > 0 {
		return h.evaluateAnd(cond.And, replicas, events)
	}
	if len(cond.Or) > 0 {
		return h.evaluateOr(cond.Or, replicas, events)
	}
	if cond.Not != nil {
		return h.evaluateNot(*cond.Not, replicas, events)
	}
	if cond.All != nil {
		return h.evaluateAll(*cond.All, replicas, events)
	}
	if cond.Any != nil {
		return h.evaluateAny(*cond.Any, replicas, events)
	}
	if cond.None != nil {
		return h.evaluateNone(*cond.None, replicas, events)
	}
	if cond.Majority != nil {
		return h.evaluateMajority(*cond.Majority, replicas, events)
	}
	return true, ""
}

func (h *AvailableEventHandler) evaluateAnd(conditions []appsv1.ComponentAvailableConditionX, replicas int32, events []probeEvent) (bool, string) {
	for _, cond := range conditions {
		ok, msg := h.evaluateConditionX(cond, replicas, events)
		if !ok {
			return false, msg
		}
	}
	return true, ""
}

func (h *AvailableEventHandler) evaluateOr(conditions []appsv1.ComponentAvailableConditionX, replicas int32, events []probeEvent) (bool, string) {
	msgs := make([]string, 0)
	for _, cond := range conditions {
		ok, msg := h.evaluateConditionX(cond, replicas, events)
		if ok {
			return true, ""
		}
		if len(msg) > 0 {
			msgs = append(msgs, msg)
		}
	}
	return false, strings.Join(msgs, ",")
}

func (h *AvailableEventHandler) evaluateNot(cond appsv1.ComponentAvailableConditionX, replicas int32, events []probeEvent) (bool, string) {
	ok, msg := h.evaluateConditionX(cond, replicas, events)
	if ok {
		return false, msg
	}
	return true, ""
}

func (h *AvailableEventHandler) evaluateAll(cond appsv1.ComponentAvailableConditionX, replicas int32, events []probeEvent) (bool, string) {
	if !h.strictCheck(cond, replicas, events) {
		return false, fmt.Sprintf("not all replicas are available: %d/%d", len(events), replicas)
	}
	for _, event := range events {
		ok, msg := h.evaluateConditionX(cond, replicas, []probeEvent{event})
		if !ok {
			return false, msg
		}
	}
	return true, ""
}

func (h *AvailableEventHandler) evaluateAny(cond appsv1.ComponentAvailableConditionX, replicas int32, events []probeEvent) (bool, string) {
	if !h.strictCheck(cond, replicas, events) {
		return false, fmt.Sprintf("not all replicas are available: %d/%d", len(events), replicas)
	}
	msgs := make([]string, 0)
	for _, event := range events {
		ok, msg := h.evaluateConditionX(cond, replicas, []probeEvent{event})
		if ok {
			return true, ""
		}
		if len(msg) > 0 {
			msgs = append(msgs, msg)
		}
	}
	return false, strings.Join(msgs, ",")
}

func (h *AvailableEventHandler) evaluateNone(cond appsv1.ComponentAvailableConditionX, replicas int32, events []probeEvent) (bool, string) {
	if !h.strictCheck(cond, replicas, events) {
		return false, fmt.Sprintf("not all replicas are available: %d/%d", len(events), replicas)
	}
	for _, event := range events {
		ok, msg := h.evaluateConditionX(cond, replicas, []probeEvent{event})
		if ok {
			return false, msg
		}
	}
	return true, ""
}

func (h *AvailableEventHandler) evaluateMajority(cond appsv1.ComponentAvailableConditionX, replicas int32, events []probeEvent) (bool, string) {
	count := 0
	msgs := make([]string, 0)
	for _, event := range events {
		ok, msg := h.evaluateConditionX(cond, replicas, []probeEvent{event})
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
	return false, strings.Join(msgs, ",")
}

func (h *AvailableEventHandler) strictCheck(cond appsv1.ComponentAvailableConditionX, replicas int32, events []probeEvent) bool {
	if cond.Strict != nil && *cond.Strict {
		if replicas != int32(len(events)) {
			return false
		}
	}
	return true
}

func (h *AvailableEventHandler) evaluateConditionX(cond appsv1.ComponentAvailableConditionX, replicas int32, events []probeEvent) (bool, string) {
	if cond.ActionCriteria != (appsv1.ActionCriteria{}) {
		return h.evaluateAction(cond.ActionCriteria, events)
	}
	if !reflect.DeepEqual(&cond.ComponentAvailableCondition, &appsv1.ComponentAvailableCondition{}) {
		return h.evaluateCondition(cond.ComponentAvailableCondition, replicas, events)
	}
	return true, ""
}

func (h *AvailableEventHandler) evaluateAction(criteria appsv1.ActionCriteria, events []probeEvent) (bool, string) {
	msgs := make([]string, 0)
	for _, event := range events {
		ok, msg := h.evaluateActionEvent(criteria, event)
		if ok {
			return true, ""
		}
		if len(msg) > 0 {
			msgs = append(msgs, msg)
		}
	}
	return false, strings.Join(msgs, ",")

}

func (h *AvailableEventHandler) evaluateActionEvent(criteria appsv1.ActionCriteria, event probeEvent) (bool, string) {
	if criteria.Succeed != nil && *criteria.Succeed != (event.Code == 0) {
		return false, fmt.Sprintf("code is not 0: %d", event.Code)
	}
	prefix16 := func(out string) string {
		if len(out) <= 16 {
			return out
		}
		return out[:16] + "..."
	}
	if criteria.Stdout != nil {
		if criteria.Stdout.EqualTo != nil && !bytes.Equal(event.Stdout, []byte(*criteria.Stdout.EqualTo)) {
			return false, fmt.Sprintf("stdout is not match: %s", prefix16(*criteria.Stdout.EqualTo))
		}
		if criteria.Stdout.Contains != nil && !bytes.Contains(event.Stdout, []byte(*criteria.Stdout.Contains)) {
			return false, fmt.Sprintf("stdout does not contain: %s", prefix16(*criteria.Stdout.Contains))
		}
	}
	if criteria.Stderr != nil {
		if criteria.Stderr.EqualTo != nil && !bytes.Equal(event.Stderr, []byte(*criteria.Stderr.EqualTo)) {
			return false, fmt.Sprintf("stderr is not match: %s", prefix16(*criteria.Stderr.EqualTo))
		}
		if criteria.Stderr.Contains != nil && !bytes.Contains(event.Stderr, []byte(*criteria.Stderr.Contains)) {
			return false, fmt.Sprintf("stderr does not contain: %s", prefix16(*criteria.Stderr.Contains))
		}
	}
	return true, ""
}

func GetComponentAvailablePolicy(compDef *appsv1.ComponentDefinition) appsv1.ComponentAvailable {
	if compDef.Spec.Available != nil {
		policy := *compDef.Spec.Available
		if policy.WithProbe != nil && policy.WithProbe.TimeWindowSeconds == nil {
			policy.WithProbe.TimeWindowSeconds = pointer.Int32(defaultTimeWindow)
			if compDef.Spec.LifecycleActions != nil && compDef.Spec.LifecycleActions.AvailableProbe != nil {
				policy.WithProbe.TimeWindowSeconds = pointer.Int32(compDef.Spec.LifecycleActions.AvailableProbe.PeriodSeconds)
			}
		}
		return policy
	}
	if compDef.Spec.LifecycleActions != nil && compDef.Spec.LifecycleActions.AvailableProbe != nil {
		return appsv1.ComponentAvailable{
			WithProbe: &appsv1.ComponentAvailableWithProbe{
				TimeWindowSeconds: pointer.Int32(compDef.Spec.LifecycleActions.AvailableProbe.PeriodSeconds),
				Condition: &appsv1.ComponentAvailableCondition{
					All: &appsv1.ComponentAvailableConditionX{
						ActionCriteria: appsv1.ActionCriteria{
							Succeed: pointer.Bool(true),
						},
					},
				},
			},
		}
	}
	return appsv1.ComponentAvailable{
		// TODO: replicas == 0, stopped, updating, abnormal?
		WithPhases: pointer.String(string(appsv1.RunningComponentPhase)),
	}
}