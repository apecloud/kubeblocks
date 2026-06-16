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
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/handler"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/multicluster"
)

var _ = Describe("cluster sharding shared transformers", func() {
	const (
		namespace    = "default"
		clusterName  = "cluster"
		shardingName = "shard"
		compDefName  = "compdef"
		accountName  = "root"
	)

	newTestScheme := func() *runtime.Scheme {
		scheme := runtime.NewScheme()
		Expect(corev1.AddToScheme(scheme)).Should(Succeed())
		Expect(appsv1.AddToScheme(scheme)).Should(Succeed())
		return scheme
	}

	newTransformContext := func(objects ...client.Object) *clusterTransformContext {
		return &clusterTransformContext{
			Context: context.Background(),
			Client: fake.NewClientBuilder().
				WithScheme(newTestScheme()).
				WithObjects(objects...).
				Build(),
			Cluster: &appsv1.Cluster{
				ObjectMeta: metav1ObjectMeta(clusterName, namespace),
			},
			OrigCluster: &appsv1.Cluster{
				ObjectMeta: metav1ObjectMeta(clusterName, namespace),
			},
		}
	}

	newComponentDefinition := func() *appsv1.ComponentDefinition {
		caFile := "ca.pem"
		certFile := "tls.crt"
		keyFile := "tls.key"
		return &appsv1.ComponentDefinition{
			ObjectMeta: metav1ObjectMeta(compDefName, ""),
			Spec: appsv1.ComponentDefinitionSpec{
				Labels:      map[string]string{"def-label": "yes"},
				Annotations: map[string]string{"def-annotation": "yes"},
				TLS: &appsv1.TLS{
					CAFile:   &caFile,
					CertFile: &certFile,
					KeyFile:  &keyFile,
				},
				SystemAccounts: []appsv1.SystemAccount{
					{
						Name: accountName,
						PasswordGenerationPolicy: appsv1.PasswordConfig{
							Length:    16,
							NumDigits: 4,
						},
					},
				},
			},
		}
	}

	newSharding := func() *appsv1.ClusterSharding {
		return &appsv1.ClusterSharding{
			Name:        shardingName,
			ShardingDef: "sharddef",
			Template: appsv1.ClusterComponentSpec{
				ComponentDef: compDefName,
				Labels:       map[string]string{"template-label": "yes"},
				Annotations:  map[string]string{"template-annotation": "yes"},
				TLS:          true,
				Issuer: &appsv1.Issuer{
					Name: appsv1.IssuerKubeBlocks,
				},
			},
		}
	}

	newShardingDefinition := func() *appsv1.ShardingDefinition {
		return &appsv1.ShardingDefinition{
			ObjectMeta: metav1ObjectMeta("sharddef", ""),
			Spec: appsv1.ShardingDefinitionSpec{
				SystemAccounts: []appsv1.ShardingSystemAccount{
					{Name: accountName, Shared: ptr.To(true)},
					{Name: "local", Shared: ptr.To(false)},
				},
				TLS: &appsv1.ShardingTLS{
					Shared: ptr.To(true),
				},
			},
		}
	}

	It("builds deterministic shared TLS secret metadata and rewrites sharding TLS config", func() {
		transformer := &clusterShardingTLSTransformer{}
		transCtx := newTransformContext()
		sharding := newSharding()
		compDef := newComponentDefinition()

		secret := transformer.newTLSSecret(transCtx, sharding, compDef)
		Expect(secret.Namespace).Should(Equal(namespace))
		Expect(secret.Name).Should(Equal(shardingTLSSecretName(clusterName, shardingName)))
		Expect(secret.Labels).Should(HaveKeyWithValue(constant.AppInstanceLabelKey, clusterName))
		Expect(secret.Labels).Should(HaveKeyWithValue(constant.KBAppShardingNameLabelKey, shardingName))
		Expect(secret.Labels).Should(HaveKeyWithValue("template-label", "yes"))
		Expect(secret.Labels).Should(HaveKeyWithValue("def-label", "yes"))
		Expect(secret.Annotations).Should(HaveKeyWithValue("template-annotation", "yes"))
		Expect(secret.Annotations).Should(HaveKeyWithValue("def-annotation", "yes"))
		Expect(secret.Data).Should(BeEmpty())

		transformer.rewriteTLSConfig(transCtx, sharding, compDef)
		Expect(sharding.Template.Issuer.Name).Should(Equal(appsv1.IssuerUserProvided))
		Expect(sharding.Template.Issuer.SecretRef.Namespace).Should(Equal(namespace))
		Expect(sharding.Template.Issuer.SecretRef.Name).Should(Equal(shardingTLSSecretName(clusterName, shardingName)))
		Expect(sharding.Template.Issuer.SecretRef.CA).Should(Equal("ca.pem"))
		Expect(sharding.Template.Issuer.SecretRef.Cert).Should(Equal("tls.crt"))
		Expect(sharding.Template.Issuer.SecretRef.Key).Should(Equal("tls.key"))
	})

	It("handles stable shared TLS reconciliation guard branches", func() {
		transformer := &clusterShardingTLSTransformer{}
		transCtx := newTransformContext()
		sharding := newSharding()

		sharding.Template.TLS = false
		sharding.Template.Issuer = nil
		Expect(transformer.reconcileShardingTLS(transCtx, nil, nil, sharding)).Should(Succeed())

		sharding.Template.TLS = true
		Expect(transformer.reconcileShardingTLS(transCtx, nil, nil, sharding)).Should(MatchError("issuer shouldn't be nil when tls enabled"))

		sharding.Template.Issuer = &appsv1.Issuer{Name: appsv1.IssuerUserProvided}
		Expect(transformer.reconcileShardingTLS(transCtx, nil, nil, sharding)).Should(Succeed())
	})

	It("reconciles only sharding definitions marked with shared TLS", func() {
		transformer := &clusterShardingTLSTransformer{}
		transCtx := newTransformContext()
		sharding := newSharding()
		sharding.Template.Issuer = &appsv1.Issuer{Name: appsv1.IssuerUserProvided}
		transCtx.shardings = []*appsv1.ClusterSharding{
			sharding,
			{Name: "plain", ShardingDef: "plain-def"},
			{Name: "missing", ShardingDef: "missing-def"},
		}
		transCtx.shardingDefs = map[string]*appsv1.ShardingDefinition{
			"sharddef":  newShardingDefinition(),
			"plain-def": {Spec: appsv1.ShardingDefinitionSpec{TLS: &appsv1.ShardingTLS{Shared: ptr.To(false)}}},
		}

		Expect(transformer.reconcileShardingTLSs(transCtx, nil, nil)).Should(Succeed())
	})

	It("checks shared TLS secret existence with a fake client", func() {
		transformer := &clusterShardingTLSTransformer{}
		sharding := newSharding()

		secret, err := transformer.checkTLSSecret(newTransformContext(), sharding)
		Expect(err).ShouldNot(HaveOccurred())
		Expect(secret).Should(BeNil())

		existing := &corev1.Secret{ObjectMeta: metav1ObjectMeta(shardingTLSSecretName(clusterName, shardingName), namespace)}
		secret, err = transformer.checkTLSSecret(newTransformContext(existing), sharding)
		Expect(err).ShouldNot(HaveOccurred())
		Expect(secret).ShouldNot(BeNil())
		Expect(secret.Name).Should(Equal(existing.Name))
	})

	It("resolves shared system accounts and applies cluster-level overrides", func() {
		transformer := &clusterShardingAccountTransformer{}
		sharding := newSharding()
		sharding.Template.SystemAccounts = []appsv1.ComponentSystemAccount{
			{
				Name: accountName,
				PasswordConfig: &appsv1.PasswordConfig{
					Length:    12,
					NumDigits: 2,
				},
			},
		}
		transCtx := newTransformContext()
		transCtx.componentDefs = map[string]*appsv1.ComponentDefinition{compDefName: newComponentDefinition()}

		account, err := transformer.definedSystemAccount(transCtx, sharding, accountName)
		Expect(err).ShouldNot(HaveOccurred())
		Expect(account.Name).Should(Equal(accountName))
		Expect(account.PasswordGenerationPolicy.Length).Should(Equal(int32(12)))
		Expect(account.PasswordGenerationPolicy.NumDigits).Should(Equal(int32(2)))

		transCtx.componentDefs = nil
		_, err = transformer.definedSystemAccount(transCtx, sharding, accountName)
		Expect(err).Should(MatchError(ContainSubstring("component definition compdef not found")))

		transCtx.componentDefs = map[string]*appsv1.ComponentDefinition{compDefName: newComponentDefinition()}
		_, err = transformer.definedSystemAccount(transCtx, sharding, "monitor")
		Expect(err).Should(MatchError(ContainSubstring("system account monitor not found")))
	})

	It("builds shared system account secrets with merged metadata and immutable data", func() {
		transformer := &clusterShardingAccountTransformer{}
		sharding := newSharding()
		transCtx := newTransformContext()
		transCtx.componentDefs = map[string]*appsv1.ComponentDefinition{compDefName: newComponentDefinition()}

		secret, err := transformer.newAccountSecretWithPassword(transCtx, sharding, accountName, []byte("password"))
		Expect(err).ShouldNot(HaveOccurred())
		Expect(secret.Namespace).Should(Equal(namespace))
		Expect(secret.Name).Should(Equal(shardingAccountSecretName(clusterName, shardingName, accountName)))
		Expect(secret.Immutable).ShouldNot(BeNil())
		Expect(*secret.Immutable).Should(BeTrue())
		Expect(secret.Labels).Should(HaveKeyWithValue(constant.AppInstanceLabelKey, clusterName))
		Expect(secret.Labels).Should(HaveKeyWithValue(constant.KBAppShardingNameLabelKey, shardingName))
		Expect(secret.Labels).Should(HaveKeyWithValue("template-label", "yes"))
		Expect(secret.Labels).Should(HaveKeyWithValue("def-label", "yes"))
		Expect(secret.Annotations).Should(HaveKeyWithValue("template-annotation", "yes"))
		Expect(secret.Annotations).Should(HaveKeyWithValue("def-annotation", "yes"))
		Expect(secret.Data).Should(HaveKeyWithValue(constant.AccountNameForSecret, []byte(accountName)))
		Expect(secret.Data).Should(HaveKeyWithValue(constant.AccountPasswdForSecret, []byte("password")))
	})

	It("generates shared system account secrets from component definitions", func() {
		transformer := &clusterShardingAccountTransformer{}
		sharding := newSharding()
		transCtx := newTransformContext()
		transCtx.componentDefs = map[string]*appsv1.ComponentDefinition{compDefName: newComponentDefinition()}

		secret, err := transformer.newSystemAccountSecret(transCtx, sharding, accountName)
		Expect(err).ShouldNot(HaveOccurred())
		Expect(secret.Name).Should(Equal(shardingAccountSecretName(clusterName, shardingName, accountName)))
		Expect(secret.Data).Should(HaveKeyWithValue(constant.AccountNameForSecret, []byte(accountName)))
		Expect(secret.Data[constant.AccountPasswdForSecret]).Should(HaveLen(16))
	})

	It("checks shared system account secret existence with a fake client", func() {
		transformer := &clusterShardingAccountTransformer{}
		sharding := newSharding()

		exist, err := transformer.checkSystemAccountSecret(newTransformContext(), sharding, accountName)
		Expect(err).ShouldNot(HaveOccurred())
		Expect(exist).Should(BeFalse())

		existing := &corev1.Secret{ObjectMeta: metav1ObjectMeta(shardingAccountSecretName(clusterName, shardingName, accountName), namespace)}
		exist, err = transformer.checkSystemAccountSecret(newTransformContext(existing), sharding, accountName)
		Expect(err).ShouldNot(HaveOccurred())
		Expect(exist).Should(BeTrue())
	})

	It("rewrites shared system accounts on sharding and generated shard components", func() {
		transformer := &clusterShardingAccountTransformer{}
		disabled := true
		sharding := newSharding()
		sharding.Template.SystemAccounts = []appsv1.ComponentSystemAccount{
			{
				Name:     accountName,
				Disabled: &disabled,
				SecretRef: &appsv1.ProvisionSecretRef{
					Password: "password-key",
				},
			},
		}
		transCtx := newTransformContext()
		transCtx.shardings = []*appsv1.ClusterSharding{sharding}
		transCtx.shardingComps = map[string][]*appsv1.ClusterComponentSpec{
			shardingName: {
				{Name: "shard-0"},
				{Name: "shard-1"},
			},
		}

		transformer.rewriteSystemAccount(transCtx, shardingName, accountName)
		rewritten := transCtx.shardings[0].Template.SystemAccounts[0]
		Expect(ptr.Deref(rewritten.Disabled, false)).Should(BeTrue())
		Expect(rewritten.SecretRef.Name).Should(Equal(shardingAccountSecretName(clusterName, shardingName, accountName)))
		Expect(rewritten.SecretRef.Namespace).Should(Equal(namespace))
		Expect(rewritten.SecretRef.Password).Should(Equal("password-key"))
		for _, comp := range transCtx.shardingComps[shardingName] {
			Expect(comp.SystemAccounts).Should(HaveLen(1))
			Expect(comp.SystemAccounts[0].SecretRef.Name).Should(Equal(rewritten.SecretRef.Name))
		}

		transformer.rewriteSystemAccount(transCtx, shardingName, "monitor")
		Expect(transCtx.shardings[0].Template.SystemAccounts).Should(HaveLen(2))
		Expect(transCtx.shardings[0].Template.SystemAccounts[1].Name).Should(Equal("monitor"))
		Expect(ptr.Deref(transCtx.shardings[0].Template.SystemAccounts[1].Disabled, true)).Should(BeFalse())
	})

	It("reconciles only sharding definitions marked with shared system accounts", func() {
		transformer := &clusterShardingAccountTransformer{}
		sharding := newSharding()
		existing := &corev1.Secret{ObjectMeta: metav1ObjectMeta(shardingAccountSecretName(clusterName, shardingName, accountName), namespace)}
		transCtx := newTransformContext(existing)
		transCtx.componentDefs = map[string]*appsv1.ComponentDefinition{compDefName: newComponentDefinition()}
		transCtx.shardings = []*appsv1.ClusterSharding{
			sharding,
			{Name: "missing", ShardingDef: "missing-def"},
		}
		transCtx.shardingDefs = map[string]*appsv1.ShardingDefinition{
			"sharddef": newShardingDefinition(),
		}
		transCtx.shardingComps = map[string][]*appsv1.ClusterComponentSpec{
			shardingName: {
				{Name: "shard-0"},
			},
		}

		Expect(transformer.reconcileShardingAccounts(transCtx, nil, nil)).Should(Succeed())
		Expect(transCtx.shardings[0].Template.SystemAccounts).Should(HaveLen(1))
		Expect(transCtx.shardings[0].Template.SystemAccounts[0].SecretRef.Name).
			Should(Equal(shardingAccountSecretName(clusterName, shardingName, accountName)))
		Expect(transCtx.shardingComps[shardingName][0].SystemAccounts).Should(HaveLen(1))
	})

	It("validates cluster topology and user-defined cluster boundaries", func() {
		transformer := &clusterValidationTransformer{}

		Expect(transformer.apiValidation(&appsv1.Cluster{
			Spec: appsv1.ClusterSpec{ComponentSpecs: []appsv1.ClusterComponentSpec{{Name: "mysql"}}},
		})).
			Should(MatchError(ContainSubstring("cluster API validate error")))
		Expect(transformer.apiValidation(&appsv1.Cluster{
			Spec: appsv1.ClusterSpec{ComponentSpecs: []appsv1.ClusterComponentSpec{{ComponentDef: compDefName}}},
		})).Should(Succeed())

		Expect(transformer.checkDefinitionNamePattern(&appsv1.Cluster{
			Spec: appsv1.ClusterSpec{ComponentSpecs: []appsv1.ClusterComponentSpec{{ComponentDef: "invalid[def"}}},
		})).Should(MatchError(ContainSubstring("invalid reference component/sharding definition name")))

		cluster := &appsv1.Cluster{
			Spec: appsv1.ClusterSpec{
				Topology:       "topology",
				ComponentSpecs: []appsv1.ClusterComponentSpec{{Name: "mysql"}},
				Shardings:      []appsv1.ClusterSharding{{Name: shardingName}},
			},
		}
		transCtx := newTransformContext()
		transCtx.clusterDef = &appsv1.ClusterDefinition{
			Spec: appsv1.ClusterDefinitionSpec{
				Topologies: []appsv1.ClusterTopology{
					{
						Name:       "topology",
						Components: []appsv1.ClusterTopologyComponent{{Name: "mysql"}},
						Shardings:  []appsv1.ClusterTopologySharding{{Name: shardingName}},
					},
				},
			},
		}
		Expect(transformer.checkNUpdateClusterTopology(transCtx, cluster)).Should(Succeed())

		cluster.Spec.ComponentSpecs = []appsv1.ClusterComponentSpec{{Name: "redis"}}
		Expect(transformer.checkNUpdateClusterTopology(transCtx, cluster)).
			Should(MatchError(ContainSubstring("component redis not defined")))

		cluster.Spec.ComponentSpecs = []appsv1.ClusterComponentSpec{{Name: "mysql"}}
		cluster.Spec.Shardings = []appsv1.ClusterSharding{{Name: "other"}}
		Expect(transformer.checkNUpdateClusterTopology(transCtx, cluster)).
			Should(MatchError(ContainSubstring("sharding other not defined")))

		cluster.Spec.Topology = "missing"
		Expect(transformer.checkNUpdateClusterTopology(transCtx, cluster)).
			Should(MatchError(ContainSubstring("specified cluster topology not found")))
	})

	It("validates and assigns multi-cluster placement deterministically", func() {
		transformer := &clusterPlacementTransformer{}
		transCtx := newTransformContext()

		Expect(transformer.enabled(transCtx)).Should(BeFalse())
		transCtx.OrigCluster.Annotations = map[string]string{constant.KBAppMultiClusterPlacementKey: ""}
		Expect(transformer.enabled(transCtx)).Should(BeTrue())
		Expect(transformer.assigned(transCtx)).Should(BeFalse())
		transCtx.OrigCluster.Annotations[constant.KBAppMultiClusterPlacementKey] = "ctx-a"
		Expect(transformer.assigned(transCtx)).Should(BeTrue())

		Expect(transformer.precheck(transCtx)).Should(MatchError(ContainSubstring("multi-cluster manager is not set up properly")))

		transformer.multiClusterMgr = fakeMultiClusterManager{contexts: []string{"ctx-c", "ctx-a", "ctx-b"}}
		transCtx.components = []*appsv1.ClusterComponentSpec{{Name: "mysql"}}
		Expect(transformer.precheck(transCtx)).Should(MatchError(ContainSubstring("components that enable the instance API: mysql")))

		transCtx.components = []*appsv1.ClusterComponentSpec{{Name: "mysql", EnableInstanceAPI: ptr.To(true)}}
		transCtx.shardings = []*appsv1.ClusterSharding{{Name: shardingName}}
		Expect(transformer.precheck(transCtx)).Should(MatchError(ContainSubstring("shardings that enable the instance API: shard")))

		transCtx.shardings = []*appsv1.ClusterSharding{{Name: shardingName, Template: appsv1.ClusterComponentSpec{EnableInstanceAPI: ptr.To(true)}}}
		Expect(transformer.precheck(transCtx)).Should(Succeed())

		transCtx.components = []*appsv1.ClusterComponentSpec{
			{Name: "mysql", Replicas: 2, EnableInstanceAPI: ptr.To(true)},
		}
		transCtx.shardingComps = map[string][]*appsv1.ClusterComponentSpec{
			shardingName: {
				{Name: "shard-0", Replicas: 1},
				{Name: "shard-1", Replicas: 3},
			},
		}
		Expect(transformer.maxReplicas(transCtx)).Should(Equal(3))

		assigned := transformer.assign(transCtx)
		Expect(assigned).Should(HaveLen(3))
		Expect(assigned).Should(ConsistOf("ctx-a", "ctx-b", "ctx-c"))

		transCtx.shardingComps[shardingName][1].Replicas = 2
		assigned = transformer.assign(transCtx)
		Expect(assigned).Should(HaveLen(2))
		for _, contextName := range assigned {
			Expect([]string{"ctx-a", "ctx-b", "ctx-c"}).Should(ContainElement(contextName))
		}

		transformCtx := newTransformContext()
		Expect(transformer.Transform(transformCtx, nil)).Should(Succeed())

		transformCtx.OrigCluster.Annotations = map[string]string{constant.KBAppMultiClusterPlacementKey: "ctx-a"}
		Expect(transformer.Transform(transformCtx, nil)).Should(Succeed())
		Expect(transformCtx.Cluster.Annotations).Should(BeNil())

		transformCtx.OrigCluster.Annotations[constant.KBAppMultiClusterPlacementKey] = ""
		transformCtx.components = []*appsv1.ClusterComponentSpec{{Name: "mysql", Replicas: 2, EnableInstanceAPI: ptr.To(true)}}
		transformCtx.shardings = []*appsv1.ClusterSharding{{Name: shardingName, Template: appsv1.ClusterComponentSpec{EnableInstanceAPI: ptr.To(true)}}}
		transformCtx.shardingComps = map[string][]*appsv1.ClusterComponentSpec{
			shardingName: {
				{Name: "shard-0", Replicas: 1},
			},
		}
		Expect(transformer.Transform(transformCtx, nil)).Should(Succeed())
		Expect(transformCtx.Cluster.Annotations[constant.KBAppMultiClusterPlacementKey]).ShouldNot(BeEmpty())

		deletingCtx := newTransformContext()
		now := metav1.Now()
		deletingCtx.OrigCluster.DeletionTimestamp = &now
		Expect(transformer.Transform(deletingCtx, nil)).Should(Succeed())
	})
})

func metav1ObjectMeta(name, namespace string) metav1.ObjectMeta {
	return metav1.ObjectMeta{
		Name:      name,
		Namespace: namespace,
	}
}

type fakeMultiClusterManager struct {
	contexts []string
}

func (m fakeMultiClusterManager) GetClient() client.Client {
	return nil
}

func (m fakeMultiClusterManager) GetContexts() []string {
	return append([]string(nil), m.contexts...)
}

func (m fakeMultiClusterManager) Bind(ctrl.Manager) error {
	return nil
}

func (m fakeMultiClusterManager) Own(*builder.Builder, client.Object, client.Object) multicluster.Manager {
	return m
}

func (m fakeMultiClusterManager) Watch(*builder.Builder, client.Object, handler.EventHandler) multicluster.Manager {
	return m
}
