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

	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
)

var (
	errShardingScaleInStatusCASConflict       = errors.New("sharding scale-in status compare-and-set conflict")
	errInvalidShardingScaleInStatusTransition = errors.New("invalid sharding scale-in status transition")
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

func patchShardingScaleInStatus(ctx context.Context, cli client.Client, cluster *appsv1.Cluster,
	shardingName string, transition shardingScaleInStatusTransition) error {
	_, patch, err := buildShardingScaleInStatusPatch(cluster, shardingName, transition)
	if err != nil {
		return err
	}
	return cli.Status().Patch(ctx, cluster, client.RawPatch(types.JSONPatchType, patch))
}

func buildShardingScaleInStatusPatch(cluster *appsv1.Cluster, shardingName string,
	transition shardingScaleInStatusTransition) (*appsv1.ShardingScaleInStatus, []byte, error) {
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
		if next.Phase != appsv1.ShardingScaleInPhasePlanned {
			return nil, fmt.Errorf("%w: initial phase must be %q",
				errInvalidShardingScaleInStatusTransition, appsv1.ShardingScaleInPhasePlanned)
		}
		if err := validateShardingScaleInBlockState(next); err != nil {
			return nil, err
		}
		return next, nil
	}

	if current.ProtocolVersion != appsv1.ShardingScaleInResultProtocolV2 ||
		current.PlanID == "" || current.Phase == "" {
		return nil, fmt.Errorf("%w: persisted protocolVersion, planID, and phase must be valid and non-empty",
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
	if !shardingScaleInPhaseTransitionAllowed(current, next) {
		return nil, fmt.Errorf("%w: phase %q cannot transition to %q",
			errInvalidShardingScaleInStatusTransition, current.Phase, next.Phase)
	}
	if err := validateShardingScaleInBlockState(next); err != nil {
		return nil, err
	}
	return next, nil
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
