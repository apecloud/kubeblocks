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
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"math/rand"
	"reflect"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/types"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
)

var _ = Describe("sharding scale-in immutable plan material", func() {
	const digestA = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	const digestB = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	const digestC = "cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"

	processUID := func(uid string) string {
		return "0000000000000000000000000000-" + uid
	}
	encodedValue := func(value string) (string, string) {
		sum := sha256.Sum256([]byte(value))
		return base64.StdEncoding.EncodeToString([]byte(value)), hex.EncodeToString(sum[:])
	}
	encodedBaseRecord := func(entries ...shardingScaleInBaseParameter) (string, string) {
		data, err := json.Marshal(shardingScaleInBaseParameterRecord{
			Version:    shardingScaleInBaseParameterRecordVersionV1,
			Parameters: entries,
		})
		Expect(err).ShouldNot(HaveOccurred())
		return encodedValue(string(data))
	}
	newPod := func(name, uid string) appsv1.ShardingScaleInPlanPod {
		return appsv1.ShardingScaleInPlanPod{
			Name:                  name,
			UID:                   types.UID(uid),
			FQDN:                  name + ".redis-headless.default.svc",
			AgentImageID:          "sha256:" + digestA,
			AgentProcessUID:       processUID(uid),
			AgentCapabilityDigest: digestB,
		}
	}
	newMember := func(name, uid string, pods ...appsv1.ShardingScaleInPlanPod) appsv1.ShardingScaleInPlanMember {
		return appsv1.ShardingScaleInPlanMember{
			ComponentName:       name,
			ComponentUID:        types.UID(uid),
			ComponentGeneration: 1,
			ComponentSpecDigest: digestA,
			ComponentShortName:  name,
			ShardTemplateName:   "redis",
			Pods:                pods,
		}
	}
	newObject := func(name, uid string) appsv1.ShardingScaleInRequestSourceObject {
		return appsv1.ShardingScaleInRequestSourceObject{
			APIVersion:       "apps.kubeblocks.io/v1",
			Kind:             "Component",
			Namespace:        "default",
			Name:             name,
			UID:              types.UID(uid),
			ProjectionKind:   appsv1.ShardingScaleInSourceProjectionComponentIdentity,
			ProjectionDigest: digestA,
		}
	}
	newCredentialSource := func(componentUID, name, uid string, keyNames ...string,
	) appsv1.ShardingScaleInCredentialSource {
		_ = componentUID
		source := appsv1.ShardingScaleInCredentialSource{
			APIVersion: "v1",
			Kind:       "Secret",
			Namespace:  "default",
			Name:       name,
			UID:        types.UID(uid),
			KeyNames:   keyNames,
		}
		source.SourceID, _ = digestShardingScaleInCredentialSourceID(source)
		source.CredentialSourceDigest, _ = digestShardingScaleInCredentialSource(source)
		return source
	}
	newCredentialBinding := func(
		podUID, componentUID, variableName string,
		source appsv1.ShardingScaleInCredentialSource,
		keyNames ...string,
	) appsv1.ShardingScaleInExecutorCredentialBinding {
		binding := appsv1.ShardingScaleInExecutorCredentialBinding{
			VariableName:             variableName,
			CredentialSourceID:       source.SourceID,
			CredentialSourceDigest:   source.CredentialSourceDigest,
			RequiredKeyNames:         keyNames,
			ResolverProjectionDigest: digestC,
		}
		binding.BindingDigest, _ = digestShardingScaleInExecutorCredentialBinding(
			binding, types.UID(podUID), types.UID(componentUID))
		return binding
	}
	refreshServerConfigurationDigest := func(template *appsv1.ShardingScaleInExecutorTemplate) {
		digest, err := digestShardingScaleInServerConfiguration(
			template.ExecutorPodUID,
			template.ServerRuntimeBinding.RegisteredActionDigest,
			template.ServerRuntimeBinding.StartupEnvironmentSchemaDigest,
			template.BaseParameterDigest,
			template.LaunchSchemaDigest,
			template.PollSchemaDigest,
			template.CancelSchemaDigest,
			template.CredentialBindings,
		)
		Expect(err).ShouldNot(HaveOccurred())
		template.ServerRuntimeBinding.ServerConfigurationDigest = digest
	}
	newExecutorTemplate := func(podUID, componentUID string) appsv1.ShardingScaleInExecutorTemplate {
		encoded, digest := encodedBaseRecord(
			shardingScaleInBaseParameter{
				Name:     shardingScaleInBaseParameterProtocolVersion,
				ValueB64: base64.StdEncoding.EncodeToString([]byte(appsv1.ShardingScaleInResultProtocolV2)),
			},
			shardingScaleInBaseParameter{
				Name:     shardingScaleInBaseParameterServicePort,
				ValueB64: base64.StdEncoding.EncodeToString([]byte("6379")),
			},
		)
		template := appsv1.ShardingScaleInExecutorTemplate{
			ExecutorPodUID:         types.UID(podUID),
			ExecutorComponentUID:   types.UID(componentUID),
			BaseParameterRecordB64: encoded,
			BaseParameterDigest:    digest,
			LaunchSchemaDigest:     digestA,
			PollSchemaDigest:       digestB,
			CancelSchemaDigest:     digestC,
			ServerRuntimeBinding: appsv1.ShardingScaleInServerRuntimeBinding{
				AgentProcessUID:                processUID(podUID),
				AgentImageID:                   "sha256:" + digestA,
				RegisteredActionDigest:         digestB,
				StartupEnvironmentSchemaDigest: digestC,
			},
		}
		refreshServerConfigurationDigest(&template)
		return template
	}
	newMaterial := func() *appsv1.ShardingScaleInPlanMaterial {
		return &appsv1.ShardingScaleInPlanMaterial{
			ProtocolVersion: appsv1.ShardingScaleInResultProtocolV2,
			ShardingName:    "redis",
			Source: appsv1.ShardingScaleInPlanSource{
				ClusterNamespace:  "default",
				ClusterName:       "demo",
				ClusterUID:        "cluster-uid",
				ClusterGeneration: 7,
				DesiredDigest:     digestA,
				DesiredShards:     2,
			},
			Action: appsv1.ShardingScaleInPlanAction{
				ShardingDefinitionName:       "valkey-cluster",
				ShardingDefinitionUID:        "sharding-definition-uid",
				ShardingDefinitionGeneration: 3,
				ActionDigest:                 digestB,
				ResultProtocol:               appsv1.ShardingScaleInResultProtocolV2,
			},
			DeletionGuard: appsv1.ShardingScaleInDeletionGuardIdentity{
				Protocol:            appsv1.ShardingScaleInDeletionGuardProtocolV1,
				InstallationUID:     "guard-uid",
				PolicyRevision:      "revision-1",
				ConfigurationDigest: digestC,
			},
			RequestAuthority: appsv1.ShardingScaleInRequestAuthority{
				Version:                            appsv1.ShardingScaleInRequestAuthorityVersionV2,
				Builder:                            appsv1.ShardingScaleInRequestBuilderTypedV2,
				GenericLifecycleSynthesisForbidden: true,
				ActionName:                         shardingRemoveShardAction,
				ActionDefinitionDigest:             digestB,
				ComponentDefinitionSources: []appsv1.ShardingScaleInComponentDefinitionSource{
					{
						Name:                 "valkey-cluster-9",
						UID:                  "component-definition-uid",
						Generation:           9,
						VarsProjectionDigest: digestA,
					},
				},
				VarSources: []appsv1.ShardingScaleInVarSource{
					{
						VariableName:                 "SERVICE_PORT",
						ResolverKind:                 appsv1.ShardingScaleInVarResolverStaticValue,
						Consumption:                  appsv1.ShardingScaleInVarConsumptionTypedBase,
						ResolvedNonSecretValueB64:    "NjM3OQ==",
						ResolvedNonSecretValueDigest: "48f599a9094eb9a4fcd2ff73dd158208d3a2e0d8769a32e3c3795fc8791a0a71",
					},
					{
						VariableName: "POD_NAME",
						ResolverKind: appsv1.ShardingScaleInVarResolverComponentVarRef,
						Consumption:  appsv1.ShardingScaleInVarConsumptionServerReservedIdentity,
						SourceObjects: []appsv1.ShardingScaleInRequestSourceObject{
							newObject("demo-redis-0", "component-0"),
						},
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
			ExecutorPrerequisites: []appsv1.ShardingScaleInExecutorPrerequisite{
				{
					APIVersion:         "workloads.kubeblocks.io/v1",
					Kind:               "InstanceSet",
					Namespace:          "default",
					Name:               "demo-redis-0",
					UID:                "instanceset-0",
					Role:               appsv1.ShardingScaleInPrerequisiteRolePodOwnerWorkload,
					Scope:              appsv1.ShardingScaleInPrerequisiteScopeComponent,
					ComponentUID:       "component-0",
					CriticalSpecDigest: digestA,
					IdentityDigest:     digestB,
				},
			},
		}
	}
	withMultiElementSlices := func(material *appsv1.ShardingScaleInPlanMaterial) {
		material.Leaving = append(material.Leaving,
			newMember("demo-redis-3", "component-3", newPod("demo-redis-3-0", "pod-3")))
		material.Staying[0].Pods = append(material.Staying[0].Pods,
			newPod("demo-redis-1-1", "pod-1b"))
		material.RequestAuthority.ComponentDefinitionSources = append(
			material.RequestAuthority.ComponentDefinitionSources,
			appsv1.ShardingScaleInComponentDefinitionSource{
				Name:                 "valkey-cluster-8",
				UID:                  "component-definition-uid-8",
				Generation:           8,
				VarsProjectionDigest: digestB,
			})
		material.RequestAuthority.VarSources[1].SourceObjects = append(
			material.RequestAuthority.VarSources[1].SourceObjects,
			newObject("demo-redis-1", "component-1"))
		material.RequestAuthority.VarSources = append(material.RequestAuthority.VarSources,
			appsv1.ShardingScaleInVarSource{
				VariableName: "DEFAULT_PASSWORD",
				ResolverKind: appsv1.ShardingScaleInVarResolverCredentialVarRef,
				Consumption:  appsv1.ShardingScaleInVarConsumptionServerStartupSecret,
			})
		material.RequestAuthority.ExecutorTemplates = append(
			material.RequestAuthority.ExecutorTemplates,
			newExecutorTemplate("pod-3", "component-3"),
			newExecutorTemplate("pod-1b", "component-1"))
		credentialSources := map[types.UID]appsv1.ShardingScaleInCredentialSource{}
		for _, members := range [][]appsv1.ShardingScaleInPlanMember{material.Leaving, material.Staying} {
			for _, member := range members {
				source := newCredentialSource(
					string(member.ComponentUID),
					member.ComponentName+"-account-default",
					"secret-"+string(member.ComponentUID),
					"password")
				credentialSources[member.ComponentUID] = source
				material.RequestAuthority.CredentialSources = append(
					material.RequestAuthority.CredentialSources, source)
			}
		}
		for i := range material.RequestAuthority.ExecutorTemplates {
			template := &material.RequestAuthority.ExecutorTemplates[i]
			source := credentialSources[template.ExecutorComponentUID]
			template.CredentialBindings = []appsv1.ShardingScaleInExecutorCredentialBinding{
				newCredentialBinding(
					string(template.ExecutorPodUID), string(template.ExecutorComponentUID),
					"DEFAULT_PASSWORD", source, "password"),
			}
			refreshServerConfigurationDigest(template)
		}
		material.ExecutorPrerequisites = append(material.ExecutorPrerequisites,
			appsv1.ShardingScaleInExecutorPrerequisite{
				APIVersion:         "v1",
				Kind:               "Service",
				Namespace:          "default",
				Name:               "demo-redis-headless",
				UID:                "service-uid",
				Role:               appsv1.ShardingScaleInPrerequisiteRoleClusterDNS,
				Scope:              appsv1.ShardingScaleInPrerequisiteScopeShared,
				CriticalSpecDigest: digestB,
			})
	}

	It("canonicalizes every set-like slice before hashing", func() {
		candidate := newMaterial()
		withMultiElementSlices(candidate)
		first, firstID, err := buildShardingScaleInPlanMaterial(candidate)
		Expect(err).ShouldNot(HaveOccurred())

		for seed := int64(0); seed < 100; seed++ {
			permuted := candidate.DeepCopy()
			random := rand.New(rand.NewSource(seed))
			random.Shuffle(len(permuted.Leaving), func(i, j int) {
				permuted.Leaving[i], permuted.Leaving[j] = permuted.Leaving[j], permuted.Leaving[i]
			})
			random.Shuffle(len(permuted.Staying), func(i, j int) {
				permuted.Staying[i], permuted.Staying[j] = permuted.Staying[j], permuted.Staying[i]
			})
			for i := range permuted.Leaving {
				random.Shuffle(len(permuted.Leaving[i].Pods), func(left, right int) {
					permuted.Leaving[i].Pods[left], permuted.Leaving[i].Pods[right] =
						permuted.Leaving[i].Pods[right], permuted.Leaving[i].Pods[left]
				})
			}
			for i := range permuted.Staying {
				random.Shuffle(len(permuted.Staying[i].Pods), func(left, right int) {
					permuted.Staying[i].Pods[left], permuted.Staying[i].Pods[right] =
						permuted.Staying[i].Pods[right], permuted.Staying[i].Pods[left]
				})
			}
			random.Shuffle(len(permuted.RequestAuthority.ComponentDefinitionSources), func(i, j int) {
				sources := permuted.RequestAuthority.ComponentDefinitionSources
				sources[i], sources[j] = sources[j], sources[i]
			})
			for i := range permuted.RequestAuthority.VarSources {
				source := &permuted.RequestAuthority.VarSources[i]
				random.Shuffle(len(source.SourceObjects), func(left, right int) {
					source.SourceObjects[left], source.SourceObjects[right] =
						source.SourceObjects[right], source.SourceObjects[left]
				})
			}
			for i := range permuted.RequestAuthority.CredentialSources {
				source := &permuted.RequestAuthority.CredentialSources[i]
				random.Shuffle(len(source.KeyNames), func(left, right int) {
					source.KeyNames[left], source.KeyNames[right] =
						source.KeyNames[right], source.KeyNames[left]
				})
			}
			random.Shuffle(len(permuted.RequestAuthority.CredentialSources), func(i, j int) {
				sources := permuted.RequestAuthority.CredentialSources
				sources[i], sources[j] = sources[j], sources[i]
			})
			random.Shuffle(len(permuted.RequestAuthority.VarSources), func(i, j int) {
				sources := permuted.RequestAuthority.VarSources
				sources[i], sources[j] = sources[j], sources[i]
			})
			for i := range permuted.RequestAuthority.ExecutorTemplates {
				bindings := permuted.RequestAuthority.ExecutorTemplates[i].CredentialBindings
				for j := range bindings {
					random.Shuffle(len(bindings[j].RequiredKeyNames), func(left, right int) {
						bindings[j].RequiredKeyNames[left], bindings[j].RequiredKeyNames[right] =
							bindings[j].RequiredKeyNames[right], bindings[j].RequiredKeyNames[left]
					})
				}
				random.Shuffle(len(bindings), func(left, right int) {
					bindings[left], bindings[right] = bindings[right], bindings[left]
				})
			}
			random.Shuffle(len(permuted.RequestAuthority.ExecutorTemplates), func(i, j int) {
				templates := permuted.RequestAuthority.ExecutorTemplates
				templates[i], templates[j] = templates[j], templates[i]
			})
			random.Shuffle(len(permuted.ExecutorPrerequisites), func(i, j int) {
				prerequisites := permuted.ExecutorPrerequisites
				prerequisites[i], prerequisites[j] = prerequisites[j], prerequisites[i]
			})

			second, secondID, err := buildShardingScaleInPlanMaterial(permuted)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(second).Should(Equal(first))
			Expect(secondID).Should(Equal(firstID))
		}
		Expect(firstID).Should(MatchRegexp("^[0-9a-f]{64}$"))
		Expect(first.RequestAuthority.SourceSnapshotDigest).Should(MatchRegexp("^[0-9a-f]{64}$"))
		Expect(first.RequestAuthority.RequestAuthorityDigest).Should(MatchRegexp("^[0-9a-f]{64}$"))
	})

	It("changes the plan identity when immutable material changes", func() {
		_, firstID, err := buildShardingScaleInPlanMaterial(newMaterial())
		Expect(err).ShouldNot(HaveOccurred())

		mutations := []struct {
			name   string
			mutate func(*appsv1.ShardingScaleInPlanMaterial)
		}{
			{
				name: "Pod FQDN",
				mutate: func(material *appsv1.ShardingScaleInPlanMaterial) {
					material.Staying[0].Pods[0].FQDN = "changed.default.svc"
				},
			},
			{
				name: "deletion guard policy revision",
				mutate: func(material *appsv1.ShardingScaleInPlanMaterial) {
					material.DeletionGuard.PolicyRevision = "revision-2"
				},
			},
			{
				name: "prerequisite role",
				mutate: func(material *appsv1.ShardingScaleInPlanMaterial) {
					material.ExecutorPrerequisites[0].Role =
						appsv1.ShardingScaleInPrerequisiteRoleKBAgentEndpoint
				},
			},
			{
				name: "vars projection",
				mutate: func(material *appsv1.ShardingScaleInPlanMaterial) {
					material.RequestAuthority.ComponentDefinitionSources[0].VarsProjectionDigest = digestB
				},
			},
			{
				name: "executor base record",
				mutate: func(material *appsv1.ShardingScaleInPlanMaterial) {
					encoded, digest := encodedValue("6380")
					material.RequestAuthority.VarSources[0].ResolvedNonSecretValueB64 = encoded
					material.RequestAuthority.VarSources[0].ResolvedNonSecretValueDigest = digest
					for i := range material.RequestAuthority.ExecutorTemplates {
						recordB64, recordDigest := encodedBaseRecord(
							shardingScaleInBaseParameter{
								Name: shardingScaleInBaseParameterProtocolVersion,
								ValueB64: base64.StdEncoding.EncodeToString(
									[]byte(appsv1.ShardingScaleInResultProtocolV2)),
							},
							shardingScaleInBaseParameter{
								Name:     shardingScaleInBaseParameterServicePort,
								ValueB64: base64.StdEncoding.EncodeToString([]byte("6380")),
							},
						)
						material.RequestAuthority.ExecutorTemplates[i].BaseParameterRecordB64 = recordB64
						material.RequestAuthority.ExecutorTemplates[i].BaseParameterDigest = recordDigest
						refreshServerConfigurationDigest(
							&material.RequestAuthority.ExecutorTemplates[i])
					}
				},
			},
		}
		for _, mutation := range mutations {
			By(mutation.name)
			changed := newMaterial()
			mutation.mutate(changed)
			_, changedID, err := buildShardingScaleInPlanMaterial(changed)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(changedID).ShouldNot(Equal(firstID))
		}
	})

	It("rejects duplicate identities and a non-canonical proof executor", func() {
		duplicate := newMaterial()
		duplicate.Staying[1].ComponentUID = duplicate.Staying[0].ComponentUID
		_, _, err := buildShardingScaleInPlanMaterial(duplicate)
		Expect(errors.Is(err, errInvalidShardingScaleInPlanMaterial)).Should(BeTrue())

		wrongProof := newMaterial()
		wrongProof.ProofExecutor = appsv1.ShardingScaleInProofExecutor{
			PodName: "demo-redis-1-0",
			PodUID:  "pod-1",
		}
		_, _, err = buildShardingScaleInPlanMaterial(wrongProof)
		Expect(errors.Is(err, errInvalidShardingScaleInPlanMaterial)).Should(BeTrue())

		duplicateSourceObject := newMaterial()
		duplicateSourceObject.RequestAuthority.VarSources[1].SourceObjects = append(
			duplicateSourceObject.RequestAuthority.VarSources[1].SourceObjects,
			newObject("demo-redis-0", "replacement-component-0"))
		_, _, err = buildShardingScaleInPlanMaterial(duplicateSourceObject)
		Expect(errors.Is(err, errInvalidShardingScaleInPlanMaterial)).Should(BeTrue())

		duplicatePrerequisite := newMaterial()
		duplicatePrerequisite.ExecutorPrerequisites = append(
			duplicatePrerequisite.ExecutorPrerequisites,
			appsv1.ShardingScaleInExecutorPrerequisite{
				APIVersion:         "v1",
				Kind:               "Service",
				Namespace:          "default",
				Name:               "demo-redis-headless",
				UID:                "instanceset-0",
				Role:               appsv1.ShardingScaleInPrerequisiteRoleClusterDNS,
				Scope:              appsv1.ShardingScaleInPrerequisiteScopeShared,
				CriticalSpecDigest: digestB,
			})
		_, _, err = buildShardingScaleInPlanMaterial(duplicatePrerequisite)
		Expect(errors.Is(err, errInvalidShardingScaleInPlanMaterial)).Should(BeTrue())
	})

	It("rejects secret bytes from persisted request material", func() {
		material := newMaterial()
		encoded, digest := encodedValue("do-not-persist")
		material.RequestAuthority.VarSources = append(material.RequestAuthority.VarSources,
			appsv1.ShardingScaleInVarSource{
				VariableName:                 "DEFAULT_PASSWORD",
				ResolverKind:                 appsv1.ShardingScaleInVarResolverCredentialVarRef,
				Consumption:                  appsv1.ShardingScaleInVarConsumptionTypedBase,
				ResolvedNonSecretValueB64:    encoded,
				ResolvedNonSecretValueDigest: digest,
			})

		_, _, err := buildShardingScaleInPlanMaterial(material)
		Expect(errors.Is(err, errInvalidShardingScaleInPlanMaterial)).Should(BeTrue())

		disguisedSecret := newMaterial()
		disguisedSecret.RequestAuthority.VarSources[1].SourceObjects[0] =
			appsv1.ShardingScaleInRequestSourceObject{
				APIVersion:       "v1",
				Kind:             "Secret",
				Namespace:        "default",
				Name:             "demo-redis-account",
				UID:              "secret-uid",
				ProjectionKind:   appsv1.ShardingScaleInSourceProjectionComponentIdentity,
				ProjectionDigest: digestA,
			}
		_, _, err = buildShardingScaleInPlanMaterial(disguisedSecret)
		Expect(errors.Is(err, errInvalidShardingScaleInPlanMaterial)).Should(BeTrue())
	})

	It("binds credential provenance per executor and keeps the compound holder out of base parameters", func() {
		material := newMaterial()
		material.Leaving = append(material.Leaving, material.Staying[0])
		material.Staying = material.Staying[1:]
		material.Source.DesiredShards = 1
		material.RequestAuthority.VarSources = append(
			material.RequestAuthority.VarSources,
			appsv1.ShardingScaleInVarSource{
				VariableName: "DEFAULT_PASSWORD",
				ResolverKind: appsv1.ShardingScaleInVarResolverCredentialVarRef,
				Consumption:  appsv1.ShardingScaleInVarConsumptionServerStartupSecret,
			})

		credentialSources := map[types.UID]appsv1.ShardingScaleInCredentialSource{
			"component-0": newCredentialSource(
				"component-0", "demo-redis-0-account-default", "secret-component-0", "password"),
			"component-1": newCredentialSource(
				"component-1", "demo-redis-1-account-default", "secret-component-1", "password"),
			"component-2": newCredentialSource(
				"component-2", "demo-redis-2-account-default", "secret-component-2", "password"),
		}
		for _, source := range credentialSources {
			material.RequestAuthority.CredentialSources = append(
				material.RequestAuthority.CredentialSources, source)
		}
		for i := range material.RequestAuthority.ExecutorTemplates {
			template := &material.RequestAuthority.ExecutorTemplates[i]
			source := credentialSources[template.ExecutorComponentUID]
			template.CredentialBindings = []appsv1.ShardingScaleInExecutorCredentialBinding{
				newCredentialBinding(
					string(template.ExecutorPodUID), string(template.ExecutorComponentUID),
					"DEFAULT_PASSWORD", source, "password"),
			}
			refreshServerConfigurationDigest(template)
		}

		canonical, _, err := buildShardingScaleInPlanMaterial(material)
		Expect(err).ShouldNot(HaveOccurred())
		Expect(canonical.Leaving).Should(HaveLen(2))
		for _, template := range canonical.RequestAuthority.ExecutorTemplates {
			_, _, values, err := canonicalizeShardingScaleInBaseParameterRecord(
				template.BaseParameterRecordB64, template.BaseParameterDigest)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(values.protocolVersion).Should(Equal(string(appsv1.ShardingScaleInResultProtocolV2)))
			Expect(values.servicePort).Should(Equal("6379"))
			Expect(template.CredentialBindings).Should(HaveLen(1))
		}

		missing := material.DeepCopy()
		missing.RequestAuthority.ExecutorTemplates[0].CredentialBindings = nil
		_, _, err = buildShardingScaleInPlanMaterial(missing)
		Expect(err).Should(MatchError(ContainSubstring("credential bindings must cover")))

		extra := material.DeepCopy()
		extraTemplate := &extra.RequestAuthority.ExecutorTemplates[0]
		extraBinding := newCredentialBinding(
			string(extraTemplate.ExecutorPodUID), string(extraTemplate.ExecutorComponentUID),
			"UNDECLARED", credentialSources[extraTemplate.ExecutorComponentUID], "password")
		extraTemplate.CredentialBindings = append(
			extraTemplate.CredentialBindings, extraBinding)
		refreshServerConfigurationDigest(extraTemplate)
		_, _, err = buildShardingScaleInPlanMaterial(extra)
		Expect(err).Should(MatchError(ContainSubstring("credential bindings must cover")))

		duplicate := material.DeepCopy()
		duplicate.RequestAuthority.ExecutorTemplates[0].CredentialBindings = append(
			duplicate.RequestAuthority.ExecutorTemplates[0].CredentialBindings,
			duplicate.RequestAuthority.ExecutorTemplates[0].CredentialBindings[0])
		_, _, err = buildShardingScaleInPlanMaterial(duplicate)
		Expect(err).Should(MatchError(ContainSubstring("credential variable")))

		unused := material.DeepCopy()
		unused.RequestAuthority.CredentialSources = append(
			unused.RequestAuthority.CredentialSources,
			newCredentialSource("component-0", "unused", "unused-secret", "password"))
		_, _, err = buildShardingScaleInPlanMaterial(unused)
		Expect(err).Should(MatchError(ContainSubstring("credential source is not referenced")))

		wrongUnion := material.DeepCopy()
		wrongUnionSource := &wrongUnion.RequestAuthority.CredentialSources[0]
		wrongUnionSource.KeyNames = append(wrongUnionSource.KeyNames, "username")
		wrongUnionSource.CredentialSourceDigest, _ =
			digestShardingScaleInCredentialSource(*wrongUnionSource)
		for i := range wrongUnion.RequestAuthority.ExecutorTemplates {
			template := &wrongUnion.RequestAuthority.ExecutorTemplates[i]
			for j := range template.CredentialBindings {
				binding := &template.CredentialBindings[j]
				if binding.CredentialSourceID != wrongUnionSource.SourceID {
					continue
				}
				binding.CredentialSourceDigest = wrongUnionSource.CredentialSourceDigest
				binding.BindingDigest, _ = digestShardingScaleInExecutorCredentialBinding(
					*binding, template.ExecutorPodUID, template.ExecutorComponentUID)
			}
			refreshServerConfigurationDigest(template)
		}
		_, _, err = buildShardingScaleInPlanMaterial(wrongUnion)
		Expect(err).Should(MatchError(ContainSubstring("exact binding union")))

		crossComponent := material.DeepCopy()
		crossTemplate := &crossComponent.RequestAuthority.ExecutorTemplates[0]
		crossTemplate.CredentialBindings[0] = newCredentialBinding(
			string(crossTemplate.ExecutorPodUID), string(crossTemplate.ExecutorComponentUID),
			"DEFAULT_PASSWORD", credentialSources["component-2"], "password")
		refreshServerConfigurationDigest(crossTemplate)
		_, _, err = buildShardingScaleInPlanMaterial(crossComponent)
		Expect(err).Should(MatchError(ContainSubstring("credential source is not referenced")))

		shared := material.DeepCopy()
		sharedSource := newCredentialSource(
			"", "shared-account-default", "shared-secret", "password")
		shared.RequestAuthority.CredentialSources =
			[]appsv1.ShardingScaleInCredentialSource{sharedSource}
		for i := range shared.RequestAuthority.ExecutorTemplates {
			template := &shared.RequestAuthority.ExecutorTemplates[i]
			template.CredentialBindings = []appsv1.ShardingScaleInExecutorCredentialBinding{
				newCredentialBinding(
					string(template.ExecutorPodUID), string(template.ExecutorComponentUID),
					"DEFAULT_PASSWORD", sharedSource, "password"),
			}
			refreshServerConfigurationDigest(template)
		}
		canonicalShared, _, err := buildShardingScaleInPlanMaterial(shared)
		Expect(err).ShouldNot(HaveOccurred())
		Expect(canonicalShared.RequestAuthority.CredentialSources).Should(HaveLen(1))
	})

	It("rejects legacy request-authority and builder contracts", func() {
		legacyAuthority := newMaterial()
		legacyAuthority.RequestAuthority.Version =
			appsv1.ShardingScaleInRequestAuthorityVersionV1
		_, _, err := buildShardingScaleInPlanMaterial(legacyAuthority)
		Expect(err).Should(MatchError(ContainSubstring("request authority contract")))

		legacyBuilder := newMaterial()
		legacyBuilder.RequestAuthority.Builder =
			appsv1.ShardingScaleInRequestBuilderTypedV1
		_, _, err = buildShardingScaleInPlanMaterial(legacyBuilder)
		Expect(err).Should(MatchError(ContainSubstring("request authority contract")))

		legacySecretSource := newMaterial()
		legacySecretSource.RequestAuthority.VarSources[0].SecretSource =
			&appsv1.ShardingScaleInSecretSource{
				APIVersion: "v1",
				Kind:       "Secret",
				Namespace:  "default",
				Name:       "legacy-secret",
				UID:        types.UID("legacy-secret-uid"),
				KeyNames:   []string{"password"},
			}
		encoded, err := json.Marshal(legacySecretSource)
		Expect(err).ShouldNot(HaveOccurred())
		decoded := &appsv1.ShardingScaleInPlanMaterial{}
		Expect(json.Unmarshal(encoded, decoded)).Should(Succeed())
		Expect(decoded.RequestAuthority.VarSources[0].SecretSource).ShouldNot(BeNil())

		_, _, err = buildShardingScaleInPlanMaterial(decoded)
		Expect(err).Should(MatchError(ContainSubstring(
			"legacy secretSource is forbidden in request authority v2")))
	})

	It("rejects TypedBase and base-record allow-list bypasses", func() {
		encodedSecret, secretDigest := encodedValue("super-secret")

		staticSecret := newMaterial()
		staticSecret.RequestAuthority.VarSources = append(staticSecret.RequestAuthority.VarSources,
			appsv1.ShardingScaleInVarSource{
				VariableName:                 "DEFAULT_PASSWORD",
				ResolverKind:                 appsv1.ShardingScaleInVarResolverStaticValue,
				Consumption:                  appsv1.ShardingScaleInVarConsumptionTypedBase,
				ResolvedNonSecretValueB64:    encodedSecret,
				ResolvedNonSecretValueDigest: secretDigest,
			})
		_, _, err := buildShardingScaleInPlanMaterial(staticSecret)
		Expect(errors.Is(err, errInvalidShardingScaleInPlanMaterial)).Should(BeTrue())

		cases := []struct {
			name    string
			entries []shardingScaleInBaseParameter
			want    string
		}{
			{
				name: "secret key",
				entries: []shardingScaleInBaseParameter{
					{Name: shardingScaleInBaseParameterProtocolVersion,
						ValueB64: base64.StdEncoding.EncodeToString(
							[]byte(appsv1.ShardingScaleInResultProtocolV2))},
					{Name: shardingScaleInBaseParameterServicePort,
						ValueB64: base64.StdEncoding.EncodeToString([]byte("6379"))},
					{Name: "DEFAULT_PASSWORD", ValueB64: encodedSecret},
				},
				want: `base parameter "DEFAULT_PASSWORD" is not allowed`,
			},
			{
				name: "unknown key",
				entries: []shardingScaleInBaseParameter{
					{Name: shardingScaleInBaseParameterProtocolVersion,
						ValueB64: base64.StdEncoding.EncodeToString(
							[]byte(appsv1.ShardingScaleInResultProtocolV2))},
					{Name: shardingScaleInBaseParameterServicePort,
						ValueB64: base64.StdEncoding.EncodeToString([]byte("6379"))},
					{Name: "EXTRA", ValueB64: base64.StdEncoding.EncodeToString([]byte("value"))},
				},
				want: `base parameter "EXTRA" is not allowed`,
			},
			{
				name: "duplicate key",
				entries: []shardingScaleInBaseParameter{
					{Name: shardingScaleInBaseParameterProtocolVersion,
						ValueB64: base64.StdEncoding.EncodeToString(
							[]byte(appsv1.ShardingScaleInResultProtocolV2))},
					{Name: shardingScaleInBaseParameterServicePort,
						ValueB64: base64.StdEncoding.EncodeToString([]byte("6379"))},
					{Name: shardingScaleInBaseParameterServicePort,
						ValueB64: base64.StdEncoding.EncodeToString([]byte("6380"))},
				},
				want: `base parameter "SERVICE_PORT" is duplicated`,
			},
			{
				name: "reserved identity key",
				entries: []shardingScaleInBaseParameter{
					{Name: shardingScaleInBaseParameterProtocolVersion,
						ValueB64: base64.StdEncoding.EncodeToString(
							[]byte(appsv1.ShardingScaleInResultProtocolV2))},
					{Name: shardingScaleInBaseParameterServicePort,
						ValueB64: base64.StdEncoding.EncodeToString([]byte("6379"))},
					{Name: "POD_UID", ValueB64: base64.StdEncoding.EncodeToString([]byte("pod-0"))},
				},
				want: `base parameter "POD_UID" is not allowed`,
			},
		}
		for _, testCase := range cases {
			By(testCase.name)
			material := newMaterial()
			recordB64, recordDigest := encodedBaseRecord(testCase.entries...)
			material.RequestAuthority.ExecutorTemplates[0].BaseParameterRecordB64 = recordB64
			material.RequestAuthority.ExecutorTemplates[0].BaseParameterDigest = recordDigest
			_, _, err = buildShardingScaleInPlanMaterial(material)
			Expect(errors.Is(err, errInvalidShardingScaleInPlanMaterial)).Should(BeTrue())
			Expect(err.Error()).Should(ContainSubstring(testCase.want))
		}
	})

	It("rejects a tag-only executor image identity", func() {
		material := newMaterial()
		material.Staying[0].Pods[0].AgentImageID = "apecloud/kbagent:latest"
		for i := range material.RequestAuthority.ExecutorTemplates {
			template := &material.RequestAuthority.ExecutorTemplates[i]
			if template.ExecutorPodUID == material.Staying[0].Pods[0].UID {
				template.ServerRuntimeBinding.AgentImageID = material.Staying[0].Pods[0].AgentImageID
			}
		}

		_, _, err := buildShardingScaleInPlanMaterial(material)
		Expect(errors.Is(err, errInvalidShardingScaleInPlanMaterial)).Should(BeTrue())
	})

	It("binds canonical material to planID and keeps it immutable", func() {
		canonical, planID, err := buildShardingScaleInPlanMaterial(newMaterial())
		Expect(err).ShouldNot(HaveOccurred())
		current := &appsv1.ShardingScaleInStatus{
			ProtocolVersion:    appsv1.ShardingScaleInResultProtocolV2,
			PlanID:             planID,
			Phase:              appsv1.ShardingScaleInPhasePlanned,
			TopologyFenceToken: digestC,
			PlanMaterial:       canonical,
		}
		reduced, err := reduceShardingScaleInStatus(nil, shardingScaleInStatusTransition{Next: current})
		Expect(err).ShouldNot(HaveOccurred())
		Expect(reduced.PlanMaterial).Should(Equal(canonical))

		mutated := current.DeepCopy()
		mutated.PlanMaterial.Staying[0].Pods[0].FQDN = "changed.default.svc"
		_, err = reduceShardingScaleInStatus(current, shardingScaleInStatusTransition{
			ExpectedProtocolVersion: current.ProtocolVersion,
			ExpectedPlanID:          current.PlanID,
			ExpectedPhase:           current.Phase,
			Next:                    mutated,
		})
		Expect(errors.Is(err, errInvalidShardingScaleInStatusTransition)).Should(BeTrue())

		nonCanonical := newMaterial()
		nonCanonical.Staying[0], nonCanonical.Staying[1] =
			nonCanonical.Staying[1], nonCanonical.Staying[0]
		_, err = reduceShardingScaleInStatus(nil, shardingScaleInStatusTransition{
			Next: &appsv1.ShardingScaleInStatus{
				ProtocolVersion:    appsv1.ShardingScaleInResultProtocolV2,
				PlanID:             planID,
				Phase:              appsv1.ShardingScaleInPhasePlanned,
				TopologyFenceToken: digestC,
				PlanMaterial:       nonCanonical,
			},
		})
		Expect(errors.Is(err, errInvalidShardingScaleInStatusTransition)).Should(BeTrue())

		withoutMaterial := current.DeepCopy()
		withoutMaterial.PlanMaterial = nil
		_, err = reduceShardingScaleInStatus(nil, shardingScaleInStatusTransition{Next: withoutMaterial})
		Expect(errors.Is(err, errInvalidShardingScaleInStatusTransition)).Should(BeTrue())

		nextWithoutMaterial := withoutMaterial.DeepCopy()
		nextWithoutMaterial.Phase = appsv1.ShardingScaleInPhaseDraining
		_, err = reduceShardingScaleInStatus(withoutMaterial, shardingScaleInStatusTransition{
			ExpectedProtocolVersion: withoutMaterial.ProtocolVersion,
			ExpectedPlanID:          withoutMaterial.PlanID,
			ExpectedPhase:           withoutMaterial.Phase,
			Next:                    nextWithoutMaterial,
		})
		Expect(errors.Is(err, errInvalidShardingScaleInStatusTransition)).Should(BeTrue())
	})

	It("contains no map in the persisted immutable material graph", func() {
		var walk func(reflect.Type)
		walk = func(t reflect.Type) {
			for t.Kind() == reflect.Pointer || t.Kind() == reflect.Slice {
				t = t.Elem()
			}
			Expect(t.Kind()).ShouldNot(Equal(reflect.Map), "map field found at %s", t.String())
			if t.Kind() != reflect.Struct {
				return
			}
			for i := 0; i < t.NumField(); i++ {
				walk(t.Field(i).Type)
			}
		}

		walk(reflect.TypeOf(appsv1.ShardingScaleInPlanMaterial{}))
	})
})
