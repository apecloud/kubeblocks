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
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	workloadsv1 "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/instanceset"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

type roleCleanupConflictClient struct {
	client.Client
	failed bool
}

func (c *roleCleanupConflictClient) Update(ctx context.Context, object client.Object, opts ...client.UpdateOption) error {
	if pod, ok := object.(*corev1.Pod); ok && pod.Name == "db-0" && pod.Labels[constant.RoleLabelKey] == "" && !c.failed {
		c.failed = true
		return apierrors.NewConflict(schema.GroupResource{Resource: "pods"}, pod.Name, context.Canceled)
	}
	return c.Client.Update(ctx, object, opts...)
}

func TestRoleEventHandlerPersistsRoleObservation(t *testing.T) {
	ctx := context.Background()
	its := &workloadsv1.InstanceSet{
		ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "db"},
		Spec:       workloadsv1.InstanceSetSpec{Roles: []workloadsv1.ReplicaRole{{Name: "primary", IsExclusive: true}}},
	}
	pod := roleEventPod("default", "db-0", "uid-0", map[string]string{instanceset.WorkloadsInstanceLabelKey: "db"})
	event := roleProbeEvent("default", "role", pod, "primary", time.Date(2026, 9, 21, 0, 0, 0, 0, time.UTC))
	cli := roleEventFakeClient(t, its, pod, event)

	if handled := handleRoleEvent(t, ctx, cli, event); !handled {
		t.Fatal("expected role event to be handled")
	}
	var stored corev1.Pod
	if err := cli.Get(ctx, client.ObjectKeyFromObject(pod), &stored); err != nil {
		t.Fatal(err)
	}
	var observation instanceset.RoleObservation
	if err := json.Unmarshal([]byte(stored.Annotations[constant.RoleObservationAnnotationKey]), &observation); err != nil {
		t.Fatal(err)
	}
	if observation.Role != "primary" || !observation.RoleDefined || observation.EventVersion == "" {
		t.Fatalf("unexpected role observation: %#v", observation)
	}
}

func TestRoleEventHandlerRetriesAcceptedExclusiveCleanup(t *testing.T) {
	ctx := context.Background()
	its := &workloadsv1.InstanceSet{
		ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "db"},
		Spec:       workloadsv1.InstanceSetSpec{Roles: []workloadsv1.ReplicaRole{{Name: "primary", IsExclusive: true}}},
	}
	oldPrimary := roleEventPod("default", "db-0", "uid-0", instanceSetRoleLabels("db", "primary"))
	newPrimary := roleEventPod("default", "db-1", "uid-1", instanceSetRoleLabels("db", "primary"))
	event := roleProbeEvent("default", "promoted", newPrimary, "primary", time.Date(2026, 9, 21, 0, 0, 0, 0, time.UTC))
	cli := &roleCleanupConflictClient{Client: roleEventFakeClient(t, its, oldPrimary, newPrimary, event)}

	if _, err := (&RoleEventHandler{}).Handle(cli, intctrlutil.RequestCtx{Ctx: ctx}, nil, event); err == nil {
		t.Fatal("expected injected peer cleanup conflict")
	}
	if handled, err := (&RoleEventHandler{}).Handle(cli, intctrlutil.RequestCtx{Ctx: ctx}, nil, event); err != nil || !handled {
		t.Fatalf("retry failed: handled=%v err=%v", handled, err)
	}
	var stored corev1.Pod
	if err := cli.Get(ctx, client.ObjectKeyFromObject(oldPrimary), &stored); err != nil {
		t.Fatal(err)
	}
	if stored.Labels[constant.RoleLabelKey] != "" {
		t.Fatalf("old exclusive role label survived retry: %#v", stored.Labels)
	}
}
