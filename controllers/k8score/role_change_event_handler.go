/*
Copyright (C) 2022-2023 ApeCloud Co., Ltd

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

package k8score

import (
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/controllers/apps/components"
	"github.com/apecloud/kubeblocks/internal/constant"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
	probeutil "github.com/apecloud/kubeblocks/internal/sqlchannel/util"
)

// RoleChangeEventHandler is the event handler for the role change event
type RoleChangeEventHandler struct{}

var _ EventHandler = &RoleChangeEventHandler{}

// Handle handles role changed event.
func (r *RoleChangeEventHandler) Handle(cli client.Client, reqCtx intctrlutil.RequestCtx, recorder record.EventRecorder, event *corev1.Event) error {
	if event.Reason != string(probeutil.CheckRoleOperation) {
		return nil
	}
	var (
		err         error
		annotations = event.GetAnnotations()
	)
	// filter role changed event that has been handled
	if annotations != nil && annotations[roleChangedAnnotKey] == trueStr {
		return nil
	}

	if _, err = handleRoleChangedEvent(cli, reqCtx, recorder, event); err != nil {
		return err
	}

	// event order is crucial in role probing, but it's not guaranteed when controller restarted, so we have to mark them to be filtered
	patch := client.MergeFrom(event.DeepCopy())
	if event.Annotations == nil {
		event.Annotations = make(map[string]string, 0)
	}
	event.Annotations[roleChangedAnnotKey] = trueStr
	return cli.Patch(reqCtx.Ctx, event, patch)
}

// handleRoleChangedEvent handles role changed event and return role.
func handleRoleChangedEvent(cli client.Client, reqCtx intctrlutil.RequestCtx, recorder record.EventRecorder, event *corev1.Event) (string, error) {
	// parse probe event message
	message := ParseProbeEventMessage(reqCtx, event)
	if message == nil {
		reqCtx.Log.Info("parse probe event message failed", "message", event.Message)
		return "", nil
	}

	// if probe event operation is not implemented, check role failed or invalid, ignore it
	if message.Event == ProbeEventOperationNotImpl || message.Event == ProbeEventCheckRoleFailed || message.Event == ProbeEventRoleInvalid {
		reqCtx.Log.Info("probe event failed", "message", message.Message)
		return "", nil
	}
	role := strings.ToLower(message.Role)

	podName := types.NamespacedName{
		Namespace: event.InvolvedObject.Namespace,
		Name:      event.InvolvedObject.Name,
	}
	// get pod
	pod := &corev1.Pod{}
	if err := cli.Get(reqCtx.Ctx, podName, pod); err != nil {
		return role, err
	}
	// event belongs to old pod with the same name, ignore it
	if pod.UID != event.InvolvedObject.UID {
		return role, nil
	}

	// compare the EventTime of the current event object with the lastTimestamp of the last recorded in the pod annotation,
	// if the current event's EventTime is earlier than the recorded lastTimestamp in the pod annotation,
	// it indicates that the current event has arrived out of order and is expired, so it should not be processed.
	lastTimestampStr, ok := pod.Annotations[constant.LastRoleChangedEventTimestampAnnotationKey]
	if ok {
		lastTimestamp, err := time.Parse(time.RFC3339Nano, lastTimestampStr)
		if err != nil {
			reqCtx.Log.Info("failed to parse last role changed event timestamp from pod annotation", "pod", pod.Name, "error", err.Error())
			return role, err
		}
		eventLastTS := event.EventTime.Time
		if !eventLastTS.After(lastTimestamp) {
			reqCtx.Log.Info("event's EventTime is earlier than the recorded lastTimestamp in the pod annotation, it should not be processed.", "event uid", event.UID, "pod", pod.Name, "role", role, "originalRole", message.OriginalRole, "event EventTime", event.EventTime.Time.String(), "annotation lastTimestamp", lastTimestampStr)
			return role, nil
		}
	}

	// get cluster obj of the pod
	cluster := &appsv1alpha1.Cluster{}
	if err := cli.Get(reqCtx.Ctx, types.NamespacedName{
		Namespace: pod.Namespace,
		Name:      pod.Labels[constant.AppInstanceLabelKey],
	}, cluster); err != nil {
		return role, err
	}
	reqCtx.Log.V(1).Info("handle role changed event", "event uid", event.UID, "cluster", cluster.Name, "pod", pod.Name, "role", role, "originalRole", message.OriginalRole)
	compName, componentDef, err := components.GetComponentInfoByPod(reqCtx.Ctx, cli, *cluster, pod)
	if err != nil {
		return role, err
	}
	switch componentDef.WorkloadType {
	case appsv1alpha1.Consensus:
		return role, components.UpdateConsensusSetRoleLabel(cli, reqCtx, event, componentDef, pod, role)
	case appsv1alpha1.Replication:
		return role, components.HandleReplicationSetRoleChangeEvent(cli, reqCtx, event, cluster, compName, pod, role)
	}
	return role, nil
}
