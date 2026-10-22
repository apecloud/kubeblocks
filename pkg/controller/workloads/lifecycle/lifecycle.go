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

// Package lifecycle provides workload state and data-task helpers shared by InstanceSet and Instance controllers.
package lifecycle

import (
	"encoding/json"
	"fmt"
	"slices"
	"sort"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/utils/ptr"

	apps "github.com/apecloud/kubeblocks/apis/apps/v1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/instancetemplate"
	publiclifecycle "github.com/apecloud/kubeblocks/pkg/controller/lifecycle"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	"github.com/apecloud/kubeblocks/pkg/kbagent"
	"github.com/apecloud/kubeblocks/pkg/kbagent/proto"
)

const taskEnv = "KB_AGENT_TASK"

func Enabled(its *workloads.InstanceSet) bool {
	a := its.Spec.LifecycleActions
	return a != nil && (a.MemberJoin != nil || a.MemberLeave != nil || a.DataDump != nil || a.DataLoad != nil)
}

func HasData(its *workloads.InstanceSet) bool {
	return its.Spec.LifecycleActions != nil && (its.Spec.LifecycleActions.DataDump != nil || its.Spec.LifecycleActions.DataLoad != nil)
}

func RetirementBlocked(its *workloads.InstanceSet, old *workloads.InstanceStatus) bool {
	if !Enabled(its) || old == nil || old.EffectiveDesiredState() == workloads.InstanceDesiredStateOffline {
		return false
	}
	established := ptr.Deref(old.MemberJoined, false) || (old.MemberJoined == nil && old.Provisioned)
	donor := established && (old.DataLoaded == nil || *old.DataLoaded)
	return slices.ContainsFunc(its.Status.InstanceStatus, func(status workloads.InstanceStatus) bool {
		if status.EffectiveDesiredState() != workloads.InstanceDesiredStateActive {
			return false
		}
		pendingData := donor && HasData(its) && status.DataLoaded != nil && !*status.DataLoaded
		pendingJoin := established && its.Spec.LifecycleActions.MemberJoin != nil && status.MemberJoined != nil && !*status.MemberJoined
		return pendingData || pendingJoin
	})
}

func Validate(its *workloads.InstanceSet) error {
	if !Enabled(its) {
		return nil
	}
	a := its.Spec.LifecycleActions
	for _, action := range []*apps.Action{a.MemberJoin, a.MemberLeave, a.Switchover, a.DataDump, a.DataLoad} {
		if action != nil && !action.Defined() {
			return fmt.Errorf("configured lifecycle actions must define an execution handler")
		}
		if action != nil && action.PreCondition != nil && *action.PreCondition != apps.ImmediatelyPreConditionType {
			return fmt.Errorf("workload-owned actions support only Immediately preconditions; runtime availability is observed by the workload controller")
		}
		if action != nil && action.NonBlocking {
			return fmt.Errorf("workload-owned lifecycle actions must use blocking execution")
		}
	}
	if HasData(its) && (a.DataDump == nil || a.DataLoad == nil) {
		return fmt.Errorf("data copying requires dataDump and dataLoad")
	}
	return nil
}

// Register records the current allocation before runtime creation or retirement.
// Existing resources prevent a lost observation from being classified as empty data.
func Register(its *workloads.InstanceSet, assignments []instancetemplate.Assignment, runtimeNames, resourceNames sets.Set[string]) bool {
	if !Enabled(its) {
		return false
	}
	bootstrap := its.Status.ObservedGeneration == 0 && len(its.Status.InstanceStatus) == 0 && runtimeNames.Len() == 0 && resourceNames.Len() == 0
	desired := sets.New[string]()
	for _, a := range assignments {
		desired.Insert(a.InstanceName)
	}
	known := make(map[string]int, len(its.Status.InstanceStatus))
	changed := false
	for i, s := range its.Status.InstanceStatus {
		known[s.PodName] = i
		if HasData(its) && !desired.Has(s.PodName) && !slices.Contains(its.Spec.OfflineInstances, s.PodName) && s.EffectiveDesiredState() != workloads.InstanceDesiredStateReleased {
			its.Status.InstanceStatus[i].DesiredState = workloads.InstanceDesiredStateReleased
			changed = true
		}
	}
	for _, a := range assignments {
		if i, ok := known[a.InstanceName]; ok {
			status := &its.Status.InstanceStatus[i]
			if HasData(its) && status.EffectiveDesiredState() == workloads.InstanceDesiredStateReleased && !runtimeNames.Has(a.InstanceName) && !resourceNames.Has(a.InstanceName) {
				status.DesiredState = workloads.InstanceDesiredStateActive
				status.TemplateName = ptr.To(a.TemplateName)
				status.DataLoaded = ptr.To(false)
				changed = true
			}
			continue
		}
		status := workloads.InstanceStatus{PodName: a.InstanceName, TemplateName: ptr.To(a.TemplateName), DesiredState: workloads.InstanceDesiredStateActive, CurrentState: workloads.InstanceCurrentStateAbsent}
		if !bootstrap && !runtimeNames.Has(a.InstanceName) {
			if HasData(its) && !resourceNames.Has(a.InstanceName) {
				status.DataLoaded = ptr.To(false)
			}
			if its.Spec.LifecycleActions.MemberJoin != nil {
				status.MemberJoined = ptr.To(false)
			}
		}
		known[a.InstanceName] = len(its.Status.InstanceStatus)
		its.Status.InstanceStatus = append(its.Status.InstanceStatus, status)
		changed = true
	}
	if bootstrap && len(assignments) == 0 {
		its.Status.ObservedGeneration = its.Generation
		changed = true
	}
	return changed
}

