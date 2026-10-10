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
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/utils/ptr"

	apps "github.com/apecloud/kubeblocks/apis/apps/v1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/controller/instancetemplate"
	"github.com/apecloud/kubeblocks/pkg/kbagent"
	"github.com/apecloud/kubeblocks/pkg/kbagent/proto"
)

func testAction(command string) *apps.Action {
	return &apps.Action{Exec: &apps.ExecAction{Command: []string{"/bin/sh", "-c", command}}}
}
func dataWorkload() *workloads.InstanceSet {
	return &workloads.InstanceSet{ObjectMeta: metav1.ObjectMeta{Name: "demo", Namespace: "runtime", Generation: 1}, Spec: workloads.InstanceSetSpec{LifecycleActions: &workloads.LifecycleActions{MemberJoin: testAction("true"), MemberLeave: testAction("true"), DataDump: testAction("printf data"), DataLoad: testAction("cat > /data/value"), DataVolume: "data", Worker: &corev1.Container{Name: "worker-context", Image: "tools", VolumeMounts: []corev1.VolumeMount{{Name: "data", MountPath: "/data"}}}}}}
}
func dataPod() *corev1.Pod {
	return &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "demo-1", Namespace: "runtime"}, Spec: corev1.PodSpec{ServiceAccountName: "loader", Containers: []corev1.Container{{Name: "db", Image: "db"}}, Volumes: []corev1.Volume{{Name: "data", VolumeSource: corev1.VolumeSource{PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: "data-demo-1"}}}}}}
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
	its := dataWorkload()
	assignments := []instancetemplate.Assignment{{InstanceName: "demo-0"}, {InstanceName: "demo-1", TemplateName: "alternate"}}
	if !Register(its, assignments, sets.New[string](), sets.New[string]()) {
		t.Fatal("bootstrap not registered")
	}
	for _, status := range its.Status.InstanceStatus {
		if status.DataLoaded != nil || status.MemberJoined != nil || status.Provisioned {
			t.Fatalf("bootstrap invented results: %+v", status)
		}
	}
	assignments = append(assignments, instancetemplate.Assignment{InstanceName: "demo-2"})
	if !Register(its, assignments, sets.New[string](), sets.New[string]()) {
		t.Fatal("new identity not registered")
	}
	status := Status(its, "demo-2")
	if status.DataLoaded == nil || *status.DataLoaded || status.MemberJoined == nil || *status.MemberJoined {
		t.Fatalf("new empty allocation not tracked: %+v", status)
	}
	its = dataWorkload()
	if !Register(its, nil, sets.New[string](), sets.New[string]()) || its.Status.ObservedGeneration != 1 {
		t.Fatal("zero bootstrap did not record processed generation")
	}
	if !Register(its, assignments[:1], sets.New[string](), sets.New[string]()) || Status(its, "demo-0").DataLoaded == nil {
		t.Fatal("zero replica increase was treated as bootstrap")
	}
	its = dataWorkload()
	Register(its, assignments, sets.New("demo-1"), sets.New("demo-0"))
	if Status(its, "demo-0").DataLoaded != nil || Status(its, "demo-1").MemberJoined != nil {
		t.Fatal("unobserved old resources were classified as empty")
	}
}

