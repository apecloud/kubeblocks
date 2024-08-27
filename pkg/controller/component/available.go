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
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	"github.com/apecloud/kubeblocks/pkg/kbagent/proto"
)

const (
	availableProbe = "availableProbe"
)

type AvailableProbeEventHandler struct{}

func (h *AvailableProbeEventHandler) Handle(cli client.Client, reqCtx intctrlutil.RequestCtx, recorder record.EventRecorder, event *corev1.Event) error {
	if event.ReportingController != proto.ProbeEventReportingController ||
		event.Reason != availableProbe ||
		event.InvolvedObject.FieldPath != proto.ProbeEventFieldPath {
		return nil
	}

	probeEvent := &proto.ProbeEvent{}
	if err := json.Unmarshal([]byte(event.Message), probeEvent); err != nil {
		return err
	}

	// TODO: event about uncertainly results

	podName := types.NamespacedName{
		Namespace: event.InvolvedObject.Namespace,
		Name:      event.InvolvedObject.Name,
	}
	pod := &corev1.Pod{}
	if err := cli.Get(reqCtx.Ctx, podName, pod, inDataContextUnspecified()); err != nil {
		return err
	}
	if pod.UID != event.InvolvedObject.UID {
		return nil // ignore it
	}

	if pod.Labels == nil {
		return fmt.Errorf("pod %s/%s has no labels", pod.Namespace, pod.Name)
	}
	clusterName := pod.Labels[constant.AppInstanceLabelKey]
	compName := pod.Labels[constant.KBAppComponentLabelKey]
	if len(clusterName) == 0 || len(compName) == 0 {
		return fmt.Errorf("pod %s/%s has no cluster or component labels", pod.Namespace, pod.Name)
	}

	compKey := types.NamespacedName{
		Namespace: event.InvolvedObject.Namespace,
		Name:      constant.GenerateClusterComponentName(clusterName, compName),
	}
	comp := &appsv1alpha1.Component{}
	if err := cli.Get(reqCtx.Ctx, compKey, comp); err != nil {
		return err
	}

	if probeEvent.Code == 0 {
		return h.available(reqCtx.Ctx, cli, recorder, comp)
	}
	return h.unavailable(reqCtx.Ctx, cli, recorder, comp, probeEvent.Message)
}

func (h *AvailableProbeEventHandler) available(ctx context.Context, cli client.Client,
	recorder record.EventRecorder, comp *appsv1alpha1.Component) error {
	return h.status(ctx, cli, recorder, comp, metav1.ConditionTrue, "Available", "Component is available")
}

func (h *AvailableProbeEventHandler) unavailable(ctx context.Context, cli client.Client,
	recorder record.EventRecorder, comp *appsv1alpha1.Component, message string) error {
	return h.status(ctx, cli, recorder, comp, metav1.ConditionFalse, "Unavailable", message)
}

func (h *AvailableProbeEventHandler) status(ctx context.Context, cli client.Client, recorder record.EventRecorder,
	comp *appsv1alpha1.Component, status metav1.ConditionStatus, reason, message string) error {
	var (
		compCopy = comp.DeepCopy()
		cond     *metav1.Condition
	)
	for i, c := range comp.Status.Conditions {
		if c.Type == appsv1alpha1.ConditionTypeAvailable {
			cond = &comp.Status.Conditions[i]
			break
		}
	}
	if cond == nil {
		comp.Status.Conditions = append(comp.Status.Conditions, metav1.Condition{
			Type: appsv1alpha1.ConditionTypeAvailable,
		})
		cond = &comp.Status.Conditions[len(comp.Status.Conditions)-1]
	}

	if cond.Status == status {
		return nil
	}

	cond.Status = status
	cond.ObservedGeneration = comp.Generation
	cond.LastTransitionTime = metav1.Now()
	cond.Reason = reason
	cond.Message = message

	recorder.Event(comp, corev1.EventTypeNormal, reason, message)

	return cli.Status().Patch(ctx, compCopy, client.MergeFrom(comp))
}
