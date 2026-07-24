/*
Copyright (C) 2022-2026 ApeCloud Co., Ltd

This file is part of KubeBlocks project

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program.  If not, see <http://www.gnu.org/licenses/>.
*/

package cluster

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
)

var (
	errShardingScaleInStatusCASConflict       = errors.New("sharding scale-in status compare-and-set conflict")
	errInvalidShardingScaleInStatusTransition = errors.New("invalid sharding scale-in status transition")
	errInvalidTopologyMutationLock            = errors.New("invalid topology mutation lock")
)

type shardingScaleInStatusTransition struct {
	ExpectedProtocolVersion appsv1.ShardingActionResultProtocol
	ExpectedPlanID          string
	ExpectedPhase           appsv1.ShardingScaleInPhase
	Next                    *appsv1.ShardingScaleInStatus
}

type shardingScaleInJSONPatchOperation struct {
	Operation string `json:"op"`
	Path      string `json:"path"`
	Value     any    `json:"value,omitempty"`
}

func patchInitialShardingScaleInPlan(ctx context.Context, cli client.Client, cluster *appsv1.Cluster,
	shardingName string, next *appsv1.ShardingScaleInStatus, lock *appsv1.TopologyMutationLockStatus) error {
	_, _, patch, err := buildInitialShardingScaleInPlanPatch(cluster, shardingName, next, lock)
	if err != nil {
		return err
	}
	return cli.Status().Patch(ctx, cluster, client.RawPatch(types.JSONPatchType, patch))
}

func buildShardingScaleInStatusPatch(cluster *appsv1.Cluster, shardingName string,
	transition shardingScaleInStatusTransition) (*appsv1.ShardingScaleInStatus, []byte, error) {
	return buildShardingScaleInStatusPatchInternal(cluster, shardingName, transition, false)
}

func buildInitialShardingScaleInPlanPatch(cluster *appsv1.Cluster, shardingName string,
	next *appsv1.ShardingScaleInStatus, lock *appsv1.TopologyMutationLockStatus,
) (*appsv1.ShardingScaleInStatus, *appsv1.TopologyMutationLockStatus, []byte, error) {
	if cluster == nil {
		return nil, nil, nil, fmt.Errorf("%w: cluster must not be nil", errInvalidTopologyMutationLock)
	}
	if cluster.Status.TopologyMutationLock != nil {
		return nil, nil, nil, fmt.Errorf("%w: topology mutation lock already exists",
			errShardingScaleInStatusCASConflict)
	}

	reduced, patch, err := buildShardingScaleInStatusPatchInternal(cluster, shardingName,
		shardingScaleInStatusTransition{Next: next}, true)
	if err != nil {
		return nil, nil, nil, err
	}
	reducedLock, err := validateInitialTopologyMutationLock(cluster, reduced, lock)
	if err != nil {
		return nil, nil, nil, err
	}

	var operations []shardingScaleInJSONPatchOperation
	if err := json.Unmarshal(patch, &operations); err != nil {
		return nil, nil, nil, err
	}
	operations = append(operations, shardingScaleInJSONPatchOperation{
		Operation: "add",
		Path:      "/status/topologyMutationLock",
		Value:     reducedLock,
	})
	patch, err = json.Marshal(operations)
	if err != nil {
		return nil, nil, nil, err
	}
	return reduced, reducedLock, patch, nil
}

