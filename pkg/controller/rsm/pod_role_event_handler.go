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

package rsm

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"

	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/common"
	"github.com/apecloud/kubeblocks/pkg/constant"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

type PodRoleEventHandler struct{}

// probeEventType defines the type of probe event.
type probeEventType string

const (
	successEvent     = "Success"
	roleChangedEvent = "roleChanged"
)

type probeMessage struct {
	Event        probeEventType `json:"event,omitempty"`
	Message      string         `json:"message,omitempty"`
	OriginalRole string         `json:"originalRole,omitempty"`
	Role         string         `json:"role,omitempty"`
}

const (
	// roleChangedAnnotKey is used to mark the role change event has been handled.
	roleChangedAnnotKey = "role.kubeblocks.io/event-handled"
)

var roleMessageRegex = regexp.MustCompile(`Readiness probe failed: .*({.*})`)

func (h *PodRoleEventHandler) Handle(cli client.Client, reqCtx intctrlutil.RequestCtx, recorder record.EventRecorder, event *corev1.Event) error {
	if event.InvolvedObject.FieldPath != readinessProbeEventFieldPath &&
		event.InvolvedObject.FieldPath != legacyEventFieldPath &&
		event.InvolvedObject.FieldPath != lorryEventFieldPath &&
		event.Reason != checkRoleEventReason {
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

	if _, err = handleRoleChangedEvent(cli, reqCtx, recorder, event); err != nil {
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

// handleRoleChangedEvent handles role changed event and return role.
func handleRoleChangedEvent(cli client.Client, reqCtx intctrlutil.RequestCtx, recorder record.EventRecorder, event *corev1.Event) (string, error) {
	// parse probe event message
	message := parseProbeEventMessage(reqCtx, event)
	if message == nil {
		reqCtx.Log.Info("parse probe event message failed", "message", event.Message)
		return "", nil
	}

	// if probe event operation is not impl, check role failed or role invalid, ignore it
	if message.Event != successEvent && message.Event != roleChangedEvent {
		reqCtx.Log.Info("probe event failed", "message", message.Message)
		return "", nil
	}
	role := strings.ToLower(message.Role)

	snapshot := parseGlobalRoleSnapshot(role, event)
	for _, pair := range snapshot.PodRoleNamePairs {
		podName := types.NamespacedName{
			Namespace: event.InvolvedObject.Namespace,
			Name:      pair.PodName,
		}
		// get pod
		pod := &corev1.Pod{}
		if err := cli.Get(reqCtx.Ctx, podName, pod); err != nil {
			return pair.RoleName, err
		}
		// event belongs to old pod with the same name, ignore it
		if pod.Name == pair.PodName && pod.UID != event.InvolvedObject.UID {
			return pair.RoleName, nil
		}

		// compare the version of the current role snapshot with the last version recorded in the pod annotation,
		// stale role snapshot will be ignored.
		lastSnapshotVersion, ok := pod.Annotations[constant.LastRoleSnapshotVersionAnnotationKey]
		if ok {

			if snapshot.Version <= lastSnapshotVersion {
				reqCtx.Log.Info("stale role snapshot received, ignore it", "snapshot", snapshot)
				return pair.RoleName, nil
			}
		}

		name, _ := intctrlutil.GetParentNameAndOrdinal(pod)
		rsm := &workloads.ReplicatedStateMachine{}
		if err := cli.Get(reqCtx.Ctx, types.NamespacedName{Namespace: pod.Namespace, Name: name}, rsm); err != nil {
			return "", err
		}
		reqCtx.Log.V(1).Info("handle role change event", "pod", pod.Name, "role", role, "originalRole", message.OriginalRole)

		if err := updatePodRoleLabel(cli, reqCtx, *rsm, pod, pair.RoleName, snapshot.Version); err != nil {
			return "", err
		}
	}
	return role, nil
}

func parseGlobalRoleSnapshot(role string, event *corev1.Event) *common.GlobalRoleSnapshot {
	snapshot := &common.GlobalRoleSnapshot{}
	if err := json.Unmarshal([]byte(role), snapshot); err == nil {
		return snapshot
	}
	snapshot.Version = event.EventTime.Time.Format(time.RFC3339Nano)
	pair := common.PodRoleNamePair{
		PodName:  event.InvolvedObject.Name,
		RoleName: role,
	}
	snapshot.PodRoleNamePairs = append(snapshot.PodRoleNamePairs, pair)
	return snapshot
}

// parseProbeEventMessage parses probe event message.
func parseProbeEventMessage(reqCtx intctrlutil.RequestCtx, event *corev1.Event) *probeMessage {
	message := &probeMessage{}

	tryUnmarshalDirectAPIServerEvent := func() error {
		return json.Unmarshal([]byte(event.Message), message)
	}
	tryUnmarshalReadinessProbeEvent := func() error {
		matches := roleMessageRegex.FindStringSubmatch(event.Message)
		if len(matches) != 2 {
			reqCtx.Log.Info("parser Readiness probe event message failed", "message", event.Message)
			return fmt.Errorf("parser Readiness probe event message failed: %s", event.Message)
		}
		msg := matches[1]
		err := json.Unmarshal([]byte(msg), message)
		if err != nil {
			// not role related message, ignore it
			reqCtx.Log.Info("not role message", "message", event.Message, "error", err)
			return err
		}
		return nil
	}

	if err := tryUnmarshalDirectAPIServerEvent(); err == nil {
		return message
	}
	if err := tryUnmarshalReadinessProbeEvent(); err == nil {
		return message
	}
	return nil
}
