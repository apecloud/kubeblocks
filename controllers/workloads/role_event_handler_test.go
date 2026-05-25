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
	"fmt"
	"testing"
	"time"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/builder"
	"github.com/apecloud/kubeblocks/pkg/controller/instanceset"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	"github.com/apecloud/kubeblocks/pkg/kbagent/proto"
)

func TestRoleEventHandlerHandlesInstanceSetRoleAndExclusiveCleanupDoesNotStampPeerAnnotation(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)
	leader := workloads.ReplicaRole{Name: "leader", IsExclusive: true}
	follower := workloads.ReplicaRole{Name: "follower"}
	its := &workloads.InstanceSet{
		ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "mysql"},
		Spec:       workloads.InstanceSetSpec{Roles: []workloads.ReplicaRole{leader, follower}},
	}
	pod := roleEventPod("default", "mysql-0", "uid-0", map[string]string{
		instanceset.WorkloadsInstanceLabelKey: "mysql",
	})
	otherPod := roleEventPod("default", "mysql-1", "uid-1", map[string]string{
		constant.AppManagedByLabelKey:          constant.AppName,
		instanceset.WorkloadsManagedByLabelKey: workloads.InstanceSetKind,
		instanceset.WorkloadsInstanceLabelKey:  "mysql",
		constant.RoleLabelKey:                  "leader",
	})
	event := roleProbeEvent("default", "event-1", pod, "leader", now)
	cli := roleEventFakeClient(t, its, pod, otherPod, event)

	if handled := handleRoleEvent(t, ctx, cli, event); !handled {
		t.Fatalf("expected event to be handled")
	}

	assertPodRole(t, ctx, cli, pod, "leader", fmt.Sprintf("%d", event.EventTime.UnixMicro()))
	// Peer's exclusive role label must be stripped, but the peer's own
	// LastRoleEventVersionAnnotationKey must stay untouched. The annotation
	// represents the peer's own roleProbe event stream; stamping it with the
	// new event's version would let the strict-newer gate later reject a
	// legitimate event from the peer at the same engine epoch.
	assertPodRole(t, ctx, cli, otherPod, "", "")
	assertPodLastRoleVersion(t, ctx, cli, otherPod, "")
}

func TestRoleEventHandlerHandlesInstanceRoleWithoutExclusiveCleanup(t *testing.T) {
	ctx := context.Background()
	newer := time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)
	older := newer.Add(-time.Hour)
	leader := workloads.ReplicaRole{Name: "leader", IsExclusive: true}
	follower := workloads.ReplicaRole{Name: "follower"}
	inst := &workloads.Instance{
		ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "mysql-0"},
		Spec:       workloads.InstanceSpec{Roles: []workloads.ReplicaRole{leader, follower}},
	}
	pod := roleEventPod("default", "mysql-0", "uid-0", map[string]string{
		constant.KBAppInstanceNameLabelKey: "mysql-0",
	})
	otherPod := roleEventPod("default", "mysql-1", "uid-1", map[string]string{
		constant.AppManagedByLabelKey:      constant.AppName,
		constant.KBAppInstanceNameLabelKey: "mysql-1",
		constant.RoleLabelKey:              "leader",
	})
	event := roleProbeEvent("default", "event-1", pod, "leader", newer)
	staleEvent := roleProbeEvent("default", "event-2", pod, "follower", older)
	cli := roleEventFakeClient(t, inst, pod, otherPod, event, staleEvent)

	if handled := handleRoleEvent(t, ctx, cli, event); !handled {
		t.Fatalf("expected event to be handled")
	}
	assertPodRole(t, ctx, cli, pod, "leader", fmt.Sprintf("%d", event.EventTime.UnixMicro()))
	assertPodRole(t, ctx, cli, otherPod, "leader", "")

	if handled := handleRoleEvent(t, ctx, cli, staleEvent); !handled {
		t.Fatalf("expected stale event to be handled")
	}
	assertPodRole(t, ctx, cli, pod, "leader", fmt.Sprintf("%d", event.EventTime.UnixMicro()))
}

func TestRoleEventHandlerDeletesUndefinedInstanceSetRole(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)
	its := &workloads.InstanceSet{
		ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "mysql"},
		Spec:       workloads.InstanceSetSpec{Roles: []workloads.ReplicaRole{{Name: "leader"}}},
	}
	pod := roleEventPod("default", "mysql-0", "uid-0", map[string]string{
		instanceset.WorkloadsInstanceLabelKey: "mysql",
		constant.RoleLabelKey:                 "follower",
	})
	event := roleProbeEvent("default", "event-1", pod, "unknown", now)
	cli := roleEventFakeClient(t, its, pod, event)

	if handled := handleRoleEvent(t, ctx, cli, event); !handled {
		t.Fatalf("expected event to be handled")
	}

	assertPodRole(t, ctx, cli, pod, "", fmt.Sprintf("%d", event.EventTime.UnixMicro()))
}

