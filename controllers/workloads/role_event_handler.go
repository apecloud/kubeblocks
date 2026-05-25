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

package workloads

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"

	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/instanceset"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	"github.com/apecloud/kubeblocks/pkg/kbagent/proto"
)

type roleEventBranch string

const (
	roleEventBranchUnknown     roleEventBranch = "unknown"
	roleEventBranchInstanceSet roleEventBranch = "instanceset"
	roleEventBranchInstance    roleEventBranch = "instance"

	instanceKind = "Instance"
)

type roleEventResult struct {
	Event          types.NamespacedName
	EventUID       types.UID
	EventPodUID    types.UID
	Pod            types.NamespacedName
	PodUID         types.UID
	Role           string
	Version        string
	Branch         roleEventBranch
	Result         string
	Reason         string
	WorkloadName   string
	PreviousRole   string
	RoleDefined    bool
	Handled        bool
	ExclusiveClean bool
}

type RoleEventHandler struct{}

func (h *RoleEventHandler) Handle(cli client.Client, reqCtx intctrlutil.RequestCtx, _ record.EventRecorder, event *corev1.Event) (bool, error) {
	if !isRoleProbeEvent(event) {
		return false, nil
	}

	result := &roleEventResult{
		Event:       client.ObjectKeyFromObject(event),
		EventUID:    event.UID,
		EventPodUID: event.InvolvedObject.UID,
		Pod: types.NamespacedName{
			Namespace: event.InvolvedObject.Namespace,
			Name:      event.InvolvedObject.Name,
		},
		Version: roleEventVersion(event),
		Branch:  roleEventBranchUnknown,
		Result:  "ignored",
		Reason:  "notHandled",
	}
	handled, err := h.handleRoleProbeEvent(reqCtx.Ctx, cli, event, result)
	result.Handled = handled
	logRoleProbeEvent(reqCtx.Log, result, err)
	return handled, err
}

func (h *RoleEventHandler) handleRoleProbeEvent(ctx context.Context, cli client.Client, event *corev1.Event, result *roleEventResult) (bool, error) {
	probeEvent := &proto.ProbeEvent{}
	if err := json.Unmarshal([]byte(event.Message), probeEvent); err != nil {
		result.Result = "skipped"
		result.Reason = "invalidProbeEventMessage"
		return true, nil
	}

	result.Role = strings.ToLower(strings.TrimSpace(string(probeEvent.Output)))
	if probeEvent.Code != 0 {
		result.Result = "skipped"
		result.Reason = fmt.Sprintf("roleProbeFailed:%s", probeEvent.Message)
		return true, nil
	}

	pod := &corev1.Pod{}
	if err := cli.Get(ctx, result.Pod, pod); err != nil {
		if apierrors.IsNotFound(err) {
			result.Result = "skipped"
			result.Reason = "podNotFound"
			return true, nil
		}
		result.Result = "failed"
		result.Reason = "getPodError"
		return false, err
	}
	result.PodUID = pod.UID
	result.PreviousRole = pod.Labels[constant.RoleLabelKey]

	if string(pod.UID) != string(event.InvolvedObject.UID) {
		result.Result = "skipped"
		result.Reason = "stalePodUID"
		return true, nil
	}

	if branch, workloadName, ok := resolveRoleEventBranchByControllerRef(pod); ok {
		result.Branch = branch
		result.WorkloadName = workloadName
		switch branch {
		case roleEventBranchInstanceSet:
			return h.handleInstanceSetRoleProbe(ctx, cli, pod, workloadName, result)
		case roleEventBranchInstance:
			return h.handleInstanceRoleProbe(ctx, cli, pod, workloadName, result)
		}
	}

	itsName, instName := pod.Labels[instanceset.WorkloadsInstanceLabelKey], pod.Labels[constant.KBAppInstanceNameLabelKey]
	switch {
	case itsName != "" && instName != "":
		result.Result = "skipped"
		result.Reason = "ambiguousPodOwner"
		result.WorkloadName = fmt.Sprintf("%s,%s", itsName, instName)
		return true, nil
	case itsName != "":
		result.Branch = roleEventBranchInstanceSet
		result.WorkloadName = itsName
		return h.handleInstanceSetRoleProbe(ctx, cli, pod, itsName, result)
	case instName != "":
		result.Branch = roleEventBranchInstance
		result.WorkloadName = instName
		return h.handleInstanceRoleProbe(ctx, cli, pod, instName, result)
	default:
		result.Result = "ignored"
		result.Reason = "unknownPodOwner"
		return false, nil
	}
}

