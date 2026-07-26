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
	discoveryv1 "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	workloadsv1 "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	"github.com/apecloud/kubeblocks/pkg/controller/instanceset"
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
		cluster.ResourceVersion = "10"
		cluster.Spec.Shardings = []appsv1.ClusterSharding{{
			Name:        "shard",
			ShardingDef: "valkey-sharding",
			Shards:      1,
			Template: appsv1.ClusterComponentSpec{
				ComponentDef: "valkey",
				Replicas:     1,
			},
		}}
		inventory.Members.Topology.Sharding = *cluster.Spec.Shardings[0].DeepCopy()
		shardingDefinition := &appsv1.ShardingDefinition{
			ObjectMeta: metav1.ObjectMeta{
				Name:            "valkey-sharding",
				UID:             "sharding-definition-uid",
				Generation:      3,
				ResourceVersion: "20",
			},
			Spec: appsv1.ShardingDefinitionSpec{
				LifecycleActions: &appsv1.ShardingLifecycleActions{
					ShardRemove: &appsv1.ShardingAction{
						ResultProtocol: appsv1.ShardingScaleInResultProtocolV2,
					},
				},
			},
		}
		inventory.Members.Topology.ShardingDefinition = shardingDefinition

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
				record := &records[i]
				member := &record.Member
				shortName, err := component.ShortName(cluster.Name, member.Component.Name)
				Expect(err).ShouldNot(HaveOccurred())
				componentLabels := constant.GetCompLabels(cluster.Name, shortName)
				componentLabels[constant.KBAppShardingNameLabelKey] = "shard"
				componentLabels[constant.ShardingDefLabelKey] = shardingDefinition.Name
				member.Component.Labels = componentLabels
				member.Component.OwnerReferences = []metav1.OwnerReference{
					*metav1.NewControllerRef(
						cluster, appsv1.SchemeGroupVersion.WithKind(appsv1.ClusterKind)),
				}
				member.Component.Spec.EnableInstanceAPI = ptr.To(true)
				member.Component.ResourceVersion = "30"

				workloadLabels := constant.GetCompLabels(
					cluster.Name, shortName, componentLabels)
				selector := map[string]string{
					constant.AppInstanceLabelKey:          cluster.Name,
					constant.KBAppComponentLabelKey:       shortName,
					instanceset.WorkloadsInstanceLabelKey: member.Workload.Name,
				}
				member.Workload.Labels = workloadLabels
				member.Workload.Spec.EnableInstanceAPI = ptr.To(true)
				member.Workload.Spec.Selector = &metav1.LabelSelector{
					MatchLabels: selector,
				}
				member.Workload.Generation = 1
				member.Workload.ResourceVersion = "40"
				objects = append(objects, member.Component.DeepCopy(), member.Workload.DeepCopy())
				for instanceIndex := range member.Instances {
					instance := &member.Instances[instanceIndex]
					instance.Labels = constant.GetCompLabels(
						cluster.Name, shortName, componentLabels)
					instance.Labels[instanceset.WorkloadsManagedByLabelKey] =
						workloadsv1.InstanceSetKind
					instance.Labels[instanceset.WorkloadsInstanceLabelKey] =
						member.Workload.Name
					instance.Spec.InstanceSetName = member.Workload.Name
					member.Instances[instanceIndex].Generation = 1
					member.Instances[instanceIndex].ResourceVersion = "50"
					objects = append(objects, member.Instances[instanceIndex].DeepCopy())
				}
				member.ShardTemplateName = shardingScaleInDefaultShardTemplate
				record.Service.OwnerReferences = []metav1.OwnerReference{
					*metav1.NewControllerRef(
						&member.Workload,
						workloadsv1.GroupVersion.WithKind(workloadsv1.InstanceSetKind)),
				}
				record.Service.Spec.Type = corev1.ServiceTypeClusterIP
				record.Service.Spec.ClusterIP = corev1.ClusterIPNone
				record.Service.Spec.ClusterIPs = []string{corev1.ClusterIPNone}
				record.Service.Spec.IPFamilies = []corev1.IPFamily{corev1.IPv4Protocol}
				record.Service.Spec.IPFamilyPolicy =
					ptr.To(corev1.IPFamilyPolicySingleStack)
				record.Service.Spec.PublishNotReadyAddresses = true
				record.Service.Spec.Selector = map[string]string{}
				for key, value := range selector {
					record.Service.Spec.Selector[key] = value
				}
				record.Service.Spec.Selector[constant.KBAppReleasePhaseKey] =
					constant.ReleasePhaseStable
				record.Service.Generation = 1
				record.Service.ResourceVersion = "60"
				objects = append(objects, record.Service.DeepCopy())
				if record.Endpoints != nil {
					record.Endpoints.Generation = 1
					record.Endpoints.ResourceVersion = "70"
					objects = append(objects, record.Endpoints.DeepCopy())
				}
				for sliceIndex := range record.EndpointSlices {
					record.EndpointSlices[sliceIndex].Labels = map[string]string{
						discoveryv1.LabelServiceName: record.Service.Name,
					}
					record.EndpointSlices[sliceIndex].OwnerReferences =
						[]metav1.OwnerReference{*metav1.NewControllerRef(
							&record.Service,
							corev1.SchemeGroupVersion.WithKind("Service"))}
					record.EndpointSlices[sliceIndex].Generation = 1
					record.EndpointSlices[sliceIndex].ResourceVersion = "80"
					objects = append(objects, record.EndpointSlices[sliceIndex].DeepCopy())
				}

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
					pod.Labels = constant.GetCompLabels(
						cluster.Name, shortName, componentLabels)
					for key, value := range selector {
						pod.Labels[key] = value
					}
					pod.Labels[instanceset.WorkloadsManagedByLabelKey] =
						workloadsv1.InstanceSetKind
					pod.Labels[constant.KBAppReleasePhaseKey] = constant.ReleasePhaseStable
					pod.Labels[constant.KBAppInstanceNameLabelKey] = pod.Name
					pod.OwnerReferences = []metav1.OwnerReference{
						*metav1.NewControllerRef(
							&member.Instances[podIndex],
							workloadsv1.GroupVersion.WithKind(shardingScaleInInstanceKind)),
					}
					pod.ResourceVersion = "pod-rv-" + string(pod.UID)
					pod.Status.Phase = corev1.PodRunning
					pod.Status.PodIP = record.EndpointSlices[0].Endpoints[0].Addresses[0]
					pod.Status.PodIPs = []corev1.PodIP{{IP: pod.Status.PodIP}}
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
					member.Pods[podIndex].FQDN = pod.Name + "." +
						pod.Spec.Subdomain + "." + pod.Namespace + ".svc."
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
		inventory.Members.Leaving = []shardingScaleInSourceMember{
			inventory.Leaving[0].Member,
		}
		inventory.Members.Staying = []shardingScaleInSourceMember{
			inventory.Staying[0].Member,
		}
		objects = append(objects, cluster.DeepCopy(), shardingDefinition.DeepCopy())

		testScheme := runtime.NewScheme()
		Expect(corev1.AddToScheme(testScheme)).Should(Succeed())
		Expect(discoveryv1.AddToScheme(testScheme)).Should(Succeed())
		Expect(appsv1.AddToScheme(testScheme)).Should(Succeed())
		Expect(workloadsv1.AddToScheme(testScheme)).Should(Succeed())
		baseReader := fake.NewClientBuilder().
			WithScheme(testScheme).
			WithObjects(objects...).
			Build()
		return inventory, &shardingScaleInRequestAuthorityTestReader{Client: baseReader},
			&shardingScaleInRequestAuthorityTestCapabilityReader{snapshots: capabilities}
	}

	buildPersistedPlan := func(
		inventory *shardingScaleInDNSInventory,
		apiReader *shardingScaleInRequestAuthorityTestReader,
		capabilityReader *shardingScaleInRequestAuthorityTestCapabilityReader,
	) *appsv1.ShardingScaleInPlanMaterial {
		authority, err := loadFreshShardingScaleInRequestAuthority(
			context.Background(), apiReader, inventory, shardingScaleInTestDigestB, capabilityReader)
		Expect(err).ShouldNot(HaveOccurred())
		source, err := buildShardingScaleInSourceMaterial(
			inventory, authority.PodRuntimeBindings)
		Expect(err).ShouldNot(HaveOccurred())

		plan := newShardingScaleInPlanMaterialFixture()
		cluster := inventory.Members.Topology.Cluster
		shardingDefinition := inventory.Members.Topology.ShardingDefinition
		plan.ShardingName = "shard"
		plan.Source.ClusterNamespace = cluster.Namespace
		plan.Source.ClusterName = cluster.Name
		plan.Source.ClusterUID = cluster.UID
		plan.Source.ClusterGeneration = cluster.Generation
		plan.Source.DesiredShards = int32(len(source.Staying))
		plan.Action.ShardingDefinitionName = shardingDefinition.Name
		plan.Action.ShardingDefinitionUID = shardingDefinition.UID
		plan.Action.ShardingDefinitionGeneration = shardingDefinition.Generation
		plan.Action.ActionDigest = shardingScaleInTestDigestB
		plan.Action.ResultProtocol = appsv1.ShardingScaleInResultProtocolV2
		plan.RequestAuthority = authority.RequestAuthority
		plan.Leaving = source.Leaving
		plan.Staying = source.Staying
		plan.ProofExecutor = source.ProofExecutor
		plan.ExecutorPrerequisites = source.ExecutorPrerequisites

		canonical, _, err := buildShardingScaleInPlanMaterial(plan)
		Expect(err).ShouldNot(HaveOccurred())
		return canonical
	}

	prerequisiteCases := []struct {
		name   string
		kind   string
		object func(*shardingScaleInDNSInventory) client.Object
	}{
		{
			name: "Instance",
			kind: shardingScaleInInstanceKind,
			object: func(inventory *shardingScaleInDNSInventory) client.Object {
				return inventory.Leaving[0].Member.Instances[0].DeepCopy()
			},
		},
		{
			name: "Service",
			kind: "Service",
			object: func(inventory *shardingScaleInDNSInventory) client.Object {
				return inventory.Leaving[0].Service.DeepCopy()
			},
		},
		{
			name: "Endpoints",
			kind: "Endpoints",
			object: func(inventory *shardingScaleInDNSInventory) client.Object {
				return inventory.Leaving[0].Endpoints.DeepCopy()
			},
		},
		{
			name: "EndpointSlice",
			kind: "EndpointSlice",
			object: func(inventory *shardingScaleInDNSInventory) client.Object {
				return inventory.Leaving[0].EndpointSlices[0].DeepCopy()
			},
		},
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

	It("rebuilds exact live bindings before authorization and rejects a cross-bound persisted edge", func() {
		inventory, apiReader, capabilityReader := newFixture()
		persisted := buildPersistedPlan(inventory, apiReader, capabilityReader)
		capabilityReader.reads = nil

		err := validateFreshShardingScaleInRequestAuthority(
			context.Background(), apiReader, inventory, capabilityReader, persisted)
		Expect(err).ShouldNot(HaveOccurred())

		crossBound := persisted.DeepCopy()
		templates := crossBound.RequestAuthority.ExecutorTemplates
		Expect(templates).Should(HaveLen(2))
		templates[0].CredentialBindings, templates[1].CredentialBindings =
			templates[1].CredentialBindings, templates[0].CredentialBindings
		for i := range templates {
			for j := range templates[i].CredentialBindings {
				templates[i].CredentialBindings[j].BindingDigest, err =
					digestShardingScaleInExecutorCredentialBinding(
						templates[i].CredentialBindings[j],
						templates[i].ExecutorPodUID,
						templates[i].ExecutorComponentUID)
				Expect(err).ShouldNot(HaveOccurred())
			}
			templates[i].ServerRuntimeBinding.ServerConfigurationDigest, err =
				digestShardingScaleInServerConfiguration(
					templates[i].ExecutorPodUID,
					templates[i].ServerRuntimeBinding.RegisteredActionDigest,
					templates[i].ServerRuntimeBinding.StartupEnvironmentSchemaDigest,
					templates[i].BaseParameterDigest,
					templates[i].LaunchSchemaDigest,
					templates[i].PollSchemaDigest,
					templates[i].CancelSchemaDigest,
					templates[i].CredentialBindings,
				)
			Expect(err).ShouldNot(HaveOccurred())
		}
		crossBound, _, err = buildShardingScaleInPlanMaterial(crossBound)
		Expect(err).ShouldNot(HaveOccurred())
		capabilityReader.reads = nil

		err = validateFreshShardingScaleInRequestAuthority(
			context.Background(), apiReader, inventory, capabilityReader, crossBound)
		Expect(errors.Is(err, errShardingScaleInRequestAuthorityChanged)).Should(BeTrue())
	})

	It("rejects a same-name Secret replacement and a live agent process change", func() {
		inventory, apiReader, capabilityReader := newFixture()
		persisted := buildPersistedPlan(inventory, apiReader, capabilityReader)
		capabilityReader.reads = nil

		secretKey := types.NamespacedName{
			Namespace: "default",
			Name:      "cluster-shard-0-account-default",
		}
		secret := &corev1.Secret{}
		Expect(apiReader.Client.Get(context.Background(), secretKey, secret)).Should(Succeed())
		replacement := secret.DeepCopy()
		replacement.UID = "replacement-secret-uid"
		replacement.ResourceVersion = ""
		Expect(apiReader.Client.Delete(context.Background(), secret)).Should(Succeed())
		Expect(apiReader.Client.Create(context.Background(), replacement)).Should(Succeed())

		err := validateFreshShardingScaleInRequestAuthority(
			context.Background(), apiReader, inventory, capabilityReader, persisted)
		Expect(errors.Is(err, errShardingScaleInRequestAuthorityChanged)).Should(BeTrue())

		inventory, apiReader, capabilityReader = newFixture()
		persisted = buildPersistedPlan(inventory, apiReader, capabilityReader)
		capabilityReader.reads = nil
		for podUID, snapshots := range capabilityReader.snapshots {
			for i := range snapshots {
				snapshots[i].AgentProcessUID += "-replacement"
				snapshots[i].AgentCapabilityDigest, err =
					digestShardingScaleInAgentCapability(snapshots[i])
				Expect(err).ShouldNot(HaveOccurred())
			}
			capabilityReader.snapshots[podUID] = snapshots
			break
		}

		err = validateFreshShardingScaleInRequestAuthority(
			context.Background(), apiReader, inventory, capabilityReader, persisted)
		Expect(errors.Is(err, errShardingScaleInRequestAuthorityChanged)).Should(BeTrue())
	})

	It("rejects caller root identities that do not match the persisted plan", func() {
		inventory, apiReader, capabilityReader := newFixture()
		persisted := buildPersistedPlan(inventory, apiReader, capabilityReader)
		capabilityReader.reads = nil
		inventory.Members.Topology.Cluster.UID = "stale-cluster-uid"

		err := validateFreshShardingScaleInRequestAuthority(
			context.Background(), apiReader, inventory, capabilityReader, persisted)
		Expect(errors.Is(err, errShardingScaleInRequestAuthorityChanged)).Should(BeTrue())

		inventory, apiReader, capabilityReader = newFixture()
		persisted = buildPersistedPlan(inventory, apiReader, capabilityReader)
		capabilityReader.reads = nil
		inventory.Members.Topology.ShardingDefinition.Generation++

		err = validateFreshShardingScaleInRequestAuthority(
			context.Background(), apiReader, inventory, capabilityReader, persisted)
		Expect(errors.Is(err, errShardingScaleInRequestAuthorityChanged)).Should(BeTrue())
	})

	It("rejects a persisted Service prerequisite that is missing from the live API", func() {
		inventory, apiReader, capabilityReader := newFixture()
		persisted := buildPersistedPlan(inventory, apiReader, capabilityReader)
		capabilityReader.reads = nil
		Expect(apiReader.Client.Delete(
			context.Background(), inventory.Leaving[0].Service.DeepCopy())).Should(Succeed())

		err := validateFreshShardingScaleInRequestAuthority(
			context.Background(), apiReader, inventory, capabilityReader, persisted)
		Expect(errors.Is(err, errShardingScaleInRequestAuthorityChanged)).Should(BeTrue())
	})

	It("rejects same-name replacement of every non-root persisted prerequisite", func() {
		for _, mutation := range prerequisiteCases {
			By(mutation.name)
			inventory, apiReader, capabilityReader := newFixture()
			persisted := buildPersistedPlan(inventory, apiReader, capabilityReader)
			capabilityReader.reads = nil
			live := mutation.object(inventory)
			replacement := live.DeepCopyObject().(client.Object)
			replacement.SetUID(types.UID("replacement-" + mutation.kind))
			replacement.SetResourceVersion("")
			Expect(apiReader.Client.Delete(context.Background(), live)).Should(Succeed())
			Expect(apiReader.Client.Create(context.Background(), replacement)).Should(Succeed())

			err := validateFreshShardingScaleInRequestAuthority(
				context.Background(), apiReader, inventory, capabilityReader, persisted)
			Expect(errors.Is(err, errShardingScaleInRequestAuthorityChanged)).Should(BeTrue())
		}
	})

	It("rejects terminating non-root persisted prerequisites", func() {
		for _, mutation := range prerequisiteCases {
			By(mutation.name)
			inventory, apiReader, capabilityReader := newFixture()
			persisted := buildPersistedPlan(inventory, apiReader, capabilityReader)
			capabilityReader.reads = nil
			live := mutation.object(inventory)
			live.SetFinalizers([]string{"test.kubeblocks.io/hold"})
			Expect(apiReader.Client.Update(context.Background(), live)).Should(Succeed())
			Expect(apiReader.Client.Delete(context.Background(), live)).Should(Succeed())
			terminating := live.DeepCopyObject().(client.Object)
			Expect(apiReader.Client.Get(context.Background(),
				client.ObjectKeyFromObject(live), terminating)).Should(Succeed())
			Expect(terminating.GetDeletionTimestamp().IsZero()).Should(BeFalse())

			err := validateFreshShardingScaleInRequestAuthority(
				context.Background(), apiReader, inventory, capabilityReader, persisted)
			Expect(errors.Is(err, errShardingScaleInRequestAuthorityChanged)).Should(BeTrue())
		}
	})

	It("rejects second-read drift of every non-root persisted prerequisite", func() {
		for _, mutation := range prerequisiteCases {
			By(mutation.name)
			inventory, apiReader, capabilityReader := newFixture()
			persisted := buildPersistedPlan(inventory, apiReader, capabilityReader)
			capabilityReader.reads = nil
			source := mutation.object(inventory)
			apiReader.mutateSecondPrerequisiteKind = mutation.kind

			err := validateFreshShardingScaleInRequestAuthority(
				context.Background(), apiReader, inventory, capabilityReader, persisted)
			Expect(errors.Is(err, errShardingScaleInRequestAuthorityChanged)).Should(BeTrue())
			key := shardingScaleInRequestAuthorityTestReadKey(mutation.kind, source.GetUID())
			Expect(apiReader.prerequisiteReads[key]).Should(Equal(2))
		}
	})

	It("rejects stale topology and member inventories against exact API objects", func() {
		inventory, apiReader, capabilityReader := newFixture()
		persisted := buildPersistedPlan(inventory, apiReader, capabilityReader)
		capabilityReader.reads = nil

		liveCluster := &appsv1.Cluster{}
		Expect(apiReader.Client.Get(context.Background(),
			client.ObjectKeyFromObject(inventory.Members.Topology.Cluster), liveCluster)).Should(Succeed())
		liveCluster.Generation++
		Expect(apiReader.Client.Update(context.Background(), liveCluster)).Should(Succeed())
		err := validateFreshShardingScaleInRequestAuthority(
			context.Background(), apiReader, inventory, capabilityReader, persisted)
		Expect(errors.Is(err, errShardingScaleInRequestAuthorityChanged)).Should(BeTrue())

		inventory, apiReader, capabilityReader = newFixture()
		persisted = buildPersistedPlan(inventory, apiReader, capabilityReader)
		capabilityReader.reads = nil
		shardingDefinition := inventory.Members.Topology.ShardingDefinition.DeepCopy()
		Expect(apiReader.Client.Delete(context.Background(), shardingDefinition)).Should(Succeed())
		err = validateFreshShardingScaleInRequestAuthority(
			context.Background(), apiReader, inventory, capabilityReader, persisted)
		Expect(errors.Is(err, errShardingScaleInRequestAuthorityChanged)).Should(BeTrue())

		inventory, apiReader, capabilityReader = newFixture()
		persisted = buildPersistedPlan(inventory, apiReader, capabilityReader)
		capabilityReader.reads = nil
		componentKey := client.ObjectKeyFromObject(&inventory.Leaving[0].Member.Component)
		liveComponent := &appsv1.Component{}
		Expect(apiReader.Client.Get(context.Background(), componentKey, liveComponent)).Should(Succeed())
		replacementComponent := liveComponent.DeepCopy()
		replacementComponent.UID = "replacement-component-uid"
		replacementComponent.ResourceVersion = ""
		Expect(apiReader.Client.Delete(context.Background(), liveComponent)).Should(Succeed())
		Expect(apiReader.Client.Create(context.Background(), replacementComponent)).Should(Succeed())
		err = validateFreshShardingScaleInRequestAuthority(
			context.Background(), apiReader, inventory, capabilityReader, persisted)
		Expect(errors.Is(err, errShardingScaleInRequestAuthorityChanged)).Should(BeTrue())

		inventory, apiReader, capabilityReader = newFixture()
		persisted = buildPersistedPlan(inventory, apiReader, capabilityReader)
		capabilityReader.reads = nil
		workloadKey := client.ObjectKeyFromObject(&inventory.Leaving[0].Member.Workload)
		liveWorkload := &workloadsv1.InstanceSet{}
		Expect(apiReader.Client.Get(context.Background(), workloadKey, liveWorkload)).Should(Succeed())
		liveWorkload.Finalizers = []string{"test.kubeblocks.io/hold"}
		Expect(apiReader.Client.Update(context.Background(), liveWorkload)).Should(Succeed())
		Expect(apiReader.Client.Delete(context.Background(), liveWorkload)).Should(Succeed())
		terminatingWorkload := &workloadsv1.InstanceSet{}
		Expect(apiReader.Client.Get(
			context.Background(), workloadKey, terminatingWorkload)).Should(Succeed())
		Expect(terminatingWorkload.DeletionTimestamp.IsZero()).Should(BeFalse())
		err = validateFreshShardingScaleInRequestAuthority(
			context.Background(), apiReader, inventory, capabilityReader, persisted)
		Expect(errors.Is(err, errShardingScaleInRequestAuthorityChanged)).Should(BeTrue())
	})

	It("rejects Component drift between the before and after strong reads", func() {
		inventory, apiReader, capabilityReader := newFixture()
		persisted := buildPersistedPlan(inventory, apiReader, capabilityReader)
		capabilityReader.reads = nil
		apiReader.mutateSecondComponentRead = true

		err := validateFreshShardingScaleInRequestAuthority(
			context.Background(), apiReader, inventory, capabilityReader, persisted)
		Expect(errors.Is(err, errShardingScaleInRequestAuthorityChanged)).Should(BeTrue())
		Expect(apiReader.componentReads[inventory.Leaving[0].Member.Component.UID]).Should(Equal(2))
	})

	It("rejects a live collection with an extra EndpointSlice", func() {
		inventory, apiReader, capabilityReader := newFixture()
		persisted := buildPersistedPlan(inventory, apiReader, capabilityReader)
		capabilityReader.reads = nil
		extra := inventory.Leaving[0].EndpointSlices[0].DeepCopy()
		extra.Name += "-extra"
		extra.UID += "-extra"
		extra.ResourceVersion = ""
		Expect(apiReader.Client.Create(context.Background(), extra)).Should(Succeed())

		err := validateFreshShardingScaleInRequestAuthority(
			context.Background(), apiReader, inventory, capabilityReader, persisted)
		Expect(errors.Is(err, errShardingScaleInRequestAuthorityChanged)).Should(BeTrue())
	})

	It("rejects a live collection when optional Endpoints becomes present", func() {
		inventory, apiReader, capabilityReader := newFixture()
		endpoints := inventory.Leaving[0].Endpoints.DeepCopy()
		Expect(apiReader.Client.Delete(context.Background(), endpoints)).Should(Succeed())
		inventory.Leaving[0].Endpoints = nil
		inventory.Members.Leaving[0] = inventory.Leaving[0].Member
		persisted := buildPersistedPlan(inventory, apiReader, capabilityReader)
		capabilityReader.reads = nil
		endpoints.ResourceVersion = ""
		Expect(apiReader.Client.Create(context.Background(), endpoints)).Should(Succeed())

		err := validateFreshShardingScaleInRequestAuthority(
			context.Background(), apiReader, inventory, capabilityReader, persisted)
		Expect(errors.Is(err, errShardingScaleInRequestAuthorityChanged)).Should(BeTrue())
	})

	It("rejects a live collection with an extra matching Component", func() {
		inventory, apiReader, capabilityReader := newFixture()
		persisted := buildPersistedPlan(inventory, apiReader, capabilityReader)
		capabilityReader.reads = nil
		extra := inventory.Leaving[0].Member.Component.DeepCopy()
		extra.Name = "cluster-shard-2"
		extra.UID = "component-extra"
		extra.ResourceVersion = ""
		extra.Labels[constant.KBAppComponentLabelKey] = "shard-2"
		Expect(apiReader.Client.Create(context.Background(), extra)).Should(Succeed())

		err := validateFreshShardingScaleInRequestAuthority(
			context.Background(), apiReader, inventory, capabilityReader, persisted)
		Expect(errors.Is(err, errShardingScaleInRequestAuthorityChanged)).Should(BeTrue())
	})

	It("rejects a live collection with an extra matching Instance and Pod", func() {
		inventory, apiReader, capabilityReader := newFixture()
		persisted := buildPersistedPlan(inventory, apiReader, capabilityReader)
		capabilityReader.reads = nil
		extraInstance := inventory.Leaving[0].Member.Instances[0].DeepCopy()
		extraInstance.Name = "cluster-shard-1-1"
		extraInstance.UID = "instance-extra"
		extraInstance.ResourceVersion = ""
		Expect(apiReader.Client.Create(context.Background(), extraInstance)).Should(Succeed())
		extraPod := inventory.Leaving[0].Member.Pods[0].Pod.DeepCopy()
		extraPod.Name = extraInstance.Name
		extraPod.UID = "pod-extra"
		extraPod.ResourceVersion = ""
		extraPod.Labels[constant.KBAppInstanceNameLabelKey] = extraPod.Name
		extraPod.OwnerReferences = []metav1.OwnerReference{
			*metav1.NewControllerRef(
				extraInstance,
				workloadsv1.GroupVersion.WithKind(shardingScaleInInstanceKind)),
		}
		Expect(apiReader.Client.Create(context.Background(), extraPod)).Should(Succeed())

		err := validateFreshShardingScaleInRequestAuthority(
			context.Background(), apiReader, inventory, capabilityReader, persisted)
		Expect(errors.Is(err, errShardingScaleInRequestAuthorityChanged)).Should(BeTrue())
	})

	It("rejects a live collection that drifts between EndpointSlice Lists", func() {
		inventory, apiReader, capabilityReader := newFixture()
		persisted := buildPersistedPlan(inventory, apiReader, capabilityReader)
		capabilityReader.reads = nil
		apiReader.mutateSecondEndpointSliceList = true

		err := validateFreshShardingScaleInRequestAuthority(
			context.Background(), apiReader, inventory, capabilityReader, persisted)
		Expect(errors.Is(err, errShardingScaleInRequestAuthorityChanged)).Should(BeTrue())
	})
})

