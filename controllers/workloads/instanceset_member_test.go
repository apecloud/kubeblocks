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
	"fmt"
	"strings"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	apps "github.com/apecloud/kubeblocks/apis/apps/v1"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	kbacli "github.com/apecloud/kubeblocks/pkg/kbagent/client"
	"github.com/apecloud/kubeblocks/pkg/kbagent/proto"
)

type memberDNSAgent struct{ requests []proto.ActionRequest }

func (a *memberDNSAgent) Close() error { return nil }
func (a *memberDNSAgent) Action(_ context.Context, req proto.ActionRequest) (proto.ActionResponse, error) {
	a.requests = append(a.requests, req)
	return proto.ActionResponse{}, nil
}

type unavailableMemberAgent struct{ calls int }

func (a *unavailableMemberAgent) Close() error { return nil }
func (a *unavailableMemberAgent) Action(_ context.Context, _ proto.ActionRequest) (proto.ActionResponse, error) {
	a.calls++
	return proto.ActionResponse{}, fmt.Errorf("member dispatch failed")
}

func TestInstanceSetMemberActionUsesValidatedExecutor(t *testing.T) {
	for _, instanceAPI := range []bool{false, true} {
		for _, legacySelector := range []bool{false, true} {
			t.Run(fmt.Sprintf("instanceAPI=%t/legacySelector=%t", instanceAPI, legacySelector), func(t *testing.T) {
				ctx := context.Background()
				f := newMemberLifecycleEntry(t, instanceAPI, 1, appsv1.ParallelPodManagement)
				f.bootstrap(t, 1)
				action := f.its.Spec.LifecycleActions.MemberJoin
				if legacySelector {
					action.Exec.TargetPodSelector = apps.AnyReplica
				} else {
					action.TargetPodSelector = apps.AnyReplica
				}
				f.its.Spec.Replicas = ptr.To[int32](3)
				f.its.Spec.ParallelPodManagementConcurrency = ptr.To(intstr.FromInt(2))
				f.its.Generation++
				if err := f.cli.Update(ctx, f.its); err != nil {
					t.Fatal(err)
				}
				for round := 0; round < 12 && len(lifecycleEntryPods(t, f.cli)) < 3; round++ {
					f.stepWithChildren(t)
				}
				if len(lifecycleEntryPods(t, f.cli)) != 3 {
					t.Fatal("parallel expansion did not create both siblings")
				}
				f.setReady(t, f.its.Name+"-1", true)
				pendingName := f.its.Name + "-2"
				agent := &unavailableMemberAgent{}
				kbacli.SetMockClient(agent, nil)
				defer kbacli.UnsetMockClient()
				for round := 0; round < 128; round++ {
					f.step(t)
					condition := meta.FindStatusCondition(f.its.Status.Conditions, "InstanceLifecycle")
					if condition != nil && strings.Contains(condition.Message, "member dispatch failed") && strings.Contains(condition.Message, "at pod "+pendingName) {
						t.Fatalf("action was dispatched to an executor rejected by endpoint validation: %s", condition.Message)
					}
					status := f.its.FindInstanceStatus(f.its.Name + "-1")
					if status == nil || status.MemberJoined == nil || *status.MemberJoined {
						t.Fatalf("failed membership request changed completion: %#v", status)
					}
				}
				if agent.calls == 0 {
					t.Fatal("no membership request reached an available executor")
				}
			})
		}
	}
}

func TestInstanceSetMemberActionsUseRuntimePodDNS(t *testing.T) {
	for _, instanceAPI := range []bool{false, true} {
		t.Run(fmt.Sprintf("instanceAPI=%t", instanceAPI), func(t *testing.T) {
			ctx := context.Background()
			f := newMemberLifecycleEntry(t, instanceAPI, 1, appsv1.OrderedReadyPodManagement)
			f.bootstrap(t, 1)
			agent := &memberDNSAgent{}
			kbacli.SetMockClient(agent, nil)
			defer kbacli.UnsetMockClient()
			f.its.Spec.Replicas = ptr.To[int32](2)
			f.its.Generation++
			if err := f.cli.Update(ctx, f.its); err != nil {
				t.Fatal(err)
			}
			for round := 0; round < 12 && len(agent.requests) == 0; round++ {
				f.stepWithChildren(t)
				for _, pod := range lifecycleEntryPods(t, f.cli) {
					f.setReady(t, pod.Name, true)
				}
			}
			pod := &corev1.Pod{}
			if err := f.cli.Get(ctx, client.ObjectKey{Namespace: f.its.Namespace, Name: f.its.Name + "-1"}, pod); err != nil {
				t.Fatal(err)
			}
			if pod.Spec.Hostname == "" || pod.Spec.Subdomain == "" {
				t.Fatalf("controller did not establish Pod DNS: %#v", pod.Spec)
			}
			want := pod.Spec.Hostname + "." + intctrlutil.ServiceFQDN(pod.Namespace, pod.Spec.Subdomain)
			assertMemberDNSRequest(t, agent.requests, "memberJoin", "KB_JOIN_MEMBER_POD_FQDN", pod.Name, want)
			agent.requests = nil
			f.its.Spec.Replicas = ptr.To[int32](1)
			f.its.Generation++
			if err := f.cli.Update(ctx, f.its); err != nil {
				t.Fatal(err)
			}
			for round := 0; round < 8 && len(agent.requests) == 0; round++ {
				f.step(t)
			}
			assertMemberDNSRequest(t, agent.requests, "memberLeave", "KB_LEAVE_MEMBER_POD_FQDN", pod.Name, want)
		})
	}
}

func assertMemberDNSRequest(t *testing.T, requests []proto.ActionRequest, action, fqdnKey, podName, want string) {
	t.Helper()
	if len(requests) != 1 || requests[0].Action != action {
		t.Fatalf("expected one %s request, got %#v", action, requests)
	}
	if got := requests[0].Parameters[fqdnKey]; got != want {
		t.Errorf("%s for Pod %s = %q, want runtime Pod DNS %q", fqdnKey, podName, got, want)
	}
}
