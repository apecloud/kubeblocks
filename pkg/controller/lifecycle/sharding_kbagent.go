/*
Copyright (C) 2022-2025 ApeCloud Co., Ltd

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

	"github.com/pkg/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
)

type shardingAgent struct {
	*kbagent
	shardName                string
	shardingLifecycleActions *appsv1.ShardingLifecycleActions
}

func (a *shardingAgent) PostProvision(ctx context.Context, cli client.Reader, opts *Options) error {
	lfa := &shardPostProvision{
		action: a.shardingLifecycleActions.PostProvision,
	}

	return a.ignoreOutput(a.checkedCallAction(ctx, cli, lfa.action, lfa, opts))
}

func (a *shardingAgent) PreTerminate(ctx context.Context, cli client.Reader, opts *Options) error {
	lfa := &shardPreTerminate{
		action: a.shardingLifecycleActions.PreTerminate,
	}

	return a.ignoreOutput(a.checkedCallAction(ctx, cli, lfa.action, lfa, opts))
}

func (a *shardingAgent) ShardAdd(ctx context.Context, cli client.Reader, opts *Options) error {
	lfa := &shardAdd{
		shardName: a.shardName,
		action:    a.shardingLifecycleActions.ShardAdd,
	}

	return a.ignoreOutput(a.checkedCallAction(ctx, cli, lfa.action, lfa, opts))
}

func (a *shardingAgent) ShardRemove(ctx context.Context, cli client.Reader, opts *Options) error {
	lfa := &shardRemove{
		shardName: a.shardName,
		action:    a.shardingLifecycleActions.ShardRemove,
	}

	return a.ignoreOutput(a.checkedCallAction(ctx, cli, lfa.action, lfa, opts))
}

func (a *shardingAgent) precondition(ctx context.Context, cli client.Reader, spec *appsv1.Action) error {
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

func (a *shardingAgent) compReadyCheck(ctx context.Context, cli client.Reader) error {
	compList := &appsv1.ComponentList{}
	labels := constant.GetClusterLabels(a.clusterName, map[string]string{constant.KBAppShardingNameLabelKey: a.shardName})
	if err := cli.List(ctx, compList, client.InNamespace(a.namespace), client.MatchingLabels(labels)); err != nil {
		return err
	}

	ready := func(object client.Object) bool {
		comp := object.(*appsv1.Component)
		return comp.Status.Phase == appsv1.RunningComponentPhase
	}

	for _, comp := range compList.Items {
		if !ready(&comp) {
			return fmt.Errorf("%w: component is not ready", ErrPreconditionFailed)
		}
	}
	return nil
}

func (a *shardingAgent) checkedCallAction(ctx context.Context, cli client.Reader, spec *appsv1.Action, lfa lifecycleAction, opts *Options) ([]byte, error) {
	if !spec.Defined() {
		return nil, errors.Wrap(ErrActionNotDefined, lfa.name())
	}
	if err := a.precondition(ctx, cli, spec); err != nil {
		return nil, err
	}
	// TODO: exactly once
	return a.callAction(ctx, cli, spec, lfa, opts)
}
