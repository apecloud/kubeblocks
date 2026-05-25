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
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/log"

	workloadsapi "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/instanceset"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	"github.com/apecloud/kubeblocks/pkg/kbagent/proto"
	viper "github.com/apecloud/kubeblocks/pkg/viperx"
)

const (
	eventHandledAnnotationKey = "kubeblocks.io/event-handled"
)

type roleEventBranch string

const (
	roleEventBranchUnknown     roleEventBranch = "unknown"
	roleEventBranchInstanceSet roleEventBranch = "instanceset"
	roleEventBranchInstance    roleEventBranch = "instance"
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

type RoleEventReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

// events API only allows ready-only, create, patch
// +kubebuilder:rbac:groups=core,resources=events,verbs=get;list;watch;create;patch

func (r *RoleEventReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx).WithValues("event", req.NamespacedName)

	event := &corev1.Event{}
	if err := r.Client.Get(ctx, req.NamespacedName, event); err != nil {
		return intctrlutil.CheckedRequeueWithError(err, logger, "getEventError")
	}

	if !isKBAgentRoleProbeEvent(event) {
		return intctrlutil.Reconciled()
	}

	result := &roleEventResult{
		Event:       req.NamespacedName,
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
	if r.isEventHandled(event) {
		result.Result = "skipped"
		result.Reason = "eventAlreadyHandled"
		result.Handled = true
		logRoleProbeEvent(logger, result, nil)
		return intctrlutil.Reconciled()
	}

	handled, err := r.handleRoleProbeEvent(ctx, event, result)
	if err == nil && handled {
		if markErr := r.markEventHandled(ctx, event); markErr != nil {
			result.Result = "failed"
			result.Reason = "markEventHandledError"
			err = markErr
		}
	}
	result.Handled = handled
	logRoleProbeEvent(logger, result, err)
	if err != nil {
		return intctrlutil.RequeueWithError(err, logger, "handleRoleProbeEventError")
	}
	return intctrlutil.Reconciled()
}

func (r *RoleEventReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return intctrlutil.NewControllerManagedBy(mgr).
		For(&corev1.Event{}).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: viper.GetInt(constant.CfgKBReconcileWorkers) / 4,
		}).
		Complete(r)
}

func (r *RoleEventReconciler) isEventHandled(event *corev1.Event) bool {
	count := fmt.Sprintf("%d", event.Count)
	annotations := event.GetAnnotations()
	if annotations != nil && annotations[eventHandledAnnotationKey] == count {
		return true
	}
	return false
}

func (r *RoleEventReconciler) markEventHandled(ctx context.Context, event *corev1.Event) error {
	patch := client.MergeFrom(event.DeepCopy())
	if event.Annotations == nil {
		event.Annotations = make(map[string]string, 0)
	}
	event.Annotations[eventHandledAnnotationKey] = fmt.Sprintf("%d", event.Count)
	return r.Client.Patch(ctx, event, patch)
}