func Status(its *workloads.InstanceSet, name string) *workloads.InstanceStatus {
	for i := range its.Status.InstanceStatus {
		if its.Status.InstanceStatus[i].PodName == name {
			return &its.Status.InstanceStatus[i]
		}
	}
	return nil
}

func NeedsLeave(its *workloads.InstanceSet, status *workloads.InstanceStatus) bool {
	if !Enabled(its) || its.Spec.LifecycleActions.MemberLeave == nil || status == nil {
		return false
	}
	return ptr.Deref(status.MemberJoined, false) || (status.MemberJoined == nil && status.Provisioned)
}

func NeedsMemberRetirement(its *workloads.InstanceSet, status *workloads.InstanceStatus) bool {
	if NeedsLeave(its, status) {
		return true
	}
	return Enabled(its) && its.Spec.LifecycleActions.MemberJoin != nil && status != nil && ptr.Deref(status.MemberJoined, false)
}

// DataTask returns the workload-owned data task matching this instance.
func DataTask(spec *corev1.PodSpec, name string) (*proto.Task, error) {
	for _, worker := range spec.InitContainers {
		if worker.Name != kbagent.ContainerName4Worker {
			continue
		}
		var tasks []proto.Task
		if value := envValueForTask(worker.Env); value != "" {
			if err := json.Unmarshal([]byte(value), &tasks); err != nil {
				return nil, err
			}
		}
		for _, task := range tasks {
			if generatedDataTask(task, name) {
				return &task, nil
			}
		}
	}
	return nil, nil
}

// ObserveData retains committed results and requires a matching task and successful worker termination.
func ObserveData(status *workloads.InstanceStatus, pod *corev1.Pod) (bool, error) {
	if status == nil || pod == nil || ptr.Deref(status.DataLoaded, false) {
		return false, nil
	}
	task, err := DataTask(&pod.Spec, pod.Name)
	if err != nil || task == nil {
		return false, err
	}
	complete := slices.ContainsFunc(pod.Status.InitContainerStatuses, func(s corev1.ContainerStatus) bool {
		return s.Name == kbagent.ContainerName4Worker && s.State.Terminated != nil && s.State.Terminated.ExitCode == 0
	})
	if complete {
		status.DataLoaded = ptr.To(true)
		return true, nil
	}
	if status.DataLoaded == nil {
		status.DataLoaded = ptr.To(false)
		return true, nil
	}
	return false, nil
}

func generatedDataTask(task proto.Task, name string) bool {
	return task.Instance != "" && task.Task == "newReplica" && task.NewReplica != nil && task.UID != "" && task.UID == task.Replicas && task.NewReplica.Replicas == task.Replicas && (name == "" || task.Replicas == name)
}

// SortDataSourcesFirst places pending data targets after other instances, preserving order within each group.
func SortDataSourcesFirst(its *workloads.InstanceSet, names []string) {
	if !HasData(its) {
		return
	}
	pending := sets.New[string]()
	for _, status := range its.Status.InstanceStatus {
		if status.DataLoaded != nil && !*status.DataLoaded {
			pending.Insert(status.PodName)
		}
	}
	sort.SliceStable(names, func(i, j int) bool {
		return !pending.Has(names[i]) && pending.Has(names[j])
	})
}

