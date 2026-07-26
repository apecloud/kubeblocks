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
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"

	"k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	"github.com/apecloud/kubeblocks/pkg/controller/graph"
)

const clusterTopologyReconcileFenceNonceBytes = 32

var errInvalidClusterTopologyReconcileLock = errors.New("invalid Cluster topology reconcile lock")

type clusterTopologyExecutionBarrierError struct {
	cause error
}

// clusterTopologyReconcileAuthorityTransformer binds an existing ordinary
// topology lock before transformers that can add non-Component write intents.
// Lock acquisition remains in the Component transformer because it depends on
// the normalized sharding diff and typed scale-in classification.
type clusterTopologyReconcileAuthorityTransformer struct{}

var _ graph.Transformer = &clusterTopologyReconcileAuthorityTransformer{}

func (t *clusterTopologyReconcileAuthorityTransformer) Transform(
	ctx graph.TransformContext,
	dag *graph.DAG,
) error {
	transCtx, ok := ctx.(*clusterTransformContext)
	if !ok || transCtx == nil || transCtx.Cluster == nil || transCtx.OrigCluster == nil {
		return fmt.Errorf("%w: Cluster transform context must be complete",
			errInvalidClusterTopologyReconcileLock)
	}
	if transCtx.OrigCluster.IsDeleting() {
		return nil
	}
	lock := transCtx.OrigCluster.Status.TopologyMutationLock
	if lock == nil {
		return nil
	}
	own, err := isClusterTopologyReconcileLock(transCtx.OrigCluster, lock)
	if err != nil {
		return err
	}
	if !own {
		return fmt.Errorf("%w: topology lock owned by %q plan %q is in state %q",
			errShardingScaleInPlanProgressionUnavailable,
			lock.OwnerKind, lock.OwnerPlanID, lock.State)
	}
	if lock.State == appsv1.TopologyMutationLockStateReleased {
		return addClusterTopologyReconcileReleasedReceiptCleanupIntent(
			transCtx,
			dag,
			lock,
		)
	}
	return bindClusterTopologyReconcileLockAuthority(transCtx, lock)
}

func (e *clusterTopologyExecutionBarrierError) Error() string {
	return e.cause.Error()
}

func (e *clusterTopologyExecutionBarrierError) Unwrap() error {
	return e.cause
}

func markClusterTopologyExecutionBarrier(err error) error {
	if err == nil || isClusterTopologyExecutionBarrierError(err) {
		return err
	}
	return &clusterTopologyExecutionBarrierError{cause: err}
}

func clusterTopologyReconcilePlanID(clusterUID types.UID) string {
	return fmt.Sprintf("%s/%s", appsv1.TopologyMutationLockOwnerClusterTopologyReconcile, clusterUID)
}

func newClusterTopologyReconcileFenceToken(clusterUID types.UID) (string, error) {
	nonce := make([]byte, clusterTopologyReconcileFenceNonceBytes)
	if _, err := rand.Read(nonce); err != nil {
		return "", fmt.Errorf("%w: generate fence nonce: %v",
			errInvalidClusterTopologyReconcileLock, err)
	}
	fenceMaterial := append([]byte(clusterUID), 0)
	fenceMaterial = append(fenceMaterial, nonce...)
	fenceToken := sha256.Sum256(fenceMaterial)
	return hex.EncodeToString(fenceToken[:]), nil
}

func buildClusterTopologyReconcileLock(
	cluster *appsv1.Cluster,
) (*appsv1.TopologyMutationLockStatus, error) {
	if cluster == nil || cluster.UID == "" {
		return nil, fmt.Errorf("%w: Cluster UID must not be empty",
			errInvalidClusterTopologyReconcileLock)
	}
	fenceToken, err := newClusterTopologyReconcileFenceToken(cluster.UID)
	if err != nil {
		return nil, err
	}
	acquiredAt := metav1.Now()
	return &appsv1.TopologyMutationLockStatus{
		Version:               appsv1.TopologyMutationLockVersionV1,
		FenceToken:            fenceToken,
		ClusterUID:            cluster.UID,
		OwnerKind:             appsv1.TopologyMutationLockOwnerClusterTopologyReconcile,
		OwnerPlanID:           clusterTopologyReconcilePlanID(cluster.UID),
		State:                 appsv1.TopologyMutationLockStateHeld,
		AcquiredAt:            &acquiredAt,
		AffectedComponentUIDs: []types.UID{},
	}, nil
}

