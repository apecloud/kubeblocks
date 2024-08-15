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
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	kbacli "github.com/apecloud/kubeblocks/pkg/kbagent/client"
	"github.com/apecloud/kubeblocks/pkg/kbagent/proto"
	"github.com/apecloud/kubeblocks/pkg/kbagent/service"
)

type lifecycleAction interface {
	name() string
	precondition(ctx context.Context, cli client.Reader) error
	parameters(ctx context.Context, cli client.Reader) (map[string]string, error)
}

type kbagent struct {
	synthesizedComp  *component.SynthesizedComponent
	lifecycleActions *appsv1alpha1.ComponentLifecycleActions
	pods             []*corev1.Pod
	pod              *corev1.Pod
}

var _ Lifecycle = &kbagent{}

func (a *kbagent) PostProvision(ctx context.Context, cli client.Reader, opts *Options) error {
	lfa := &postProvision{
		synthesizedComp: a.synthesizedComp,
		action:          a.lifecycleActions.PostProvision,
	}
	return a.checkedCallAction(ctx, cli, a.lifecycleActions.PostProvision, lfa, opts)
}

func (a *kbagent) PreTerminate(ctx context.Context, cli client.Reader, opts *Options) error {
	lfa := &preTerminate{
		synthesizedComp: a.synthesizedComp,
		action:          a.lifecycleActions.PreTerminate,
	}
	return a.checkedCallAction(ctx, cli, a.lifecycleActions.PreTerminate, lfa, opts)
}

func (a *kbagent) Switchover(ctx context.Context, cli client.Reader, opts *Options) error {
	lfa := &switchover{
		synthesizedComp: a.synthesizedComp,
		pod:             a.pod,
	}
	return a.checkedCallAction(ctx, cli, a.lifecycleActions.Switchover, lfa, opts)
}

func (a *kbagent) MemberJoin(ctx context.Context, cli client.Reader, opts *Options) error {
	lfa := &memberJoin{
		synthesizedComp: a.synthesizedComp,
		pod:             a.pod,
	}
	return a.checkedCallAction(ctx, cli, a.lifecycleActions.MemberJoin, lfa, opts)
}

func (a *kbagent) MemberLeave(ctx context.Context, cli client.Reader, opts *Options) error {
	lfa := &memberLeave{
		synthesizedComp: a.synthesizedComp,
		pod:             a.pod,
	}
	return a.checkedCallAction(ctx, cli, a.lifecycleActions.MemberLeave, lfa, opts)
}

func (a *kbagent) DataDump(ctx context.Context, cli client.Reader, opts *Options) error {
	lfa := &dataDump{}
	return a.checkedCallAction(ctx, cli, a.lifecycleActions.DataDump, lfa, opts)
}

func (a *kbagent) DataLoad(ctx context.Context, cli client.Reader, opts *Options) error {
	lfa := &dataLoad{}
	return a.checkedCallAction(ctx, cli, a.lifecycleActions.DataLoad, lfa, opts)
}

func (a *kbagent) AccountProvision(ctx context.Context, cli client.Reader, opts *Options, args ...any) error {
	lfa := &accountProvision{args: args}
	return a.checkedCallAction(ctx, cli, a.lifecycleActions.AccountProvision, lfa, opts)
}

func (a *kbagent) checkedCallAction(ctx context.Context, cli client.Reader, action *appsv1alpha1.Action, lfa lifecycleAction, opts *Options) error {
	if action == nil {
		return errors.Wrap(ErrActionNotDefined, lfa.name())
	}
	if err := lfa.precondition(ctx, cli); err != nil {
		return err
	}
	// TODO: exactly once
	return a.callAction(ctx, cli, action, lfa, opts)
}

func (a *kbagent) callAction(ctx context.Context, cli client.Reader, spec *appsv1alpha1.Action, lfa lifecycleAction, opts *Options) error {
	req, err1 := a.buildActionRequest(ctx, cli, lfa, opts)
	if err1 != nil {
		return err1
	}
	return a.callActionWithSelector(ctx, spec, lfa, req)
}

func (a *kbagent) buildActionRequest(ctx context.Context, cli client.Reader, lfa lifecycleAction, opts *Options) (*proto.ActionRequest, error) {
	parameters, err := lfa.parameters(ctx, cli)
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

func (a *kbagent) callActionWithSelector(ctx context.Context, spec *appsv1alpha1.Action, lfa lifecycleAction, req *proto.ActionRequest) error {
	pods, err := a.selectTargetPods(spec)
	if err != nil {
		return err
	}
	if len(pods) == 0 {
		return fmt.Errorf("no available pod to call action %s", lfa.name())
	}

	// TODO: impl
	//  - back-off to retry
	//  - timeout
	for _, pod := range a.pods {
		cli, err1 := kbacli.NewClient(*pod)
		if err1 != nil {
			return err1
		}
		if cli == nil {
			continue // not defined, for test only
		}
		_, err2 := cli.CallAction(ctx, *req)
		if err2 != nil {
			return a.error2(lfa, err2)
		}
	}
	return nil
}

func (a *kbagent) selectTargetPods(spec *appsv1alpha1.Action) ([]*corev1.Pod, error) {
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
	case appsv1alpha1.AnyReplica:
		return anyPod(), nil
	case appsv1alpha1.AllReplicas:
		return allPods(), nil
	case appsv1alpha1.RoleSelector:
		return podsWithRole(), nil
	case appsv1alpha1.OrdinalSelector:
		return nil, fmt.Errorf("ordinal selector is not supported")
	default:
		return nil, fmt.Errorf("unknown pod selector: %s", spec.Exec.TargetPodSelector)
	}
}

func (a *kbagent) error2(lfa lifecycleAction, err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, service.ErrNotDefined):
		return errors.Wrap(ErrActionNotDefined, lfa.name())
	case errors.Is(err, service.ErrNotImplemented):
		return errors.Wrap(ErrActionNotImplemented, lfa.name())
	case errors.Is(err, service.ErrInProgress):
		return errors.Wrap(ErrActionInProgress, lfa.name())
	case errors.Is(err, service.ErrBusy):
		return errors.Wrap(ErrActionBusy, lfa.name())
	case errors.Is(err, service.ErrTimeout):
		return errors.Wrap(ErrActionTimeout, lfa.name())
	case errors.Is(err, service.ErrFailed):
		return errors.Wrap(ErrActionFailed, lfa.name())
	case errors.Is(err, service.ErrInternalError):
		return errors.Wrap(ErrActionInternalError, lfa.name())
	default:
		return err
	}
}