func buildShardingScaleInStatusPatchInternal(cluster *appsv1.Cluster, shardingName string,
	transition shardingScaleInStatusTransition, allowInitial bool,
) (*appsv1.ShardingScaleInStatus, []byte, error) {
	if cluster == nil {
		return nil, nil, fmt.Errorf("%w: cluster must not be nil", errInvalidShardingScaleInStatusTransition)
	}
	if cluster.UID == "" || cluster.ResourceVersion == "" {
		return nil, nil, fmt.Errorf("%w: cluster UID and resourceVersion must not be empty",
			errInvalidShardingScaleInStatusTransition)
	}
	if shardingName == "" {
		return nil, nil, fmt.Errorf("%w: sharding name must not be empty",
			errInvalidShardingScaleInStatusTransition)
	}

	shardingStatus, shardingExists := cluster.Status.Shardings[shardingName]
	if shardingStatus.ScaleIn == nil && !allowInitial {
		return nil, nil, fmt.Errorf("%w: initial plan must be created with its topology mutation lock",
			errInvalidShardingScaleInStatusTransition)
	}
	reduced, err := reduceShardingScaleInStatus(shardingStatus.ScaleIn, transition)
	if err != nil {
		return nil, nil, err
	}

	escapedName := escapeJSONPointerToken(shardingName)
	scaleInPath := "/status/shardings/" + escapedName + "/scaleIn"
	operations := []shardingScaleInJSONPatchOperation{
		{Operation: "test", Path: "/metadata/uid", Value: string(cluster.UID)},
		{Operation: "test", Path: "/metadata/resourceVersion", Value: cluster.ResourceVersion},
	}
	if shardingStatus.ScaleIn != nil {
		operations = append(operations,
			shardingScaleInJSONPatchOperation{
				Operation: "test",
				Path:      scaleInPath + "/protocolVersion",
				Value:     string(transition.ExpectedProtocolVersion),
			},
			shardingScaleInJSONPatchOperation{
				Operation: "test",
				Path:      scaleInPath + "/planID",
				Value:     transition.ExpectedPlanID,
			},
			shardingScaleInJSONPatchOperation{
				Operation: "test",
				Path:      scaleInPath + "/phase",
				Value:     string(transition.ExpectedPhase),
			},
		)
	}

	switch {
	case len(cluster.Status.Shardings) == 0:
		operations = append(operations, shardingScaleInJSONPatchOperation{
			Operation: "add",
			Path:      "/status/shardings",
			Value: map[string]appsv1.ClusterShardingStatus{
				shardingName: {ScaleIn: reduced},
			},
		})
	case !shardingExists:
		operations = append(operations, shardingScaleInJSONPatchOperation{
			Operation: "add",
			Path:      "/status/shardings/" + escapedName,
			Value:     appsv1.ClusterShardingStatus{ScaleIn: reduced},
		})
	case shardingStatus.ScaleIn == nil:
		operations = append(operations, shardingScaleInJSONPatchOperation{
			Operation: "add",
			Path:      scaleInPath,
			Value:     reduced,
		})
	default:
		operations = append(operations, shardingScaleInJSONPatchOperation{
			Operation: "replace",
			Path:      scaleInPath,
			Value:     reduced,
		})
	}

	patch, err := json.Marshal(operations)
	if err != nil {
		return nil, nil, err
	}
	return reduced, patch, nil
}

