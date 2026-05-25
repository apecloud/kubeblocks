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

	workloadsapi "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/builder"
	"github.com/apecloud/kubeblocks/pkg/controller/instanceset"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	"github.com/apecloud/kubeblocks/pkg/kbagent/proto"
)

func TestRoleEventHandlerHandlesInstanceSetRoleAndExclusive(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)
	leader := workloadsapi.ReplicaRole{Name: "leader", IsExclusive: true}
	follower := workloadsapi.ReplicaRole{Name: "follower"}
	its := &workloadsapi.InstanceSet{
		ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "mysql"},
		Spec:       workloadsapi.InstanceSetSpec{Roles: []workloadsapi.ReplicaRole{leader, follower}},
	}
	pod := roleEventPod("default", "mysql-0", "uid-0", map[string]string{
		instanceset.WorkloadsInstanceLabelKey: "mysql",
	})
	otherPod := roleEventPod("default", "mysql-1", "uid-1", map[string]string{
		constant.AppManagedByLabelKey:          constant.AppName,
		instanceset.WorkloadsManagedByLabelKey: workloadsapi.InstanceSetKind,
		instanceset.WorkloadsInstanceLabelKey:  "mysql",
		constant.RoleLabelKey:                  "leader",
	})
	event := roleProbeEvent("default", "event-1", pod, "leader", now)
	cli := roleEventFakeClient(t, its, pod, otherPod, event)

	if handled := handleRoleEvent(t, ctx, cli, event); !handled {
		t.Fatalf("expected event to be handled")
	}

	assertPodRole(t, ctx, cli, pod, "leader", fmt.Sprintf("%d", event.EventTime.UnixMicro()))
	assertPodRole(t, ctx, cli, otherPod, "", fmt.Sprintf("%d", event.EventTime.UnixMicro()))
}

func TestRoleEventHandlerHandlesInstanceRoleWithoutExclusiveCleanup(t *testing.T) {
	ctx := context.Background()
	newer := time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)
	older := newer.Add(-time.Hour)
	leader := workloadsapi.ReplicaRole{Name: "leader", IsExclusive: true}
	follower := workloadsapi.ReplicaRole{Name: "follower"}
	inst := &workloadsapi.Instance{
		ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "mysql-0"},
		Spec:       workloadsapi.InstanceSpec{Roles: []workloadsapi.ReplicaRole{leader, follower}},
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

func TestRoleEventHandlerIgnoresUnknownOwnerWithoutMarkingHandled(t *testing.T) {
	ctx := context.Background()
	pod := roleEventPod("default", "mysql-0", "uid-0", nil)
	event := roleProbeEvent("default", "event-1", pod, "leader", time.Now())
	cli := roleEventFakeClient(t, pod, event)

	if handled := handleRoleEvent(t, ctx, cli, event); handled {
		t.Fatalf("expected event not to be handled")
	}
}

func TestRoleEventHandlerUsesTimestampFallbackForRoleVersion(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)
	inst := &workloadsapi.Instance{
		ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "mysql-0"},
		Spec:       workloadsapi.InstanceSpec{Roles: []workloadsapi.ReplicaRole{{Name: "leader"}}},
	}
	pod := roleEventPod("default", "mysql-0", "uid-0", map[string]string{
		constant.KBAppInstanceNameLabelKey: "mysql-0",
	})
	event := roleProbeEvent("default", "event-1", pod, "leader", now)
	event.EventTime = metav1.MicroTime{}
	event.LastTimestamp = metav1.NewTime(now)
	cli := roleEventFakeClient(t, inst, pod, event)

	if handled := handleRoleEvent(t, ctx, cli, event); !handled {
		t.Fatalf("expected event to be handled")
	}

	assertPodRole(t, ctx, cli, pod, "leader", fmt.Sprintf("%d", event.LastTimestamp.UnixMicro()))
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
	if err := workloadsapi.AddToScheme(scheme); err != nil {
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

func roleProbeEvent(namespace, name string, pod *corev1.Pod, role string, eventTime time.Time) *corev1.Event {
	message, err := json.Marshal(proto.ProbeEvent{
		Probe:  "roleProbe",
		Code:   0,
		Output: []byte(role),
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
