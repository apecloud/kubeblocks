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

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	"github.com/apecloud/kubeblocks/pkg/kbagent/proto"
)

const (
	availableProbe = "availableProbe"
)

type AvailableEventHandler struct{}

func (h *AvailableEventHandler) Handle(cli client.Client, reqCtx intctrlutil.RequestCtx, recorder record.EventRecorder, event *corev1.Event) error {
	if !h.isAvailableEvent(event) {
		return nil
	}

	probeEvent := &proto.ProbeEvent{}
	if err := json.Unmarshal([]byte(event.Message), probeEvent); err != nil {
		return err
	}

	compKey := types.NamespacedName{
		Namespace: event.InvolvedObject.Namespace,
		Name:      probeEvent.Instance,
	}
	comp := &appsv1.Component{}
	if err := cli.Get(reqCtx.Ctx, compKey, comp); err != nil {
		return err
	}

	available, message, err := h.handleEvent(reqCtx.Ctx, cli, event, probeEvent, comp)
	if err != nil {
		return err
	}
	if available == nil {
		return nil
	}
	if *available {
		return h.available(reqCtx.Ctx, cli, recorder, comp)
	}
	return h.unavailable(reqCtx.Ctx, cli, recorder, comp, message)
}

func (h *AvailableEventHandler) isAvailableEvent(event *corev1.Event) bool {
	return event.ReportingController == proto.ProbeEventReportingController &&
		event.Reason == availableProbe && event.InvolvedObject.FieldPath == proto.ProbeEventFieldPath
}

func (h *AvailableEventHandler) available(ctx context.Context, cli client.Client,
	recorder record.EventRecorder, comp *appsv1.Component) error {
	return h.status(ctx, cli, recorder, comp, metav1.ConditionTrue, "Available", "Component is available")
}

func (h *AvailableEventHandler) unavailable(ctx context.Context, cli client.Client,
	recorder record.EventRecorder, comp *appsv1.Component, message string) error {
	return h.status(ctx, cli, recorder, comp, metav1.ConditionFalse, "Unavailable", message)
}

func (h *AvailableEventHandler) status(ctx context.Context, cli client.Client, recorder record.EventRecorder,
	comp *appsv1.Component, status metav1.ConditionStatus, reason, message string) error {
	newCond := func() metav1.Condition {
		return metav1.Condition{
			Type:               appsv1.ConditionTypeAvailable,
			Status:             status,
			ObservedGeneration: comp.Generation, // TODO: ???
			LastTransitionTime: metav1.Now(),
			Reason:             reason,
			Message:            message,
		}
	}()

	idx := -1
	for i, c := range comp.Status.Conditions {
		if c.Type == appsv1.ConditionTypeAvailable {
			idx = i
			break
		}
	}
	if idx >= 0 && h.condEqual(comp.Status.Conditions[idx], newCond) {
		return nil
	}

	compCopy := comp.DeepCopy()
	if idx < 0 {
		comp.Status.Conditions = append(comp.Status.Conditions, newCond)
	} else {
		comp.Status.Conditions[idx] = newCond
	}

	recorder.Event(comp, corev1.EventTypeNormal, reason, message)

	return cli.Status().Patch(ctx, comp, client.MergeFrom(compCopy))
}

func (h *AvailableEventHandler) condEqual(cond1, cond2 metav1.Condition) bool {
	return cond1.Type == cond2.Type && cond1.Status == cond2.Status &&
		cond1.ObservedGeneration == cond2.ObservedGeneration && cond1.Reason == cond2.Reason
}

