/*
Copyright (C) 2022-2025 ApeCloud Co., Ltd

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

package instance

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"

	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	"github.com/apecloud/kubeblocks/pkg/kbagent/proto"
)

type PodRoleEventHandler struct{}

const (
	roleChangedAnnotKey = "role.kubeblocks.io/event-handled"
)

func (h *PodRoleEventHandler) Handle(cli client.Client, reqCtx intctrlutil.RequestCtx, recorder record.EventRecorder, event *corev1.Event) error {
	if !h.isRoleProbeEvent(event) {
		return nil
	}

	// skip role changed event that has been handled
	count := fmt.Sprintf("count-%d", event.Count)
	if event.GetAnnotations() != nil && event.GetAnnotations()[roleChangedAnnotKey] == count {
		return nil
	}

	if err := h.handleRoleChangedEvent(cli, reqCtx, recorder, event); err != nil {
		return err
	}

	// event order is crucial in role probing, but it's not guaranteed when controller restarted, so we have to mark them to be filtered
	patch := client.MergeFrom(event.DeepCopy())
	if event.Annotations == nil {
		event.Annotations = make(map[string]string)
	}
	event.Annotations[roleChangedAnnotKey] = count
	return cli.Patch(reqCtx.Ctx, event, patch)
}

func (h *PodRoleEventHandler) isRoleProbeEvent(event *corev1.Event) bool {
	return event.InvolvedObject.FieldPath == proto.ProbeEventFieldPath &&
		event.ReportingController == proto.ProbeEventReportingController && event.Reason == "roleProbe"
}

func (h *PodRoleEventHandler) handleRoleChangedEvent(cli client.Client, reqCtx intctrlutil.RequestCtx, _ record.EventRecorder, event *corev1.Event) error {
	probeEvent := &proto.ProbeEvent{}
	if err := json.Unmarshal([]byte(event.Message), probeEvent); err != nil {
		reqCtx.Log.Error(err, "unmarshal role probe event failed")
		return nil
	}

	if probeEvent.Code != 0 {
		reqCtx.Log.Info("role probe failed", "message", probeEvent.Message)
		return nil
	}

	pod := &corev1.Pod{}
	podKey := types.NamespacedName{
		Namespace: event.InvolvedObject.Namespace,
		Name:      event.InvolvedObject.Name,
	}
	if err := cli.Get(reqCtx.Ctx, podKey, pod); err != nil {
		return err
	}
	// event belongs to the old pod with the same name, ignore it
	if strings.Compare(string(pod.UID), string(event.InvolvedObject.UID)) != 0 {
		reqCtx.Log.Info("stale role probe event received, ignore it")
		return nil
	}

	snapshotVersion := strconv.FormatInt(event.EventTime.UnixMicro(), 10)
	lastSnapshotVersion, ok := pod.Annotations[constant.LastRoleSnapshotVersionAnnotationKey]
	if ok {
		if snapshotVersion <= lastSnapshotVersion && !strings.Contains(lastSnapshotVersion, ":") {
			reqCtx.Log.Info("stale role probe event received, ignore it")
			return nil
		}
	}

	var instName string
	if pod.Labels != nil {
		if n, ok := pod.Labels[constant.KBAppInstanceNameLabelKey]; ok {
			instName = n
		}
	}
	inst := &workloads.Instance{}
	if err := cli.Get(reqCtx.Ctx, types.NamespacedName{Namespace: pod.Namespace, Name: instName}, inst); err != nil {
		return err
	}

	role := strings.ToLower(string(probeEvent.Output))
	reqCtx.Log.Info("handle role change event", "pod", pod.Name, "role", role)
	return h.updatePodRoleLabel(cli, reqCtx, inst, pod, role, snapshotVersion)
}

func (h *PodRoleEventHandler) updatePodRoleLabel(cli client.Client, reqCtx intctrlutil.RequestCtx,
	inst *workloads.Instance, pod *corev1.Pod, roleName string, snapshotVersion string) error {
	var (
		newPod  = pod.DeepCopy()
		roleMap = composeRoleMap(inst)
	)

	role, ok := roleMap[roleName]
	switch ok {
	case true:
		newPod.Labels[constant.RoleLabelKey] = role.Name
	case false:
		delete(newPod.Labels, constant.RoleLabelKey)
	}

	if newPod.Annotations == nil {
		newPod.Annotations = map[string]string{}
	}
	newPod.Annotations[constant.LastRoleSnapshotVersionAnnotationKey] = snapshotVersion
	return cli.Update(reqCtx.Ctx, newPod)
}
