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

package lifecycle

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/utils/ptr"

	apps "github.com/apecloud/kubeblocks/apis/apps/v1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/instancetemplate"
	publiclifecycle "github.com/apecloud/kubeblocks/pkg/controller/lifecycle"
	"github.com/apecloud/kubeblocks/pkg/kbagent"
	kbacli "github.com/apecloud/kubeblocks/pkg/kbagent/client"
	"github.com/apecloud/kubeblocks/pkg/kbagent/proto"
)

func memberWorkload() *workloads.InstanceSet {
	return &workloads.InstanceSet{ObjectMeta: metav1.ObjectMeta{Name: "demo", Namespace: "default", Generation: 1}, Spec: workloads.InstanceSetSpec{LifecycleActions: &workloads.LifecycleActions{MemberJoin: testAction("join"), MemberLeave: testAction("leave")}}}
}
func memberPod() *corev1.Pod {
	return &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "demo-0", Namespace: "default"}, Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: "db"}, {Name: kbagent.ContainerName}}}}
}
func testAction(command string) *apps.Action {
	return &apps.Action{Exec: &apps.ExecAction{Command: []string{"/bin/sh", "-c", command}}}
}

func envValue(c corev1.Container, name string) string {
	for _, env := range c.Env {
		if env.Name == name {
			return env.Value
		}
	}
	return ""
}

func TestRegistrationSeparatesBootstrapAndLaterAllocation(t *testing.T) {
	its := memberWorkload()
	assignments := []instancetemplate.Assignment{{InstanceName: "demo-0"}, {InstanceName: "demo-1", TemplateName: "alternate"}}
	if !Register(its, assignments, sets.New[string](), sets.New[string]()) {
		t.Fatal("bootstrap not registered")
	}
	for _, status := range its.Status.InstanceStatus {
		if status.MemberJoined != nil || status.Provisioned {
			t.Fatalf("bootstrap invented results: %+v", status)
		}
	}
	assignments = append(assignments, instancetemplate.Assignment{InstanceName: "demo-2"})
	if !Register(its, assignments, sets.New[string](), sets.New[string]()) {
		t.Fatal("new identity not registered")
	}
	status := its.FindInstanceStatus("demo-2")
	if status.MemberJoined == nil || *status.MemberJoined {
		t.Fatalf("new empty allocation not tracked: %+v", status)
	}
	its = memberWorkload()
	if !Register(its, nil, sets.New[string](), sets.New[string]()) || its.Status.ObservedGeneration != 1 {
		t.Fatal("zero bootstrap did not record processed generation")
	}
	if !Register(its, assignments[:1], sets.New[string](), sets.New[string]()) || its.FindInstanceStatus("demo-0").MemberJoined == nil {
		t.Fatal("zero replica increase was treated as bootstrap")
	}
	its = memberWorkload()
	Register(its, assignments, sets.New("demo-1"), sets.New("demo-0"))
	if its.FindInstanceStatus("demo-1").MemberJoined != nil {
		t.Fatal("unobserved old resources were classified as empty")
	}
}

func TestMemberOnlyMergesActionsAndPreservesServerConfiguration(t *testing.T) {
	its := memberWorkload()
	pod := memberPod()
	existing, _ := json.Marshal([]proto.Action{{Name: "reconfigure", Exec: &proto.ExecAction{Commands: []string{"old"}}}})
	pod.Spec.InitContainers = nil
	pod.Spec.Containers[1] = corev1.Container{Name: kbagent.ContainerName, Image: "tools", Env: []corev1.EnvVar{{Name: "KB_AGENT_ACTION", Value: string(existing)}, {Name: "KB_AGENT_PROBE", Value: "existing-probe"}, {Name: "BUSINESS", Value: "keep"}}}
	if err := ConfigurePod(its, pod); err != nil {
		t.Fatal(err)
	}
	server := pod.Spec.Containers[1]
	var actions []proto.Action
	if err := json.Unmarshal([]byte(envValue(server, "KB_AGENT_ACTION")), &actions); err != nil || len(actions) != 3 {
		t.Fatalf("actions not merged: %+v %v", actions, err)
	}
	if actions[0].Name != "reconfigure" || envValue(server, "KB_AGENT_PROBE") != "existing-probe" || envValue(server, "BUSINESS") != "keep" || len(pod.Spec.InitContainers) != 0 {
		t.Fatal("member-only configuration discarded existing server context")
	}
	if _, err := ValidateExecution(its.Spec.LifecycleActions.MemberJoin, pod, []*corev1.Pod{pod}); err == nil {
		t.Fatal("missing agent endpoint would record success")
	}
}

type memberAgent struct{ calls int }

func (a *memberAgent) Close() error { return nil }

func (a *memberAgent) Action(_ context.Context, _ proto.ActionRequest) (proto.ActionResponse, error) {
	a.calls++
	return proto.ActionResponse{}, nil
}

