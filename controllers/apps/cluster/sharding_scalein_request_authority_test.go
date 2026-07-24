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
	"errors"
	"slices"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	"github.com/apecloud/kubeblocks/pkg/kbagent"
)

var _ = Describe("fresh sharding scale-in request authority", func() {
	newFixture := func() (
		*shardingScaleInDNSInventory,
		*shardingScaleInRequestAuthorityTestReader,
		*shardingScaleInRequestAuthorityTestCapabilityReader,
	) {
		inventory, _ := newShardingScaleInSourceMaterialFixture()
		cluster := inventory.Members.Topology.Cluster
		cluster.ResourceVersion = "cluster-rv"

		required := appsv1.VarRequired
		componentDefinition := &appsv1.ComponentDefinition{
			ObjectMeta: metav1.ObjectMeta{
				Name:            "valkey",
				UID:             "component-definition-uid",
				Generation:      3,
				ResourceVersion: "1",
			},
			Spec: appsv1.ComponentDefinitionSpec{
				Vars: []appsv1.EnvVar{
					{Name: shardingScaleInBaseParameterServicePort, Value: "6379"},
					{
						Name: "DEFAULT_USERNAME",
						ValueFrom: &appsv1.VarSource{
							CredentialVarRef: &appsv1.CredentialVarSelector{
								ClusterObjectReference: appsv1.ClusterObjectReference{Name: "default"},
								CredentialVars: appsv1.CredentialVars{
									Username: &required,
								},
							},
						},
					},
					{
						Name: "DEFAULT_PASSWORD",
						ValueFrom: &appsv1.VarSource{
							CredentialVarRef: &appsv1.CredentialVarSelector{
								ClusterObjectReference: appsv1.ClusterObjectReference{Name: "default"},
								CredentialVars: appsv1.CredentialVars{
									Password: &required,
								},
							},
						},
					},
					{
						Name: "ALL_POD_FQDNS",
						ValueFrom: &appsv1.VarSource{
							ComponentVarRef: &appsv1.ComponentVarSelector{
								ComponentVars: appsv1.ComponentVars{
									PodFQDNs: &required,
								},
							},
						},
					},
				},
			},
		}

		objects := []client.Object{componentDefinition}
		varsProjectionDigest, err := digestShardingScaleInCanonicalJSON(struct {
			Version string          `json:"version"`
			Vars    []appsv1.EnvVar `json:"vars"`
		}{
			Version: shardingScaleInRequestSourceProjectionVersionV1,
			Vars:    componentDefinition.Spec.Vars,
		})
		Expect(err).ShouldNot(HaveOccurred())
		componentDefinitionSource := appsv1.ShardingScaleInComponentDefinitionSource{
			Name:                 componentDefinition.Name,
			UID:                  componentDefinition.UID,
			Generation:           componentDefinition.Generation,
			VarsProjectionDigest: varsProjectionDigest,
		}
		requirements := []shardingScaleInCredentialRequirement{
			{
				VariableName: "DEFAULT_USERNAME",
				AccountName:  "default",
				KeyName:      constant.AccountNameForSecret,
				Selector:     *componentDefinition.Spec.Vars[1].ValueFrom.CredentialVarRef.DeepCopy(),
			},
			{
				VariableName: "DEFAULT_PASSWORD",
				AccountName:  "default",
				KeyName:      constant.AccountPasswdForSecret,
				Selector:     *componentDefinition.Spec.Vars[2].ValueFrom.CredentialVarRef.DeepCopy(),
			},
		}
		capabilities := map[types.UID][]shardingScaleInAgentCapability{}
		for _, records := range [][]shardingScaleInSourceDNS{inventory.Leaving, inventory.Staying} {
			for i := range records {
				member := &records[i].Member
				member.Component.ResourceVersion = "component-rv-" + string(member.Component.UID)
				member.Workload.ResourceVersion = "workload-rv-" + string(member.Component.UID)
				shortName, err := component.ShortName(cluster.Name, member.Component.Name)
				Expect(err).ShouldNot(HaveOccurred())

				secret := &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Namespace:       cluster.Namespace,
						Name:            constant.GenerateAccountSecretName(cluster.Name, shortName, "default"),
						UID:             types.UID("secret-" + string(member.Component.UID)),
						ResourceVersion: "1",
					},
					Data: map[string][]byte{
						constant.AccountNameForSecret:   []byte("not-persisted"),
						constant.AccountPasswdForSecret: []byte("not-persisted"),
					},
				}
				objects = append(objects, secret)
				credentialSource := appsv1.ShardingScaleInCredentialSource{
					APIVersion: "v1",
					Kind:       "Secret",
					Namespace:  secret.Namespace,
					Name:       secret.Name,
					UID:        secret.UID,
					KeyNames: []string{
						constant.AccountPasswdForSecret,
						constant.AccountNameForSecret,
					},
				}
				credentialSource.SourceID, err =
					digestShardingScaleInCredentialSourceID(credentialSource)
				Expect(err).ShouldNot(HaveOccurred())
				credentialSource.CredentialSourceDigest, err =
					digestShardingScaleInCredentialSource(credentialSource)
				Expect(err).ShouldNot(HaveOccurred())
				resolutions := make([]shardingScaleInCredentialResolution, 0, len(requirements))
				for _, requirement := range requirements {
					requiredKeyNames := []string{requirement.KeyName}
					resolverDigest, err :=
						digestShardingScaleInCredentialResolverProjection(
							componentDefinitionSource, member.Component.UID, requirement,
							credentialSource.SourceID, requiredKeyNames)
					Expect(err).ShouldNot(HaveOccurred())
					resolutions = append(resolutions, shardingScaleInCredentialResolution{
						VariableName:             requirement.VariableName,
						CredentialSourceID:       credentialSource.SourceID,
						CredentialSourceDigest:   credentialSource.CredentialSourceDigest,
						RequiredKeyNames:         requiredKeyNames,
						ResolverProjectionDigest: resolverDigest,
					})
				}
				_, baseDigest, err := buildShardingScaleInBaseParameterRecord("6379")
				Expect(err).ShouldNot(HaveOccurred())

				for podIndex := range member.Pods {
					pod := member.Pods[podIndex].Pod.DeepCopy()
					pod.ResourceVersion = "pod-rv-" + string(pod.UID)
					pod.Status.Phase = corev1.PodRunning
					pod.Status.ContainerStatuses = []corev1.ContainerStatus{{
						Name:    kbagent.ContainerName,
						ImageID: "containerd://sha256:" + shardingScaleInTestDigestA,
						State: corev1.ContainerState{
							Running: &corev1.ContainerStateRunning{
								StartedAt: metav1.Now(),
							},
						},
					}}
					member.Pods[podIndex].Pod = *pod.DeepCopy()
					objects = append(objects, pod)
					bindings, err := buildShardingScaleInExecutorCredentialBindings(
						pod.UID, member.Component.UID, resolutions)
					Expect(err).ShouldNot(HaveOccurred())

					capability := shardingScaleInAgentCapability{
						PodUID:                         pod.UID,
						AgentProcessUID:                "000000000000000000000000-" + string(pod.UID),
						RegisteredActionDigest:         shardingScaleInTestDigestB,
						StartupEnvironmentSchemaDigest: shardingScaleInTestDigestC,
						LaunchSchemaDigest:             shardingScaleInTestDigestA,
						PollSchemaDigest:               shardingScaleInTestDigestB,
						CancelSchemaDigest:             shardingScaleInTestDigestC,
						BaseParameterDigest:            baseDigest,
						CredentialBindings:             bindings,
					}
					capability.ServerConfigurationDigest, err =
						digestShardingScaleInAgentServerConfiguration(capability)
					Expect(err).ShouldNot(HaveOccurred())
					capability.AgentCapabilityDigest, err =
						digestShardingScaleInAgentCapability(capability)
					Expect(err).ShouldNot(HaveOccurred())
					capabilities[pod.UID] = []shardingScaleInAgentCapability{
						capability, capability,
					}
				}
			}
		}

		baseReader := fake.NewClientBuilder().
			WithScheme(scheme.Scheme).
			WithObjects(objects...).
			Build()
		return inventory, &shardingScaleInRequestAuthorityTestReader{Client: baseReader},
			&shardingScaleInRequestAuthorityTestCapabilityReader{snapshots: capabilities}
	}

	It("builds exact credentials and target-free bases from stable APIReader and capability snapshots", func() {
		inventory, apiReader, capabilityReader := newFixture()
		material, err := loadFreshShardingScaleInRequestAuthority(
			context.Background(), apiReader, inventory, shardingScaleInTestDigestB, capabilityReader)
		Expect(err).ShouldNot(HaveOccurred())
		Expect(material.PodRuntimeBindings).Should(HaveLen(2))
		Expect(material.RequestAuthority.CredentialSources).Should(HaveLen(2))
		Expect(material.RequestAuthority.ExecutorTemplates).Should(HaveLen(2))

		for _, template := range material.RequestAuthority.ExecutorTemplates {
			Expect(template.CredentialBindings).Should(HaveLen(2))
			_, _, values, err := canonicalizeShardingScaleInBaseParameterRecord(
				template.BaseParameterRecordB64, template.BaseParameterDigest)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(values.protocolVersion).Should(Equal(string(appsv1.ShardingScaleInResultProtocolV2)))
			Expect(values.servicePort).Should(Equal("6379"))
		}
		for _, source := range material.RequestAuthority.CredentialSources {
			Expect(source.KeyNames).Should(Equal([]string{
				constant.AccountPasswdForSecret,
				constant.AccountNameForSecret,
			}))
		}
		Expect(material.RequestAuthority.VarSources).Should(ContainElement(
			HaveField("Consumption", appsv1.ShardingScaleInVarConsumptionDurableEnvelope)))
	})

	It("deduplicates one Component credential source across multiple executor Pods", func() {
		inventory, apiReader, capabilityReader := newFixture()
		member := &inventory.Staying[0].Member
		originalPod := member.Pods[0].Pod
		sharedPod := originalPod.DeepCopy()
		sharedPod.Name += "-shared"
		sharedPod.UID += "-shared"
		sharedPod.ResourceVersion = ""
		Expect(apiReader.Client.Create(context.Background(), sharedPod)).Should(Succeed())
		liveSharedPod := &corev1.Pod{}
		Expect(apiReader.Client.Get(context.Background(), client.ObjectKeyFromObject(sharedPod),
			liveSharedPod)).Should(Succeed())
		member.Pods = append(member.Pods, shardingScaleInSourcePod{
			Pod:  *liveSharedPod.DeepCopy(),
			FQDN: sharedPod.Name + ".default.svc",
		})

		capability := capabilityReader.snapshots[originalPod.UID][0]
		capability.CredentialBindings = slices.Clone(capability.CredentialBindings)
		capability.PodUID = liveSharedPod.UID
		capability.AgentProcessUID = "000000000000000000000000-" + string(liveSharedPod.UID)
		for i := range capability.CredentialBindings {
			capability.CredentialBindings[i].BindingDigest, _ =
				digestShardingScaleInExecutorCredentialBinding(
					capability.CredentialBindings[i],
					liveSharedPod.UID, member.Component.UID)
		}
		var err error
		capability.ServerConfigurationDigest, err =
			digestShardingScaleInAgentServerConfiguration(capability)
		Expect(err).ShouldNot(HaveOccurred())
		capability.AgentCapabilityDigest, err =
			digestShardingScaleInAgentCapability(capability)
		Expect(err).ShouldNot(HaveOccurred())
		capabilityReader.snapshots[liveSharedPod.UID] =
			[]shardingScaleInAgentCapability{capability, capability}

		material, err := loadFreshShardingScaleInRequestAuthority(
			context.Background(), apiReader, inventory, shardingScaleInTestDigestB, capabilityReader)
		Expect(err).ShouldNot(HaveOccurred())
		Expect(material.RequestAuthority.CredentialSources).Should(HaveLen(2))
		Expect(material.RequestAuthority.ExecutorTemplates).Should(HaveLen(3))
		componentSourceIDs := map[string]struct{}{}
		for _, template := range material.RequestAuthority.ExecutorTemplates {
			if template.ExecutorComponentUID != member.Component.UID {
				continue
			}
			Expect(template.CredentialBindings).Should(HaveLen(2))
			for _, binding := range template.CredentialBindings {
				componentSourceIDs[binding.CredentialSourceID] = struct{}{}
			}
		}
		Expect(componentSourceIDs).Should(HaveLen(1))
	})

	It("fails closed when a Pod or capability changes between exact reads", func() {
		inventory, apiReader, capabilityReader := newFixture()
		apiReader.mutateSecondPodRead = true
		_, err := loadFreshShardingScaleInRequestAuthority(
			context.Background(), apiReader, inventory, shardingScaleInTestDigestB, capabilityReader)
		Expect(errors.Is(err, errInvalidShardingScaleInRequestAuthoritySource)).Should(BeTrue())
		Expect(err.Error()).Should(ContainSubstring("Pod snapshot changed"))

		inventory, apiReader, capabilityReader = newFixture()
		for uid, snapshots := range capabilityReader.snapshots {
			snapshots[1].PollSchemaDigest = shardingScaleInTestDigestA
			capabilityReader.snapshots[uid] = snapshots
			break
		}
		_, err = loadFreshShardingScaleInRequestAuthority(
			context.Background(), apiReader, inventory, shardingScaleInTestDigestB, capabilityReader)
		Expect(errors.Is(err, errInvalidShardingScaleInRequestAuthoritySource)).Should(BeTrue())
		Expect(err.Error()).Should(ContainSubstring("capability snapshot changed"))
	})

	It("fails closed on an unsupported resolver or a missing credential key", func() {
		inventory, apiReader, capabilityReader := newFixture()
		componentDefinition := &appsv1.ComponentDefinition{}
		Expect(apiReader.Client.Get(context.Background(), types.NamespacedName{Name: "valkey"},
			componentDefinition)).Should(Succeed())
		componentDefinition.Spec.Vars[0] = appsv1.EnvVar{
			Name: shardingScaleInBaseParameterServicePort,
			ValueFrom: &appsv1.VarSource{
				SecretKeyRef: &corev1.SecretKeySelector{},
			},
		}
		Expect(apiReader.Client.Update(context.Background(), componentDefinition)).Should(Succeed())
		_, err := loadFreshShardingScaleInRequestAuthority(
			context.Background(), apiReader, inventory, shardingScaleInTestDigestB, capabilityReader)
		Expect(errors.Is(err, errInvalidShardingScaleInRequestAuthoritySource)).Should(BeTrue())
		Expect(err.Error()).Should(ContainSubstring("unsupported resolver"))

		inventory, apiReader, capabilityReader = newFixture()
		secret := &corev1.Secret{}
		secretKey := types.NamespacedName{
			Namespace: "default",
			Name:      "cluster-shard-0-account-default",
		}
		Expect(apiReader.Client.Get(context.Background(), secretKey, secret)).Should(Succeed())
		delete(secret.Data, constant.AccountPasswdForSecret)
		Expect(apiReader.Client.Update(context.Background(), secret)).Should(Succeed())
		_, err = loadFreshShardingScaleInRequestAuthority(
			context.Background(), apiReader, inventory, shardingScaleInTestDigestB, capabilityReader)
		Expect(errors.Is(err, errInvalidShardingScaleInRequestAuthoritySource)).Should(BeTrue())
		Expect(err.Error()).Should(ContainSubstring("credential resolver failed"))
	})
})