type shardingScaleInRequestAuthorityTestReader struct {
	client.Client
	podReads                      map[types.UID]int
	componentReads                map[types.UID]int
	prerequisiteReads             map[string]int
	mutateSecondPodRead           bool
	mutateSecondComponentRead     bool
	mutateSecondPrerequisiteKind  string
	endpointSliceLists            int
	mutateSecondEndpointSliceList bool
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
	if component, ok := object.(*appsv1.Component); ok {
		if r.componentReads == nil {
			r.componentReads = map[types.UID]int{}
		}
		r.componentReads[component.UID]++
		if r.mutateSecondComponentRead && r.componentReads[component.UID] == 2 {
			component.ResourceVersion += "-replacement"
		}
		return nil
	}
	prerequisiteKind := ""
	switch object.(type) {
	case *workloadsv1.Instance:
		prerequisiteKind = shardingScaleInInstanceKind
	case *corev1.Service:
		prerequisiteKind = "Service"
	case *corev1.Endpoints:
		prerequisiteKind = "Endpoints"
	case *discoveryv1.EndpointSlice:
		prerequisiteKind = "EndpointSlice"
	}
	if prerequisiteKind != "" {
		if r.prerequisiteReads == nil {
			r.prerequisiteReads = map[string]int{}
		}
		key := shardingScaleInRequestAuthorityTestReadKey(
			prerequisiteKind, object.GetUID())
		r.prerequisiteReads[key]++
		if r.mutateSecondPrerequisiteKind == prerequisiteKind &&
			r.prerequisiteReads[key] == 2 {
			object.SetResourceVersion(object.GetResourceVersion() + "-replacement")
		}
		return nil
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

func (r *shardingScaleInRequestAuthorityTestReader) List(
	ctx context.Context,
	list client.ObjectList,
	options ...client.ListOption,
) error {
	if err := r.Client.List(ctx, list, options...); err != nil {
		return err
	}
	slices, ok := list.(*discoveryv1.EndpointSliceList)
	if !ok {
		return nil
	}
	r.endpointSliceLists++
	if r.mutateSecondEndpointSliceList && r.endpointSliceLists == 2 &&
		len(slices.Items) > 0 {
		slices.Items[0].ResourceVersion += "-replacement"
	}
	return nil
}

func shardingScaleInRequestAuthorityTestReadKey(kind string, uid types.UID) string {
	return kind + "\x00" + string(uid)
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