func TestRoleEventHandlerDeletesUndefinedInstanceRole(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)
	inst := &workloads.Instance{
		ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "mysql-0"},
		Spec:       workloads.InstanceSpec{Roles: []workloads.ReplicaRole{{Name: "leader"}}},
	}
	pod := roleEventPod("default", "mysql-0", "uid-0", map[string]string{
		constant.KBAppInstanceNameLabelKey: "mysql-0",
		constant.RoleLabelKey:              "follower",
	})
	event := roleProbeEvent("default", "event-1", pod, "unknown", now)
	cli := roleEventFakeClient(t, inst, pod, event)

	if handled := handleRoleEvent(t, ctx, cli, event); !handled {
		t.Fatalf("expected event to be handled")
	}

	assertPodRole(t, ctx, cli, pod, "", fmt.Sprintf("%d", event.EventTime.UnixMicro()))
}

func TestRoleEventHandlerPrefersControllerRefOverLabels(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)
	its := &workloads.InstanceSet{
		ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "mysql"},
		Spec:       workloads.InstanceSetSpec{Roles: []workloads.ReplicaRole{{Name: "leader"}}},
	}
	pod := roleEventPod("default", "mysql-0", "uid-0", map[string]string{
		instanceset.WorkloadsInstanceLabelKey: "mysql",
		constant.KBAppInstanceNameLabelKey:    "mysql-0",
	})
	setControllerRef(pod, workloads.GroupVersion.String(), workloads.InstanceSetKind, its.Name)
	event := roleProbeEvent("default", "event-1", pod, "leader", now)
	cli := roleEventFakeClient(t, its, pod, event)

	if handled := handleRoleEvent(t, ctx, cli, event); !handled {
		t.Fatalf("expected event to be handled")
	}

	assertPodRole(t, ctx, cli, pod, "leader", fmt.Sprintf("%d", event.EventTime.UnixMicro()))
}

func TestRoleEventHandlerConsumesInvalidProbeMessageWithoutPodUpdate(t *testing.T) {
	ctx := context.Background()
	pod := roleEventPod("default", "mysql-0", "uid-0", map[string]string{
		constant.KBAppInstanceNameLabelKey: "mysql-0",
		constant.RoleLabelKey:              "leader",
	})
	event := roleProbeEvent("default", "event-1", pod, "follower", time.Now())
	event.Message = "{"
	cli := roleEventFakeClient(t, pod, event)

	if handled := handleRoleEvent(t, ctx, cli, event); !handled {
		t.Fatalf("expected event to be handled")
	}

	assertPodRole(t, ctx, cli, pod, "leader", "")
	assertPodLastRoleVersion(t, ctx, cli, pod, "")
}

func TestRoleEventHandlerConsumesProbeFailureWithoutPodUpdate(t *testing.T) {
	ctx := context.Background()
	pod := roleEventPod("default", "mysql-0", "uid-0", map[string]string{
		constant.KBAppInstanceNameLabelKey: "mysql-0",
		constant.RoleLabelKey:              "leader",
	})
	event := roleProbeEventWithCode("default", "event-1", pod, "follower", time.Now(), 1, "probe failed")
	cli := roleEventFakeClient(t, pod, event)

	if handled := handleRoleEvent(t, ctx, cli, event); !handled {
		t.Fatalf("expected event to be handled")
	}

	assertPodRole(t, ctx, cli, pod, "leader", "")
	assertPodLastRoleVersion(t, ctx, cli, pod, "")
}

func TestRoleEventHandlerConsumesStalePodUIDWithoutPodUpdate(t *testing.T) {
	ctx := context.Background()
	pod := roleEventPod("default", "mysql-0", "uid-new", map[string]string{
		constant.KBAppInstanceNameLabelKey: "mysql-0",
		constant.RoleLabelKey:              "leader",
	})
	event := roleProbeEvent("default", "event-1", pod, "follower", time.Now())
	event.InvolvedObject.UID = "uid-old"
	cli := roleEventFakeClient(t, pod, event)

	if handled := handleRoleEvent(t, ctx, cli, event); !handled {
		t.Fatalf("expected event to be handled")
	}

	assertPodRole(t, ctx, cli, pod, "leader", "")
	assertPodLastRoleVersion(t, ctx, cli, pod, "")
}

func TestRoleEventHandlerConsumesPodNotFound(t *testing.T) {
	ctx := context.Background()
	pod := roleEventPod("default", "mysql-0", "uid-0", map[string]string{
		constant.KBAppInstanceNameLabelKey: "mysql-0",
	})
	event := roleProbeEvent("default", "event-1", pod, "leader", time.Now())
	cli := roleEventFakeClient(t, event)

	if handled := handleRoleEvent(t, ctx, cli, event); !handled {
		t.Fatalf("expected event to be handled")
	}
}

