/*
Copyright (C) 2022-2026 ApeCloud Co., Ltd

This file is part of KubeBlocks project

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program.  If not, see <http://www.gnu.org/licenses/>.
*/

package cluster

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"

	"k8s.io/apimachinery/pkg/types"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
)

const (
	shardingScaleInTestDigestA = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	shardingScaleInTestDigestB = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	shardingScaleInTestDigestC = "cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"
)

func newShardingScaleInPlanMaterialFixture() *appsv1.ShardingScaleInPlanMaterial {
	processUID := func(uid string) string {
		return "0000000000000000000000000000-" + uid
	}
	newPod := func(name, uid string) appsv1.ShardingScaleInPlanPod {
		return appsv1.ShardingScaleInPlanPod{
			Name:                  name,
			UID:                   types.UID(uid),
			FQDN:                  name + ".redis-headless.default.svc",
			AgentImageID:          "sha256:" + shardingScaleInTestDigestA,
			AgentProcessUID:       processUID(uid),
			AgentCapabilityDigest: shardingScaleInTestDigestB,
		}
	}
	newMember := func(name, uid string, pods ...appsv1.ShardingScaleInPlanPod) appsv1.ShardingScaleInPlanMember {
		return appsv1.ShardingScaleInPlanMember{
			ComponentName:       name,
			ComponentUID:        types.UID(uid),
			ComponentGeneration: 1,
			ComponentSpecDigest: shardingScaleInTestDigestA,
			ComponentShortName:  name,
			ShardTemplateName:   "redis",
			Pods:                pods,
		}
	}
	newExecutorTemplate := func(podUID, componentUID string) appsv1.ShardingScaleInExecutorTemplate {
		record := shardingScaleInBaseParameterRecord{
			Version: shardingScaleInBaseParameterRecordVersionV1,
			Parameters: []shardingScaleInBaseParameter{
				{
					Name: shardingScaleInBaseParameterProtocolVersion,
					ValueB64: base64.StdEncoding.EncodeToString(
						[]byte(appsv1.ShardingScaleInResultProtocolV2)),
				},
				{
					Name:     shardingScaleInBaseParameterServicePort,
					ValueB64: base64.StdEncoding.EncodeToString([]byte("6379")),
				},
			},
		}
		data, err := json.Marshal(record)
		if err != nil {
			panic(err)
		}
		sum := sha256.Sum256(data)
		template := appsv1.ShardingScaleInExecutorTemplate{
			ExecutorPodUID:         types.UID(podUID),
			ExecutorComponentUID:   types.UID(componentUID),
			BaseParameterRecordB64: base64.StdEncoding.EncodeToString(data),
			BaseParameterDigest:    hex.EncodeToString(sum[:]),
			LaunchSchemaDigest:     shardingScaleInTestDigestA,
			PollSchemaDigest:       shardingScaleInTestDigestB,
			CancelSchemaDigest:     shardingScaleInTestDigestC,
			ServerRuntimeBinding: appsv1.ShardingScaleInServerRuntimeBinding{
				AgentProcessUID:                processUID(podUID),
				AgentImageID:                   "sha256:" + shardingScaleInTestDigestA,
				RegisteredActionDigest:         shardingScaleInTestDigestB,
				StartupEnvironmentSchemaDigest: shardingScaleInTestDigestC,
			},
		}
		template.ServerRuntimeBinding.ServerConfigurationDigest, err =
			digestShardingScaleInServerConfiguration(
				template.ExecutorPodUID,
				template.ServerRuntimeBinding.RegisteredActionDigest,
				template.ServerRuntimeBinding.StartupEnvironmentSchemaDigest,
				template.BaseParameterDigest,
				template.LaunchSchemaDigest,
				template.PollSchemaDigest,
				template.CancelSchemaDigest,
				template.CredentialBindings,
			)
		if err != nil {
			panic(err)
		}
		return template
	}

	return &appsv1.ShardingScaleInPlanMaterial{
		ProtocolVersion: appsv1.ShardingScaleInResultProtocolV2,
		ShardingName:    "redis",
		Source: appsv1.ShardingScaleInPlanSource{
			ClusterNamespace:  "default",
			ClusterName:       "demo",
			ClusterUID:        "cluster-uid",
			ClusterGeneration: 7,
			DesiredDigest:     shardingScaleInTestDigestA,
			DesiredShards:     2,
		},
		Action: appsv1.ShardingScaleInPlanAction{
			ShardingDefinitionName:       "valkey-cluster",
			ShardingDefinitionUID:        "sharding-definition-uid",
			ShardingDefinitionGeneration: 3,
			ActionDigest:                 shardingScaleInTestDigestB,
			ResultProtocol:               appsv1.ShardingScaleInResultProtocolV2,
		},
		DeletionGuard: appsv1.ShardingScaleInDeletionGuardIdentity{
			Protocol:            appsv1.ShardingScaleInDeletionGuardProtocolV1,
			InstallationUID:     "guard-uid",
			PolicyRevision:      "revision-1",
			ConfigurationDigest: shardingScaleInTestDigestC,
		},
		RequestAuthority: appsv1.ShardingScaleInRequestAuthority{
			Version:                            appsv1.ShardingScaleInRequestAuthorityVersionV2,
			Builder:                            appsv1.ShardingScaleInRequestBuilderTypedV2,
			GenericLifecycleSynthesisForbidden: true,
			ActionName:                         shardingRemoveShardAction,
			ActionDefinitionDigest:             shardingScaleInTestDigestB,
			ComponentDefinitionSources: []appsv1.ShardingScaleInComponentDefinitionSource{{
				Name:                 "valkey-cluster-9",
				UID:                  "component-definition-uid",
				Generation:           9,
				VarsProjectionDigest: shardingScaleInTestDigestA,
			}},
			VarSources: []appsv1.ShardingScaleInVarSource{
				{
					VariableName:                 shardingScaleInBaseParameterServicePort,
					ResolverKind:                 appsv1.ShardingScaleInVarResolverStaticValue,
					Consumption:                  appsv1.ShardingScaleInVarConsumptionTypedBase,
					ResolvedNonSecretValueB64:    "NjM3OQ==",
					ResolvedNonSecretValueDigest: "48f599a9094eb9a4fcd2ff73dd158208d3a2e0d8769a32e3c3795fc8791a0a71",
				},
				{
					VariableName: "POD_NAME",
					ResolverKind: appsv1.ShardingScaleInVarResolverComponentVarRef,
					Consumption:  appsv1.ShardingScaleInVarConsumptionServerReservedIdentity,
					SourceObjects: []appsv1.ShardingScaleInRequestSourceObject{{
						APIVersion:       "apps.kubeblocks.io/v1",
						Kind:             "Component",
						Namespace:        "default",
						Name:             "demo-redis-0",
						UID:              "component-0",
						ProjectionKind:   appsv1.ShardingScaleInSourceProjectionComponentIdentity,
						ProjectionDigest: shardingScaleInTestDigestA,
					}},
				},
			},
			ExecutorTemplates: []appsv1.ShardingScaleInExecutorTemplate{
				newExecutorTemplate("pod-0", "component-0"),
				newExecutorTemplate("pod-2", "component-2"),
				newExecutorTemplate("pod-1", "component-1"),
			},
		},
		Leaving: []appsv1.ShardingScaleInPlanMember{
			newMember("demo-redis-2", "component-2", newPod("demo-redis-2-0", "pod-2")),
		},
		Staying: []appsv1.ShardingScaleInPlanMember{
			newMember("demo-redis-1", "component-1", newPod("demo-redis-1-0", "pod-1")),
			newMember("demo-redis-0", "component-0", newPod("demo-redis-0-0", "pod-0")),
		},
		ProofExecutor: appsv1.ShardingScaleInProofExecutor{
			PodName: "demo-redis-0-0",
			PodUID:  "pod-0",
		},
		ExecutorPrerequisites: []appsv1.ShardingScaleInExecutorPrerequisite{{
			APIVersion:         "workloads.kubeblocks.io/v1",
			Kind:               "InstanceSet",
			Namespace:          "default",
			Name:               "demo-redis-0",
			UID:                "instanceset-0",
			Role:               appsv1.ShardingScaleInPrerequisiteRolePodOwnerWorkload,
			Scope:              appsv1.ShardingScaleInPrerequisiteScopeComponent,
			ComponentUID:       "component-0",
			CriticalSpecDigest: shardingScaleInTestDigestA,
			IdentityDigest:     shardingScaleInTestDigestB,
		}},
	}
}
