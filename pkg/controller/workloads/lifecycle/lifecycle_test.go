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
	status := Status(its, "demo-2")
	if status.MemberJoined == nil || *status.MemberJoined {
		t.Fatalf("new empty allocation not tracked: %+v", status)
	}
	its = memberWorkload()
	if !Register(its, nil, sets.New[string](), sets.New[string]()) || its.Status.ObservedGeneration != 1 {
		t.Fatal("zero bootstrap did not record processed generation")
	}
	if !Register(its, assignments[:1], sets.New[string](), sets.New[string]()) || Status(its, "demo-0").MemberJoined == nil {
		t.Fatal("zero replica increase was treated as bootstrap")
	}
	its = memberWorkload()
	Register(its, assignments, sets.New("demo-1"), sets.New("demo-0"))
	if Status(its, "demo-1").MemberJoined != nil {
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
			lfa, err := NewLifecycle(its, pods[0], pods)
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

func TestStandaloneAgentRegistersSwitchover(t *testing.T) {
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
		t.Fatal("standalone server cannot execute its configured switchover")
	}
	its.Spec.LifecycleActions.Switchover.PreCondition = ptr.To(apps.ComponentReadyPreConditionType)
	if err := Validate(its); err == nil {
		t.Fatal("new switchover accepts a Component dependency")
	}
}

func dataWorkload() *workloads.InstanceSet {
	return &workloads.InstanceSet{ObjectMeta: metav1.ObjectMeta{Name: "demo", Namespace: "runtime", Generation: 1}, Spec: workloads.InstanceSetSpec{LifecycleActions: &workloads.LifecycleActions{MemberJoin: testAction("true"), MemberLeave: testAction("true"), DataDump: testAction("printf data"), DataLoad: testAction("cat > /data/value")}}}
}

func dataPod() *corev1.Pod {
	return &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "demo-1", Namespace: "runtime"}, Spec: corev1.PodSpec{ServiceAccountName: "loader", Containers: []corev1.Container{{Name: "db", Image: "db"}, {Name: kbagent.ContainerName, Image: "tools"}}, InitContainers: []corev1.Container{{Name: kbagent.ContainerName4Worker, Image: "tools", Args: []string{"--server=false"}, VolumeMounts: []corev1.VolumeMount{{Name: "data", MountPath: "/data"}}}}, Volumes: []corev1.Volume{{Name: "data", VolumeSource: corev1.VolumeSource{PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: "data-demo-1"}}}}}}
}

func TestCleanupPreservesExplicitTasksAndActualConfiguration(t *testing.T) {
	pod := dataPod()
	env, _ := kbagent.BuildEnv4Worker([]proto.Task{{Task: "explicit", UID: "retained"}, {Instance: "demo", Task: "newReplica", UID: pod.Name, Replicas: pod.Name, NewReplica: &proto.NewReplicaTask{Replicas: pod.Name}}})
	pod.Spec.InitContainers[0].Env = []corev1.EnvVar{*env, {Name: "USER", Value: "keep"}}
	before := pod.Spec.DeepCopy()
	if changed, err := CleanTask(&pod.Spec, pod.Name); err != nil || !changed {
		t.Fatalf("cleanup=%v, %v", changed, err)
	}
	var tasks []proto.Task
	if err := json.Unmarshal([]byte(envValue(pod.Spec.InitContainers[0], taskEnv)), &tasks); err != nil || len(tasks) != 1 || tasks[0].UID != "retained" {
		t.Fatalf("remaining tasks=%v, %v", tasks, err)
	}
	FilterTransient(before)
	if !reflect.DeepEqual(before, &pod.Spec) {
		t.Fatal("cleanup changed persistent configuration")
	}
}

func TestDataObservationRequiresMatchingSuccessfulWorker(t *testing.T) {
	for _, mode := range []string{"no-task", "other-target", "running", "failed", "other-init", "completed"} {
		t.Run(mode, func(t *testing.T) {
			pod := dataPod()
			target := pod.Name
			if mode == "other-target" {
				target = "other"
			}
			if mode != "no-task" {
				env, _ := kbagent.BuildEnv4Worker([]proto.Task{{Instance: "demo", Task: "newReplica", UID: target, Replicas: target, NewReplica: &proto.NewReplicaTask{Replicas: target}}})
				pod.Spec.InitContainers[0].Env = []corev1.EnvVar{*env}
			}
			state := corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{ExitCode: 0}}
			worker := kbagent.ContainerName4Worker
			if mode == "running" {
				state = corev1.ContainerState{Running: &corev1.ContainerStateRunning{}}
			}
			if mode == "failed" {
				state.Terminated.ExitCode = 1
			}
			if mode == "other-init" {
				worker = "other"
			}
			pod.Status.InitContainerStatuses = []corev1.ContainerStatus{{Name: worker, State: state}}
			status := &workloads.InstanceStatus{DataLoaded: ptr.To(false)}
			if _, err := ObserveData(status, pod); err != nil {
				t.Fatal(err)
			}
			if *status.DataLoaded != (mode == "completed") {
				t.Fatalf("unexpected result=%v", *status.DataLoaded)
			}
			status.DataLoaded = ptr.To(true)
			if _, err := ObserveData(status, pod); err != nil || !*status.DataLoaded {
				t.Fatal("committed result regressed", err)
			}
		})
	}
}

