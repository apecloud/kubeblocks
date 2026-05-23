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
	"time"

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

var roleEventTraceStart = time.Now()

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

func logRoleEventTrace(reqCtx intctrlutil.RequestCtx, msg string, keysAndValues ...any) {
	now := time.Now()
	fields := []any{
		"traceTime", now.Format(time.RFC3339Nano),
		"traceUnixNano", now.UnixNano(),
		"traceMonotonicNano", time.Since(roleEventTraceStart).Nanoseconds(),
	}
	fields = append(fields, keysAndValues...)
	reqCtx.Log.V(2).Info(msg, fields...)
}

// handleRoleChangedEvent handles role changed event and return role.
func handleRoleChangedEvent(cli client.Client, reqCtx intctrlutil.RequestCtx, event *corev1.Event) (string, error) {
	logRoleEventTrace(reqCtx, "role event trace received",
		"eventName", event.Name,
		"eventNamespace", event.Namespace,
		"eventUID", event.UID,
		"eventResourceVersion", event.ResourceVersion,
		"eventCount", event.Count,
		"eventTime", event.EventTime.Time.Format(time.RFC3339Nano),
		"eventTimeUnixMicro", event.EventTime.UnixMicro(),
		"reportingController", event.ReportingController,
		"sourceComponent", event.Source.Component,
		"sourceHost", event.Source.Host,
		"involvedNamespace", event.InvolvedObject.Namespace,
		"involvedPodName", event.InvolvedObject.Name,
		"involvedPodUID", event.InvolvedObject.UID,
		"message", event.Message)

	probeEvent := &proto.ProbeEvent{}
	if err := json.Unmarshal([]byte(event.Message), probeEvent); err != nil {
		logRoleEventTrace(reqCtx, "role event trace parse failed",
			"eventName", event.Name,
			"podName", event.InvolvedObject.Name,
			"decision", "reject_unmarshal_error",
			"error", err.Error())
		reqCtx.Log.Error(err, "unmarshal role probe event failed")
		return "", nil
	}

	if probeEvent.Code != 0 {
		logRoleEventTrace(reqCtx, "role event trace probe failed",
			"eventName", event.Name,
			"podName", event.InvolvedObject.Name,
			"probeCode", probeEvent.Code,
			"probeMessage", probeEvent.Message,
			"probeOutput", string(probeEvent.Output),
			"decision", "reject_probe_failed")
		reqCtx.Log.Info("role probe failed", "message", probeEvent.Message)
		return "", nil
	}
	role := strings.ToLower(strings.TrimSpace(string(probeEvent.Output)))
	version := roleEventVersion(event)
	logRoleEventTrace(reqCtx, "role event trace parsed",
		"eventName", event.Name,
		"podName", event.InvolvedObject.Name,
		"role", role,
		"version", version,
		"probeCode", probeEvent.Code,
		"probeMessage", probeEvent.Message,
		"probeOutput", string(probeEvent.Output))

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
		logRoleEventTrace(reqCtx, "role event trace uid mismatch",
			"eventName", event.Name,
			"podName", pod.Name,
			"role", role,
			"version", version,
			"podUID", pod.UID,
			"eventPodUID", event.InvolvedObject.UID,
			"decision", "reject_uid_mismatch")
		return role, nil
	}

	stale, staleReason, lastRoleEventVersion := staleLastRoleEventVersionDecision(version, pod)
	if stale {
		logRoleEventTrace(reqCtx, "role event trace stale",
			"eventName", event.Name,
			"podName", pod.Name,
			"role", role,
			"version", version,
			"lastRoleEventVersion", lastRoleEventVersion,
			"staleReason", staleReason,
			"decision", "reject_stale")
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
	logRoleEventTrace(reqCtx, "role event trace accepted",
		"eventName", event.Name,
		"podName", pod.Name,
		"role", role,
		"version", version,
		"lastRoleEventVersion", lastRoleEventVersion,
		"staleReason", staleReason,
		"decision", "accept")

	if err := updatePodRoleLabel(cli, reqCtx, *its, pod, role, version); err != nil {
		return "", err
	}
	return role, nil
}

// compare the version of the current role event with the last version recorded in the pod annotation,
// stale role event will be ignored.
func checkStaleLastRoleEventVersion(version string, pod *corev1.Pod) bool {
	stale, _, _ := staleLastRoleEventVersionDecision(version, pod)
	return stale
}

func staleLastRoleEventVersionDecision(version string, pod *corev1.Pod) (bool, string, string) {
	lastRoleEventVersion, ok := pod.Annotations[constant.LastRoleEventVersionAnnotationKey]
	if !ok {
		return false, "no_last_role_event_version", ""
	}
	if version <= lastRoleEventVersion && !strings.Contains(lastRoleEventVersion, ":") {
		return true, "event_version_not_newer_than_last_plain_version", lastRoleEventVersion
	}
	return false, "event_version_accepted_by_current_stale_check", lastRoleEventVersion
}

func roleEventVersion(event *corev1.Event) string {
	return fmt.Sprintf("%d", event.EventTime.UnixMicro())
}

