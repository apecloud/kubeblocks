/*
Copyright (C) 2022-2026 ApeCloud Co., Ltd

This file is part of KubeBlocks project

KubeBlocks is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

KubeBlocks is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with KubeBlocks.  If not, see <http://www.gnu.org/licenses/>.
*/

package cluster

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"

	"k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	"github.com/apecloud/kubeblocks/pkg/controller/graph"
)

var (
	errShardingScaleInInitialPlanSourceUnavailable = errors.New("sharding scale-in initial plan source unavailable")
	errInvalidShardingScaleInInitialPlanSource     = errors.New("invalid sharding scale-in initial plan source")
	errShardingScaleInPlanProgressionUnavailable   = errors.New("sharding scale-in plan progression unavailable")
)

// shardingScaleInInitialPlanMaterialSource must build the complete canonical
// PlanMaterial from uncached reads. It is injected separately from the graph
// client so the initial authority cannot be derived from stale graph objects.
type shardingScaleInInitialPlanMaterialSource interface {
	BuildInitialShardingScaleInPlanMaterial(
		context.Context,
		client.Reader,
		types.NamespacedName,
		types.UID,
		string,
	) (*appsv1.ShardingScaleInPlanMaterial, error)
}

func (h *clusterShardingHandler) reconcileTypedScaleInInitialPlan(
	transCtx *clusterTransformContext,
	dag *graph.DAG,
	shardingName string,
	toDelete sets.Set[string],
) (bool, error) {
	if transCtx == nil || transCtx.Cluster == nil {
		return true, fmt.Errorf("%w: Cluster transform context must be complete",
			errInvalidShardingScaleInInitialPlanSource)
	}
	if transCtx.clusterTopologyReconcileLockAcquisitionPending {
		return true, nil
	}
	if shardingStatus, ok := transCtx.Cluster.Status.Shardings[shardingName]; ok &&
		shardingStatus.ScaleIn != nil {
		return true, fmt.Errorf("%w: sharding %q already has plan %q in phase %q",
			errShardingScaleInPlanProgressionUnavailable,
			shardingName, shardingStatus.ScaleIn.PlanID, shardingStatus.ScaleIn.Phase)
	}
	cachedOrdinaryLock := false
	if lock := transCtx.Cluster.Status.TopologyMutationLock; lock != nil {
		own, err := isClusterTopologyReconcileLock(transCtx.Cluster, lock)
		if err != nil {
			return true, err
		}
		if !own {
			return true, fmt.Errorf("%w: topology lock owned by %q plan %q is in state %q",
				errShardingScaleInPlanProgressionUnavailable,
				lock.OwnerKind, lock.OwnerPlanID, lock.State)
		}
		cachedOrdinaryLock = true
	}
	if transCtx.APIReader == nil {
		return true, fmt.Errorf("%w: sharding %q",
			errShardingScaleInInitialPlanSourceUnavailable, shardingName)
	}
	clusterKey := types.NamespacedName{
		Namespace: transCtx.Cluster.Namespace,
		Name:      transCtx.Cluster.Name,
	}
	freshCluster, err := loadFreshClusterForShardingScaleIn(
		transCtx.Context,
		transCtx.APIReader,
		clusterKey,
		transCtx.Cluster.UID,
		transCtx.Cluster.Generation,
	)
	if err != nil {
		return true, markClusterTopologyExecutionBarrier(err)
	}
	if shardingStatus, ok := freshCluster.Status.Shardings[shardingName]; ok &&
		shardingStatus.ScaleIn != nil {
		return true, fmt.Errorf("%w: fresh sharding %q already has plan %q in phase %q",
			errShardingScaleInPlanProgressionUnavailable,
			shardingName, shardingStatus.ScaleIn.PlanID, shardingStatus.ScaleIn.Phase)
	}
	if lock := freshCluster.Status.TopologyMutationLock; lock != nil {
		own, err := isClusterTopologyReconcileLock(freshCluster, lock)
		if err != nil {
			return true, err
		}
		if !cachedOrdinaryLock || !own ||
			!equality.Semantic.DeepEqual(lock, transCtx.Cluster.Status.TopologyMutationLock) {
			return true, fmt.Errorf("%w: fresh topology lock owned by %q plan %q is in state %q",
				errShardingScaleInPlanProgressionUnavailable,
				lock.OwnerKind, lock.OwnerPlanID, lock.State)
		}
	} else if cachedOrdinaryLock {
		return true, fmt.Errorf("%w: cached ordinary topology lock disappeared from the fresh Cluster",
			errShardingScaleInPlanProgressionUnavailable)
	}
	if len(toDelete) == 0 {
		if cachedOrdinaryLock {
			if err := requestClusterTopologyReconcileLockRelease(
				transCtx, freshCluster.Status.TopologyMutationLock); err != nil {
				return true, err
			}
			return false, nil
		}
		return true, addClusterTopologyReconcileLockIntent(transCtx, dag, freshCluster)
	}
	freshShardingDef, typed, err := usesFreshTypedScaleInPlan(
		transCtx.Context,
		transCtx.APIReader,
		freshCluster,
		shardingName,
	)
	if err != nil {
		return true, markClusterTopologyExecutionBarrier(err)
	}
	if !typed {
		if err := h.validateFreshLegacyScaleInDefinition(
			transCtx, shardingName, freshShardingDef); err != nil {
			return true, err
		}
		if cachedOrdinaryLock {
			if err := requestClusterTopologyReconcileLockRelease(
				transCtx, freshCluster.Status.TopologyMutationLock); err != nil {
				return true, err
			}
			return false, nil
		}
		return true, addClusterTopologyReconcileLockIntent(transCtx, dag, freshCluster)
	}
	if cachedOrdinaryLock {
		if err := requestClusterTopologyReconcileLockRelease(
			transCtx, freshCluster.Status.TopologyMutationLock); err != nil {
			return true, err
		}
		return true, nil
	}
	if transCtx.shardingScaleInInitialPlanSource == nil {
		return true, fmt.Errorf("%w: sharding %q",
			errShardingScaleInInitialPlanSourceUnavailable, shardingName)
	}

	err = addInitialShardingScaleInPlanIntent(
		transCtx.Context,
		dag,
		transCtx.APIReader,
		transCtx.shardingScaleInInitialPlanSource,
		clusterKey,
		transCtx.Cluster.UID,
		shardingName,
	)
	return true, markClusterTopologyExecutionBarrier(err)
}