func SourcePod(its *workloads.InstanceSet, pods []*corev1.Pod, target string, current *corev1.PodSpec) (*corev1.Pod, error) {
	candidates := slices.Clone(pods)
	slices.SortFunc(candidates, func(a, b *corev1.Pod) int { return strings.Compare(a.Name, b.Name) })
	available := make([]*corev1.Pod, 0, len(candidates))
	for _, pod := range candidates {
		if pod.Name == target || pod.DeletionTimestamp != nil || !intctrlutil.IsPodAvailable(pod, its.Spec.MinReadySeconds) {
			continue
		}
		if status := Status(its, pod.Name); status != nil {
			state := status.EffectiveDesiredState()
			if state == workloads.InstanceDesiredStateOffline || (status.DataLoaded != nil && !*status.DataLoaded) {
				continue
			}
			if state == workloads.InstanceDesiredStateReleased && status.MemberJoined != nil && !*status.MemberJoined {
				continue
			}
		}
		if port, err := intctrlutil.GetPortByName(*pod, kbagent.ContainerName, kbagent.DefaultStreamingPortName); err == nil && port > 0 && pod.Status.PodIP != "" {
			available = append(available, pod)
		}
	}
	if len(available) > 0 {
		action := its.Spec.LifecycleActions.DataDump
		selector := action.TargetPodSelector
		if selector == "" && action.Exec != nil {
			selector = action.Exec.TargetPodSelector
		}
		if selector != "" && selector != apps.AnyReplica {
			selected, err := publiclifecycle.SelectTargetPods(available, available[0], action)
			if err != nil {
				return nil, err
			}
			available = selected
		}
	}
	if current != nil {
		task, err := DataTask(current, target)
		if err != nil {
			return nil, err
		}
		if task != nil {
			for _, pod := range available {
				port, _ := intctrlutil.GetPortByName(*pod, kbagent.ContainerName, kbagent.DefaultStreamingPortName)
				if pod.Status.PodIP == task.NewReplica.Remote && port == task.NewReplica.Port {
					return pod, nil
				}
			}
		}
	}

	if len(available) > 0 {
		return available[0], nil
	}
	return nil, fmt.Errorf("instance %s is waiting for an available data source", target)
}

func NewLifecycle(its *workloads.InstanceSet, pod *corev1.Pod, pods []*corev1.Pod) (publiclifecycle.Lifecycle, error) {
	a := its.Spec.LifecycleActions
	return publiclifecycle.New(its.Namespace, its.Labels[constant.AppInstanceLabelKey], its.Labels[constant.KBAppComponentLabelKey], &apps.ComponentLifecycleActions{Switchover: a.Switchover, Reconfigure: a.Reconfigure, MemberJoin: a.MemberJoin, MemberLeave: a.MemberLeave}, a.TemplateVars, pod, pods)
}

// ConfigurePod merges actions into the agent runtime supplied by the caller.
func ConfigurePod(its *workloads.InstanceSet, pod *corev1.Pod) error {
	if !Enabled(its) {
		return nil
	}
	if err := Validate(its); err != nil {
		return err
	}
	a := its.Spec.LifecycleActions
	serverIndex := slices.IndexFunc(pod.Spec.Containers, func(c corev1.Container) bool { return c.Name == kbagent.ContainerName })
	if serverIndex < 0 {
		return fmt.Errorf("instance %s requires a prepared kbagent server container", pod.Name)
	}
	server := &pod.Spec.Containers[serverIndex]
	serverActions := *a
	serverActions.DataLoad = nil
	if err := mergeActions(server, &serverActions); err != nil {
		return err
	}
	if !HasData(its) {
		return nil
	}
	index := slices.IndexFunc(pod.Spec.InitContainers, func(c corev1.Container) bool { return c.Name == kbagent.ContainerName4Worker })
	if index < 0 {
		return fmt.Errorf("instance %s requires a prepared kbagent worker init container", pod.Name)
	}
	worker := &pod.Spec.InitContainers[index]
	workerActions := &workloads.LifecycleActions{DataLoad: a.DataLoad}
	if err := mergeActions(worker, workerActions); err != nil {
		return err
	}
	return nil
}

