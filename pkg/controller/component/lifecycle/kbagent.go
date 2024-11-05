/*
Copyright (C) 2022-2024 ApeCloud Co., Ltd

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
	"fmt"
	"math/rand"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	"github.com/apecloud/kubeblocks/pkg/controller/instanceset"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	kbagt "github.com/apecloud/kubeblocks/pkg/kbagent"
	kbacli "github.com/apecloud/kubeblocks/pkg/kbagent/client"
	"github.com/apecloud/kubeblocks/pkg/kbagent/proto"
)

type lifecycleAction interface {
	name() string
	parameters(ctx context.Context, cli client.Reader) (map[string]string, error)
}

type kbagent struct {
	synthesizedComp *component.SynthesizedComponent
	pods            []*corev1.Pod
	pod             *corev1.Pod
}

var _ Lifecycle = &kbagent{}

func (a *kbagent) PostProvision(ctx context.Context, cli client.Reader, opts *Options) error {
	lfa := &postProvision{
		namespace:   a.synthesizedComp.Namespace,
		clusterName: a.synthesizedComp.ClusterName,
		compName:    a.synthesizedComp.Name,
		action:      a.synthesizedComp.LifecycleActions.PostProvision,
	}
	return a.ignoreOutput(a.checkedCallAction(ctx, cli, lfa.action, lfa, opts))
}

func (a *kbagent) PreTerminate(ctx context.Context, cli client.Reader, opts *Options) error {
	lfa := &preTerminate{
		namespace:   a.synthesizedComp.Namespace,
		clusterName: a.synthesizedComp.ClusterName,
		compName:    a.synthesizedComp.Name,
		action:      a.synthesizedComp.LifecycleActions.PreTerminate,
	}
	return a.ignoreOutput(a.checkedCallAction(ctx, cli, lfa.action, lfa, opts))
}

func (a *kbagent) RoleProbe(ctx context.Context, cli client.Reader, opts *Options) ([]byte, error) {
	return a.checkedCallProbe(ctx, cli, a.synthesizedComp.LifecycleActions.RoleProbe, &roleProbe{}, opts)
}

func (a *kbagent) Switchover(ctx context.Context, cli client.Reader, opts *Options, candidate string) error {
	lfa := &switchover{
		namespace:   a.synthesizedComp.Namespace,
		clusterName: a.synthesizedComp.ClusterName,
		compName:    a.synthesizedComp.Name,
		roles:       a.synthesizedComp.Roles,
		candidate:   candidate,
	}
	return a.ignoreOutput(a.checkedCallAction(ctx, cli, a.synthesizedComp.LifecycleActions.Switchover, lfa, opts))
}

func (a *kbagent) MemberJoin(ctx context.Context, cli client.Reader, opts *Options) error {
	lfa := &memberJoin{
		namespace:   a.synthesizedComp.Namespace,
		clusterName: a.synthesizedComp.ClusterName,
		compName:    a.synthesizedComp.Name,
		pod:         a.pod,
	}
	return a.ignoreOutput(a.checkedCallAction(ctx, cli, a.synthesizedComp.LifecycleActions.MemberJoin, lfa, opts))
}

func (a *kbagent) MemberLeave(ctx context.Context, cli client.Reader, opts *Options) error {
	lfa := &memberLeave{
		namespace:   a.synthesizedComp.Namespace,
		clusterName: a.synthesizedComp.ClusterName,
		compName:    a.synthesizedComp.Name,
		pod:         a.pod,
	}
	return a.ignoreOutput(a.checkedCallAction(ctx, cli, a.synthesizedComp.LifecycleActions.MemberLeave, lfa, opts))
}

func (a *kbagent) DataDump(ctx context.Context, cli client.Reader, opts *Options, replicas []string) error {
	lfa := &dataDump{
		replicas: replicas,
	}
	return a.ignoreOutput(a.checkedCallAction(ctx, cli, a.synthesizedComp.LifecycleActions.DataDump, lfa, opts))
}

func (a *kbagent) DataLoad(ctx context.Context, cli client.Reader, opts *Options, replicas []string) error {
	lfa := &dataLoad{
		replicas: replicas,
	}
	return a.ignoreOutput(a.checkedCallAction(ctx, cli, a.synthesizedComp.LifecycleActions.DataLoad, lfa, opts))
}

func (a *kbagent) AccountProvision(ctx context.Context, cli client.Reader, opts *Options, statement, user, password string) error {
	lfa := &accountProvision{
		statement: statement,
		user:      user,
		password:  password,
	}
	return a.ignoreOutput(a.checkedCallAction(ctx, cli, a.synthesizedComp.LifecycleActions.AccountProvision, lfa, opts))
}

func (a *kbagent) ignoreOutput(_ []byte, err error) error {
	return err
}

func (a *kbagent) checkedCallAction(ctx context.Context, cli client.Reader, spec *appsv1.Action, lfa lifecycleAction, opts *Options) ([]byte, error) {
	if spec == nil || spec.Exec == nil {
		return nil, errors.Wrap(ErrActionNotDefined, lfa.name())
	}
	if err := a.precondition(ctx, cli, spec); err != nil {
		return nil, err
	}
	// TODO: exactly once
	return a.callAction(ctx, cli, spec, lfa, opts)
}

func (a *kbagent) checkedCallProbe(ctx context.Context, cli client.Reader, spec *appsv1.Probe, lfa lifecycleAction, opts *Options) ([]byte, error) {
	if spec == nil || spec.Exec == nil {
		return nil, errors.Wrap(ErrActionNotDefined, lfa.name())
	}
	return a.checkedCallAction(ctx, cli, &spec.Action, lfa, opts)
}

func (a *kbagent) precondition(ctx context.Context, cli client.Reader, spec *appsv1.Action) error {
	if spec.PreCondition == nil {
		return nil
	}
	switch *spec.PreCondition {
	case appsv1.ImmediatelyPreConditionType:
		return nil
	case appsv1.RuntimeReadyPreConditionType:
		return a.runtimeReadyCheck(ctx, cli)
	case appsv1.ComponentReadyPreConditionType:
		return a.compReadyCheck(ctx, cli)
	case appsv1.ClusterReadyPreConditionType:
		return a.clusterReadyCheck(ctx, cli)
	default:
		return fmt.Errorf("unknown precondition type %s", *spec.PreCondition)
	}
}

func (a *kbagent) clusterReadyCheck(ctx context.Context, cli client.Reader) error {
	ready := func(object client.Object) bool {
		cluster := object.(*appsv1.Cluster)
		return cluster.Status.Phase == appsv1.RunningClusterPhase
	}
	return a.readyCheck(ctx, cli, a.synthesizedComp.ClusterName, "cluster", &appsv1.Cluster{}, ready)
}

func (a *kbagent) compReadyCheck(ctx context.Context, cli client.Reader) error {
	ready := func(object client.Object) bool {
		comp := object.(*appsv1.Component)
		return comp.Status.Phase == appsv1.RunningClusterCompPhase
	}
	compName := constant.GenerateClusterComponentName(a.synthesizedComp.ClusterName, a.synthesizedComp.Name)
	return a.readyCheck(ctx, cli, compName, "component", &appsv1.Component{}, ready)
}

func (a *kbagent) runtimeReadyCheck(ctx context.Context, cli client.Reader) error {
	name := constant.GenerateWorkloadNamePattern(a.synthesizedComp.ClusterName, a.synthesizedComp.Name)
	ready := func(object client.Object) bool {
		its := object.(*workloads.InstanceSet)
		return instanceset.IsInstancesReady(its)
	}
	return a.readyCheck(ctx, cli, name, "runtime", &workloads.InstanceSet{}, ready)
}

func (a *kbagent) readyCheck(ctx context.Context, cli client.Reader, name, kind string, obj client.Object, ready func(object client.Object) bool) error {
	key := types.NamespacedName{
		Namespace: a.synthesizedComp.Namespace,
		Name:      name,
	}
	if err := cli.Get(ctx, key, obj); err != nil {
		return errors.Wrap(err, fmt.Sprintf("precondition check error for %s ready", kind))
	}
	if !ready(obj) {
		return fmt.Errorf("precondition check error, %s is not ready", kind)
	}
	return nil
}

func (a *kbagent) callAction(ctx context.Context, cli client.Reader, spec *appsv1.Action, lfa lifecycleAction, opts *Options) ([]byte, error) {
	req, err1 := a.buildActionRequest(ctx, cli, lfa, opts)
	if err1 != nil {
		return nil, err1
	}
	return a.callActionWithSelector(ctx, spec, lfa, req)
}

func (a *kbagent) buildActionRequest(ctx context.Context, cli client.Reader, lfa lifecycleAction, opts *Options) (*proto.ActionRequest, error) {
	parameters, err := a.parameters(ctx, cli, lfa)
	if err != nil {
		return nil, err
	}
	req := &proto.ActionRequest{
		Action:     lfa.name(),
		Parameters: parameters,
	}
	if opts != nil {
		if opts.NonBlocking != nil {
			req.NonBlocking = opts.NonBlocking
		}
		if opts.TimeoutSeconds != nil {
			req.TimeoutSeconds = opts.TimeoutSeconds
		}
		if opts.RetryPolicy != nil {
			req.RetryPolicy = &proto.RetryPolicy{
				MaxRetries:    opts.RetryPolicy.MaxRetries,
				RetryInterval: opts.RetryPolicy.RetryInterval,
			}
		}
	}
	return req, nil
}

func (a *kbagent) parameters(ctx context.Context, cli client.Reader, lfa lifecycleAction) (map[string]string, error) {
	m, err := a.templateVarsParameters()
	if err != nil {
		return nil, err
	}
	sys, err := lfa.parameters(ctx, cli)
	if err != nil {
		return nil, err
	}

	for k, v := range sys {
		// template vars take precedence
		if _, ok := m[k]; !ok {
			m[k] = v
		}
	}
	return m, nil
}

func (a *kbagent) templateVarsParameters() (map[string]string, error) {
	m := map[string]string{}
	for k, v := range a.synthesizedComp.TemplateVars {
		m[k] = v.(string)
	}
	return m, nil
}

func (a *kbagent) callActionWithSelector(ctx context.Context, spec *appsv1.Action, lfa lifecycleAction, req *proto.ActionRequest) ([]byte, error) {
	pods, err := a.selectTargetPods(spec)
	if err != nil {
		return nil, err
	}
	if len(pods) == 0 {
		return nil, fmt.Errorf("no available pod to execute action %s", lfa.name())
	}

	// TODO: impl
	//  - back-off to retry
	//  - timeout
	var output []byte
	for _, pod := range pods {
		host, port, err := a.serverEndpoint(pod)
		if err != nil {
			return nil, errors.Wrapf(err, "pod %s is unavailable to execute action %s", pod.Name, lfa.name())
		}
		cli, err := kbacli.NewClient(host, port)
		if err != nil {
			return nil, err // mock client error
		}
		if cli == nil {
			continue // not kb-agent container and port defined, for test only
		}
		rsp, err := cli.Action(ctx, *req)
		if err != nil {
			return nil, errors.Wrapf(err, "http error occurred when executing action %s at pod %s", lfa.name(), pod.Name)
		}
		if len(rsp.Error) > 0 {
			return nil, a.formatError(lfa, rsp)
		}
		// take first non-nil output
		if output == nil && rsp.Output != nil {
			output = rsp.Output
		}
	}
	return output, nil
}

func (a *kbagent) selectTargetPods(spec *appsv1.Action) ([]*corev1.Pod, error) {
	if spec.Exec == nil || len(spec.Exec.TargetPodSelector) == 0 {
		return []*corev1.Pod{a.pod}, nil
	}

	anyPod := func() []*corev1.Pod {
		i := rand.Int() % len(a.pods)
		return []*corev1.Pod{a.pods[i]}
	}

	allPods := func() []*corev1.Pod {
		return a.pods
	}

	podsWithRole := func() []*corev1.Pod {
		roleName := spec.Exec.MatchingKey
		var pods []*corev1.Pod
		for i, pod := range a.pods {
			if len(pod.Labels) != 0 {
				if pod.Labels[constant.RoleLabelKey] == roleName {
					pods = append(pods, a.pods[i])
				}
			}
		}
		return pods
	}

	switch spec.Exec.TargetPodSelector {
	case appsv1.AnyReplica:
		return anyPod(), nil
	case appsv1.AllReplicas:
		return allPods(), nil
	case appsv1.RoleSelector:
		return podsWithRole(), nil
	case appsv1.OrdinalSelector:
		return nil, fmt.Errorf("ordinal selector is not supported")
	default:
		return nil, fmt.Errorf("unknown pod selector: %s", spec.Exec.TargetPodSelector)
	}
}

func (a *kbagent) serverEndpoint(pod *corev1.Pod) (string, int32, error) {
	port, err := intctrlutil.GetPortByName(*pod, kbagt.ContainerName, kbagt.DefaultPortName)
	if err != nil {
		// has no kb-agent defined
		return "", 0, nil
	}
	host := pod.Status.PodIP
	if host == "" {
		return "", 0, fmt.Errorf("pod %v has no ip", pod.Name)
	}
	return host, port, nil
}

func (a *kbagent) formatError(lfa lifecycleAction, rsp proto.ActionResponse) error {
	wrapError := func(err error) error {
		return errors.Wrapf(err, "action: %s, error: %s", lfa.name(), rsp.Message)
	}
	err := proto.Type2Error(rsp.Error)
	switch {
	case err == nil:
		return nil
	case errors.Is(err, proto.ErrNotDefined):
		return wrapError(ErrActionNotDefined)
	case errors.Is(err, proto.ErrNotImplemented):
		return wrapError(ErrActionNotImplemented)
	case errors.Is(err, proto.ErrBadRequest):
		return wrapError(ErrActionInternalError)
	case errors.Is(err, proto.ErrInProgress):
		return wrapError(ErrActionInProgress)
	case errors.Is(err, proto.ErrBusy):
		return wrapError(ErrActionBusy)
	case errors.Is(err, proto.ErrTimedOut):
		return wrapError(ErrActionTimedOut)
	case errors.Is(err, proto.ErrFailed):
		return wrapError(ErrActionFailed)
	case errors.Is(err, proto.ErrInternalError):
		return wrapError(ErrActionInternalError)
	default:
		return wrapError(err)
	}
}