func podRoleLabel(pod *corev1.Pod) string {
	if pod.Labels == nil {
		return ""
	}
	return pod.Labels[RoleLabelKey]
}

func podLastRoleEventVersion(pod *corev1.Pod) string {
	if pod.Annotations == nil {
		return ""
	}
	return pod.Annotations[constant.LastRoleEventVersionAnnotationKey]
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
	oldRoleLabel := podRoleLabel(pod)
	oldRoleEventVersion := podLastRoleEventVersion(pod)
	newRoleLabel := ""
	if defined {
		newPod.Labels[RoleLabelKey] = normalizedRoleName
		newRoleLabel = normalizedRoleName
	} else {
		delete(newPod.Labels, RoleLabelKey)
	}
	if newPod.Annotations == nil {
		newPod.Annotations = map[string]string{}
	}
	newPod.Annotations[constant.LastRoleEventVersionAnnotationKey] = version
	patchRequired := oldRoleLabel != newRoleLabel || oldRoleEventVersion != version
	logRoleEventTrace(reqCtx, "role event trace update pod role label begin",
		"podName", pod.Name,
		"role", normalizedRoleName,
		"roleDefined", defined,
		"roleExclusive", role.IsExclusive,
		"oldRoleLabel", oldRoleLabel,
		"newRoleLabel", newRoleLabel,
		"oldRoleEventVersion", oldRoleEventVersion,
		"newRoleEventVersion", version,
		"patchRequired", patchRequired)
	if err := cli.Update(ctx, newPod); err != nil {
		logRoleEventTrace(reqCtx, "role event trace update pod role label failed",
			"podName", pod.Name,
			"role", normalizedRoleName,
			"oldRoleLabel", oldRoleLabel,
			"newRoleLabel", newRoleLabel,
			"oldRoleEventVersion", oldRoleEventVersion,
			"newRoleEventVersion", version,
			"patchRequired", patchRequired,
			"error", err.Error())
		return err
	}
	logRoleEventTrace(reqCtx, "role event trace update pod role label done",
		"podName", pod.Name,
		"role", normalizedRoleName,
		"oldRoleLabel", oldRoleLabel,
		"newRoleLabel", newRoleLabel,
		"oldRoleEventVersion", oldRoleEventVersion,
		"newRoleEventVersion", version,
		"patchRequired", patchRequired,
		"updateIssued", true)

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
	logRoleEventTrace(reqCtx, "role event trace remove exclusive role labels begin",
		"triggerPod", newPodName,
		"role", roleName,
		"version", version,
		"matchedPods", len(pods.Items),
		"reason", "exclusive_role_assigned_to_trigger_pod")

	var errs []error
	for i, pod := range pods.Items {
		if pod.Name == newPodName {
			continue
		}
		stale, staleReason, lastRoleEventVersion := staleLastRoleEventVersionDecision(version, &pod)
		if stale {
			logRoleEventTrace(reqCtx, "role event trace remove exclusive role label stale",
				"triggerPod", newPodName,
				"affectedPod", pod.Name,
				"role", roleName,
				"version", version,
				"lastRoleEventVersion", lastRoleEventVersion,
				"staleReason", staleReason,
				"decision", "skip_stale")
			reqCtx.Log.Info("stale remove exclusive role label event, ignore it", "role event version", version, "pod", pod.Name)
			continue
		}

		newPod := pods.Items[i].DeepCopy()
		oldRoleLabel := podRoleLabel(&pod)
		oldRoleEventVersion := podLastRoleEventVersion(&pod)
		delete(newPod.Labels, RoleLabelKey)
		if newPod.Annotations == nil {
			newPod.Annotations = map[string]string{}
		}
		newPod.Annotations[constant.LastRoleEventVersionAnnotationKey] = version
		logRoleEventTrace(reqCtx, "role event trace remove exclusive role label update begin",
			"triggerPod", newPodName,
			"affectedPod", pod.Name,
			"role", roleName,
			"oldRoleLabel", oldRoleLabel,
			"newRoleLabel", "",
			"oldRoleEventVersion", oldRoleEventVersion,
			"newRoleEventVersion", version,
			"reason", "exclusive_role_assigned_to_trigger_pod")
		if err := cli.Update(reqCtx.Ctx, newPod); err != nil {
			logRoleEventTrace(reqCtx, "role event trace remove exclusive role label update failed",
				"triggerPod", newPodName,
				"affectedPod", pod.Name,
				"role", roleName,
				"oldRoleLabel", oldRoleLabel,
				"newRoleLabel", "",
				"oldRoleEventVersion", oldRoleEventVersion,
				"newRoleEventVersion", version,
				"error", err.Error())
			errs = append(errs, err)
		} else {
			logRoleEventTrace(reqCtx, "role event trace remove exclusive role label update done",
				"triggerPod", newPodName,
				"affectedPod", newPod.Name,
				"role", roleName,
				"oldRoleLabel", oldRoleLabel,
				"newRoleLabel", "",
				"oldRoleEventVersion", oldRoleEventVersion,
				"newRoleEventVersion", version)
			reqCtx.Log.Info("remove exclusive role label", "pod", newPod.Name, "role", roleName)
		}
	}
	return errors.Join(errs...)
}
