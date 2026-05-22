/*
Copyright (C) 2022-2026 ApeCloud Co., Ltd

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

package instanceset

import (
	"encoding/json"
	"errors"
	"fmt"
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
	// roleChangedAnnotKey is used to mark the role change event has been handled.
	roleChangedAnnotKey = "role.kubeblocks.io/event-handled"
)

func (h *PodRoleEventHandler) Handle(cli client.Client, reqCtx intctrlutil.RequestCtx, _ record.EventRecorder, event *corev1.Event) error {
	if !isKBAgentRoleProbeEvent(event) {
		return nil
	}
	var (
		err         error
		annotations = event.GetAnnotations()
	)
	// filter role changed event that has been handled
	count := fmt.Sprintf("count-%d", event.Count)
	if annotations != nil && annotations[roleChangedAnnotKey] == count {
		return nil
	}

	if _, err = handleRoleChangedEvent(cli, reqCtx, event); err != nil {
		return err
	}

	// event order is crucial in role probing, but it's not guaranteed when controller restarted, so we have to mark them to be filtered
	patch := client.MergeFrom(event.DeepCopy())
	if event.Annotations == nil {
		event.Annotations = make(map[string]string, 0)
	}
	event.Annotations[roleChangedAnnotKey] = count
	return cli.Patch(reqCtx.Ctx, event, patch)
}

func isKBAgentRoleProbeEvent(event *corev1.Event) bool {
	return event.ReportingController == proto.ProbeEventReportingController &&
		event.Reason == "roleProbe" &&
		event.InvolvedObject.FieldPath == proto.ProbeEventFieldPath
}

// handleRoleChangedEvent handles role changed event and return role.
func handleRoleChangedEvent(cli client.Client, reqCtx intctrlutil.RequestCtx, event *corev1.Event) (string, error) {
	probeEvent := &proto.ProbeEvent{}
	if err := json.Unmarshal([]byte(event.Message), probeEvent); err != nil {
		reqCtx.Log.Error(err, "unmarshal role probe event failed")
		return "", nil
	}

	if probeEvent.Code != 0 {
		reqCtx.Log.Info("role probe failed", "message", probeEvent.Message)
		return "", nil
	}
	role := strings.ToLower(strings.TrimSpace(string(probeEvent.Output)))
	version := roleEventVersion(event)

	podName := types.NamespacedName{
		Namespace: event.InvolvedObject.Namespace,
		Name:      event.InvolvedObject.Name,
	}
	pod := &corev1.Pod{}
	if err := cli.Get(reqCtx.Ctx, podName, pod); err != nil {
		return role, err
	}
	// event belongs to old pod with the same name, ignore it
	if pod.Name == event.InvolvedObject.Name && string(pod.UID) != string(event.InvolvedObject.UID) {
		return role, nil
	}

	if checkStaleLastRoleEventVersion(version, pod) {
		reqCtx.Log.Info("stale role event received, ignore it", "version", version, "pod", pod.Name)
		return role, nil
	}

	var name string
	if pod.Labels != nil {
		if n, ok := pod.Labels[WorkloadsInstanceLabelKey]; ok {
			name = n
		}
	}
	its := &workloads.InstanceSet{}
	if err := cli.Get(reqCtx.Ctx, types.NamespacedName{Namespace: pod.Namespace, Name: name}, its); err != nil {
		return "", err
	}
	reqCtx.Log.Info("handle role change event", "pod", pod.Name, "role", role)

	if err := updatePodRoleLabel(cli, reqCtx, *its, pod, role, version); err != nil {
		return "", err
	}
	return role, nil
}

// compare the version of the current role event with the last version recorded in the pod annotation,
// stale role event will be ignored.
func checkStaleLastRoleEventVersion(version string, pod *corev1.Pod) bool {
	lastRoleEventVersion, ok := pod.Annotations[constant.LastRoleEventVersionAnnotationKey]
	if ok {
		if version <= lastRoleEventVersion && !strings.Contains(lastRoleEventVersion, ":") {
			return true
		}
	}
	return false
}

func roleEventVersion(event *corev1.Event) string {
	return fmt.Sprintf("%d", event.EventTime.UnixMicro())
}

func updatePodRoleLabel(cli client.Client, reqCtx intctrlutil.RequestCtx, its workloads.InstanceSet,
	pod *corev1.Pod, roleName string, version string) error {
	var (
		ctx                = reqCtx.Ctx
		roleMap            = composeRoleMap(its)
		normalizedRoleName = strings.ToLower(roleName)
		role, defined      = roleMap[normalizedRoleName]
	)
	// update pod role label
	newPod := pod.DeepCopy()
	if defined {
		newPod.Labels[RoleLabelKey] = normalizedRoleName
	} else {
		delete(newPod.Labels, RoleLabelKey)
	}
	if newPod.Annotations == nil {
		newPod.Annotations = map[string]string{}
	}
	newPod.Annotations[constant.LastRoleEventVersionAnnotationKey] = version
	if err := cli.Update(ctx, newPod); err != nil {
		return err
	}

	if role.IsExclusive {
		return removeExclusiveRoleLabels(cli, reqCtx, its, pod.Name, normalizedRoleName, version)
	}
	return nil
}

func removeExclusiveRoleLabels(cli client.Client, reqCtx intctrlutil.RequestCtx, its workloads.InstanceSet, newPodName, roleName, version string) error {
	labels := getMatchLabels(its.Name)
	labels[RoleLabelKey] = roleName
	var pods corev1.PodList
	if err := cli.List(reqCtx.Ctx, &pods, client.InNamespace(its.Namespace), client.MatchingLabels(labels)); err != nil {
		return err
	}

	var errs []error
	for i, pod := range pods.Items {
		if pod.Name == newPodName {
			continue
		}
		if checkStaleLastRoleEventVersion(version, &pod) {
			reqCtx.Log.Info("stale remove exclusive role label event, ignore it", "role event version", version, "pod", pod.Name)
			continue
		}

		newPod := pods.Items[i].DeepCopy()
		delete(newPod.Labels, RoleLabelKey)
		if newPod.Annotations == nil {
			newPod.Annotations = map[string]string{}
		}
		newPod.Annotations[constant.LastRoleEventVersionAnnotationKey] = version
		if err := cli.Update(reqCtx.Ctx, newPod); err != nil {
			errs = append(errs, err)
		} else {
			reqCtx.Log.Info("remove exclusive role label", "pod", newPod.Name, "role", roleName)
		}
	}
	return errors.Join(errs...)
}
