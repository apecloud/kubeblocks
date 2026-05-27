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
	"strconv"
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

// roleProbeOutput is the parsed view of a kbagent roleProbe stdout payload.
type roleProbeOutput struct {
	role                    string
	authoritativeVersion    uint64
	hasAuthoritativeVersion bool
}

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
	parsed         roleProbeOutput
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
		Branch:  roleEventBranchUnknown,
		Result:  "ignored",
		Reason:  "notHandled",
		Version: fmt.Sprintf("%d", event.EventTime.UnixMicro()),
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

	parsed, parseErr := parseRoleProbeOutput(probeEvent.Output)
	result.Role = parsed.role
	result.parsed = parsed
	if parsed.hasAuthoritativeVersion {
		result.Version = fmt.Sprintf("roleVersion:%d", parsed.authoritativeVersion)
	}

	if probeEvent.Code != 0 {
		result.Result = "skipped"
		result.Reason = fmt.Sprintf("roleProbeFailed:%s", probeEvent.Message)
		return true, nil
	}
	if parseErr != nil {
		result.Result = "skipped"
		result.Reason = "malformedRoleProbeOutput"
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
			return h.handleInstanceSetRoleProbe(ctx, cli, pod, workloadName, event, result)
		case roleEventBranchInstance:
			return h.handleInstanceRoleProbe(ctx, cli, pod, workloadName, event, result)
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
		return h.handleInstanceSetRoleProbe(ctx, cli, pod, itsName, event, result)
	case instName != "":
		result.Branch = roleEventBranchInstance
		result.WorkloadName = instName
		return h.handleInstanceRoleProbe(ctx, cli, pod, instName, event, result)
	default:
		result.Result = "ignored"
		result.Reason = "unknownPodOwner"
		return false, nil
	}
}