func TestValidatedExecutionPreservesActionSelection(t *testing.T) {
	for _, tc := range []struct {
		name     string
		action   *apps.Action
		podCount int
		calls    int
	}{
		{name: "default", action: testAction("join"), podCount: 2, calls: 1},
		{name: "any", action: &apps.Action{TargetPodSelector: apps.AnyReplica, Exec: testAction("join").Exec}, podCount: 2, calls: 1},
		{name: "legacy any", action: &apps.Action{Exec: &apps.ExecAction{Command: []string{"join"}, TargetPodSelector: apps.AnyReplica}}, podCount: 2, calls: 1},
		{name: "all", action: &apps.Action{TargetPodSelector: apps.AllReplicas, Exec: testAction("join").Exec}, podCount: 2, calls: 2},
		{name: "single all", action: &apps.Action{TargetPodSelector: apps.AllReplicas, Exec: testAction("join").Exec}, podCount: 1, calls: 1},
		{name: "role", action: &apps.Action{TargetPodSelector: apps.RoleSelector, MatchingKey: "replica", Exec: testAction("join").Exec}, podCount: 2, calls: 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			its := memberWorkload()
			its.Spec.LifecycleActions.MemberJoin = tc.action
			pods := []*corev1.Pod{memberPod(), memberPod()}
			pods[1].Name = "demo-2"
			pods = pods[:tc.podCount]
			for _, pod := range pods {
				pod.Labels = map[string]string{constant.RoleLabelKey: "replica"}
				pod.Status.PodIP = "10.0.0.1"
				pod.Spec.InitContainers = nil
				pod.Spec.Containers[1] = corev1.Container{Name: kbagent.ContainerName, Ports: []corev1.ContainerPort{{Name: kbagent.DefaultHTTPPortName, ContainerPort: 3501}}}
			}
			opts, err := ValidateExecution(tc.action, pods[0], pods)
			if err != nil {
				t.Fatal(err)
			}
			if (tc.name == "any" || tc.name == "legacy any") && (opts == nil || opts.TargetPodName == "") {
				t.Fatal("random executor was not retained for execution")
			}
			if tc.name == "single all" && opts != nil {
				t.Fatal("AllReplicas lost its aggregate-error semantics")
			}
			lfa, err := publiclifecycle.New(its.Namespace, "", "", &apps.ComponentLifecycleActions{MemberJoin: tc.action}, its.Spec.LifecycleActions.TemplateVars, pods[0], pods)
			if err != nil {
				t.Fatal(err)
			}
			agent := &memberAgent{}
			kbacli.SetMockClient(agent, nil)
			defer kbacli.UnsetMockClient()
			if err := lfa.MemberJoin(context.Background(), nil, opts); err != nil {
				t.Fatal(err)
			}
			if agent.calls != tc.calls {
				t.Fatalf("action executed %d times, want %d", agent.calls, tc.calls)
			}
		})
	}
}

func TestValidatedExecutionWaitsWithoutExecutors(t *testing.T) {
	for _, legacy := range []bool{false, true} {
		t.Run(fmt.Sprintf("legacy=%t", legacy), func(t *testing.T) {
			defer func() {
				if got := recover(); got != nil {
					t.Fatalf("missing executor must return an error instead of panic: %v", got)
				}
			}()
			action := testAction("leave")
			if legacy {
				action.Exec.TargetPodSelector = apps.AnyReplica
			} else {
				action.TargetPodSelector = apps.AnyReplica
			}
			if _, err := ValidateExecution(action, memberPod(), nil); err == nil {
				t.Fatal("missing executor was accepted")
			}
		})
	}
}

func TestValidationRejectsUnownedPreconditionsAndNonblockingActions(t *testing.T) {
	if err := ConfigurePod(memberWorkload(), memberPod()); err != nil {
		t.Fatalf("blocking actions did not configure a bootstrap Pod: %v", err)
	}
	for _, condition := range []apps.PreConditionType{apps.RuntimeReadyPreConditionType, apps.ComponentReadyPreConditionType, apps.ClusterReadyPreConditionType} {
		its := memberWorkload()
		its.Spec.LifecycleActions.MemberJoin.PreCondition = ptr.To(condition)
		if err := Validate(its); err == nil {
			t.Fatalf("accepted unowned precondition %s", condition)
		}
	}
	for _, name := range []string{"join", "leave", "switch"} {
		its := memberWorkload()
		switch name {
		case "join":
			its.Spec.LifecycleActions.MemberJoin.NonBlocking = true
		case "leave":
			its.Spec.LifecycleActions.MemberLeave.NonBlocking = true
		case "switch":
			its.Spec.LifecycleActions.Switchover = testAction("true")
			its.Spec.LifecycleActions.Switchover.NonBlocking = true
		}
		pod := memberPod()
		before := pod.DeepCopy()
		if err := ConfigurePod(its, pod); err == nil {
			t.Fatalf("accepted nonblocking %s without a completion contract", name)
		}
		if !reflect.DeepEqual(before, pod) {
			t.Fatalf("invalid %s action changed the Pod", name)
		}
	}
}

func TestConfigurePodRegistersSwitchover(t *testing.T) {
	its := memberWorkload()
	its.Spec.LifecycleActions.Switchover = testAction("switch")
	pod := memberPod()
	if err := ConfigurePod(its, pod); err != nil {
		t.Fatal(err)
	}
	var actions []proto.Action
	if err := json.Unmarshal([]byte(envValue(pod.Spec.Containers[1], "KB_AGENT_ACTION")), &actions); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, action := range actions {
		found = found || action.Name == "switchover"
	}
	if !found {
		t.Fatal("prepared agent server is missing the configured switchover")
	}
	its.Spec.LifecycleActions.Switchover.PreCondition = ptr.To(apps.ComponentReadyPreConditionType)
	if err := Validate(its); err == nil {
		t.Fatal("new switchover accepts a Component dependency")
	}
}
