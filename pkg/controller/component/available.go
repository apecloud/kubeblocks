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
	"context"
	"encoding/json"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	"github.com/apecloud/kubeblocks/pkg/kbagent/proto"
)

const (
	availableProbe = "availableProbe"
)

type AvailableProbeEventHandler struct{}

func (h *AvailableProbeEventHandler) Handle(cli client.Client, reqCtx intctrlutil.RequestCtx, recorder record.EventRecorder, event *corev1.Event) error {
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

	if probeEvent.Code == 0 {
		return h.available(reqCtx.Ctx, cli, recorder, comp)
	}
	return h.unavailable(reqCtx.Ctx, cli, recorder, comp, probeEvent.Message)
}

func (h *AvailableProbeEventHandler) isAvailableEvent(event *corev1.Event) bool {
	return event.ReportingController == proto.ProbeEventReportingController &&
		event.Reason == availableProbe && event.InvolvedObject.FieldPath == proto.ProbeEventFieldPath
}

func (h *AvailableProbeEventHandler) available(ctx context.Context, cli client.Client,
	recorder record.EventRecorder, comp *appsv1.Component) error {
	return h.status(ctx, cli, recorder, comp, metav1.ConditionTrue, "Available", "Component is available")
}

func (h *AvailableProbeEventHandler) unavailable(ctx context.Context, cli client.Client,
	recorder record.EventRecorder, comp *appsv1.Component, message string) error {
	return h.status(ctx, cli, recorder, comp, metav1.ConditionFalse, "Unavailable", message)
}

func (h *AvailableProbeEventHandler) status(ctx context.Context, cli client.Client, recorder record.EventRecorder,
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

	var (
		compCopy = comp.DeepCopy()
		cond     *metav1.Condition
	)
	for i, c := range comp.Status.Conditions {
		if c.Type == appsv1.ConditionTypeAvailable {
			cond = &comp.Status.Conditions[i]
			break
		}
	}
	if cond == nil {
		comp.Status.Conditions = append(comp.Status.Conditions, newCond)
		cond = &comp.Status.Conditions[len(comp.Status.Conditions)-1]
	}

	if h.condEqual(*cond, newCond) {
		return nil
	}

	recorder.Event(comp, corev1.EventTypeNormal, reason, message)

	return cli.Status().Patch(ctx, compCopy, client.MergeFrom(comp))
}

func (h *AvailableProbeEventHandler) condEqual(cond1, cond2 metav1.Condition) bool {
	return cond1.Type == cond2.Type && cond1.Status == cond2.Status &&
		cond1.ObservedGeneration == cond2.ObservedGeneration && cond1.Reason == cond2.Reason
}
