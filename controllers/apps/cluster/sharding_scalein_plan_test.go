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
	newExecutorTemplate := func(podUID, componentUID, base string) appsv1.ShardingScaleInExecutorTemplate {
		encoded, digest := encodedValue(base)
		return appsv1.ShardingScaleInExecutorTemplate{
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
				ServerConfigurationDigest:      digestA,
			},
		}
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
				Version:                            appsv1.ShardingScaleInRequestAuthorityVersionV1,
				Builder:                            appsv1.ShardingScaleInRequestBuilderTypedV1,
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
					newExecutorTemplate("pod-0", "component-0", "base-0"),
					newExecutorTemplate("pod-2", "component-2", "base-2"),
					newExecutorTemplate("pod-1", "component-1", "base-1"),
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
				SecretSource: &appsv1.ShardingScaleInSecretSource{
					APIVersion: "v1",
					Kind:       "Secret",
					Namespace:  "default",
					Name:       "demo-redis-account",
					UID:        "secret-uid",
					KeyNames:   []string{"username", "password"},
				},
			})
		material.RequestAuthority.ExecutorTemplates = append(
			material.RequestAuthority.ExecutorTemplates,
			newExecutorTemplate("pod-3", "component-3", "base-3"),
			newExecutorTemplate("pod-1b", "component-1", "base-1b"))
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
				if source.SecretSource != nil {
					random.Shuffle(len(source.SecretSource.KeyNames), func(left, right int) {
						source.SecretSource.KeyNames[left], source.SecretSource.KeyNames[right] =
							source.SecretSource.KeyNames[right], source.SecretSource.KeyNames[left]
					})
				}
			}
			random.Shuffle(len(permuted.RequestAuthority.VarSources), func(i, j int) {
				sources := permuted.RequestAuthority.VarSources
				sources[i], sources[j] = sources[j], sources[i]
			})
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
					encoded, digest := encodedValue("changed-base")
					material.RequestAuthority.ExecutorTemplates[0].BaseParameterRecordB64 = encoded
					material.RequestAuthority.ExecutorTemplates[0].BaseParameterDigest = digest
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
				SecretSource: &appsv1.ShardingScaleInSecretSource{
					APIVersion: "v1",
					Kind:       "Secret",
					Namespace:  "default",
					Name:       "demo-redis-account",
					UID:        "secret-uid",
					KeyNames:   []string{"password"},
				},
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