func loadFreshClusterForShardingScaleIn(
	ctx context.Context,
	apiReader client.Reader,
	clusterKey types.NamespacedName,
	expectedClusterUID types.UID,
	expectedClusterGeneration int64,
) (*appsv1.Cluster, error) {
	invalid := func(format string, args ...any) error {
		return fmt.Errorf("%w: %s",
			errInvalidShardingScaleInInitialPlanSource, fmt.Sprintf(format, args...))
	}
	if ctx == nil || apiReader == nil {
		return nil, invalid("context and APIReader must be non-nil")
	}
	if clusterKey.Namespace == "" || clusterKey.Name == "" ||
		expectedClusterUID == "" || expectedClusterGeneration <= 0 {
		return nil, invalid("Cluster key and expected identity must be complete")
	}

	freshCluster := &appsv1.Cluster{}
	if err := apiReader.Get(ctx, clusterKey, freshCluster); err != nil {
		return nil, markClusterTopologyExecutionBarrier(
			fmt.Errorf("read fresh Cluster for sharding scale-in authority: %w", err))
	}
	if freshCluster.Namespace != clusterKey.Namespace ||
		freshCluster.Name != clusterKey.Name ||
		freshCluster.UID != expectedClusterUID ||
		freshCluster.Generation != expectedClusterGeneration {
		return nil, invalid("fresh Cluster identity does not match the transform input")
	}
	if freshCluster.ResourceVersion == "" || !freshCluster.DeletionTimestamp.IsZero() {
		return nil, invalid("fresh Cluster must have a resourceVersion and not be deleting")
	}
	return freshCluster, nil
}

