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

// Package lifecycle provides membership policy and action helpers shared by the two InstanceSet implementations.
package lifecycle

import (
	"encoding/json"
	"fmt"
	"reflect"
	"slices"
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

func Enabled(its *workloads.InstanceSet) bool {
	a := its.Spec.LifecycleActions
	return a != nil && (a.MemberJoin != nil || a.MemberLeave != nil)
}

func RetirementBlocked(its *workloads.InstanceSet, old *workloads.InstanceStatus) bool {
	if !Enabled(its) || old == nil || old.EffectiveDesiredState() == workloads.InstanceDesiredStateOffline {
		return false
	}
	established := ptr.Deref(old.MemberJoined, false) || (old.MemberJoined == nil && old.Provisioned)
	return slices.ContainsFunc(its.Status.InstanceStatus, func(status workloads.InstanceStatus) bool {
		if status.EffectiveDesiredState() != workloads.InstanceDesiredStateActive {
			return false
		}
		pendingJoin := established && its.Spec.LifecycleActions.MemberJoin != nil && status.MemberJoined != nil && !*status.MemberJoined
		return pendingJoin
	})
}

func Validate(its *workloads.InstanceSet) error {
	if !Enabled(its) {
		return nil
	}
	a := its.Spec.LifecycleActions
	for _, action := range []*apps.Action{a.MemberJoin, a.MemberLeave, a.Switchover} {
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
	return nil
}

// Register records a complete current allocation before any runtime is created.
// Existing resources prevent a lost observation from being classified as a fresh bootstrap.
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
			if its.Spec.LifecycleActions.MemberJoin != nil {
				status.MemberJoined = ptr.To(false)
			}
		}
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

func NewLifecycle(its *workloads.InstanceSet, pod *corev1.Pod, pods []*corev1.Pod) (publiclifecycle.Lifecycle, error) {
	a := its.Spec.LifecycleActions
	return publiclifecycle.New(its.Namespace, its.Labels[constant.AppInstanceLabelKey], its.Labels[constant.KBAppComponentLabelKey], &apps.ComponentLifecycleActions{Switchover: a.Switchover, Reconfigure: a.Reconfigure, MemberJoin: a.MemberJoin, MemberLeave: a.MemberLeave}, a.TemplateVars, pod, pods)
}

// ConfigurePod merges member actions into the agent server supplied by the caller.
func ConfigurePod(its *workloads.InstanceSet, pod *corev1.Pod) error {
	if !Enabled(its) {
		return nil
	}
	if err := Validate(its); err != nil {
		return err
	}
	i := slices.IndexFunc(pod.Spec.Containers, func(c corev1.Container) bool { return c.Name == kbagent.ContainerName })
	if i < 0 {
		return fmt.Errorf("instance %s requires a prepared kbagent server container", pod.Name)
	}
	return mergeActions(&pod.Spec.Containers[i], its.Spec.LifecycleActions)
}

// ActionConfigured observes the action definition installed in the actual agent runtime.
func ActionConfigured(its *workloads.InstanceSet, pod *corev1.Pod, name string) (bool, error) {
	if pod == nil {
		return false, nil
	}
	var action *apps.Action
	switch name {
	case "memberJoin":
		action = its.Spec.LifecycleActions.MemberJoin
	case "memberLeave":
		action = its.Spec.LifecycleActions.MemberLeave
	case "switchover":
		action = its.Spec.LifecycleActions.Switchover
	default:
		return false, fmt.Errorf("unknown workload action %s", name)
	}
	if action == nil {
		return true, nil
	}
	index := slices.IndexFunc(pod.Spec.Containers, func(c corev1.Container) bool { return c.Name == kbagent.ContainerName })
	if index < 0 {
		return false, nil
	}
	actual := pod.Spec.Containers[index]
	var actions []proto.Action
	for _, env := range actual.Env {
		if env.Name == "KB_AGENT_ACTION" && env.Value != "" {
			if err := json.Unmarshal([]byte(env.Value), &actions); err != nil {
				return false, err
			}
		}
	}
	index = slices.IndexFunc(actions, func(a proto.Action) bool { return a.Name == name })
	if index < 0 || !reflect.DeepEqual(actions[index], *Action(name, action)) {
		return false, nil
	}
	expected := pod.DeepCopy()
	if err := ConfigurePod(its, expected); err != nil {
		return false, err
	}
	server := expected.Spec.Containers[slices.IndexFunc(expected.Spec.Containers, func(c corev1.Container) bool { return c.Name == kbagent.ContainerName })]
	if action.Exec != nil {
		for _, required := range action.Exec.Env {
			desiredIndex := slices.IndexFunc(server.Env, func(e corev1.EnvVar) bool { return e.Name == required.Name })
			actualIndex := slices.IndexFunc(actual.Env, func(e corev1.EnvVar) bool { return e.Name == required.Name })
			if actualIndex < 0 || !reflect.DeepEqual(actual.Env[actualIndex], server.Env[desiredIndex]) {
				return false, nil
			}
		}
	}
	return true, nil
}

// ActionTargets includes every possible AnyReplica executor before selection is pinned for execution.
func ActionTargets(action *apps.Action, pod *corev1.Pod, pods []*corev1.Pod) ([]*corev1.Pod, error) {
	selection := action.DeepCopy()
	selector := selection.TargetPodSelector
	if selector == "" && selection.Exec != nil {
		selector = selection.Exec.TargetPodSelector
	}
	if selector == apps.AnyReplica {
		selection.TargetPodSelector = apps.AllReplicas
	}
	return publiclifecycle.SelectTargetPods(pods, pod, selection)
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
	}{{"memberJoin", a.MemberJoin}, {"memberLeave", a.MemberLeave}, {"switchover", a.Switchover}}
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

// CriticalStatusChanged reports facts that must be committed before changing runtime inputs or membership.
// Ordinary health, revision, and progress observations do not stop the reconciliation chain.
func CriticalStatusChanged(previous, current []workloads.InstanceStatus) bool {
	for _, status := range current {
		for _, old := range previous {
			if status.PodName != old.PodName {
				continue
			}
			if status.Provisioned != old.Provisioned || !sameBoolResult(status.MemberJoined, old.MemberJoined) {
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
