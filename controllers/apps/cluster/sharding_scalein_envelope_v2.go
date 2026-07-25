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
	"errors"
	"fmt"
	"reflect"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	kbagentproto "github.com/apecloud/kubeblocks/pkg/kbagent/proto"
)

var errInvalidShardingScaleInEnvelopeV2 = errors.New(
	"invalid sharding scale-in envelope v2 material")

type shardingScaleInEnvelopeV2Input struct {
	TopologyFenceToken  string
	BaseParameterDigest string
	Phase               appsv1.ShardingScaleInPhase
	HolderTarget        shardingScaleInHolderTarget
	ReceiptID           string
}

func renderShardingScaleInEnvelopeV2(
	persisted *appsv1.ShardingScaleInPlanMaterial,
	planID string,
	input shardingScaleInEnvelopeV2Input,
) ([]byte, error) {
	invalid := func(format string, args ...any) error {
		return fmt.Errorf("%w: %s", errInvalidShardingScaleInEnvelopeV2,
			fmt.Sprintf(format, args...))
	}
	if persisted == nil {
		return nil, invalid("plan material must not be nil")
	}
	canonical, expectedPlanID, err := buildShardingScaleInPlanMaterial(persisted)
	if err != nil {
		return nil, invalid("plan material is invalid: %v", err)
	}
	if !reflect.DeepEqual(canonical, persisted) {
		return nil, invalid("plan material is not canonical")
	}
	if planID != expectedPlanID {
		return nil, invalid("plan ID does not match canonical plan material")
	}
	if !isShardingScaleInSHA256(input.TopologyFenceToken) {
		return nil, invalid("topology fence token must be a SHA256 digest")
	}
	if !isShardingScaleInDispatchablePhase(input.Phase) {
		return nil, invalid("phase is not dispatchable")
	}
	if input.ReceiptID != "" && !isShardingScaleInSHA256(input.ReceiptID) {
		return nil, invalid("receipt ID must be empty or a SHA256 digest")
	}
	if !isShardingScaleInSHA256(input.BaseParameterDigest) {
		return nil, invalid("base parameter digest must be a SHA256 digest")
	}
	baseDigestBound := false
	for _, template := range canonical.RequestAuthority.ExecutorTemplates {
		if template.BaseParameterDigest == input.BaseParameterDigest {
			baseDigestBound = true
			break
		}
	}
	if !baseDigestBound {
		return nil, invalid(
			"base parameter digest is absent from request authority")
	}
	if input.HolderTarget.HolderIndex < 0 ||
		int(input.HolderTarget.HolderIndex) >= len(canonical.Leaving) {
		return nil, invalid("holder index is outside leaving")
	}
	expectedHolder, err := buildShardingScaleInHolderTarget(
		planID,
		input.HolderTarget.HolderIndex,
		canonical.Leaving[input.HolderTarget.HolderIndex],
	)
	if err != nil {
		return nil, invalid("holder target is invalid: %v", err)
	}
	if !reflect.DeepEqual(expectedHolder, input.HolderTarget) {
		return nil, invalid(
			"holder target does not match canonical leaving member")
	}

	envelope := kbagentproto.ShardingScaleInEnvelopeV2{
		PlanID:                 planID,
		TopologyFenceToken:     input.TopologyFenceToken,
		RequestAuthorityDigest: canonical.RequestAuthority.RequestAuthorityDigest,
		BaseParameterDigest:    input.BaseParameterDigest,
		Phase:                  string(input.Phase),
		Holder: kbagentproto.ShardingScaleInHolderTarget{
			ParameterName:      input.HolderTarget.ParameterName,
			HolderIndex:        uint32(input.HolderTarget.HolderIndex),
			ComponentName:      input.HolderTarget.ComponentName,
			ComponentUID:       string(input.HolderTarget.ComponentUID),
			ComponentShortName: input.HolderTarget.ComponentShortName,
			SourceDigest:       input.HolderTarget.SourceDigest,
			ValueB64:           input.HolderTarget.ValueB64,
			ValueDigest:        input.HolderTarget.ValueDigest,
		},
		Members: kbagentproto.ShardingScaleInMemberSet{
			Leaving: shardingScaleInPlanMembersToProtocol(canonical.Leaving),
			Staying: shardingScaleInPlanMembersToProtocol(canonical.Staying),
		},
		ReceiptID: input.ReceiptID,
	}
	encoded, err := kbagentproto.EncodeShardingScaleInEnvelopeV2(envelope)
	if err != nil {
		return nil, invalid("render shared envelope: %v", err)
	}
	decoded, err := kbagentproto.DecodeShardingScaleInEnvelopeV2(encoded)
	if err != nil {
		return nil, invalid("verify shared envelope: %v", err)
	}
	if !reflect.DeepEqual(decoded, envelope) {
		return nil, invalid("shared envelope round trip mismatch")
	}
	return encoded, nil
}