func validateClusterTopologyReconcileLock(
	cluster *appsv1.Cluster,
	lock *appsv1.TopologyMutationLockStatus,
) error {
	invalid := func(format string, args ...any) error {
		return fmt.Errorf("%w: %s",
			errInvalidClusterTopologyReconcileLock, fmt.Sprintf(format, args...))
	}
	if cluster == nil || cluster.UID == "" || lock == nil {
		return invalid("Cluster identity and lock must be complete")
	}
	if lock.Version != appsv1.TopologyMutationLockVersionV1 {
		return invalid("version must be %q", appsv1.TopologyMutationLockVersionV1)
	}
	fenceToken, err := hex.DecodeString(lock.FenceToken)
	if err != nil || len(fenceToken) != sha256.Size {
		return invalid("fence token must be a %d-byte hex digest", sha256.Size)
	}
	if lock.ClusterUID != cluster.UID {
		return invalid("lock Cluster UID does not match the Cluster")
	}
	if lock.OwnerKind != appsv1.TopologyMutationLockOwnerClusterTopologyReconcile {
		return invalid("owner kind must be %q",
			appsv1.TopologyMutationLockOwnerClusterTopologyReconcile)
	}
	if lock.OwnerPlanID != clusterTopologyReconcilePlanID(cluster.UID) {
		return invalid("owner plan ID does not match the Cluster")
	}
	switch lock.State {
	case appsv1.TopologyMutationLockStateHeld,
		appsv1.TopologyMutationLockStateExecuting,
		appsv1.TopologyMutationLockStateReleased:
	default:
		return invalid("state must be %q, %q, or %q",
			appsv1.TopologyMutationLockStateHeld,
			appsv1.TopologyMutationLockStateExecuting,
			appsv1.TopologyMutationLockStateReleased)
	}
	if lock.AcquiredAt == nil || lock.AcquiredAt.IsZero() {
		return invalid("acquiredAt must not be empty")
	}
	if len(lock.AffectedComponentUIDs) != 0 {
		return invalid("cluster-wide ordinary reconcile lock must not narrow affected Components")
	}
	return nil
}

func isClusterTopologyReconcileLock(
	cluster *appsv1.Cluster,
	lock *appsv1.TopologyMutationLockStatus,
) (bool, error) {
	if lock == nil ||
		lock.OwnerKind != appsv1.TopologyMutationLockOwnerClusterTopologyReconcile {
		return false, nil
	}
	if err := validateClusterTopologyReconcileLock(cluster, lock); err != nil {
		return true, err
	}
	return true, nil
}

func isClusterTopologyExecutionBarrierError(err error) bool {
	var barrierErr *clusterTopologyExecutionBarrierError
	return errors.Is(err, errShardingScaleInInitialPlanSourceUnavailable) ||
		errors.Is(err, errInvalidShardingScaleInInitialPlanSource) ||
		errors.Is(err, errShardingScaleInPlanProgressionUnavailable) ||
		errors.Is(err, errInvalidClusterTopologyReconcileLock) ||
		errors.As(err, &barrierErr)
}

func addClusterTopologyReconcileLockIntent(
	transCtx *clusterTransformContext,
	dag *graph.DAG,
	freshCluster *appsv1.Cluster,
) error {
	if transCtx == nil || dag == nil || freshCluster == nil {
		return fmt.Errorf("%w: transform context, DAG, and fresh Cluster must be complete",
			errInvalidClusterTopologyReconcileLock)
	}
	if freshCluster.Status.TopologyMutationLock != nil {
		return fmt.Errorf("%w: fresh Cluster already has a topology mutation lock",
			errInvalidClusterTopologyReconcileLock)
	}
	lock, err := buildClusterTopologyReconcileLock(freshCluster)
	if err != nil {
		return err
	}
	operations := []shardingScaleInJSONPatchOperation{
		{
			Operation: "test",
			Path:      "/metadata/uid",
			Value:     string(freshCluster.UID),
		},
		{
			Operation: "test",
			Path:      "/metadata/resourceVersion",
			Value:     freshCluster.ResourceVersion,
		},
		{
			Operation: "add",
			Path:      "/status/topologyMutationLock",
			Value:     lock,
		},
	}
	patch, err := json.Marshal(operations)
	if err != nil {
		return fmt.Errorf("%w: marshal acquisition patch: %v",
			errInvalidClusterTopologyReconcileLock, err)
	}
	if err := addClusterStatusCASVertex(dag, freshCluster, patch); err != nil {
		return markClusterTopologyExecutionBarrier(err)
	}
	transCtx.clusterTopologyReconcileLockAcquisitionPending = true
	return graph.ErrPrematureStop
}