// ConfigureDataTask prepares a pending target using the selected healthy source.
func ConfigureDataTask(its *workloads.InstanceSet, pod, source *corev1.Pod) error {
	index := slices.IndexFunc(pod.Spec.InitContainers, func(c corev1.Container) bool { return IsDataWorker(&c) })
	if index < 0 {
		return fmt.Errorf("instance %s requires a prepared kbagent worker init container", pod.Name)
	}
	worker := &pod.Spec.InitContainers[index]
	if source == nil {
		return fmt.Errorf("instance %s is waiting for an available data source", pod.Name)
	}
	port, err := intctrlutil.GetPortByName(*source, kbagent.ContainerName, kbagent.DefaultStreamingPortName)
	if err != nil {
		return err
	}
	if source.Status.PodIP == "" {
		return fmt.Errorf("source instance %s has no observed runtime address", source.Name)
	}
	task := proto.Task{Instance: its.Name, Task: "newReplica", UID: pod.Name, Replicas: pod.Name, NewReplica: &proto.NewReplicaTask{Remote: source.Status.PodIP, Port: port, Replicas: pod.Name, Parameters: its.Spec.LifecycleActions.TemplateVars}}
	var tasks []proto.Task
	if existing := envValueForTask(worker.Env); existing != "" {
		if err := json.Unmarshal([]byte(existing), &tasks); err != nil {
			return err
		}
	}
	tasks = slices.DeleteFunc(tasks, func(t proto.Task) bool { return generatedDataTask(t, pod.Name) })
	tasks = append(tasks, task)
	env, err := kbagent.BuildEnv4Worker(tasks)
	if err != nil {
		return err
	}
	setEnv(worker, *env)
	return nil
}

func setEnv(c *corev1.Container, env corev1.EnvVar) {
	i := slices.IndexFunc(c.Env, func(e corev1.EnvVar) bool { return e.Name == env.Name })
	if i < 0 {
		c.Env = append(c.Env, env)
	} else {
		c.Env[i] = env
	}
}

func mergeActions(container *corev1.Container, a *workloads.LifecycleActions) error {
	var actions []proto.Action
	var streaming []string
	for _, e := range container.Env {
		if e.Name == "KB_AGENT_ACTION" && e.Value != "" {
			if err := json.Unmarshal([]byte(e.Value), &actions); err != nil {
				return err
			}
		}
		if e.Name == "KB_AGENT_STREAMING" && e.Value != "" {
			streaming = strings.Split(e.Value, ",")
		}
	}
	defs := []struct {
		name   string
		action *apps.Action
	}{{"memberJoin", a.MemberJoin}, {"memberLeave", a.MemberLeave}, {"switchover", a.Switchover}, {"dataDump", a.DataDump}, {"dataLoad", a.DataLoad}}
	for _, def := range defs {
		action := Action(def.name, def.action)
		if action == nil {
			continue
		}
		i := slices.IndexFunc(actions, func(a proto.Action) bool { return a.Name == def.name })
		if i < 0 {
			actions = append(actions, *action)
		} else {
			actions[i] = *action
		}
		if def.name == "dataDump" || def.name == "dataLoad" {
			if !slices.Contains(streaming, def.name) {
				streaming = append(streaming, def.name)
			}
		}
		if def.action.Exec != nil {
			for _, env := range def.action.Exec.Env {
				setEnv(container, env)
			}
		}
	}
	envs, err := kbagent.BuildEnv4Server(actions, nil, streaming)
	if err != nil {
		return err
	}
	for _, env := range envs {
		if env.Name == "KB_AGENT_PROBE" || (IsDataWorker(container) && env.Name != "KB_AGENT_ACTION" && env.Name != "KB_AGENT_STREAMING") {
			continue
		}
		setEnv(container, env)
	}
	return nil
}

// Action translates the public Action API without depending on the Component controller.
func Action(name string, action *apps.Action) *proto.Action {
	if !action.Defined() {
		return nil
	}
	a := &proto.Action{Name: name, NonBlocking: action.NonBlocking, TimeoutSeconds: action.TimeoutSeconds}
	if action.Exec != nil {
		a.Exec = &proto.ExecAction{Commands: action.Exec.Command, Args: action.Exec.Args}
	}
	if action.HTTP != nil {
		h := action.HTTP
		a.HTTP = &proto.HTTPAction{Port: h.Port, Host: h.Host, Scheme: h.Scheme, Path: h.Path, Method: h.Method, Body: h.Body}
		for _, header := range h.Headers {
			a.HTTP.Headers = append(a.HTTP.Headers, proto.HTTPHeader{Name: header.Name, Value: header.Value})
		}
	}
	if action.GRPC != nil {
		g := action.GRPC
		a.GRPC = &proto.GRPCAction{Port: g.Port, Host: g.Host, Service: g.Service, Method: g.Method, Request: g.Request, Response: proto.GRPCResponse{Status: g.Response.Status, Message: g.Response.Message}}
	}
	if action.RetryPolicy != nil {
		a.RetryPolicy = publiclifecycle.BuildKBAgentRetryPolicy(action.RetryPolicy)
	}
	return a
}