func (h *AvailableEventHandler) handleEvent(ctx context.Context, cli client.Client,
	event *corev1.Event, probeEvent *proto.ProbeEvent, comp *appsv1.Component) (*bool, string, error) {
	cmpd, err := h.getNCheckCompDefinition(ctx, cli, comp.Spec.CompDef)
	if err != nil {
		return nil, "", err
	}

	policy := h.getComponentAvailablePolicy(cmpd)
	if policy.WithProbe == nil || policy.WithProbe.Condition == nil {
		if policy.WithPhases != nil || policy.WithRoles != nil {
			return nil, "", nil
		}
		return nil, "", fmt.Errorf("the referenced ComponentDefinition does not have available probe defined, but we got a probe event? %s", cmpd.Name)
	}

	events, err := h.pickupProbeEvents(event, probeEvent, comp)
	if err != nil {
		return nil, "", err
	}
	available, message, err := h.evaluateCondition(*policy.WithProbe.Condition, comp.Spec.Replicas, events)
	return &available, message, err
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

func (h *AvailableEventHandler) getComponentAvailablePolicy(cmpd *appsv1.ComponentDefinition) appsv1.ComponentAvailable {
	if cmpd.Spec.Available != nil {
		return *cmpd.Spec.Available
	}
	if cmpd.Spec.LifecycleActions != nil && cmpd.Spec.LifecycleActions.AvailableProbe != nil {
		return appsv1.ComponentAvailable{
			WithProbe: &appsv1.ComponentAvailableWithProbe{
				TimeWindow: pointer.Int32(cmpd.Spec.LifecycleActions.AvailableProbe.PeriodSeconds),
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
		WithPhases: pointer.String(string(appsv1.RunningClusterCompPhase)),
	}
}

func (h *AvailableEventHandler) pickupProbeEvents(event *corev1.Event, probeEvent *proto.ProbeEvent, comp *appsv1.Component) ([]proto.ProbeEvent, error) {

	return nil, nil
}

func (h *AvailableEventHandler) evaluateCondition(cond appsv1.ComponentAvailableCondition, replicas int32, events []proto.ProbeEvent) (bool, string, error) {
	if len(cond.And) > 0 {
		return h.evaluateAnd(cond.And, replicas, events), "", nil
	}
	if len(cond.Or) > 0 {
		return h.evaluateOr(cond.Or, replicas, events), "", nil
	}
	if cond.Not != nil {
		return h.evaluateNot(*cond.Not, replicas, events), "", nil
	}
	if cond.All != nil {
		return h.evaluateAll(*cond.All, replicas, events), "", nil
	}
	if cond.Any != nil {
		return h.evaluateAny(*cond.Any, replicas, events), "", nil
	}
	if cond.None != nil {
		return h.evaluateNone(*cond.None, replicas, events), "", nil
	}
	if cond.Majority != nil {
		return h.evaluateMajority(*cond.Majority, replicas, events), "", nil
	}
	return true, "", nil
}

func (h *AvailableEventHandler) evaluateAnd(conditions []appsv1.ComponentAvailableConditionX, replicas int32, events []proto.ProbeEvent) bool {
	for _, cond := range conditions {
		ok, _, _ := h.evaluateConditionX(cond, replicas, events)
		if !ok {
			return false
		}
	}
	return true
}

func (h *AvailableEventHandler) evaluateOr(conditions []appsv1.ComponentAvailableConditionX, replicas int32, events []proto.ProbeEvent) bool {
	for _, cond := range conditions {
		ok, _, _ := h.evaluateConditionX(cond, replicas, events)
		if ok {
			return true
		}
	}
	return false
}

func (h *AvailableEventHandler) evaluateNot(cond appsv1.ComponentAvailableConditionX, replicas int32, events []proto.ProbeEvent) bool {
	ok, _, _ := h.evaluateConditionX(cond, replicas, events)
	return !ok
}

func (h *AvailableEventHandler) evaluateAll(cond appsv1.ComponentAvailableConditionX, replicas int32, events []proto.ProbeEvent) bool {
	for _, event := range events {
		ok, _, _ := h.evaluateConditionX(cond, replicas, []proto.ProbeEvent{event})
		if !ok {
			return false
		}
	}
	return true
}

func (h *AvailableEventHandler) evaluateAny(cond appsv1.ComponentAvailableConditionX, replicas int32, events []proto.ProbeEvent) bool {
	for _, event := range events {
		ok, _, _ := h.evaluateConditionX(cond, replicas, []proto.ProbeEvent{event})
		if ok {
			return true
		}
	}
	return false
}

func (h *AvailableEventHandler) evaluateNone(cond appsv1.ComponentAvailableConditionX, replicas int32, events []proto.ProbeEvent) bool {
	for _, event := range events {
		ok, _, _ := h.evaluateConditionX(cond, replicas, []proto.ProbeEvent{event})
		if ok {
			return false
		}
	}
	return true
}

func (h *AvailableEventHandler) evaluateMajority(cond appsv1.ComponentAvailableConditionX, replicas int32, events []proto.ProbeEvent) bool {
	count := 0
	for _, event := range events {
		ok, _, _ := h.evaluateConditionX(cond, replicas, []proto.ProbeEvent{event})
		if ok {
			count++
		}
	}
	if int32(count) > replicas/2 {
		return true
	}
	return false
}

func (h *AvailableEventHandler) evaluateConditionX(cond appsv1.ComponentAvailableConditionX, replicas int32, events []proto.ProbeEvent) (bool, string, error) {
	if cond.ActionCriteria != (appsv1.ActionCriteria{}) {
		return h.evaluateAction(cond.ActionCriteria, events), "", nil
	}
	if !reflect.DeepEqual(&cond.ComponentAvailableCondition, &appsv1.ComponentAvailableCondition{}) {
		return h.evaluateCondition(cond.ComponentAvailableCondition, replicas, events)
	}
	return true, "", nil
}

func (h *AvailableEventHandler) evaluateAction(criteria appsv1.ActionCriteria, events []proto.ProbeEvent) bool {
	for _, event := range events {
		if h.evaluateActionEvent(criteria, event) {
			return true
		}
	}
	return false

}

func (h *AvailableEventHandler) evaluateActionEvent(criteria appsv1.ActionCriteria, event proto.ProbeEvent) bool {
	if criteria.Succeed != nil && *criteria.Succeed != (event.Code == 0) {
		return false
	}
	if criteria.Stdout != nil {
		if criteria.Stdout.EqualTo != nil && !bytes.Equal(event.Output, []byte(*criteria.Stdout.EqualTo)) {
			return false
		}
		if criteria.Stdout.Contains != nil && !bytes.Contains(event.Output, []byte(*criteria.Stdout.Contains)) {
			return false
		}
	}
	if criteria.Stderr != nil {
		if criteria.Stderr.EqualTo != nil && event.Message != *criteria.Stderr.EqualTo {
			return false
		}
		if criteria.Stderr.Contains != nil && !strings.Contains(event.Message, *criteria.Stderr.Contains) {
			return false
		}
	}
	return true
}