func addClusterTopologyReconcileReleasedReceiptCleanupIntent(
	transCtx *clusterTransformContext,
	dag *graph.DAG,
	releasedLock *appsv1.TopologyMutationLockStatus,
) error {
	if transCtx == nil || dag == nil || transCtx.OrigCluster == nil {
		return fmt.Errorf("%w: transform context, DAG, and original Cluster must be complete",
			errInvalidClusterTopologyReconcileLock)
	}
	if releasedLock == nil ||
		releasedLock.State != appsv1.TopologyMutationLockStateReleased ||
		!equality.Semantic.DeepEqual(
			transCtx.OrigCluster.Status.TopologyMutationLock,
			releasedLock,
		) {
		return fmt.Errorf("%w: released receipt cleanup requires the exact original receipt",
			errInvalidClusterTopologyReconcileLock)
	}
	operations := []shardingScaleInJSONPatchOperation{
		{
			Operation: "test",
			Path:      "/metadata/uid",
			Value:     string(transCtx.OrigCluster.UID),
		},
		{
			Operation: "test",
			Path:      "/metadata/resourceVersion",
			Value:     transCtx.OrigCluster.ResourceVersion,
		},
		{
			Operation: "test",
			Path:      "/status/topologyMutationLock",
			Value:     releasedLock,
		},
		{
			Operation: "remove",
			Path:      "/status/topologyMutationLock",
		},
	}
	patch, err := json.Marshal(operations)
	if err != nil {
		return fmt.Errorf("%w: marshal released receipt cleanup patch: %v",
			errInvalidClusterTopologyReconcileLock, err)
	}
	if err := addClusterStatusCASVertex(
		dag,
		transCtx.OrigCluster,
		patch,
	); err != nil {
		return markClusterTopologyExecutionBarrier(err)
	}
	return graph.ErrPrematureStop
}

func bindClusterTopologyReconcileLockAuthority(
	transCtx *clusterTransformContext,
	lock *appsv1.TopologyMutationLockStatus,
) error {
	if transCtx == nil || transCtx.Cluster == nil {
		return fmt.Errorf("%w: transform context and Cluster must be complete",
			errInvalidClusterTopologyReconcileLock)
	}
	if err := validateClusterTopologyReconcileLock(transCtx.Cluster, lock); err != nil {
		return err
	}
	if transCtx.clusterTopologyReconcileLockAuthority != nil &&
		!equality.Semantic.DeepEqual(transCtx.clusterTopologyReconcileLockAuthority, lock) {
		return fmt.Errorf("%w: conflicting execution authority identities",
			errInvalidClusterTopologyReconcileLock)
	}
	transCtx.clusterTopologyReconcileLockAuthority = lock.DeepCopy()
	return nil
}

func requestClusterTopologyReconcileLockRelease(
	transCtx *clusterTransformContext,
	lock *appsv1.TopologyMutationLockStatus,
) error {
	if transCtx == nil || transCtx.Cluster == nil {
		return fmt.Errorf("%w: transform context and Cluster must be complete",
			errInvalidClusterTopologyReconcileLock)
	}
	if err := validateClusterTopologyReconcileLock(transCtx.Cluster, lock); err != nil {
		return err
	}
	if err := bindClusterTopologyReconcileLockAuthority(transCtx, lock); err != nil {
		return err
	}
	if transCtx.clusterTopologyReconcileLockRelease != nil &&
		!equality.Semantic.DeepEqual(transCtx.clusterTopologyReconcileLockRelease, lock) {
		return fmt.Errorf("%w: conflicting release identities",
			errInvalidClusterTopologyReconcileLock)
	}
	transCtx.clusterTopologyReconcileLockRelease = lock.DeepCopy()
	return nil
}