func (h *RoleEventHandler) handleInstanceSetRoleProbe(ctx context.Context, cli client.Client, pod *corev1.Pod, itsName string, event *corev1.Event, result *roleEventResult) (bool, error) {
	if !acceptRoleProbeEvent(result.parsed, pod, event.EventTime.UnixMicro()) {
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

	if defined && role.IsExclusive {
		if !result.parsed.hasAuthoritativeVersion {
			held, err := versionedPeerHoldsExclusiveRole(ctx, cli, *its, pod.Name, result.Role)
			if err != nil {
				result.Result = "failed"
				result.Reason = "checkVersionedExclusiveRoleError"
				return false, err
			}
			if held {
				result.Result = "skipped"
				result.Reason = "versionedPeerHeldExclusiveRole"
				return true, nil
			}
		} else {
			stale, err := newerOrEqualVersionedExclusiveRoleHeldByPeer(ctx, cli, *its, pod.Name, result.Role, result.parsed.authoritativeVersion)
			if err != nil {
				result.Result = "failed"
				result.Reason = "checkPeerRoleVersionError"
				return false, err
			}
			if stale {
				result.Result = "skipped"
				result.Reason = "stalePeerRoleVersion"
				return true, nil
			}
		}
	}

	if err := updatePodRoleLabel(ctx, cli, pod, result.Role, result.parsed, event.EventTime.UnixMicro(), defined); err != nil {
		result.Result = "failed"
		result.Reason = "updatePodRoleLabelError"
		return false, err
	}

	if defined && role.IsExclusive {
		result.ExclusiveClean = true
		if err := removeExclusiveRoleLabels(ctx, cli, *its, pod.Name, result.Role, result.parsed, event.EventTime.UnixMicro()); err != nil {
			result.Result = "failed"
			result.Reason = "removeExclusiveRoleLabelsError"
			return false, err
		}
	}
	result.Result = "handled"
	result.Reason = "updated"
	return true, nil
}

func (h *RoleEventHandler) handleInstanceRoleProbe(ctx context.Context, cli client.Client, pod *corev1.Pod, instName string, event *corev1.Event, result *roleEventResult) (bool, error) {
	if !acceptRoleProbeEvent(result.parsed, pod, event.EventTime.UnixMicro()) {
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
	if err := updatePodRoleLabel(ctx, cli, pod, result.Role, result.parsed, event.EventTime.UnixMicro(), defined); err != nil {
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

// parseRoleProbeOutput parses kbagent roleProbe stdout into a role name plus
// an optional authoritative role version. The grammar splits stdout on any
// whitespace (spaces, tabs, newlines) into:
//
//	<role>                  // single-token form
//	<role> <uint64-version> // versioned form
//
// A probe script that emits a non-uint64 second token or three or more tokens
// is a parse error. Falling back to EventTime would let a typo bypass the
// authoritative-version ordering the probe script meant to use.
func parseRoleProbeOutput(stdout []byte) (roleProbeOutput, error) {
	if len(stdout) == 0 {
		return roleProbeOutput{}, nil
	}
	tokens := strings.Fields(string(stdout))
	switch len(tokens) {
	case 0:
		return roleProbeOutput{}, nil
	case 1:
		return roleProbeOutput{
			role: strings.ToLower(tokens[0]),
		}, nil
	case 2:
		v, err := strconv.ParseUint(tokens[1], 10, 64)
		if err != nil {
			return roleProbeOutput{
				role: strings.ToLower(tokens[0]),
			}, fmt.Errorf("invalid role version %q: %w", tokens[1], err)
		}
		return roleProbeOutput{
			role:                    strings.ToLower(tokens[0]),
			authoritativeVersion:    v,
			hasAuthoritativeVersion: true,
		}, nil
	default:
		return roleProbeOutput{
			role: strings.ToLower(tokens[0]),
		}, fmt.Errorf("invalid role probe output: expected one or two tokens, got %d", len(tokens))
	}
}

// acceptRoleProbeEvent decides whether to accept a parsed roleProbe event for
// a particular Pod. Each output form is gated by its own staleness anchor:
//
//   - Versioned result (`<role> <uint64>`) is accepted iff its version is
//     strictly greater than the Pod's recorded authoritative role version.
//   - Single-token result (`<role>`) is accepted iff its EventTime micros are
//     strictly greater than the recorded single-token EventTime anchor.
//     A single-token result is also rejected once the same Pod has accepted
//     any versioned result, avoiding downgrade from authoritative ordering.
//
// Versioned results do not consult the single-token anchor. The two anchors
// stay independent; the only cross-anchor rule is the same-Pod
// versioned-to-single-token downgrade block described above.
func acceptRoleProbeEvent(parsed roleProbeOutput, pod *corev1.Pod, eventTimeMicros int64) bool {
	if parsed.hasAuthoritativeVersion {
		last := podAnnotation(pod, constant.LastRoleProbeVersionAnnotationKey)
		if last == "" {
			return true
		}
		lastV, err := strconv.ParseUint(last, 10, 64)
		if err != nil || parsed.authoritativeVersion > lastV {
			return true
		}
		return false
	}

	if podAnnotation(pod, constant.LastRoleProbeVersionAnnotationKey) != "" {
		return false
	}
	last := podAnnotation(pod, constant.LastRoleEventVersionAnnotationKey)
	if last == "" {
		return true
	}
	lastV, err := strconv.ParseUint(last, 10, 64)
	return err != nil || uint64(eventTimeMicros) > lastV
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

// updatePodRoleLabel writes the new role label (or removes it when the role
// is not in the workload's role list) and advances the path-specific
// annotation for the accepted event. Versioned results stamp
// LastRoleProbeVersionAnnotationKey only; single-token results stamp
// LastRoleEventVersionAnnotationKey only. The other key is left untouched so
// that a migration window does not silently downgrade either stream's anchor.
func updatePodRoleLabel(ctx context.Context, cli client.Client, pod *corev1.Pod, roleName string, parsed roleProbeOutput, eventTimeMicros int64, roleDefined bool) error {
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
	if parsed.hasAuthoritativeVersion {
		newPod.Annotations[constant.LastRoleProbeVersionAnnotationKey] = strconv.FormatUint(parsed.authoritativeVersion, 10)
	} else {
		newPod.Annotations[constant.LastRoleEventVersionAnnotationKey] = strconv.FormatInt(eventTimeMicros, 10)
	}
	if reflect.DeepEqual(newPod.Labels, pod.Labels) && reflect.DeepEqual(newPod.Annotations, pod.Annotations) {
		return nil
	}
	return cli.Update(ctx, newPod)
}

// removeExclusiveRoleLabels strips the exclusive role label from peers when
// a new owner of the exclusive role has been accepted. Peer-cleanup
// behavior is path-specific:
//
//   - Single-token (EventTime) path: each accepted strip also stamps the peer's
//     LastRoleEventVersionAnnotationKey with the cleanup event's EventTime
//     micros. Without this stamp, a delayed single-token event from the demoted
//     primary whose EventTime is older than the cleanup event but newer
//     than the peer's own previous annotation would still pass the gate
//     and write the exclusive role back. Stamping is the one-way ratchet
//     the single-token stream relies on.
//   - Versioned path: each accepted strip leaves the peer's
//     LastRoleProbeVersionAnnotationKey untouched. The role version is
//     a per-pod monotonically increasing number; stamping the peer with
//     the new primary's role version (which originated from a different
//     pod's kbagent) would let the strict-newer gate later reject a
//     legitimate event from the peer at the same role epoch — for
//     example after failover the demoted pod's next event is
//     `secondary <same-epoch>`. The peer's own previous role-version annotation
//     is already a sufficient staleness anchor for queued events from the
//     demoted primary, because those events' versions are <= the peer's
//     recorded version.
//
// Whether to strip is still decided per peer by the same gate: a peer that
// has already advanced past the new event on the matching key is left
// alone.
func removeExclusiveRoleLabels(ctx context.Context, cli client.Client, its workloads.InstanceSet, newPodName, roleName string, parsed roleProbeOutput, eventTimeMicros int64) error {
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
		if !acceptRoleProbeEvent(parsed, &pod, eventTimeMicros) {
			continue
		}

		newPod := pods.Items[i].DeepCopy()
		if _, has := newPod.Labels[constant.RoleLabelKey]; !has {
			continue
		}
		delete(newPod.Labels, constant.RoleLabelKey)
		if !parsed.hasAuthoritativeVersion {
			if newPod.Annotations == nil {
				newPod.Annotations = map[string]string{}
			}
			newPod.Annotations[constant.LastRoleEventVersionAnnotationKey] = strconv.FormatInt(eventTimeMicros, 10)
		}
		if err := cli.Update(ctx, newPod); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func podAnnotation(pod *corev1.Pod, key string) string {
	if pod.Annotations == nil {
		return ""
	}
	return pod.Annotations[key]
}

// versionedPeerHoldsExclusiveRole reports whether any peer pod in the
// InstanceSet currently holds the exclusive role label and carries a
// LastRoleProbeVersionAnnotationKey value. It is used to keep a
// single-token roleProbe event from displacing an exclusive role that
// a versioned peer already owns: the single-token gate consults only
// the EventTime anchor, so a versioned peer's anchor is
// invisible to it; without this guard a single-token fallback event
// from a probe script could strip the role label off the
// authoritative primary and the versioned peer's next same-version event
// would then be rejected by the strict-newer gate and the role label could
// not be restored.
func versionedPeerHoldsExclusiveRole(ctx context.Context, cli client.Client, its workloads.InstanceSet, selfName, roleName string) (bool, error) {
	labels := map[string]string{
		constant.AppManagedByLabelKey:          constant.AppName,
		instanceset.WorkloadsManagedByLabelKey: workloads.InstanceSetKind,
		instanceset.WorkloadsInstanceLabelKey:  its.Name,
		constant.RoleLabelKey:                  roleName,
	}
	var pods corev1.PodList
	if err := cli.List(ctx, &pods, client.InNamespace(its.Namespace), client.MatchingLabels(labels)); err != nil {
		return false, err
	}
	for _, p := range pods.Items {
		if p.Name == selfName {
			continue
		}
		if podAnnotation(&p, constant.LastRoleProbeVersionAnnotationKey) != "" {
			return true, nil
		}
	}
	return false, nil
}

// newerOrEqualVersionedExclusiveRoleHeldByPeer reports whether any peer pod
// already holds the same exclusive role with an authoritative role version
// that is at least as new as the incoming versioned event. The caller must run this
// before writing the current pod's role label; otherwise a stale delayed
// versioned event can label itself while peer cleanup correctly refuses to
// strip the newer peer, leaving two pods with the exclusive role.
func newerOrEqualVersionedExclusiveRoleHeldByPeer(ctx context.Context, cli client.Client, its workloads.InstanceSet, selfName, roleName string, incomingVersion uint64) (bool, error) {
	labels := map[string]string{
		constant.AppManagedByLabelKey:          constant.AppName,
		instanceset.WorkloadsManagedByLabelKey: workloads.InstanceSetKind,
		instanceset.WorkloadsInstanceLabelKey:  its.Name,
		constant.RoleLabelKey:                  roleName,
	}
	var pods corev1.PodList
	if err := cli.List(ctx, &pods, client.InNamespace(its.Namespace), client.MatchingLabels(labels)); err != nil {
		return false, err
	}
	for _, p := range pods.Items {
		if p.Name == selfName {
			continue
		}
		last := podAnnotation(&p, constant.LastRoleProbeVersionAnnotationKey)
		if last == "" {
			continue
		}
		lastV, err := strconv.ParseUint(last, 10, 64)
		if err != nil {
			continue
		}
		if lastV >= incomingVersion {
			return true, nil
		}
	}
	return false, nil
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