func (h *RoleEventHandler) handleInstanceSetRoleProbe(ctx context.Context, cli client.Client, pod *corev1.Pod, itsName string, result *roleEventResult) (bool, error) {
	if checkStaleLastRoleEventVersion(result.Version, pod) {
		result.Result = "skipped"
		result.Reason = "staleRoleEventVersion"
		return true, nil
	}

	its := &workloads.InstanceSet{}
	if err := cli.Get(ctx, types.NamespacedName{Namespace: pod.Namespace, Name: itsName}, its); err != nil {
		if apierrors.IsNotFound(err) {
			result.Result = "skipped"
			result.Reason = "instanceSetNotFound"
			return true, nil
		}
		result.Result = "failed"
		result.Reason = "getInstanceSetError"
		return false, err
	}

	roleMap := composeRoleMap(its.Spec.Roles)
	role, defined := roleMap[result.Role]
	result.RoleDefined = defined
	if err := updatePodRoleLabel(ctx, cli, pod, result.Role, result.Version, defined); err != nil {
		result.Result = "failed"
		result.Reason = "updatePodRoleLabelError"
		return false, err
	}

	if defined && role.IsExclusive {
		result.ExclusiveClean = true
		if err := removeExclusiveRoleLabels(ctx, cli, *its, pod.Name, result.Role, result.Version); err != nil {
			result.Result = "failed"
			result.Reason = "removeExclusiveRoleLabelsError"
			return false, err
		}
	}
	result.Result = "handled"
	result.Reason = "updated"
	return true, nil
}

func (h *RoleEventHandler) handleInstanceRoleProbe(ctx context.Context, cli client.Client, pod *corev1.Pod, instName string, result *roleEventResult) (bool, error) {
	if checkStaleLastRoleEventVersion(result.Version, pod) {
		result.Result = "skipped"
		result.Reason = "staleRoleEventVersion"
		return true, nil
	}

	inst := &workloads.Instance{}
	if err := cli.Get(ctx, types.NamespacedName{Namespace: pod.Namespace, Name: instName}, inst); err != nil {
		if apierrors.IsNotFound(err) {
			result.Result = "skipped"
			result.Reason = "instanceNotFound"
			return true, nil
		}
		result.Result = "failed"
		result.Reason = "getInstanceError"
		return false, err
	}

	_, defined := composeRoleMap(inst.Spec.Roles)[result.Role]
	result.RoleDefined = defined
	if err := updatePodRoleLabel(ctx, cli, pod, result.Role, result.Version, defined); err != nil {
		result.Result = "failed"
		result.Reason = "updatePodRoleLabelError"
		return false, err
	}
	result.Result = "handled"
	result.Reason = "updated"
	return true, nil
}

func isRoleProbeEvent(event *corev1.Event) bool {
	return event.ReportingController == proto.ProbeEventReportingController &&
		event.Reason == "roleProbe" &&
		event.InvolvedObject.FieldPath == proto.ProbeEventFieldPath
}

func roleEventVersion(event *corev1.Event) string {
	return fmt.Sprintf("%d", event.EventTime.UnixMicro())
}

func checkStaleLastRoleEventVersion(version string, pod *corev1.Pod) bool {
	lastRoleEventVersion, ok := pod.Annotations[constant.LastRoleEventVersionAnnotationKey]
	if ok {
		if version <= lastRoleEventVersion && !strings.Contains(lastRoleEventVersion, ":") {
			return true
		}
	}
	return false
}