func requestCachedClusterTopologyReconcileLockReleaseIfPresent(
	transCtx *clusterTransformContext,
) (bool, error) {
	if transCtx == nil || transCtx.Cluster == nil {
		return false, fmt.Errorf("%w: transform context and Cluster must be complete",
			errInvalidClusterTopologyReconcileLock)
	}
	lock := transCtx.Cluster.Status.TopologyMutationLock
	own, err := isClusterTopologyReconcileLock(transCtx.Cluster, lock)
	if err != nil || !own {
		return own, err
	}
	return true, requestClusterTopologyReconcileLockRelease(transCtx, lock)
}

func activateClusterTopologyReconcileLock(
	ctx context.Context,
	cli client.Client,
	expectedCluster *appsv1.Cluster,
	expectedLock *appsv1.TopologyMutationLockStatus,
) (*appsv1.TopologyMutationLockStatus, error) {
	invalid := func(format string, args ...any) error {
		return fmt.Errorf("%w: %s",
			errInvalidClusterTopologyReconcileLock, fmt.Sprintf(format, args...))
	}
	if ctx == nil || cli == nil || expectedCluster == nil {
		return nil, invalid("context, client, and expected Cluster must be complete")
	}
	if expectedCluster.ResourceVersion == "" {
		return nil, invalid("expected Cluster resourceVersion must not be empty")
	}
	if err := validateClusterTopologyReconcileLock(expectedCluster, expectedLock); err != nil {
		return nil, err
	}
	if !equality.Semantic.DeepEqual(
		expectedCluster.Status.TopologyMutationLock, expectedLock) {
		return nil, invalid("cached Cluster lock does not match the execution authority")
	}
	executionLock := expectedLock.DeepCopy()
	fenceToken, err := newClusterTopologyReconcileFenceToken(expectedCluster.UID)
	if err != nil {
		return nil, err
	}
	executionLock.FenceToken = fenceToken
	executionLock.State = appsv1.TopologyMutationLockStateExecuting

	operations := []shardingScaleInJSONPatchOperation{
		{
			Operation: "test",
			Path:      "/metadata/uid",
			Value:     string(expectedCluster.UID),
		},
		{
			Operation: "test",
			Path:      "/metadata/resourceVersion",
			Value:     expectedCluster.ResourceVersion,
		},
		{
			Operation: "test",
			Path:      "/status/topologyMutationLock",
			Value:     expectedLock,
		},
		{
			Operation: "replace",
			Path:      "/status/topologyMutationLock",
			Value:     executionLock,
		},
	}
	patch, err := json.Marshal(operations)
	if err != nil {
		return nil, invalid("marshal execution authority patch: %v", err)
	}
	if err := cli.Status().Patch(
		ctx,
		expectedCluster.DeepCopy(),
		client.RawPatch(types.JSONPatchType, patch),
	); err != nil {
		return nil, err
	}
	return executionLock, nil
}