func TestRoleEventHandlerConsumesMissingWorkloadWithoutPodUpdate(t *testing.T) {
	testCases := []struct {
		name   string
		labels map[string]string
	}{
		{
			name: "missing InstanceSet",
			labels: map[string]string{
				instanceset.WorkloadsInstanceLabelKey: "mysql",
				constant.RoleLabelKey:                 "leader",
			},
		},
		{
			name: "missing Instance",
			labels: map[string]string{
				constant.KBAppInstanceNameLabelKey: "mysql-0",
				constant.RoleLabelKey:              "leader",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			pod := roleEventPod("default", "mysql-0", "uid-0", tc.labels)
			event := roleProbeEvent("default", "event-1", pod, "follower", time.Now())
			cli := roleEventFakeClient(t, pod, event)

			if handled := handleRoleEvent(t, ctx, cli, event); !handled {
				t.Fatalf("expected event to be handled")
			}

			assertPodRole(t, ctx, cli, pod, "leader", "")
			assertPodLastRoleVersion(t, ctx, cli, pod, "")
		})
	}
}

func TestRoleEventHandlerIgnoresUnknownOwnerWithoutMarkingHandled(t *testing.T) {
	ctx := context.Background()
	pod := roleEventPod("default", "mysql-0", "uid-0", nil)
	event := roleProbeEvent("default", "event-1", pod, "leader", time.Now())
	cli := roleEventFakeClient(t, pod, event)

	if handled := handleRoleEvent(t, ctx, cli, event); handled {
		t.Fatalf("expected event not to be handled")
	}
}

func handleRoleEvent(t *testing.T, ctx context.Context, cli client.Client, event *corev1.Event) bool {
	t.Helper()
	handled, err := (&RoleEventHandler{}).Handle(cli, intctrlutil.RequestCtx{
		Ctx: ctx,
		Log: logr.Discard(),
	}, nil, event)
	if err != nil {
		t.Fatalf("handle event failed: %v", err)
	}
	return handled
}

func roleEventFakeClient(t *testing.T, objects ...client.Object) client.Client {
	t.Helper()
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("add core scheme: %v", err)
	}
	if err := workloads.AddToScheme(scheme); err != nil {
		t.Fatalf("add workloads scheme: %v", err)
	}
	return fake.NewClientBuilder().WithScheme(scheme).WithObjects(objects...).Build()
}

func roleEventPod(namespace, name, uid string, labels map[string]string) *corev1.Pod {
	pod := builder.NewPodBuilder(namespace, name).SetUID(types.UID(uid)).GetObject()
	if labels != nil {
		pod.Labels = labels
	}
	return pod
}

func setControllerRef(obj client.Object, apiVersion, kind, name string) {
	controller := true
	obj.SetOwnerReferences([]metav1.OwnerReference{{
		APIVersion: apiVersion,
		Kind:       kind,
		Name:       name,
		UID:        types.UID(name + "-uid"),
		Controller: &controller,
	}})
}

func roleProbeEvent(namespace, name string, pod *corev1.Pod, role string, eventTime time.Time) *corev1.Event {
	return roleProbeEventWithCode(namespace, name, pod, role, eventTime, 0, "")
}

func roleProbeEventWithCode(namespace, name string, pod *corev1.Pod, role string, eventTime time.Time, code int32, messageText string) *corev1.Event {
	message, err := json.Marshal(proto.ProbeEvent{
		Probe:   "roleProbe",
		Code:    code,
		Output:  []byte(role),
		Message: messageText,
	})
	if err != nil {
		panic(err)
	}
	return builder.NewEventBuilder(namespace, name).
		SetInvolvedObject(corev1.ObjectReference{
			APIVersion: "v1",
			Kind:       "Pod",
			Namespace:  pod.Namespace,
			Name:       pod.Name,
			UID:        pod.UID,
			FieldPath:  proto.ProbeEventFieldPath,
		}).
		SetReason("roleProbe").
		SetMessage(string(message)).
		SetReportingController(proto.ProbeEventReportingController).
		SetEventTime(metav1.NewMicroTime(eventTime)).
		GetObject()
}

func assertPodRole(t *testing.T, ctx context.Context, cli client.Client, pod *corev1.Pod, role, version string) {
	t.Helper()
	var stored corev1.Pod
	if err := cli.Get(ctx, client.ObjectKeyFromObject(pod), &stored); err != nil {
		t.Fatalf("get pod failed: %v", err)
	}
	if role == "" {
		if stored.Labels[constant.RoleLabelKey] != "" {
			t.Fatalf("expected role label to be empty, got %q", stored.Labels[constant.RoleLabelKey])
		}
	} else if stored.Labels[constant.RoleLabelKey] != role {
		t.Fatalf("expected role %q, got %q", role, stored.Labels[constant.RoleLabelKey])
	}
	if version != "" && stored.Annotations[constant.LastRoleEventVersionAnnotationKey] != version {
		t.Fatalf("expected version %q, got %q", version, stored.Annotations[constant.LastRoleEventVersionAnnotationKey])
	}
}

func assertPodLastRoleVersion(t *testing.T, ctx context.Context, cli client.Client, pod *corev1.Pod, version string) {
	t.Helper()
	var stored corev1.Pod
	if err := cli.Get(ctx, client.ObjectKeyFromObject(pod), &stored); err != nil {
		t.Fatalf("get pod failed: %v", err)
	}
	if stored.Annotations[constant.LastRoleEventVersionAnnotationKey] != version {
		t.Fatalf("expected version %q, got %q", version, stored.Annotations[constant.LastRoleEventVersionAnnotationKey])
	}
}
