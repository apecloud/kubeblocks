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
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
)

var _ = Describe("cluster sharding shared TLS transformer", func() {
	const (
		namespace    = "default"
		clusterName  = "cluster"
		shardingName = "shard"
		compDefName  = "compdef"
	)

	newTestScheme := func() *runtime.Scheme {
		scheme := runtime.NewScheme()
		Expect(corev1.AddToScheme(scheme)).Should(Succeed())
		Expect(appsv1.AddToScheme(scheme)).Should(Succeed())
		return scheme
	}
	newObjectMeta := func(name, namespace string) metav1.ObjectMeta {
		return metav1.ObjectMeta{Name: name, Namespace: namespace}
	}

	newTransformContext := func(objects ...client.Object) *clusterTransformContext {
		return &clusterTransformContext{
			Context: context.Background(),
			Client: fake.NewClientBuilder().
				WithScheme(newTestScheme()).
				WithObjects(objects...).
				Build(),
			Cluster: &appsv1.Cluster{
				ObjectMeta: newObjectMeta(clusterName, namespace),
			},
			OrigCluster: &appsv1.Cluster{
				ObjectMeta: newObjectMeta(clusterName, namespace),
			},
		}
	}

	newComponentDefinition := func() *appsv1.ComponentDefinition {
		caFile := "ca.pem"
		certFile := "tls.crt"
		keyFile := "tls.key"
		return &appsv1.ComponentDefinition{
			ObjectMeta: newObjectMeta(compDefName, ""),
			Spec: appsv1.ComponentDefinitionSpec{
				Labels:      map[string]string{"def-label": "yes"},
				Annotations: map[string]string{"def-annotation": "yes"},
				TLS: &appsv1.TLS{
					CAFile:   &caFile,
					CertFile: &certFile,
					KeyFile:  &keyFile,
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
				Issuer:       &appsv1.Issuer{Name: appsv1.IssuerKubeBlocks},
			},
		}
	}

	newShardingDefinition := func() *appsv1.ShardingDefinition {
		return &appsv1.ShardingDefinition{
			ObjectMeta: newObjectMeta("sharddef", ""),
			Spec: appsv1.ShardingDefinitionSpec{
				TLS: &appsv1.ShardingTLS{Shared: ptr.To(true)},
			},
		}
	}

	It("builds deterministic shared TLS secret metadata and rewrites expanded shard specs", func() {
		transformer := &clusterShardingTLSTransformer{}
		transCtx := newTransformContext()
		sharding := newSharding()
		flattenedComp := sharding.Template.DeepCopy()
		flattenedComp.Name = "default-shard"
		templatedComp := sharding.Template.DeepCopy()
		templatedComp.Name = "templated-shard"
		transCtx.shardingComps = map[string][]*appsv1.ClusterComponentSpec{
			sharding.Name: {flattenedComp, templatedComp},
		}
		transCtx.shardingCompsWithTpl = map[string]map[string][]*appsv1.ClusterComponentSpec{
			sharding.Name: {"": {flattenedComp}, "template": {templatedComp}},
		}

		secret := transformer.newTLSSecret(transCtx, sharding)
		Expect(secret.Namespace).Should(Equal(namespace))
		Expect(secret.Name).Should(Equal(shardingTLSSecretName(clusterName, shardingName)))
		Expect(secret.Labels).Should(HaveKeyWithValue(constant.AppInstanceLabelKey, clusterName))
		Expect(secret.Labels).Should(HaveKeyWithValue(constant.KBAppShardingNameLabelKey, shardingName))
		Expect(secret.Labels).Should(HaveKeyWithValue("template-label", "yes"))
		Expect(secret.Annotations).Should(HaveKeyWithValue("template-annotation", "yes"))
		Expect(secret.Data).Should(BeEmpty())

		transformer.rewriteTLSConfig(transCtx, sharding, sets.New("", "template"))
		Expect(sharding.Template.Issuer.Name).Should(Equal(appsv1.IssuerKubeBlocks))
		for _, comp := range []*appsv1.ClusterComponentSpec{flattenedComp, templatedComp} {
			Expect(comp.Issuer.Name).Should(Equal(appsv1.IssuerUserProvided))
			Expect(comp.Issuer.SecretRef.Name).Should(Equal(shardingTLSSecretName(clusterName, shardingName)))
			Expect(comp.Issuer.SecretRef.CA).Should(Equal(shardingTLSCAKey))
			Expect(comp.Issuer.SecretRef.Cert).Should(Equal(shardingTLSCertKey))
			Expect(comp.Issuer.SecretRef.Key).Should(Equal(shardingTLSKeyKey))
		}
	})

	It("keeps managed labels authoritative on the shared TLS secret", func() {
		transformer := &clusterShardingTLSTransformer{}
		transCtx := newTransformContext()
		sharding := newSharding()
		sharding.Template.Labels[constant.AppInstanceLabelKey] = "other-cluster"
		sharding.Template.Labels[constant.KBAppShardingNameLabelKey] = "other-sharding"

		secret := transformer.newTLSSecret(transCtx, sharding)
		Expect(secret.Labels).Should(HaveKeyWithValue(constant.AppInstanceLabelKey, clusterName))
		Expect(secret.Labels).Should(HaveKeyWithValue(constant.KBAppShardingNameLabelKey, shardingName))
	})

	It("handles stable shared TLS reconciliation guard branches", func() {
		transformer := &clusterShardingTLSTransformer{}
		transCtx := newTransformContext()
		sharding := newSharding()

		sharding.Template.TLS = false
		sharding.Template.Issuer = nil
		Expect(transformer.reconcileShardingTLS(transCtx, nil, nil, sharding, sets.New(""))).Should(Succeed())

		sharding.Template.TLS = true
		Expect(transformer.reconcileShardingTLS(transCtx, nil, nil, sharding, sets.New(""))).Should(
			MatchError("issuer shouldn't be nil when tls enabled"))

		sharding.Template.Issuer = &appsv1.Issuer{Name: appsv1.IssuerName("unsupported")}
		Expect(transformer.reconcileShardingTLS(transCtx, nil, nil, sharding, sets.New(""))).Should(
			MatchError(`unsupported TLS issuer "unsupported"`))

		sharding.Template.Issuer = &appsv1.Issuer{Name: appsv1.IssuerUserProvided}
		Expect(transformer.reconcileShardingTLS(transCtx, nil, nil, sharding, sets.New(""))).Should(Succeed())
	})

	It("honors TLS sharing overrides from each shard template's ShardingDefinition", func() {
		transformer := &clusterShardingTLSTransformer{}
		transCtx := newTransformContext()
		sharding := newSharding()
		sharding.ShardTemplates = []appsv1.ShardTemplate{
			{Name: "private", ShardingDef: ptr.To("plain-def")},
			{Name: "shared", ShardingDef: ptr.To("shared-def")},
		}
		transCtx.shardingDefs = map[string]*appsv1.ShardingDefinition{
			"sharddef":   newShardingDefinition(),
			"shared-def": newShardingDefinition(),
			"plain-def":  {Spec: appsv1.ShardingDefinitionSpec{TLS: &appsv1.ShardingTLS{Shared: ptr.To(false)}}},
		}

		defaultComp := sharding.Template.DeepCopy()
		defaultComp.Name = "default"
		privateComp := sharding.Template.DeepCopy()
		privateComp.Name = "private"
		sharedComp := sharding.Template.DeepCopy()
		sharedComp.Name = "shared"
		transCtx.shardingComps = map[string][]*appsv1.ClusterComponentSpec{
			sharding.Name: {defaultComp, privateComp, sharedComp},
		}
		transCtx.shardingCompsWithTpl = map[string]map[string][]*appsv1.ClusterComponentSpec{
			sharding.Name: {"": {defaultComp}, "private": {privateComp}, "shared": {sharedComp}},
		}

		sharedTemplates := transformer.sharedShardTemplates(transCtx, sharding)
		Expect(sharedTemplates).Should(Equal(sets.New("", "shared")))
		transformer.rewriteTLSConfig(transCtx, sharding, sharedTemplates)
		Expect(defaultComp.Issuer.Name).Should(Equal(appsv1.IssuerUserProvided))
		Expect(sharedComp.Issuer.Name).Should(Equal(appsv1.IssuerUserProvided))
		Expect(privateComp.Issuer.Name).Should(Equal(appsv1.IssuerKubeBlocks))
		Expect(sharding.Template.Issuer.Name).Should(Equal(appsv1.IssuerKubeBlocks))
	})

	It("uses fixed source keys when ComponentDefinition TLS file names are omitted", func() {
		transformer := &clusterShardingTLSTransformer{}
		transCtx := newTransformContext()
		sharding := newSharding()
		compDef := newComponentDefinition()
		compDef.Spec.TLS.CAFile = nil
		compDef.Spec.TLS.CertFile = nil
		compDef.Spec.TLS.KeyFile = nil
		transCtx.componentDefs = map[string]*appsv1.ComponentDefinition{compDef.Name: compDef}

		secret, err := transformer.buildTLSSecret(transCtx, sharding)
		Expect(err).ShouldNot(HaveOccurred())
		Expect(secret.Data).Should(HaveKey(shardingTLSCAKey))
		Expect(secret.Data).Should(HaveKey(shardingTLSCertKey))
		Expect(secret.Data).Should(HaveKey(shardingTLSKeyKey))

		comp := sharding.Template.DeepCopy()
		transCtx.shardingComps = map[string][]*appsv1.ClusterComponentSpec{sharding.Name: {comp}}
		transCtx.shardingCompsWithTpl = map[string]map[string][]*appsv1.ClusterComponentSpec{
			sharding.Name: {"": {comp}},
		}
		transformer.rewriteTLSConfig(transCtx, sharding, sets.New(""))
		Expect(comp.Issuer.SecretRef.CA).Should(Equal(shardingTLSCAKey))
		Expect(comp.Issuer.SecretRef.Cert).Should(Equal(shardingTLSCertKey))
		Expect(comp.Issuer.SecretRef.Key).Should(Equal(shardingTLSKeyKey))
	})

	It("checks shared TLS secret existence with a fake client", func() {
		transformer := &clusterShardingTLSTransformer{}
		sharding := newSharding()

		secret, err := transformer.checkTLSSecret(newTransformContext(), sharding)
		Expect(err).ShouldNot(HaveOccurred())
		Expect(secret).Should(BeNil())

		existing := &corev1.Secret{ObjectMeta: newObjectMeta(shardingTLSSecretName(clusterName, shardingName), namespace)}
		existing.Labels = constant.GetClusterLabels(clusterName, map[string]string{
			constant.KBAppShardingNameLabelKey: shardingName,
		})
		secret, err = transformer.checkTLSSecret(newTransformContext(existing), sharding)
		Expect(err).ShouldNot(HaveOccurred())
		Expect(secret).ShouldNot(BeNil())

		unmanaged := existing.DeepCopy()
		unmanaged.Labels = nil
		secret, err = transformer.checkTLSSecret(newTransformContext(unmanaged), sharding)
		Expect(err).Should(MatchError(ContainSubstring("is not managed by sharding")))
		Expect(secret).Should(BeNil())
	})

	It("rejects shared TLS before dereferencing an unavailable TLS definition", func() {
		transformer := &clusterShardingTLSTransformer{}
		sharding := newSharding()
		transCtx := newTransformContext()
		transCtx.componentDefs = map[string]*appsv1.ComponentDefinition{}

		Expect(transformer.reconcileShardingTLS(transCtx, nil, nil, sharding, sets.New(""))).Should(
			MatchError(ContainSubstring("component definition \"compdef\" not found")))

		compDef := newComponentDefinition()
		compDef.Spec.TLS = nil
		transCtx.componentDefs[compDefName] = compDef
		Expect(transformer.reconcileShardingTLS(transCtx, nil, nil, sharding, sets.New(""))).Should(
			MatchError(ContainSubstring("doesn't support it")))
	})
})
