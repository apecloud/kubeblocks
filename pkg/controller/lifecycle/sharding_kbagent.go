package lifecycle

import (
	"context"
	"fmt"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
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

		err := compAgent.ignoreOutput(compAgent.nonPreconditionCallAction(ctx, cli, lfa.action, lfa, opts))
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

		err := compAgent.ignoreOutput(compAgent.nonPreconditionCallAction(ctx, cli, lfa.action, lfa, opts))
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

		err := compAgent.ignoreOutput(compAgent.nonPreconditionCallAction(ctx, cli, lfa.action, lfa, opts))
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