func (r *RoleEventReconciler) handleRoleProbeEvent(ctx context.Context, event *corev1.Event, result *roleEventResult) (bool, error) {
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
	if err := r.Client.Get(ctx, result.Pod, pod); err != nil {
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

	itsName := pod.Labels[instanceset.WorkloadsInstanceLabelKey]
	instName := pod.Labels[constant.KBAppInstanceNameLabelKey]
	switch {
	case itsName != "" && instName != "":
		result.Result = "skipped"
		result.Reason = "ambiguousPodOwner"
		result.WorkloadName = fmt.Sprintf("%s,%s", itsName, instName)
		return true, nil
	case itsName != "":
		result.Branch = roleEventBranchInstanceSet
		result.WorkloadName = itsName
		return r.handleInstanceSetRoleProbe(ctx, pod, itsName, result)
	case instName != "":
		result.Branch = roleEventBranchInstance
		result.WorkloadName = instName
		return r.handleInstanceRoleProbe(ctx, pod, instName, result)
	default:
		result.Result = "ignored"
		result.Reason = "unknownPodOwner"
		return false, nil
	}
}

func (r *RoleEventReconciler) handleInstanceSetRoleProbe(ctx context.Context, pod *corev1.Pod, itsName string, result *roleEventResult) (bool, error) {
	if checkStaleLastRoleEventVersion(result.Version, pod) {
		result.Result = "skipped"
		result.Reason = "staleRoleEventVersion"
		return true, nil
	}

	its := &workloadsapi.InstanceSet{}
	if err := r.Client.Get(ctx, types.NamespacedName{Namespace: pod.Namespace, Name: itsName}, its); err != nil {
		if apierrors.IsNotFound(err) {
			result.Result = "skipped"
			result.Reason = "instanceSetNotFound"
			return true, nil
		}
		result.Result = "failed"
		result.Reason = "getInstanceSetError"
		return false, err
	}

	roleMap := composeInstanceSetRoleMap(*its)
	role, defined := roleMap[result.Role]
	result.RoleDefined = defined
	if err := updatePodRoleLabel(ctx, r.Client, pod, result.Role, result.Version, defined); err != nil {
		result.Result = "failed"
		result.Reason = "updatePodRoleLabelError"
		return false, err
	}

	if defined && role.IsExclusive {
		result.ExclusiveClean = true
		if err := removeExclusiveRoleLabels(ctx, r.Client, *its, pod.Name, result.Role, result.Version); err != nil {
			result.Result = "failed"
			result.Reason = "removeExclusiveRoleLabelsError"
			return false, err
		}
	}
	result.Result = "handled"
	result.Reason = "updated"
	return true, nil
}

func (r *RoleEventReconciler) handleInstanceRoleProbe(ctx context.Context, pod *corev1.Pod, instName string, result *roleEventResult) (bool, error) {
	if checkStaleLastRoleEventVersion(result.Version, pod) {
		result.Result = "skipped"
		result.Reason = "staleRoleEventVersion"
		return true, nil
	}

	inst := &workloadsapi.Instance{}
	if err := r.Client.Get(ctx, types.NamespacedName{Namespace: pod.Namespace, Name: instName}, inst); err != nil {
		if apierrors.IsNotFound(err) {
			result.Result = "skipped"
			result.Reason = "instanceNotFound"
			return true, nil
		}
		result.Result = "failed"
		result.Reason = "getInstanceError"
		return false, err
	}

	_, defined := composeInstanceRoleMap(*inst)[result.Role]
	result.RoleDefined = defined
	if err := updatePodRoleLabel(ctx, r.Client, pod, result.Role, result.Version, defined); err != nil {
		result.Result = "failed"
		result.Reason = "updatePodRoleLabelError"
		return false, err
	}
	result.Result = "handled"
	result.Reason = "updated"
	return true, nil
}

func isKBAgentRoleProbeEvent(event *corev1.Event) bool {
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

func composeInstanceSetRoleMap(its workloadsapi.InstanceSet) map[string]workloadsapi.ReplicaRole {
	roleMap := make(map[string]workloadsapi.ReplicaRole)
	for _, role := range its.Spec.Roles {
		roleMap[strings.ToLower(role.Name)] = role
	}
	return roleMap
}

func composeInstanceRoleMap(inst workloadsapi.Instance) map[string]workloadsapi.ReplicaRole {
	roleMap := make(map[string]workloadsapi.ReplicaRole)
	for _, role := range inst.Spec.Roles {
		roleMap[strings.ToLower(role.Name)] = role
	}
	return roleMap
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

func removeExclusiveRoleLabels(ctx context.Context, cli client.Client, its workloadsapi.InstanceSet, newPodName, roleName, version string) error {
	labels := map[string]string{
		constant.AppManagedByLabelKey:          constant.AppName,
		instanceset.WorkloadsManagedByLabelKey: workloadsapi.InstanceSetKind,
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