func TestConfigurePodKeepsBootstrapIndependentAndBuildsTrackedTask(t *testing.T) {
	its := dataWorkload()
	pod := dataPod()
	if err := ConfigurePod(its, pod, nil, false, false); err != nil {
		t.Fatal(err)
	}
	worker := pod.Spec.InitContainers[0]
	if envValue(worker, resultEnv) != "" || envValue(worker, taskEnv) != "" {
		t.Fatal("bootstrap requires a load result")
	}
	var actions []proto.Action
	if err := json.Unmarshal([]byte(envValue(pod.Spec.Containers[1], "KB_AGENT_ACTION")), &actions); err != nil {
		t.Fatal(err)
	}
	if len(actions) != 4 || !strings.Contains(envValue(pod.Spec.Containers[1], "KB_AGENT_STREAMING"), "dataDump") {
		t.Fatal("source streaming actions not registered")
	}
	source := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "demo-0", Namespace: "runtime"}, Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: kbagent.ContainerName, Ports: []corev1.ContainerPort{{Name: kbagent.DefaultStreamingPortName, ContainerPort: 3502}}}}}, Status: corev1.PodStatus{PodIP: "10.0.0.2"}}
	if err := ConfigurePod(its, pod, source, true, false); err != nil {
		t.Fatal(err)
	}
	worker = pod.Spec.InitContainers[0]
	var tasks []proto.Task
	if err := json.Unmarshal([]byte(envValue(worker, taskEnv)), &tasks); err != nil {
		t.Fatal(err)
	}
	task := tasks[0]
	if task.NotifyAtFinish || task.ReportPeriodSeconds != 0 || task.NewReplica.DataLoadResult.Namespace != "runtime" || task.NewReplica.DataLoadResult.PVCName != "data-demo-1" || task.NewReplica.Remote != "10.0.0.2" {
		t.Fatalf("wrong new task %+v", task)
	}
	before := pod.Spec.DeepCopy()
	FilterTransient(before)
	if err := ConfigurePod(its, pod, nil, true, true); err != nil {
		t.Fatal(err)
	}
	if envValue(pod.Spec.InitContainers[0], taskEnv) != "" || envValue(pod.Spec.InitContainers[0], resultEnv) == "" {
		t.Fatal("cleanup removed startup guard or left task")
	}
	after := pod.Spec.DeepCopy()
	FilterTransient(after)
	if !reflect.DeepEqual(before, after) {
		t.Fatal("task cleanup changed persistent revision inputs")
	}
	after.InitContainers[0].VolumeMounts[0].MountPath = "/changed"
	if reflect.DeepEqual(before, after) {
		t.Fatal("real mount configuration excluded")
	}
}
func TestMemberOnlyMergesActionsAndPreservesServerConfiguration(t *testing.T) {
	its := dataWorkload()
	its.Spec.LifecycleActions.DataDump = nil
	its.Spec.LifecycleActions.DataLoad = nil
	its.Spec.LifecycleActions.Worker = nil
	pod := dataPod()
	existing, _ := json.Marshal([]proto.Action{{Name: "reconfigure", Exec: &proto.ExecAction{Commands: []string{"old"}}}})
	pod.Spec.Containers = append(pod.Spec.Containers, corev1.Container{Name: kbagent.ContainerName, Image: "tools", Env: []corev1.EnvVar{{Name: "KB_AGENT_ACTION", Value: string(existing)}, {Name: "KB_AGENT_PROBE", Value: "existing-probe"}, {Name: "BUSINESS", Value: "keep"}}})
	if err := ConfigurePod(its, pod, nil, false, false); err != nil {
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
	if err := ValidateExecution(its.Spec.LifecycleActions.MemberJoin, pod, []*corev1.Pod{pod}); err == nil {
		t.Fatal("missing agent endpoint would record success")
	}
}

func TestCleanupPreservesExplicitTasksAndActualConfiguration(t *testing.T) {
	task, _ := kbagent.BuildEnv4Worker([]proto.Task{{Task: "explicit", UID: "retained"}, {Task: "newReplica", NewReplica: &proto.NewReplicaTask{DataLoadResult: &proto.DataLoadResult{Namespace: "runtime", PVCName: "data"}}}})
	guard, _ := kbagent.BuildEnv4DataLoadResult(&proto.DataLoadResult{Namespace: "runtime", PVCName: "data"})
	spec := &corev1.PodSpec{InitContainers: []corev1.Container{{Name: kbagent.ContainerName4Worker, Image: "tools", Env: []corev1.EnvVar{*task, *guard}}}}
	changed, err := CleanTask(spec)
	if err != nil || !changed {
		t.Fatalf("cleanup: %v %v", changed, err)
	}
	var tasks []proto.Task
	if err := json.Unmarshal([]byte(envValue(spec.InitContainers[0], taskEnv)), &tasks); err != nil || len(tasks) != 1 || tasks[0].UID != "retained" {
		t.Fatal("explicit task discarded")
	}
	changed, err = CleanTask(spec)
	if err != nil || changed {
		t.Fatal("cleanup is not idempotent")
	}
	original := spec.DeepCopy()
	FilterTransient(spec)
	if envValue(spec.InitContainers[0], taskEnv) == "" || spec.InitContainers[0].Image != original.InitContainers[0].Image {
		t.Fatal("revision removed explicit configuration")
	}
}

func TestDataObservationDistinguishesUnavailableAndReplacementPVC(t *testing.T) {
	status := &workloads.InstanceStatus{DataLoaded: ptr.To(true)}
	if ObserveData(status, nil) || !*status.DataLoaded {
		t.Fatal("observation failure cleared result")
	}
	if !ObserveData(status, &corev1.PersistentVolumeClaim{}) || *status.DataLoaded {
		t.Fatal("new unmarked data inherited old result")
	}
	if !ObserveData(status, &corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{Annotations: map[string]string{proto.DataLoadedAnnotationKey: "true"}}}) || !*status.DataLoaded {
		t.Fatal("actual result not published")
	}
	status.DataLoaded = nil
	if ObserveData(status, &corev1.PersistentVolumeClaim{}) || status.DataLoaded != nil {
		t.Fatal("bootstrap acquired tracked result")
	}
}

