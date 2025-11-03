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

	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
)

type shardingAgent struct {
	compAgents               []kbagent
	shardingLifecycleActions *appsv1.ShardingLifecycleActions
}

func (a *shardingAgent) PostProvision(ctx context.Context, cli client.Reader, opts *Options) ([]string, error) {
	err := a.precondition(ctx, cli, a.shardingLifecycleActions.PostProvision)
	if err != nil {
		return nil, err
	}

	finishedComps := make([]string, 0)
	for _, compAgent := range a.compAgents {
		lfa := &postProvision{
			namespace:   compAgent.namespace,
			clusterName: compAgent.clusterName,
			compName:    compAgent.compName,
			action:      a.shardingLifecycleActions.PostProvision,
		}

		err = compAgent.ignoreOutput(compAgent.nonPreconditionCallAction(ctx, cli, lfa.action, lfa, opts))
		if err != nil {
			return nil, err
		}
		finishedComps = append(finishedComps, compAgent.compName)
	}
	return finishedComps, nil
}

func (a *shardingAgent) PreTerminate(ctx context.Context, cli client.Reader, opts *Options) ([]string, error) {
	finishedComps := make([]string, 0)
	for _, compAgent := range a.compAgents {
		lfa := &preTerminate{
			namespace:   compAgent.namespace,
			clusterName: compAgent.clusterName,
			compName:    compAgent.compName,
			action:      a.shardingLifecycleActions.PreTerminate,
		}

		err := compAgent.ignoreOutput(compAgent.checkedCallAction(ctx, cli, lfa.action, lfa, opts))
		if err != nil {
			return nil, err
		}
		finishedComps = append(finishedComps, compAgent.compName)
	}

	return finishedComps, nil
}

func (a *shardingAgent) ShardAdd(ctx context.Context, cli client.Reader, opts *Options) error {
	for _, compAgent := range a.compAgents {
		lfa := &shardAdd{
			namespace:   compAgent.namespace,
			clusterName: compAgent.clusterName,
			compName:    compAgent.compName,
			action:      a.shardingLifecycleActions.ShardAdd,
		}

		err := compAgent.ignoreOutput(compAgent.checkedCallAction(ctx, cli, lfa.action, lfa, opts))
		if err != nil {
			return err
		}
	}

	return nil
}

func (a *shardingAgent) ShardRemove(ctx context.Context, cli client.Reader, opts *Options) error {
	for _, compAgent := range a.compAgents {
		lfa := &shardRemove{
			namespace:   compAgent.namespace,
			clusterName: compAgent.clusterName,
			compName:    compAgent.compName,
			action:      a.shardingLifecycleActions.ShardRemove,
		}

		err := compAgent.ignoreOutput(compAgent.checkedCallAction(ctx, cli, lfa.action, lfa, opts))
		if err != nil {
			return err
		}
	}

	return nil
}

func (a *shardingAgent) precondition(ctx context.Context, cli client.Reader, spec *appsv1.Action) error {
	if spec.PreCondition == nil {
		return nil
	}
	switch *spec.PreCondition {
	case appsv1.ImmediatelyPreConditionType:
		return nil
	case appsv1.ComponentReadyPreConditionType:
		for _, compAgent := range a.compAgents {
			err := compAgent.compReadyCheck(ctx, cli)
			if err != nil {
				return err
			}
		}
		return nil
	case appsv1.ClusterReadyPreConditionType:
		return a.compAgents[0].clusterReadyCheck(ctx, cli)
	default:
		return fmt.Errorf("unknown precondition type %s", *spec.PreCondition)
	}
}