// CriticalStatusChanged reports facts that must be committed before changing runtime inputs or membership.
// Ordinary health, revision, and progress observations do not stop the reconciliation chain.
func CriticalStatusChanged(previous, current []workloads.InstanceStatus) bool {
	for _, status := range current {
		for _, old := range previous {
			if status.PodName != old.PodName {
				continue
			}
			if status.Provisioned != old.Provisioned || !sameBoolResult(status.DataLoaded, old.DataLoaded) || !sameBoolResult(status.MemberJoined, old.MemberJoined) {
				return true
			}
			break
		}
	}
	return false
}
func sameBoolResult(a, b *bool) bool {
	return (a == nil && b == nil) || (a != nil && b != nil && *a == *b)
}

func CleanTask(spec *corev1.PodSpec, name string) (bool, error) {
	changed := false
	for i := range spec.InitContainers {
		worker := &spec.InitContainers[i]
		if worker.Name != kbagent.ContainerName4Worker {
			continue
		}
		for j := 0; j < len(worker.Env); j++ {
			if worker.Env[j].Name != taskEnv {
				continue
			}
			value := worker.Env[j].Value
			if value == "" {
				continue
			}
			var tasks []proto.Task
			if err := json.Unmarshal([]byte(value), &tasks); err != nil {
				return false, err
			}
			before := len(tasks)
			tasks = slices.DeleteFunc(tasks, func(task proto.Task) bool {
				return generatedDataTask(task, name)
			})
			if before == len(tasks) {
				continue
			}
			changed = true
			if len(tasks) == 0 {
				worker.Env = append(worker.Env[:j], worker.Env[j+1:]...)
				j--
			} else {
				encoded, err := json.Marshal(tasks)
				if err != nil {
					return false, err
				}
				worker.Env[j].Value = string(encoded)
			}
		}
	}
	return changed, nil
}

// FilterTransient excludes only workload-generated task inputs from revisions.
func FilterTransient(spec *corev1.PodSpec) { _, _ = CleanTask(spec, "") }

// ValidateExecution keeps AnyReplica execution on the endpoint validated in this invocation.
func ValidateExecution(action *apps.Action, target *corev1.Pod, pods []*corev1.Pod) (*publiclifecycle.Options, error) {
	if target == nil {
		return nil, fmt.Errorf("member target runtime is absent")
	}
	if len(pods) == 0 {
		return nil, fmt.Errorf("no available executor for member action")
	}
	selected, err := publiclifecycle.SelectTargetPods(pods, target, action)
	if err != nil {
		return nil, err
	}
	if len(selected) == 0 {
		return nil, fmt.Errorf("no available executor for member action")
	}
	for _, pod := range selected {
		port, err := intctrlutil.GetPortByName(*pod, kbagent.ContainerName, kbagent.DefaultHTTPPortName)
		if err != nil || port <= 0 || pod.Status.PodIP == "" || pod.DeletionTimestamp != nil {
			return nil, fmt.Errorf("instance %s has no available agent endpoint", pod.Name)
		}
	}
	selector := action.TargetPodSelector
	if selector == "" && action.Exec != nil {
		selector = action.Exec.TargetPodSelector
	}
	if selector == apps.AnyReplica {
		return &publiclifecycle.Options{TargetPodName: selected[0].Name}, nil
	}
	return nil, nil
}

func envValueForTask(envs []corev1.EnvVar) string {
	for _, env := range envs {
		if env.Name == taskEnv {
			return env.Value
		}
	}
	return ""
}

func IsDataWorker(container *corev1.Container) bool {
	return container.Name == kbagent.ContainerName4Worker
}

func GeneratedTask(spec *corev1.PodSpec) (string, error) {
	task, err := DataTask(spec, "")
	if err != nil || task == nil {
		return "", err
	}
	data, err := json.Marshal(task)
	return string(data), err
}