type shardingScaleInRequestAuthorityTestReader struct {
	client.Client
	podReads            map[types.UID]int
	mutateSecondPodRead bool
}

func (r *shardingScaleInRequestAuthorityTestReader) Get(
	ctx context.Context,
	key client.ObjectKey,
	object client.Object,
	options ...client.GetOption,
) error {
	if err := r.Client.Get(ctx, key, object, options...); err != nil {
		return err
	}
	pod, ok := object.(*corev1.Pod)
	if !ok {
		return nil
	}
	if r.podReads == nil {
		r.podReads = map[types.UID]int{}
	}
	r.podReads[pod.UID]++
	if r.mutateSecondPodRead && r.podReads[pod.UID] == 2 {
		pod.Status.ContainerStatuses[0].ImageID =
			"containerd://sha256:" + shardingScaleInTestDigestC
	}
	return nil
}

type shardingScaleInRequestAuthorityTestCapabilityReader struct {
	snapshots map[types.UID][]shardingScaleInAgentCapability
	reads     map[types.UID]int
}

func (r *shardingScaleInRequestAuthorityTestCapabilityReader) ReadShardingScaleInCapability(
	_ context.Context,
	_ types.NamespacedName,
	podUID types.UID,
) (shardingScaleInAgentCapability, error) {
	if r.reads == nil {
		r.reads = map[types.UID]int{}
	}
	index := r.reads[podUID]
	r.reads[podUID]++
	snapshots := r.snapshots[podUID]
	if index >= len(snapshots) {
		return shardingScaleInAgentCapability{}, errors.New("capability snapshot is unavailable")
	}
	snapshot := snapshots[index]
	snapshot.CredentialBindings = slices.Clone(snapshot.CredentialBindings)
	return snapshot, nil
}

var _ client.Reader = &shardingScaleInRequestAuthorityTestReader{}