func TestDataSourceRetirementUsesMembershipResult(t *testing.T) {
	for _, tc := range []struct {
		name   string
		status workloads.InstanceStatus
		usable bool
	}{
		{name: "retiring bootstrap", status: workloads.InstanceStatus{DesiredState: workloads.InstanceDesiredStateReleased}, usable: true},
		{name: "retiring member", status: workloads.InstanceStatus{DesiredState: workloads.InstanceDesiredStateReleased, MemberJoined: ptr.To(true)}, usable: true},
		{name: "left member", status: workloads.InstanceStatus{DesiredState: workloads.InstanceDesiredStateReleased, MemberJoined: ptr.To(false)}},
		{name: "offline member", status: workloads.InstanceStatus{DesiredState: workloads.InstanceDesiredStateOffline, MemberJoined: ptr.To(true)}},
		{name: "loading member", status: workloads.InstanceStatus{DesiredState: workloads.InstanceDesiredStateActive, DataLoaded: ptr.To(false)}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			its := dataWorkload()
			tc.status.PodName = "demo-0"
			its.Status.InstanceStatus = []workloads.InstanceStatus{tc.status}
			pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "demo-0"}, Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: kbagent.ContainerName, Ports: []corev1.ContainerPort{{Name: kbagent.DefaultStreamingPortName, ContainerPort: 3502}}}}}, Status: corev1.PodStatus{Phase: corev1.PodRunning, PodIP: "10.0.0.1", Conditions: []corev1.PodCondition{{Type: corev1.PodReady, Status: corev1.ConditionTrue}}}}
			source, err := SourcePod(its, []*corev1.Pod{pod}, "demo-1", nil)
			if tc.usable {
				if err != nil || source != pod {
					t.Fatalf("live source was rejected: %v", err)
				}
			} else if err == nil || source != nil {
				t.Fatalf("unusable source was selected: %v %v", source, err)
			}
		})
	}
}

func TestTaskCleanupPreservesOtherInstances(t *testing.T) {
	own := proto.Task{Instance: "demo", Task: "newReplica", UID: "demo-1", Replicas: "demo-1", NewReplica: &proto.NewReplicaTask{Replicas: "demo-1"}}
	other := proto.Task{Instance: "demo", Task: "newReplica", UID: "demo-2", Replicas: "demo-2", NewReplica: &proto.NewReplicaTask{Replicas: "demo-2"}}
	env, _ := kbagent.BuildEnv4Worker([]proto.Task{own, other})
	cm := &corev1.ConfigMap{Data: map[string]string{taskEnv: env.Value}}
	if changed, err := CleanTaskConfigMap(cm, "demo-1"); err != nil || !changed {
		t.Fatalf("cleanup=%v,%v", changed, err)
	}
	var tasks []proto.Task
	if err := json.Unmarshal([]byte(cm.Data[taskEnv]), &tasks); err != nil || len(tasks) != 1 || tasks[0].UID != "demo-2" {
		t.Fatalf("tasks=%v,%v", tasks, err)
	}
}

func TestDataActionsPreservePreparedWorkerEnvironment(t *testing.T) {
	its, pod := dataWorkload(), dataPod()
	identity := corev1.EnvVar{Name: "KB_AGENT_POD_NAME", ValueFrom: &corev1.EnvVarSource{FieldRef: &corev1.ObjectFieldSelector{FieldPath: "metadata.name"}}}
	pod.Spec.InitContainers[0].Env = []corev1.EnvVar{identity, {Name: "USER_CONFIG", Value: "keep"}}
	if err := ConfigurePod(its, pod); err != nil {
		t.Fatal(err)
	}
	worker := &pod.Spec.InitContainers[0]
	if len(worker.Env) != 4 || !reflect.DeepEqual(worker.Env[0], identity) || envValue(*worker, "USER_CONFIG") != "keep" {
		t.Fatalf("action merge changed runtime environment: %#v", worker.Env)
	}
	if task, err := DataTask(&pod.Spec, pod.Name); err != nil || task != nil {
		t.Fatal("action configuration generated a task", err)
	}
	var actions []proto.Action
	if err := json.Unmarshal([]byte(envValue(*worker, "KB_AGENT_ACTION")), &actions); err != nil {
		t.Fatal(err)
	}
	if len(actions) != 1 || actions[0].Name != "dataLoad" || envValue(*worker, "KB_AGENT_STREAMING") != "dataLoad" {
		t.Fatalf("worker actions=%#v", actions)
	}
	pod = dataPod()
	if err := ConfigurePod(its, pod); err != nil {
		t.Fatal(err)
	}
	for _, env := range pod.Spec.InitContainers[0].Env {
		if env.Name == "KB_AGENT_POD_NAME" {
			t.Fatal("workload injected missing caller identity environment")
		}
	}
}