func validateInitialTopologyMutationLock(cluster *appsv1.Cluster, status *appsv1.ShardingScaleInStatus,
	lock *appsv1.TopologyMutationLockStatus) (*appsv1.TopologyMutationLockStatus, error) {
	if lock == nil {
		return nil, fmt.Errorf("%w: lock must not be nil", errInvalidTopologyMutationLock)
	}
	reduced := lock.DeepCopy()
	if reduced.Version != appsv1.TopologyMutationLockVersionV1 {
		return nil, fmt.Errorf("%w: version must be %q",
			errInvalidTopologyMutationLock, appsv1.TopologyMutationLockVersionV1)
	}
	if reduced.FenceToken == "" || reduced.FenceToken != status.TopologyFenceToken {
		return nil, fmt.Errorf("%w: fence token must be non-empty and match the plan",
			errInvalidTopologyMutationLock)
	}
	if reduced.ClusterUID == "" || reduced.ClusterUID != cluster.UID {
		return nil, fmt.Errorf("%w: cluster UID must be non-empty and match the Cluster",
			errInvalidTopologyMutationLock)
	}
	if reduced.OwnerKind != appsv1.TopologyMutationLockOwnerShardingScaleIn {
		return nil, fmt.Errorf("%w: owner kind must be %q",
			errInvalidTopologyMutationLock, appsv1.TopologyMutationLockOwnerShardingScaleIn)
	}
	if reduced.OwnerPlanID == "" || reduced.OwnerPlanID != status.PlanID {
		return nil, fmt.Errorf("%w: owner plan ID must be non-empty and match the plan",
			errInvalidTopologyMutationLock)
	}
	if reduced.State != appsv1.TopologyMutationLockStateInstallingAuthority {
		return nil, fmt.Errorf("%w: initial state must be %q",
			errInvalidTopologyMutationLock, appsv1.TopologyMutationLockStateInstallingAuthority)
	}
	if reduced.AcquiredAt == nil || reduced.AcquiredAt.IsZero() {
		return nil, fmt.Errorf("%w: acquiredAt must not be empty", errInvalidTopologyMutationLock)
	}
	if len(reduced.AffectedComponentUIDs) == 0 {
		return nil, fmt.Errorf("%w: affected Component UIDs must not be empty", errInvalidTopologyMutationLock)
	}
	for i, uid := range reduced.AffectedComponentUIDs {
		if uid == "" {
			return nil, fmt.Errorf("%w: affected Component UID must not be empty", errInvalidTopologyMutationLock)
		}
		if i > 0 && reduced.AffectedComponentUIDs[i-1] >= uid {
			return nil, fmt.Errorf("%w: affected Component UIDs must be sorted and unique",
				errInvalidTopologyMutationLock)
		}
	}
	return reduced, nil
}

func reduceShardingScaleInStatus(current *appsv1.ShardingScaleInStatus,
	transition shardingScaleInStatusTransition) (*appsv1.ShardingScaleInStatus, error) {
	if transition.Next == nil {
		return nil, fmt.Errorf("%w: next status must not be nil", errInvalidShardingScaleInStatusTransition)
	}

	next := transition.Next.DeepCopy()
	if current == nil {
		if transition.ExpectedProtocolVersion != "" || transition.ExpectedPlanID != "" || transition.ExpectedPhase != "" {
			return nil, fmt.Errorf("%w: expected an existing plan", errShardingScaleInStatusCASConflict)
		}
		if next.ProtocolVersion != appsv1.ShardingScaleInResultProtocolV2 {
			return nil, fmt.Errorf("%w: initial protocolVersion must be %q",
				errInvalidShardingScaleInStatusTransition, appsv1.ShardingScaleInResultProtocolV2)
		}
		if next.PlanID == "" {
			return nil, fmt.Errorf("%w: initial planID must not be empty",
				errInvalidShardingScaleInStatusTransition)
		}
		if next.TopologyFenceToken == "" {
			return nil, fmt.Errorf("%w: initial topologyFenceToken must not be empty",
				errInvalidShardingScaleInStatusTransition)
		}
		if next.ExternalWriteAuthorized {
			return nil, fmt.Errorf("%w: initial plan must not authorize external writes",
				errInvalidShardingScaleInStatusTransition)
		}
		if next.Phase != appsv1.ShardingScaleInPhasePlanned {
			return nil, fmt.Errorf("%w: initial phase must be %q",
				errInvalidShardingScaleInStatusTransition, appsv1.ShardingScaleInPhasePlanned)
		}
		if err := validateShardingScaleInPlanMaterialBinding(next); err != nil {
			return nil, err
		}
		if err := validateShardingScaleInBlockState(next); err != nil {
			return nil, err
		}
		return next, nil
	}

	if current.ProtocolVersion != appsv1.ShardingScaleInResultProtocolV2 ||
		current.PlanID == "" || current.Phase == "" || current.TopologyFenceToken == "" {
		return nil, fmt.Errorf("%w: persisted protocolVersion, planID, phase, and topologyFenceToken must be valid and non-empty",
			errInvalidShardingScaleInStatusTransition)
	}
	if transition.ExpectedProtocolVersion != current.ProtocolVersion ||
		transition.ExpectedPlanID != current.PlanID ||
		transition.ExpectedPhase != current.Phase {
		return nil, fmt.Errorf("%w: expected protocol=%q plan=%q phase=%q, got protocol=%q plan=%q phase=%q",
			errShardingScaleInStatusCASConflict,
			transition.ExpectedProtocolVersion, transition.ExpectedPlanID, transition.ExpectedPhase,
			current.ProtocolVersion, current.PlanID, current.Phase)
	}
	if next.ProtocolVersion != current.ProtocolVersion || next.PlanID != current.PlanID {
		return nil, fmt.Errorf("%w: protocolVersion and planID are immutable",
			errInvalidShardingScaleInStatusTransition)
	}
	if next.TopologyFenceToken != current.TopologyFenceToken {
		return nil, fmt.Errorf("%w: topologyFenceToken is immutable",
			errInvalidShardingScaleInStatusTransition)
	}
	if (current.PlanMaterial == nil) != (next.PlanMaterial == nil) {
		return nil, fmt.Errorf("%w: planMaterial presence is immutable",
			errInvalidShardingScaleInStatusTransition)
	}
	if current.PlanMaterial != nil {
		if err := validateShardingScaleInPlanMaterialBinding(current); err != nil {
			return nil, err
		}
		if err := validateShardingScaleInPlanMaterialBinding(next); err != nil {
			return nil, err
		}
		if !equality.Semantic.DeepEqual(current.PlanMaterial, next.PlanMaterial) {
			return nil, fmt.Errorf("%w: planMaterial is immutable",
				errInvalidShardingScaleInStatusTransition)
		}
	}
	if current.ExternalWriteAuthorized && !next.ExternalWriteAuthorized {
		return nil, fmt.Errorf("%w: externalWriteAuthorized cannot be revoked",
			errInvalidShardingScaleInStatusTransition)
	}
	if !current.ExternalWriteAuthorized && next.ExternalWriteAuthorized {
		return nil, fmt.Errorf("%w: external writes require a lock-bound authority transition",
			errInvalidShardingScaleInStatusTransition)
	}
	if !shardingScaleInPhaseTransitionAllowed(current, next) {
		return nil, fmt.Errorf("%w: phase %q cannot transition to %q",
			errInvalidShardingScaleInStatusTransition, current.Phase, next.Phase)
	}
	if err := validateShardingScaleInBlockState(next); err != nil {
		return nil, err
	}
	return next, nil
}