// A nil definition with typed=false identifies only an inline, user-defined
// legacy sharding. Definition-backed legacy paths always return the fresh object.
func usesFreshTypedScaleInPlan(
	ctx context.Context,
	apiReader client.Reader,
	freshCluster *appsv1.Cluster,
	shardingName string,
) (*appsv1.ShardingDefinition, bool, error) {
	invalid := func(format string, args ...any) error {
		return fmt.Errorf("%w: %s",
			errInvalidShardingScaleInInitialPlanSource, fmt.Sprintf(format, args...))
	}
	if ctx == nil || apiReader == nil || freshCluster == nil {
		return nil, false, invalid("context, APIReader, and fresh Cluster must be non-nil")
	}
	if freshCluster.Namespace == "" || freshCluster.Name == "" ||
		freshCluster.UID == "" || freshCluster.Generation <= 0 ||
		freshCluster.ResourceVersion == "" || shardingName == "" {
		return nil, false, invalid("fresh Cluster identity and sharding name must be complete")
	}
	if !freshCluster.DeletionTimestamp.IsZero() {
		return nil, false, invalid("fresh Cluster must not be deleting")
	}

	shardingSpec, err := exactClusterSharding(freshCluster, shardingName)
	if err != nil {
		return nil, false, invalid("resolve fresh Cluster sharding: %v", err)
	}
	if shardingSpec.ShardingDef == "" {
		if freshCluster.Spec.Topology != "" {
			return nil, false, invalid(
				"topology-based Cluster does not expose the effective sharding definition selector")
		}
		return nil, false, nil
	}
	shardingDef, err := resolveShardingDefinition(ctx, apiReader, shardingSpec.ShardingDef)
	if err != nil {
		return nil, false, markClusterTopologyExecutionBarrier(
			fmt.Errorf("resolve fresh ShardingDefinition for scale-in protocol: %w", err))
	}
	if shardingDef == nil || shardingDef.Name == "" || shardingDef.UID == "" ||
		shardingDef.Generation <= 0 || !shardingDef.DeletionTimestamp.IsZero() {
		return nil, false, invalid("fresh ShardingDefinition identity must be complete and not deleting")
	}
	if shardingDef.Spec.LifecycleActions == nil ||
		shardingDef.Spec.LifecycleActions.ShardRemove == nil {
		return shardingDef.DeepCopy(), false, nil
	}
	switch protocol := shardingDef.Spec.LifecycleActions.ShardRemove.ResultProtocol; protocol {
	case "":
		return shardingDef.DeepCopy(), false, nil
	case appsv1.ShardingScaleInResultProtocolV2:
		return shardingDef.DeepCopy(), true, nil
	default:
		return nil, false, invalid("fresh ShardingDefinition has unsupported shard-remove result protocol %q",
			protocol)
	}
}

func (h *clusterShardingHandler) validateFreshLegacyScaleInDefinition(
	transCtx *clusterTransformContext,
	shardingName string,
	freshShardingDef *appsv1.ShardingDefinition,
) error {
	invalid := func(format string, args ...any) error {
		return fmt.Errorf("%w: %s",
			errInvalidShardingScaleInInitialPlanSource, fmt.Sprintf(format, args...))
	}
	cachedShardingDef := h.shardingDef(transCtx, shardingName)
	if freshShardingDef == nil {
		if cachedShardingDef != nil {
			return invalid("inline legacy sharding has a cached ShardingDefinition")
		}
		return nil
	}
	if cachedShardingDef == nil ||
		cachedShardingDef.Name != freshShardingDef.Name ||
		cachedShardingDef.UID != freshShardingDef.UID ||
		cachedShardingDef.Generation != freshShardingDef.Generation ||
		!cachedShardingDef.DeletionTimestamp.IsZero() ||
		!equality.Semantic.DeepEqual(cachedShardingDef.Spec, freshShardingDef.Spec) {
		return invalid("cached ShardingDefinition does not match the fresh legacy definition")
	}
	return nil
}

