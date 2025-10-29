package cluster

import (
	"fmt"
	"time"

	"github.com/pkg/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	"github.com/apecloud/kubeblocks/pkg/common"
	"github.com/apecloud/kubeblocks/pkg/controller/graph"
	"github.com/apecloud/kubeblocks/pkg/controller/lifecycle"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
	"github.com/apecloud/kubeblocks/pkg/controller/sharding"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

const (
	kbShardingPostProvisionDoneKey = "kubeblocks.io/sharding-post-provision-done"
)

type clusterShardingPostProvisionTransformer struct{}

var _ graph.Transformer = &clusterShardingPostProvisionTransformer{}

func (t *clusterShardingPostProvisionTransformer) Transform(ctx graph.TransformContext, dag *graph.DAG) error {
	transCtx, _ := ctx.(*clusterTransformContext)
	cluster := transCtx.OrigCluster
	if !cluster.IsDeleting() {
		return nil
	}

	if common.IsCompactMode(transCtx.Cluster.Annotations) {
		transCtx.V(1).Info("Cluster is in compact mode, no need to create post provision related objects", "cluster", client.ObjectKeyFromObject(transCtx.Cluster))
		return nil
	}

	return t.reconcileShardingPostProvision(transCtx, dag)
}

func (t *clusterShardingPostProvisionTransformer) reconcileShardingPostProvision(transCtx *clusterTransformContext, _ *graph.DAG) error {
	for _, shard := range transCtx.shardings {
		shardDef, ok := transCtx.shardingDefs[shard.ShardingDef]
		if !ok {
			continue
		}

		if shardDef.Spec.LifecycleActions == nil || shardDef.Spec.LifecycleActions.PostProvision == nil {
			continue
		}

		comps, err := sharding.ListShardingComponents(transCtx.Context, transCtx.Client, transCtx.Cluster, shard.Name)
		if err != nil {
			return err
		}
		unfinishedComps := checkPostProvisionDone(comps)
		if len(unfinishedComps) == 0 {
			continue
		}

		finishedComps, err := t.shardingPostProvision(transCtx, unfinishedComps, shardDef.Spec.LifecycleActions)
		if err != nil {
			err = lifecycle.IgnoreNotDefined(err)
			if errors.Is(err, lifecycle.ErrPreconditionFailed) {
				err = fmt.Errorf("%w: %w", intctrlutil.NewDelayedRequeueError(time.Second*10, "wait for lifecycle action precondition"), err)
			}
			return err
		}

		t.markShardingPostProvisionDone(transCtx, finishedComps)
	}
	return nil
}

func checkPostProvisionDone(comps []v1.Component) []v1.Component {
	var unfinished []v1.Component
	for _, comp := range comps {
		if model.IsObjectDeleting(&comp) {
			continue
		}

		if comp.Annotations == nil {
			unfinished = append(unfinished, comp)
			continue
		}

		_, ok := comp.Annotations[kbShardingPostProvisionDoneKey]
		if !ok {
			unfinished = append(unfinished, comp)
		}
	}
	return unfinished
}

func (t *clusterShardingPostProvisionTransformer) shardingPostProvision(transCtx *clusterTransformContext, comps []v1.Component, lifecycleAction *v1.ShardingLifecycleActions) ([]string, error) {
	lfa, err := t.lifecycleAction4Sharding(transCtx, comps, lifecycleAction)
	if err != nil {
		return nil, err
	}
	return lfa.PostProvision(transCtx.Context, transCtx.Client, nil)
}

func (t *clusterShardingPostProvisionTransformer) lifecycleAction4Sharding(transCtx *clusterTransformContext, comps []v1.Component, lifecycleAction *v1.ShardingLifecycleActions) (lifecycle.ShardingLifecycle, error) {
	compTemplateVarsMap, compPodsMap, err := buildCompMaps(transCtx, comps)
	if err != nil {
		return nil, err
	}

	return lifecycle.NewShardingLifecycle(transCtx.Cluster.Namespace, transCtx.Cluster.Name, lifecycleAction, compTemplateVarsMap, nil, compPodsMap)
}

func (t *clusterShardingPostProvisionTransformer) markShardingPostProvisionDone(transCtx *clusterTransformContext, comps []string) {
	now := time.Now().Format(time.RFC3339Nano)

	for _, comp := range comps {
		if transCtx.annotations == nil {
			transCtx.annotations = make(map[string]map[string]string)
		}
		if transCtx.annotations[comp] == nil {
			transCtx.annotations[comp] = make(map[string]string)
		}

		_, ok := transCtx.annotations[comp][kbShardingPostProvisionDoneKey]
		if ok {
			return
		}

		transCtx.annotations[comp][kbShardingPostProvisionDoneKey] = now
	}
}
