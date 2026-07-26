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
)

var errInvalidShardingScaleInPreparedLaunch = errors.New(
	"invalid sharding scale-in prepared launch")

const shardingScaleInLaunchRequestVersionV2 = "kb.sharding.scalein.launch-request/v2"

type shardingScaleInLaunchRequestMaterial struct {
	Version                  string                                `json:"version"`
	ActionName               string                                `json:"actionName"`
	ExecutionID              string                                `json:"executionID"`
	ExecutorDispatchSequence int64                                 `json:"executorDispatchSequence"`
	ExpectedAgentProcessUID  string                                `json:"expectedAgentProcessUID"`
	RequestPayloadDigest     string                                `json:"requestPayloadDigest"`
	RequestPayload           shardingScaleInRequestPayloadMaterial `json:"requestPayload"`
}

type shardingScaleInPreparedLaunch struct {
	Material            shardingScaleInLaunchRequestMaterial `json:"material"`
	LaunchRequestDigest string                               `json:"launchRequestDigest"`
}

func buildShardingScaleInPreparedLaunch(
	persisted *appsv1.ShardingScaleInPlanMaterial,
	planID string,
	prepared *shardingScaleInPreparedRequestMaterial,
) (*shardingScaleInPreparedLaunch, error) {
	invalid := func(format string, args ...any) error {
		return fmt.Errorf("%w: %s", errInvalidShardingScaleInPreparedLaunch,
			fmt.Sprintf(format, args...))
	}
	if prepared == nil {
		return nil, invalid("prepared request must not be nil")
	}

	payload := prepared.RequestPayload
	var receiptID *string
	if payload.ReceiptID != "" {
		receipt := payload.ReceiptID
		receiptID = &receipt
	}
	rebuilt, err := buildShardingScaleInPreparedRequest(
		persisted,
		planID,
		shardingScaleInPreparedRequestInput{
			HolderIndex:              payload.HolderIndex,
			TopologyFenceToken:       payload.Envelope.TopologyFenceToken,
			Phase:                    payload.Phase,
			ResultRevision:           payload.ResultRevision,
			ReceiptID:                receiptID,
			StepKey:                  payload.StepKey,
			ClaimClass:               payload.ClaimClass,
			ExecutorPodUID:           payload.Executor.PodUID,
			ExecutorDispatchSequence: payload.ExecutorDispatchSequence,
		},
	)
	if err != nil {
		return nil, invalid("prepared request is invalid: %v", err)
	}
	if !reflect.DeepEqual(rebuilt, prepared) {
		return nil, invalid("prepared request does not match its canonical plan")
	}
	if persisted.RequestAuthority.ActionName == "" {
		return nil, invalid("action name must not be empty")
	}

	material := shardingScaleInLaunchRequestMaterial{
		Version:                  shardingScaleInLaunchRequestVersionV2,
		ActionName:               persisted.RequestAuthority.ActionName,
		ExecutionID:              rebuilt.ExecutionID,
		ExecutorDispatchSequence: rebuilt.RequestPayload.ExecutorDispatchSequence,
		ExpectedAgentProcessUID:  rebuilt.RequestPayload.Executor.AgentProcessUID,
		RequestPayloadDigest:     rebuilt.RequestPayloadDigest,
		RequestPayload:           cloneShardingScaleInRequestPayload(rebuilt.RequestPayload),
	}
	digest, err := digestShardingScaleInCanonicalJSON(material)
	if err != nil {
		return nil, invalid("launch request digest failed: %v", err)
	}
	return &shardingScaleInPreparedLaunch{
		Material:            material,
		LaunchRequestDigest: digest,
	}, nil
}

func cloneShardingScaleInRequestPayload(
	payload shardingScaleInRequestPayloadMaterial,
) shardingScaleInRequestPayloadMaterial {
	payload.Envelope = cloneShardingScaleInDurableEnvelope(payload.Envelope)
	return payload
}