func addInitialShardingScaleInPlanIntent(
	ctx context.Context,
	dag *graph.DAG,
	apiReader client.Reader,
	source shardingScaleInInitialPlanMaterialSource,
	clusterKey types.NamespacedName,
	expectedClusterUID types.UID,
	shardingName string,
) error {
	invalid := func(format string, args ...any) error {
		return fmt.Errorf("%w: %s",
			errInvalidShardingScaleInInitialPlanSource, fmt.Sprintf(format, args...))
	}
	if ctx == nil || dag == nil || apiReader == nil || source == nil {
		return invalid("context, DAG, APIReader, and source must be non-nil")
	}
	if clusterKey.Namespace == "" || clusterKey.Name == "" ||
		expectedClusterUID == "" || shardingName == "" {
		return invalid("Cluster key, expected UID, and sharding name must be complete")
	}

	material, err := source.BuildInitialShardingScaleInPlanMaterial(
		ctx, apiReader, clusterKey, expectedClusterUID, shardingName)
	if err != nil {
		return markClusterTopologyExecutionBarrier(
			fmt.Errorf("build initial sharding scale-in PlanMaterial: %w", err))
	}
	if material == nil {
		return invalid("source returned nil PlanMaterial")
	}
	canonical, _, err := buildShardingScaleInPlanMaterial(material)
	if err != nil {
		return invalid("source returned invalid PlanMaterial: %v", err)
	}
	if !equality.Semantic.DeepEqual(canonical, material) {
		return invalid("source returned non-canonical PlanMaterial")
	}

	freshCluster := &appsv1.Cluster{}
	if err := apiReader.Get(ctx, clusterKey, freshCluster); err != nil {
		return markClusterTopologyExecutionBarrier(
			fmt.Errorf("read fresh Cluster for initial sharding scale-in plan: %w", err))
	}
	if freshCluster.Namespace != clusterKey.Namespace ||
		freshCluster.Name != clusterKey.Name ||
		freshCluster.UID != expectedClusterUID {
		return invalid("fresh Cluster name or UID does not match the requested identity")
	}
	if freshCluster.ResourceVersion == "" || freshCluster.Generation <= 0 {
		return invalid("fresh Cluster resourceVersion and generation must be non-empty")
	}
	if !freshCluster.DeletionTimestamp.IsZero() {
		return invalid("fresh Cluster is deleting")
	}
	if canonical.ShardingName != shardingName {
		return invalid("PlanMaterial sharding name %q does not match %q",
			canonical.ShardingName, shardingName)
	}
	if canonical.Source.ClusterNamespace != freshCluster.Namespace ||
		canonical.Source.ClusterName != freshCluster.Name ||
		canonical.Source.ClusterUID != freshCluster.UID ||
		canonical.Source.ClusterGeneration != freshCluster.Generation {
		return invalid("PlanMaterial source does not match the fresh Cluster identity")
	}

	fenceNonce := make([]byte, shardingScaleInTopologyFenceNonceBytes)
	if _, err := rand.Read(fenceNonce); err != nil {
		return markClusterTopologyExecutionBarrier(
			fmt.Errorf("generate sharding scale-in topology fence nonce: %w", err))
	}
	acquiredAt := metav1.Now()
	status, lock, err := buildInitialShardingScaleInState(canonical, fenceNonce, &acquiredAt)
	if err != nil {
		return markClusterTopologyExecutionBarrier(err)
	}
	_, _, patch, err := buildInitialShardingScaleInPlanPatch(
		freshCluster, shardingName, status, lock)
	if err != nil {
		return markClusterTopologyExecutionBarrier(err)
	}
	return markClusterTopologyExecutionBarrier(
		addClusterStatusCASVertex(dag, freshCluster, patch))
}
