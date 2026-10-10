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

// Package lifecycle contains the inputs shared by both InstanceSet implementations.
package lifecycle

import (
	"encoding/json"
	"fmt"
	"slices"
	"strconv"
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
const resultEnv = "KB_AGENT_DATA_LOAD_RESULT"

func Enabled(its *workloads.InstanceSet) bool {
	a := its.Spec.LifecycleActions
	return a != nil && (a.MemberJoin != nil || a.MemberLeave != nil || a.DataDump != nil || a.DataLoad != nil)
}

func HasData(its *workloads.InstanceSet) bool {
	return its.Spec.LifecycleActions != nil && (its.Spec.LifecycleActions.DataDump != nil || its.Spec.LifecycleActions.DataLoad != nil)
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
	}
	for _, action := range []*apps.Action{a.MemberJoin, a.MemberLeave, a.Switchover} {
		if action != nil && action.NonBlocking {
			return fmt.Errorf("workload membership and switchover actions must use blocking execution")
		}
	}
	if HasData(its) && (a.DataDump == nil || a.DataLoad == nil || a.Worker == nil || a.Worker.Image == "" || a.DataVolume == "") {
		return fmt.Errorf("data copying requires dataDump, dataLoad, dataVolume and a worker image")
	}
	return nil
}

// Register records a complete current allocation before any runtime is created.
// Existing resources prevent a lost observation from being classified as empty data.
func Register(its *workloads.InstanceSet, assignments []instancetemplate.Assignment, runtimeNames, resourceNames sets.Set[string]) bool {
	if !Enabled(its) {
		return false
	}
	bootstrap := its.Status.ObservedGeneration == 0 && len(its.Status.InstanceStatus) == 0 && runtimeNames.Len() == 0 && resourceNames.Len() == 0
	known := sets.New[string]()
	for _, s := range its.Status.InstanceStatus {
		known.Insert(s.PodName)
	}
	changed := false
	for _, a := range assignments {
		if known.Has(a.InstanceName) {
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
		its.Status.InstanceStatus = append(its.Status.InstanceStatus, status)
		changed = true
	}
	// Zero initial replicas still need an observable first commit. A later increase is expansion.
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

func DataPVCName(its *workloads.InstanceSet, pod *corev1.Pod) (string, error) {
	for _, volume := range pod.Spec.Volumes {
		if volume.Name == its.Spec.LifecycleActions.DataVolume && volume.PersistentVolumeClaim != nil && volume.PersistentVolumeClaim.ClaimName != "" {
			return volume.PersistentVolumeClaim.ClaimName, nil
		}
	}
	return "", fmt.Errorf("instance %s data volume %q must reference a PVC", pod.Name, its.Spec.LifecycleActions.DataVolume)
}

// ObserveData only changes a result after an actual data PVC was observed.
func ObserveData(status *workloads.InstanceStatus, pvc *corev1.PersistentVolumeClaim) bool {
	if status == nil || pvc == nil {
		return false
	}
	complete := pvc.Annotations[proto.DataLoadedAnnotationKey] == "true"
	if status.DataLoaded == nil && !complete {
		return false
	}
	if status.DataLoaded != nil && *status.DataLoaded == complete {
		return false
	}
	status.DataLoaded = ptr.To(complete)
	return true
}

func SourcePod(its *workloads.InstanceSet, pods []*corev1.Pod, target string) (*corev1.Pod, error) {
	candidates := slices.Clone(pods)
	slices.SortFunc(candidates, func(a, b *corev1.Pod) int { return strings.Compare(a.Name, b.Name) })
	available := make([]*corev1.Pod, 0, len(candidates))
	for _, pod := range candidates {
		if pod.Name == target || pod.DeletionTimestamp != nil || !intctrlutil.IsPodAvailable(pod, its.Spec.MinReadySeconds) {
			continue
		}
		if status := Status(its, pod.Name); status != nil && (status.EffectiveDesiredState() != workloads.InstanceDesiredStateActive || (status.DataLoaded != nil && !*status.DataLoaded)) {
			continue
		}
		if _, err := intctrlutil.GetPortByName(*pod, kbagent.ContainerName, kbagent.DefaultStreamingPortName); err == nil {
			available = append(available, pod)
		}
	}
	if len(available) > 0 {
		action := its.Spec.LifecycleActions.DataDump.DeepCopy()
		if action.TargetPodSelector == "" && (action.Exec == nil || action.Exec.TargetPodSelector == "") {
			action.TargetPodSelector = apps.AnyReplica
		}
		selected, err := publiclifecycle.SelectTargetPods(available, available[0], action)
		if err != nil {
			return nil, err
		}
		if len(selected) > 0 {
			return selected[0], nil
		}
	}
	return nil, fmt.Errorf("instance %s is waiting for an available data source", target)
}

func NewLifecycle(its *workloads.InstanceSet, pod *corev1.Pod, pods []*corev1.Pod) (publiclifecycle.Lifecycle, error) {
	a := its.Spec.LifecycleActions
	return publiclifecycle.New(its.Namespace, its.Labels[constant.AppInstanceLabelKey], its.Labels[constant.KBAppComponentLabelKey], &apps.ComponentLifecycleActions{Switchover: a.Switchover, Reconfigure: a.Reconfigure, MemberJoin: a.MemberJoin, MemberLeave: a.MemberLeave}, a.TemplateVars, pod, pods)
}

// ConfigurePod adds persistent action configuration and, for expansion only, a data-result guard.
func ConfigurePod(its *workloads.InstanceSet, pod *corev1.Pod, source *corev1.Pod, trackData, completed bool) error {
	if !Enabled(its) {
		return nil
	}
	if err := Validate(its); err != nil {
		return err
	}
	a := its.Spec.LifecycleActions
	serverIndex := slices.IndexFunc(pod.Spec.Containers, func(c corev1.Container) bool { return c.Name == kbagent.ContainerName })
	var server *corev1.Container
	if serverIndex >= 0 {
		server = &pod.Spec.Containers[serverIndex]
	} else {
		if a.Worker == nil {
			return fmt.Errorf("membership actions require a configured kbagent server container or worker")
		}
		c := *a.Worker.DeepCopy()
		c.Name = kbagent.ContainerName
		if len(c.Command) == 0 {
			c.Command = []string{kbagent.BinaryPath}
		}
		c.Args = append(c.Args, "--port", strconv.Itoa(kbagent.DefaultHTTPPort), "--streaming-port", strconv.Itoa(kbagent.DefaultStreamingPort))
		c.Ports = append(c.Ports, corev1.ContainerPort{Name: kbagent.DefaultHTTPPortName, ContainerPort: kbagent.DefaultHTTPPort}, corev1.ContainerPort{Name: kbagent.DefaultStreamingPortName, ContainerPort: kbagent.DefaultStreamingPort})
		pod.Spec.Containers = append(pod.Spec.Containers, c)
		server = &pod.Spec.Containers[len(pod.Spec.Containers)-1]
	}
	if err := applyExecContext(server, pod, a); err != nil {
		return err
	}
	if err := mergeActions(server, a); err != nil {
		return err
	}
	if !HasData(its) {
		return nil
	}
	if trackData && (pod.Spec.ServiceAccountName == "" || (pod.Spec.AutomountServiceAccountToken != nil && !*pod.Spec.AutomountServiceAccountToken)) {
		return fmt.Errorf("instance %s data worker requires an explicit authorized serviceAccountName and API credentials", pod.Name)
	}
	index := slices.IndexFunc(pod.Spec.InitContainers, func(c corev1.Container) bool { return c.Name == kbagent.ContainerName4Worker })
	var worker corev1.Container
	worker = *a.Worker.DeepCopy()
	worker.Name = kbagent.ContainerName4Worker
	if index >= 0 {
		for _, env := range pod.Spec.InitContainers[index].Env {
			if !slices.ContainsFunc(worker.Env, func(e corev1.EnvVar) bool { return e.Name == env.Name }) {
				worker.Env = append(worker.Env, env)
			}
		}
	}
	if err := applyExecContext(&worker, pod, a); err != nil {
		return err
	}
	if len(worker.Command) == 0 {
		worker.Command = []string{kbagent.BinaryPath}
	}
	worker.Args = append(worker.Args, "--server=false")
	if err := mergeActions(&worker, a); err != nil {
		return err
	}
	setEnv(&worker, corev1.EnvVar{Name: "KB_AGENT_DATA_VOLUME", Value: a.DataVolume})
	if !trackData {
		if index < 0 {
			pod.Spec.InitContainers = append(pod.Spec.InitContainers, worker)
		} else {
			pod.Spec.InitContainers[index] = worker
		}
		return nil
	}
	pvcName, err := DataPVCName(its, pod)
	if err != nil {
		return err
	}
	result := &proto.DataLoadResult{Namespace: pod.Namespace, PVCName: pvcName}
	guard, err := kbagent.BuildEnv4DataLoadResult(result)
	if err != nil {
		return err
	}
	setEnv(&worker, *guard)
	if completed {
		spec := corev1.PodSpec{InitContainers: []corev1.Container{worker}}
		if _, err := CleanTask(&spec); err != nil {
			return err
		}
		worker = spec.InitContainers[0]
	} else {
		if source == nil {
			return fmt.Errorf("instance %s is waiting for an available data source", pod.Name)
		}
		port, err := intctrlutil.GetPortByName(*source, kbagent.ContainerName, kbagent.DefaultStreamingPortName)
		if err != nil {
			return err
		}
		host := source.Status.PodIP
		if host == "" {
			return fmt.Errorf("source instance %s has no observed runtime address", source.Name)
		}
		task := proto.Task{Instance: its.Name, Task: "newReplica", UID: pod.Name, Replicas: pod.Name, NewReplica: &proto.NewReplicaTask{Remote: host, Port: port, Replicas: pod.Name, Parameters: a.TemplateVars, DataLoadResult: result}}
		var tasks []proto.Task
		if existing := envValueForTask(worker.Env); existing != "" {
			if err := json.Unmarshal([]byte(existing), &tasks); err != nil {
				return err
			}
		}
		tasks = slices.DeleteFunc(tasks, func(t proto.Task) bool { return t.NewReplica != nil && sameResult(t.NewReplica.DataLoadResult, result) })
		tasks = append(tasks, task)
		env, err := kbagent.BuildEnv4Worker(tasks)
		if err != nil {
			return err
		}
		setEnv(&worker, *env)
	}
	if index < 0 {
		pod.Spec.InitContainers = append(pod.Spec.InitContainers, worker)
	} else {
		pod.Spec.InitContainers[index] = worker
	}
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
		if env.Name != "KB_AGENT_PROBE" {
			setEnv(container, env)
		}
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

func CleanTask(spec *corev1.PodSpec) (bool, error) {
	changed := false
	for i := range spec.InitContainers {
		c := &spec.InitContainers[i]
		if c.Name != kbagent.ContainerName4Worker || !slices.ContainsFunc(c.Env, func(e corev1.EnvVar) bool { return e.Name == resultEnv }) {
			continue
		}
		for j := 0; j < len(c.Env); j++ {
			if c.Env[j].Name != taskEnv {
				continue
			}
			cm := &corev1.ConfigMap{Data: map[string]string{taskEnv: c.Env[j].Value}}
			var result proto.DataLoadResult
			for _, env := range c.Env {
				if env.Name == resultEnv {
					if err := json.Unmarshal([]byte(env.Value), &result); err != nil {
						return false, err
					}
				}
			}
			cleaned, err := CleanTaskConfigMap(cm, &result)
			if err != nil {
				return false, err
			}
			if !cleaned {
				continue
			}
			changed = true
			if value, ok := cm.Data[taskEnv]; ok {
				c.Env[j].Value = value
			} else {
				c.Env = append(c.Env[:j], c.Env[j+1:]...)
				j--
			}
		}
	}
	return changed, nil
}

// FilterTransient removes generated tasks and normalizes their runtime PVC binding.
// The data volume, startup-check capability, and agent configuration remain revision inputs.
func FilterTransient(spec *corev1.PodSpec) {
	_, _ = CleanTask(spec)
	for i := range spec.InitContainers {
		c := &spec.InitContainers[i]
		if c.Name != kbagent.ContainerName4Worker {
			continue
		}
		volume := ""
		for _, env := range c.Env {
			if env.Name == "KB_AGENT_DATA_VOLUME" {
				volume = env.Value
			}
		}
		if volume == "" {
			continue
		}
		// Both bootstrap and expansion workers share this capability. Only the runtime PVC binding differs.
		normalized, _ := kbagent.BuildEnv4DataLoadResult(&proto.DataLoadResult{Namespace: "$runtime", PVCName: "$volume:" + volume})
		setEnv(c, *normalized)
	}
}

// ValidateExecution rejects a missing agent before the public lifecycle client can skip it.
func ValidateExecution(action *apps.Action, target *corev1.Pod, pods []*corev1.Pod) error {
	if target == nil {
		return fmt.Errorf("member target runtime is absent")
	}
	selected, err := publiclifecycle.SelectTargetPods(pods, target, action)
	if err != nil {
		return err
	}
	if len(selected) == 0 {
		return fmt.Errorf("no available executor for member action")
	}
	for _, pod := range selected {
		port, err := intctrlutil.GetPortByName(*pod, kbagent.ContainerName, kbagent.DefaultHTTPPortName)
		if err != nil || port <= 0 || pod.Status.PodIP == "" || pod.DeletionTimestamp != nil {
			return fmt.Errorf("instance %s has no available agent endpoint", pod.Name)
		}
	}
	return nil
}

// CleanTaskConfigMap preserves unrelated tasks and clears only workload-generated data-copy instructions.
func CleanTaskConfigMap(cm *corev1.ConfigMap, result *proto.DataLoadResult) (bool, error) {
	value := cm.Data[taskEnv]
	if value == "" {
		return false, nil
	}
	var tasks []proto.Task
	if err := json.Unmarshal([]byte(value), &tasks); err != nil {
		return false, err
	}
	before := len(tasks)
	tasks = slices.DeleteFunc(tasks, func(task proto.Task) bool {
		return task.NewReplica != nil && sameResult(task.NewReplica.DataLoadResult, result)
	})
	if before == len(tasks) {
		return false, nil
	}
	if len(tasks) == 0 {
		delete(cm.Data, taskEnv)
	} else {
		encoded, err := json.Marshal(tasks)
		if err != nil {
			return false, err
		}
		cm.Data[taskEnv] = string(encoded)
	}
	return true, nil
}

// Explicit action container/image references must agree with the supplied worker context.
func applyExecContext(agent *corev1.Container, pod *corev1.Pod, a *workloads.LifecycleActions) error {
	for _, action := range []*apps.Action{a.MemberJoin, a.MemberLeave, a.Switchover, a.DataDump, a.DataLoad} {
		if action == nil || action.Exec == nil {
			continue
		}
		exec := action.Exec
		if exec.Image != "" && exec.Image != agent.Image {
			return fmt.Errorf("action image %s does not match agent image %s", exec.Image, agent.Image)
		}
		if exec.Container == "" {
			continue
		}
		i := slices.IndexFunc(pod.Spec.Containers, func(c corev1.Container) bool { return c.Name == exec.Container })
		if i < 0 {
			return fmt.Errorf("action execution container %s is not present", exec.Container)
		}
		context := &pod.Spec.Containers[i]
		for _, mount := range context.VolumeMounts {
			if !slices.ContainsFunc(agent.VolumeMounts, func(m corev1.VolumeMount) bool { return m.Name == mount.Name }) {
				agent.VolumeMounts = append(agent.VolumeMounts, mount)
			}
		}
		for _, env := range context.Env {
			if !slices.ContainsFunc(agent.Env, func(e corev1.EnvVar) bool { return e.Name == env.Name }) {
				agent.Env = append(agent.Env, env)
			}
		}
		if agent.SecurityContext == nil && context.SecurityContext != nil {
			agent.SecurityContext = context.SecurityContext.DeepCopy()
		}
		if agent.WorkingDir == "" {
			agent.WorkingDir = context.WorkingDir
		}
	}
	return nil
}

func envValueForTask(envs []corev1.EnvVar) string {
	for _, env := range envs {
		if env.Name == taskEnv {
			return env.Value
		}
	}
	return ""
}

// IsDataWorker distinguishes generated data-copy execution context from ordinary init containers.
func IsDataWorker(container *corev1.Container) bool {
	return container.Name == kbagent.ContainerName4Worker && slices.ContainsFunc(container.Env, func(e corev1.EnvVar) bool { return e.Name == "KB_AGENT_DATA_VOLUME" && e.Value != "" })
}

func sameResult(a, b *proto.DataLoadResult) bool {
	return a != nil && b != nil && a.Namespace == b.Namespace && a.PVCName == b.PVCName
}

// GeneratedTask returns just the task bound to this Pod's data-result guard.
func GeneratedTask(spec *corev1.PodSpec) (string, error) {
	for _, container := range spec.InitContainers {
		if container.Name != kbagent.ContainerName4Worker {
			continue
		}
		var result proto.DataLoadResult
		for _, env := range container.Env {
			if env.Name == resultEnv {
				if err := json.Unmarshal([]byte(env.Value), &result); err != nil {
					return "", err
				}
			}
		}
		var tasks []proto.Task
		if value := envValueForTask(container.Env); value != "" {
			if err := json.Unmarshal([]byte(value), &tasks); err != nil {
				return "", err
			}
		}
		for _, task := range tasks {
			if task.NewReplica != nil && sameResult(task.NewReplica.DataLoadResult, &result) {
				data, err := json.Marshal(task)
				return string(data), err
			}
		}
	}
	return "", nil
}
