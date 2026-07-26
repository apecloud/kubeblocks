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
	"fmt"
	"math"
	"reflect"

	"k8s.io/apimachinery/pkg/types"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	kbagentproto "github.com/apecloud/kubeblocks/pkg/kbagent/proto"
)

type shardingScaleInPlanMemberSet struct {
	Leaving []appsv1.ShardingScaleInPlanMember
	Staying []appsv1.ShardingScaleInPlanMember
}

func renderShardingScaleInMemberLines(
	material *appsv1.ShardingScaleInPlanMaterial,
) ([]string, error) {
	if material == nil {
		return nil, fmt.Errorf("%w: material must not be nil",
			errInvalidShardingScaleInPlanMaterial)
	}
	canonical, _, err := buildShardingScaleInPlanMaterial(material)
	if err != nil {
		return nil, err
	}
	if !reflect.DeepEqual(canonical, material) {
		return nil, fmt.Errorf("%w: persisted member roster is not canonical",
			errInvalidShardingScaleInPlanMaterial)
	}

	value := kbagentproto.ShardingScaleInMemberSet{
		Leaving: shardingScaleInPlanMembersToProtocol(material.Leaving),
		Staying: shardingScaleInPlanMembersToProtocol(material.Staying),
	}
	lines, err := kbagentproto.EncodeShardingScaleInMemberSet(value)
	if err != nil {
		return nil, fmt.Errorf("%w: render member record: %v",
			errInvalidShardingScaleInPlanMaterial, err)
	}
	decoded, err := kbagentproto.DecodeShardingScaleInMemberSet(lines)
	if err != nil {
		return nil, fmt.Errorf("%w: verify member record: %v",
			errInvalidShardingScaleInPlanMaterial, err)
	}
	roundTrip, err := shardingScaleInPlanMembersFromProtocol(decoded)
	if err != nil {
		return nil, err
	}
	if !reflect.DeepEqual(roundTrip.Leaving, material.Leaving) ||
		!reflect.DeepEqual(roundTrip.Staying, material.Staying) {
		return nil, fmt.Errorf("%w: member record round trip mismatch",
			errInvalidShardingScaleInPlanMaterial)
	}
	return lines, nil
}

func shardingScaleInPlanMembersToProtocol(
	members []appsv1.ShardingScaleInPlanMember,
) []kbagentproto.ShardingScaleInMember {
	result := make([]kbagentproto.ShardingScaleInMember, 0, len(members))
	for _, member := range members {
		protocolMember := kbagentproto.ShardingScaleInMember{
			ComponentName:       member.ComponentName,
			ComponentUID:        string(member.ComponentUID),
			ComponentGeneration: uint64(member.ComponentGeneration),
			ComponentSpecDigest: member.ComponentSpecDigest,
			ComponentShortName:  member.ComponentShortName,
			ShardTemplateName:   member.ShardTemplateName,
			Pods: make([]kbagentproto.ShardingScaleInMemberPod, 0,
				len(member.Pods)),
		}
		for _, pod := range member.Pods {
			protocolMember.Pods = append(protocolMember.Pods,
				kbagentproto.ShardingScaleInMemberPod{
					Name:                  pod.Name,
					UID:                   string(pod.UID),
					FQDN:                  pod.FQDN,
					AgentImageID:          pod.AgentImageID,
					AgentProcessUID:       pod.AgentProcessUID,
					AgentCapabilityDigest: pod.AgentCapabilityDigest,
				})
		}
		result = append(result, protocolMember)
	}
	return result
}

func shardingScaleInPlanMembersFromProtocol(
	value kbagentproto.ShardingScaleInMemberSet,
) (*shardingScaleInPlanMemberSet, error) {
	convert := func(
		members []kbagentproto.ShardingScaleInMember,
	) ([]appsv1.ShardingScaleInPlanMember, error) {
		result := make([]appsv1.ShardingScaleInPlanMember, 0, len(members))
		for _, member := range members {
			if member.ComponentGeneration > math.MaxInt64 {
				return nil, fmt.Errorf("%w: component generation overflows int64",
					errInvalidShardingScaleInPlanMaterial)
			}
			planMember := appsv1.ShardingScaleInPlanMember{
				ComponentName:       member.ComponentName,
				ComponentUID:        types.UID(member.ComponentUID),
				ComponentGeneration: int64(member.ComponentGeneration),
				ComponentSpecDigest: member.ComponentSpecDigest,
				ComponentShortName:  member.ComponentShortName,
				ShardTemplateName:   member.ShardTemplateName,
				Pods: make([]appsv1.ShardingScaleInPlanPod, 0,
					len(member.Pods)),
			}
			for _, pod := range member.Pods {
				planMember.Pods = append(planMember.Pods,
					appsv1.ShardingScaleInPlanPod{
						Name:                  pod.Name,
						UID:                   types.UID(pod.UID),
						FQDN:                  pod.FQDN,
						AgentImageID:          pod.AgentImageID,
						AgentProcessUID:       pod.AgentProcessUID,
						AgentCapabilityDigest: pod.AgentCapabilityDigest,
					})
			}
			result = append(result, planMember)
		}
		return result, nil
	}

	leaving, err := convert(value.Leaving)
	if err != nil {
		return nil, err
	}
	staying, err := convert(value.Staying)
	if err != nil {
		return nil, err
	}
	return &shardingScaleInPlanMemberSet{
		Leaving: leaving,
		Staying: staying,
	}, nil
}
