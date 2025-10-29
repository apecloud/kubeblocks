package cluster

import (
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	appsutil "github.com/apecloud/kubeblocks/controllers/apps/util"
	"github.com/apecloud/kubeblocks/pkg/common"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	"github.com/apecloud/kubeblocks/pkg/controller/graph"
	"github.com/apecloud/kubeblocks/pkg/controller/lifecycle"
	"github.com/apecloud/kubeblocks/pkg/controller/sharding"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

const (
	kbShardingPreTerminateDoneKey = "kubeblocks.io/sharding-pre-terminate-done"
)

type clusterShardingPreTerminateTransformer struct{}

var _ graph.Transformer = &clusterShardingPreTerminateTransformer{}

func (t *clusterShardingPreTerminateTransformer) Transform(ctx graph.TransformContext, dag *graph.DAG) error {
	transCtx, _ := ctx.(*clusterTransformContext)
	cluster := transCtx.OrigCluster
	if !cluster.IsDeleting() || cluster.Status.Phase != v1.DeletingClusterPhase {
		return nil
	}

	if common.IsCompactMode(transCtx.Cluster.Annotations) {
		transCtx.V(1).Info("Cluster is in compact mode, no need to create pre terminate related objects", "cluster", client.ObjectKeyFromObject(transCtx.Cluster))
		return nil
	}

	return t.reconcileShardingPreTerminate(transCtx, dag)
}

func (t *clusterShardingPreTerminateTransformer) reconcileShardingPreTerminate(transCtx *clusterTransformContext, _ *graph.DAG) error {
	for _, shard := range transCtx.shardings {
		shardDef, ok := transCtx.shardingDefs[shard.ShardingDef]
		if !ok {
			continue
		}

		if shardDef.Spec.LifecycleActions == nil || shardDef.Spec.LifecycleActions.PreTerminate == nil {
			continue
		}

		comps, err := sharding.ListShardingComponents(transCtx.Context, transCtx.Client, transCtx.Cluster, shard.Name)
		if err != nil {
			return err
		}
		unfinishedComps := checkPreTerminateDone(comps)
		if len(unfinishedComps) == 0 {
			continue
		}

		finishedComps, err := t.shardingPreTerminate(transCtx, unfinishedComps, shardDef.Spec.LifecycleActions)
		if err != nil {
			return lifecycle.IgnoreNotDefined(err)
		}

		t.markShardingPreTerminateDone(transCtx, finishedComps)
	}
	return nil
}

func checkPreTerminateDone(comps []v1.Component) []v1.Component {
	var unfinished []v1.Component
	for _, comp := range comps {
		if comp.Annotations == nil {
			unfinished = append(unfinished, comp)
			continue
		}

		_, ok := comp.Annotations[kbShardingPreTerminateDoneKey]
		if !ok {
			unfinished = append(unfinished, comp)
		}
	}
	return unfinished
}

func (t *clusterShardingPreTerminateTransformer) shardingPreTerminate(transCtx *clusterTransformContext, comps []v1.Component, lifecycleAction *v1.ShardingLifecycleActions) ([]string, error) {
	lfa, err := t.lifecycleAction4Sharding(transCtx, comps, lifecycleAction)
	if err != nil {
		return nil, err
	}
	return lfa.PreTerminate(transCtx.Context, transCtx.Client, nil)
}

func (t *clusterShardingPreTerminateTransformer) lifecycleAction4Sharding(transCtx *clusterTransformContext, comps []v1.Component, lifecycleAction *v1.ShardingLifecycleActions) (lifecycle.ShardingLifecycle, error) {
	compTemplateVarsMap, compPodsMap, err := buildCompMaps(transCtx, comps)
	if err != nil {
		return nil, err
	}

	return lifecycle.NewShardingLifecycle(transCtx.Cluster.Namespace, transCtx.Cluster.Name, lifecycleAction, compTemplateVarsMap, nil, compPodsMap)
}

func (t *clusterShardingPreTerminateTransformer) markShardingPreTerminateDone(transCtx *clusterTransformContext, comps []string) {
	now := time.Now().Format(time.RFC3339Nano)

	for _, comp := range comps {
		if transCtx.annotations == nil {
			transCtx.annotations = make(map[string]map[string]string)
		}
		if transCtx.annotations[comp] == nil {
			transCtx.annotations[comp] = make(map[string]string)
		}

		_, ok := transCtx.annotations[comp][kbShardingPreTerminateDoneKey]
		if ok {
			return
		}

		transCtx.annotations[comp][kbShardingPreTerminateDoneKey] = now
	}
}

func buildCompMaps(transCtx *clusterTransformContext, comps []v1.Component) (map[string]map[string]string, map[string][]*corev1.Pod, error) {
	compTemplateVarsMap := make(map[string]map[string]string)
	compPodsMap := make(map[string][]*corev1.Pod)

	for _, comp := range comps {
		synthesizedComp, err := synthesizedComponent(transCtx, &comp)
		if err != nil {
			return nil, nil, err
		}
		compTemplateVarsMap[comp.Name] = synthesizedComp.TemplateVars

		pods, err := component.ListOwnedPods(transCtx.Context, transCtx.Client,
			synthesizedComp.Namespace, synthesizedComp.ClusterName, synthesizedComp.Name)
		if err != nil {
			return nil, nil, err
		}
		compPodsMap[comp.Name] = pods
	}
	return compTemplateVarsMap, compPodsMap, nil
}

func synthesizedComponent(transCtx *clusterTransformContext, comp *v1.Component) (*component.SynthesizedComponent, error) {
	synthesizedComp, err := component.BuildSynthesizedComponent(transCtx.Context, transCtx.Client, transCtx.componentDefs[comp.Spec.CompDef], comp)
	if err != nil {
		return nil, intctrlutil.NewRequeueError(appsutil.RequeueDuration,
			fmt.Sprintf("build synthesized component failed at pre-terminate: %s", err.Error()))
	}
	synthesizedComp.TemplateVars, _, err = component.ResolveTemplateNEnvVars(transCtx.Context, transCtx.Client, synthesizedComp, transCtx.componentDefs[comp.Spec.CompDef].Spec.Vars)
	if err != nil {
		return nil, err
	}
	return synthesizedComp, nil
}