func releaseClusterTopologyReconcileLock(
	ctx context.Context,
	apiReader client.Reader,
	cli client.Client,
	expectedCluster *appsv1.Cluster,
	expectedLock *appsv1.TopologyMutationLockStatus,
) error {
	invalid := func(format string, args ...any) error {
		return fmt.Errorf("%w: %s",
			errInvalidClusterTopologyReconcileLock, fmt.Sprintf(format, args...))
	}
	if ctx == nil || apiReader == nil || cli == nil || expectedCluster == nil {
		return invalid("context, clients, and expected Cluster must be complete")
	}
	if err := validateClusterTopologyReconcileLock(expectedCluster, expectedLock); err != nil {
		return err
	}
	if expectedLock.State != appsv1.TopologyMutationLockStateExecuting {
		return invalid("release identity state must be %q",
			appsv1.TopologyMutationLockStateExecuting)
	}
	freshCluster := &appsv1.Cluster{}
	key := client.ObjectKeyFromObject(expectedCluster)
	if err := apiReader.Get(ctx, key, freshCluster); err != nil {
		return fmt.Errorf("read fresh Cluster for ordinary topology lock release: %w", err)
	}
	if freshCluster.Namespace != expectedCluster.Namespace ||
		freshCluster.Name != expectedCluster.Name ||
		freshCluster.UID != expectedCluster.UID ||
		freshCluster.ResourceVersion == "" {
		return invalid("fresh Cluster identity does not match the lock owner")
	}
	if freshCluster.Status.TopologyMutationLock == nil {
		return invalid("fresh Cluster is missing the expected Executing receipt")
	}
	if !equality.Semantic.DeepEqual(freshCluster.Status.TopologyMutationLock, expectedLock) {
		return invalid("fresh Cluster topology lock does not match the release identity")
	}
	releasedLock := expectedLock.DeepCopy()
	releasedLock.State = appsv1.TopologyMutationLockStateReleased
	operations := []shardingScaleInJSONPatchOperation{
		{
			Operation: "test",
			Path:      "/metadata/uid",
			Value:     string(freshCluster.UID),
		},
		{
			Operation: "test",
			Path:      "/metadata/resourceVersion",
			Value:     freshCluster.ResourceVersion,
		},
		{
			Operation: "test",
			Path:      "/status/topologyMutationLock",
			Value:     expectedLock,
		},
		{
			Operation: "replace",
			Path:      "/status/topologyMutationLock",
			Value:     releasedLock,
		},
	}
	patch, err := json.Marshal(operations)
	if err != nil {
		return invalid("marshal release receipt patch: %v", err)
	}
	if err := cli.Status().Patch(
		ctx,
		freshCluster,
		client.RawPatch(types.JSONPatchType, patch),
	); err != nil {
		readback, readErr := readClusterTopologyReconcileLock(
			ctx,
			apiReader,
			expectedCluster,
		)
		if readErr != nil {
			return fmt.Errorf("resolve ordinary topology release receipt after patch error %v: %w",
				err, readErr)
		}
		switch {
		case equality.Semantic.DeepEqual(
			readback.Status.TopologyMutationLock,
			releasedLock,
		):
		case equality.Semantic.DeepEqual(
			readback.Status.TopologyMutationLock,
			expectedLock,
		):
			return err
		default:
			return invalid("release receipt outcome is ambiguous after patch error: %v", err)
		}
	}

	releasedCluster, err := readClusterTopologyReconcileLock(
		ctx,
		apiReader,
		expectedCluster,
	)
	if err != nil {
		return err
	}
	if !equality.Semantic.DeepEqual(
		releasedCluster.Status.TopologyMutationLock,
		releasedLock,
	) {
		return invalid("fresh Cluster does not contain the exact Released receipt")
	}
	cleanupOperations := []shardingScaleInJSONPatchOperation{
		{
			Operation: "test",
			Path:      "/metadata/uid",
			Value:     string(releasedCluster.UID),
		},
		{
			Operation: "test",
			Path:      "/metadata/resourceVersion",
			Value:     releasedCluster.ResourceVersion,
		},
		{
			Operation: "test",
			Path:      "/status/topologyMutationLock",
			Value:     releasedLock,
		},
		{
			Operation: "remove",
			Path:      "/status/topologyMutationLock",
		},
	}
	cleanupPatch, err := json.Marshal(cleanupOperations)
	if err != nil {
		return invalid("marshal released receipt cleanup patch: %v", err)
	}
	if err := cli.Status().Patch(
		ctx,
		releasedCluster,
		client.RawPatch(types.JSONPatchType, cleanupPatch),
	); err != nil {
		readback, readErr := readClusterTopologyReconcileLock(
			ctx,
			apiReader,
			expectedCluster,
		)
		if readErr != nil {
			return fmt.Errorf("resolve ordinary topology release cleanup after patch error %v: %w",
				err, readErr)
		}
		switch {
		case readback.Status.TopologyMutationLock == nil:
			return nil
		case equality.Semantic.DeepEqual(
			readback.Status.TopologyMutationLock,
			releasedLock,
		):
			return err
		default:
			return invalid("release cleanup outcome is ambiguous after patch error: %v", err)
		}
	}
	return nil
}

func readClusterTopologyReconcileLock(
	ctx context.Context,
	apiReader client.Reader,
	expectedCluster *appsv1.Cluster,
) (*appsv1.Cluster, error) {
	freshCluster := &appsv1.Cluster{}
	if err := apiReader.Get(
		ctx,
		client.ObjectKeyFromObject(expectedCluster),
		freshCluster,
	); err != nil {
		return nil, fmt.Errorf("read fresh Cluster for ordinary topology lock release: %w", err)
	}
	if freshCluster.Namespace != expectedCluster.Namespace ||
		freshCluster.Name != expectedCluster.Name ||
		freshCluster.UID != expectedCluster.UID ||
		freshCluster.ResourceVersion == "" {
		return nil, fmt.Errorf("%w: fresh Cluster identity does not match the lock owner",
			errInvalidClusterTopologyReconcileLock)
	}
	return freshCluster, nil
}