func composeRoleMap(roles []workloads.ReplicaRole) map[string]workloads.ReplicaRole {
	roleMap := make(map[string]workloads.ReplicaRole)
	for _, role := range roles {
		roleMap[strings.ToLower(role.Name)] = role
	}
	return roleMap
}

func resolveRoleEventBranchByControllerRef(pod *corev1.Pod) (roleEventBranch, string, bool) {
	ownerRef := metav1.GetControllerOf(pod)
	if ownerRef == nil {
		return roleEventBranchUnknown, "", false
	}
	groupVersion, err := schema.ParseGroupVersion(ownerRef.APIVersion)
	if err != nil || groupVersion.Group != workloads.GroupVersion.Group {
		return roleEventBranchUnknown, "", false
	}
	switch ownerRef.Kind {
	case workloads.InstanceSetKind:
		return roleEventBranchInstanceSet, ownerRef.Name, true
	case instanceKind:
		return roleEventBranchInstance, ownerRef.Name, true
	default:
		return roleEventBranchUnknown, "", false
	}
}

func updatePodRoleLabel(ctx context.Context, cli client.Client, pod *corev1.Pod, roleName, version string, roleDefined bool) error {
	newPod := pod.DeepCopy()
	if newPod.Labels == nil {
		newPod.Labels = make(map[string]string)
	}
	if roleDefined {
		newPod.Labels[constant.RoleLabelKey] = roleName
	} else {
		delete(newPod.Labels, constant.RoleLabelKey)
	}
	if newPod.Annotations == nil {
		newPod.Annotations = map[string]string{}
	}
	newPod.Annotations[constant.LastRoleEventVersionAnnotationKey] = version
	if reflect.DeepEqual(newPod.Labels, pod.Labels) && reflect.DeepEqual(newPod.Annotations, pod.Annotations) {
		return nil
	}
	return cli.Update(ctx, newPod)
}

func removeExclusiveRoleLabels(ctx context.Context, cli client.Client, its workloads.InstanceSet, newPodName, roleName, version string) error {
	labels := map[string]string{
		constant.AppManagedByLabelKey:          constant.AppName,
		instanceset.WorkloadsManagedByLabelKey: workloads.InstanceSetKind,
		instanceset.WorkloadsInstanceLabelKey:  its.Name,
		constant.RoleLabelKey:                  roleName,
	}
	var pods corev1.PodList
	if err := cli.List(ctx, &pods, client.InNamespace(its.Namespace), client.MatchingLabels(labels)); err != nil {
		return err
	}

	var errs []error
	for i, pod := range pods.Items {
		if pod.Name == newPodName {
			continue
		}
		if checkStaleLastRoleEventVersion(version, &pod) {
			continue
		}

		newPod := pods.Items[i].DeepCopy()
		delete(newPod.Labels, constant.RoleLabelKey)
		if newPod.Annotations == nil {
			newPod.Annotations = map[string]string{}
		}
		newPod.Annotations[constant.LastRoleEventVersionAnnotationKey] = version
		if err := cli.Update(ctx, newPod); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func logRoleProbeEvent(logger logr.Logger, result *roleEventResult, err error) {
	values := []any{
		"eventName", result.Event.String(),
		"eventUID", result.EventUID,
		"eventPodUID", result.EventPodUID,
		"podName", result.Pod.String(),
		"podUID", result.PodUID,
		"role", result.Role,
		"eventVersion", result.Version,
		"branch", result.Branch,
		"result", result.Result,
		"reason", result.Reason,
		"workload", result.WorkloadName,
		"previousRole", result.PreviousRole,
		"roleDefined", result.RoleDefined,
		"handled", result.Handled,
		"exclusiveClean", result.ExclusiveClean,
	}
	if err != nil {
		logger.Error(err, "role probe event processed", values...)
		return
	}
	logger.Info("role probe event processed", values...)
}