func validateShardingScaleInPlanMaterialBinding(status *appsv1.ShardingScaleInStatus) error {
	if status.PlanMaterial == nil {
		return nil
	}
	canonical, planID, err := buildShardingScaleInPlanMaterial(status.PlanMaterial)
	if err != nil {
		return fmt.Errorf("%w: %v", errInvalidShardingScaleInStatusTransition, err)
	}
	if planID != status.PlanID {
		return fmt.Errorf("%w: planMaterial digest must match planID",
			errInvalidShardingScaleInStatusTransition)
	}
	if !equality.Semantic.DeepEqual(canonical, status.PlanMaterial) {
		return fmt.Errorf("%w: planMaterial must be canonical",
			errInvalidShardingScaleInStatusTransition)
	}
	return nil
}

func shardingScaleInPhaseTransitionAllowed(current, next *appsv1.ShardingScaleInStatus) bool {
	if current.Phase == appsv1.ShardingScaleInPhaseBlocked {
		if next.Phase == appsv1.ShardingScaleInPhaseBlocked {
			return next.BlockedFrom == current.BlockedFrom &&
				!(current.BlockClass == appsv1.ShardingScaleInBlockClassTerminal &&
					next.BlockClass != appsv1.ShardingScaleInBlockClassTerminal)
		}
		return current.BlockClass == appsv1.ShardingScaleInBlockClassRecoverable &&
			current.BlockedFrom != "" &&
			next.Phase == current.BlockedFrom
	}
	if next.Phase == appsv1.ShardingScaleInPhaseBlocked && next.BlockedFrom != current.Phase {
		return false
	}

	allowed := map[appsv1.ShardingScaleInPhase]map[appsv1.ShardingScaleInPhase]struct{}{
		appsv1.ShardingScaleInPhasePlanned: {
			appsv1.ShardingScaleInPhasePlanned:    {},
			appsv1.ShardingScaleInPhaseDraining:   {},
			appsv1.ShardingScaleInPhaseSuperseded: {},
			appsv1.ShardingScaleInPhaseBlocked:    {},
		},
		appsv1.ShardingScaleInPhaseSuperseded: {
			appsv1.ShardingScaleInPhaseSuperseded: {},
			appsv1.ShardingScaleInPhasePlanned:    {},
		},
		appsv1.ShardingScaleInPhaseDraining: {
			appsv1.ShardingScaleInPhaseDraining:      {},
			appsv1.ShardingScaleInPhasePurgePrepared: {},
			appsv1.ShardingScaleInPhaseBlocked:       {},
		},
		appsv1.ShardingScaleInPhasePurgePrepared: {
			appsv1.ShardingScaleInPhasePurgePrepared: {},
			appsv1.ShardingScaleInPhaseResetting:     {},
			appsv1.ShardingScaleInPhaseBlocked:       {},
		},
		appsv1.ShardingScaleInPhaseResetting: {
			appsv1.ShardingScaleInPhaseResetting:  {},
			appsv1.ShardingScaleInPhaseForgetting: {},
			appsv1.ShardingScaleInPhaseBlocked:    {},
		},
		appsv1.ShardingScaleInPhaseForgetting: {
			appsv1.ShardingScaleInPhaseForgetting: {},
			appsv1.ShardingScaleInPhaseVerified:   {},
			appsv1.ShardingScaleInPhaseBlocked:    {},
		},
		appsv1.ShardingScaleInPhaseVerified: {
			appsv1.ShardingScaleInPhaseVerified:        {},
			appsv1.ShardingScaleInPhaseDeleteCommitted: {},
			appsv1.ShardingScaleInPhaseBlocked:         {},
		},
		appsv1.ShardingScaleInPhaseDeleteCommitted: {
			appsv1.ShardingScaleInPhaseDeleting: {},
		},
		appsv1.ShardingScaleInPhaseDeleting: {
			appsv1.ShardingScaleInPhaseDeleting:      {},
			appsv1.ShardingScaleInPhaseHolderPlanned: {},
			appsv1.ShardingScaleInPhaseCompleted:     {},
		},
		appsv1.ShardingScaleInPhaseHolderPlanned: {
			appsv1.ShardingScaleInPhaseDraining: {},
			appsv1.ShardingScaleInPhaseBlocked:  {},
		},
		appsv1.ShardingScaleInPhaseCompleted: {
			appsv1.ShardingScaleInPhaseCompleted: {},
		},
	}
	_, ok := allowed[current.Phase][next.Phase]
	return ok
}

func validateShardingScaleInBlockState(status *appsv1.ShardingScaleInStatus) error {
	if status.Phase == appsv1.ShardingScaleInPhaseBlocked {
		if status.BlockedFrom == "" {
			return fmt.Errorf("%w: blockedFrom must not be empty for Blocked",
				errInvalidShardingScaleInStatusTransition)
		}
		if status.BlockClass != appsv1.ShardingScaleInBlockClassRecoverable &&
			status.BlockClass != appsv1.ShardingScaleInBlockClassTerminal {
			return fmt.Errorf("%w: invalid blockClass %q",
				errInvalidShardingScaleInStatusTransition, status.BlockClass)
		}
		return nil
	}
	if status.BlockedFrom != "" || status.BlockClass != "" {
		return fmt.Errorf("%w: blockedFrom and blockClass require Blocked phase",
			errInvalidShardingScaleInStatusTransition)
	}
	return nil
}

func escapeJSONPointerToken(value string) string {
	return strings.NewReplacer("~", "~0", "/", "~1").Replace(value)
}