func TestValidationRejectsUnownedPreconditionsAndNonblockingMembers(t *testing.T) {
	for _, condition := range []apps.PreConditionType{apps.RuntimeReadyPreConditionType, apps.ComponentReadyPreConditionType, apps.ClusterReadyPreConditionType} {
		its := dataWorkload()
		its.Spec.LifecycleActions.MemberJoin.PreCondition = ptr.To(condition)
		if err := Validate(its); err == nil {
			t.Fatalf("accepted unowned precondition %s", condition)
		}
	}
	for _, name := range []string{"join", "leave", "switch"} {
		its := dataWorkload()
		if name == "join" {
			its.Spec.LifecycleActions.MemberJoin.NonBlocking = true
		} else if name == "leave" {
			its.Spec.LifecycleActions.MemberLeave.NonBlocking = true
		} else {
			its.Spec.LifecycleActions.Switchover = testAction("true")
			its.Spec.LifecycleActions.Switchover.NonBlocking = true
		}
		if err := Validate(its); err == nil {
			t.Fatalf("accepted nonblocking %s without a completion contract", name)
		}
	}
}

func TestDataTaskUsesObservedSourceAddressWhenHeadlessDNSIsUnavailable(t *testing.T) {
	for _, disabled := range []bool{false, true} {
		its := dataWorkload()
		its.Spec.DisableDefaultHeadlessService = disabled
		source := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "demo-0", Namespace: "runtime"}, Spec: corev1.PodSpec{Subdomain: "demo-headless", Containers: []corev1.Container{{Name: kbagent.ContainerName, Ports: []corev1.ContainerPort{{Name: kbagent.DefaultStreamingPortName, ContainerPort: 3502}}}}}, Status: corev1.PodStatus{PodIP: "10.0.0.1"}}
		pod := dataPod()
		if err := ConfigurePod(its, pod, source, true, false); err != nil {
			t.Fatal(err)
		}
		first, err := GeneratedTask(&pod.Spec)
		if err != nil {
			t.Fatal(err)
		}
		source.Status.PodIP = "10.0.0.99"
		if err := ConfigurePod(its, pod, source, true, false); err != nil {
			t.Fatal(err)
		}
		current, err := GeneratedTask(&pod.Spec)
		if err != nil || current == first {
			t.Fatalf("source replacement did not refresh current task: %v", err)
		}
		var task proto.Task
		if err := json.Unmarshal([]byte(current), &task); err != nil {
			t.Fatal(err)
		}
		if task.NewReplica.Remote != "10.0.0.99" {
			t.Fatalf("task guessed DNS instead of observed runtime address: %+v", task)
		}
	}
}

func TestTaskCleanupMatchesExactRuntimeDataResult(t *testing.T) {
	own := &proto.DataLoadResult{Namespace: "one", PVCName: "data"}
	other := &proto.DataLoadResult{Namespace: "two", PVCName: "data"}
	env, err := kbagent.BuildEnv4Worker([]proto.Task{{UID: "own", NewReplica: &proto.NewReplicaTask{DataLoadResult: own}}, {UID: "other", NewReplica: &proto.NewReplicaTask{DataLoadResult: other}}})
	if err != nil {
		t.Fatal(err)
	}
	cm := &corev1.ConfigMap{Data: map[string]string{taskEnv: env.Value}}
	changed, err := CleanTaskConfigMap(cm, own)
	if err != nil || !changed {
		t.Fatalf("cleanup %v %v", changed, err)
	}
	var tasks []proto.Task
	if err := json.Unmarshal([]byte(cm.Data[taskEnv]), &tasks); err != nil || len(tasks) != 1 || tasks[0].UID != "other" {
		t.Fatalf("cleanup removed another runtime's task: %+v %v", tasks, err)
	}
}

func TestStandaloneAgentRegistersSwitchover(t *testing.T) {
	its := dataWorkload()
	its.Spec.LifecycleActions.Switchover = testAction("switch")
	pod := dataPod()
	if err := ConfigurePod(its, pod, nil, false, false); err != nil {
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
