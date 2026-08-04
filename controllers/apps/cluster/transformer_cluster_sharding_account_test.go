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
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
)

var _ = Describe("cluster sharding shared system account password contract", func() {
	const (
		namespace   = "default"
		clusterName = "cluster"
		sharding    = "sharding"
		compDefName = "compdef"
		accountName = "root"
	)

	newContext := func(account appsv1.SystemAccount, objects ...client.Object) (*clusterTransformContext, *appsv1.ClusterSharding) {
		scheme := runtime.NewScheme()
		Expect(corev1.AddToScheme(scheme)).To(Succeed())
		compDef := &appsv1.ComponentDefinition{
			ObjectMeta: metav1.ObjectMeta{Name: compDefName},
			Spec: appsv1.ComponentDefinitionSpec{
				SystemAccounts: []appsv1.SystemAccount{account},
			},
		}
		return &clusterTransformContext{
				Context: context.Background(),
				Client: fake.NewClientBuilder().
					WithScheme(scheme).
					WithObjects(objects...).
					Build(),
				Cluster: &appsv1.Cluster{
					ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: clusterName},
				},
				componentDefs: map[string]*appsv1.ComponentDefinition{compDefName: compDef},
			}, &appsv1.ClusterSharding{
				Name: sharding,
				Template: appsv1.ClusterComponentSpec{
					ComponentDef: compDefName,
				},
			}
	}

	It("generates a password from the presence-aware configuration", func() {
		transCtx, shardingSpec := newContext(appsv1.SystemAccount{
			Name: accountName,
			PasswordConfig: &appsv1.PasswordConfig{
				Length:    16,
				NumDigits: ptr.To[int32](0),
			},
		})

		secret, err := (&clusterShardingAccountTransformer{}).
			newSystemAccountSecret(transCtx, shardingSpec, accountName)
		Expect(err).NotTo(HaveOccurred())
		Expect(secret.Data[constant.AccountPasswdForSecret]).To(HaveLen(16))
	})

	It("keeps the deprecated generation policy compatible", func() {
		transCtx, shardingSpec := newContext(appsv1.SystemAccount{
			Name: accountName,
			PasswordGenerationPolicy: appsv1.PasswordConfig{
				Length:    12,
				NumDigits: ptr.To[int32](2),
			},
		})

		secret, err := (&clusterShardingAccountTransformer{}).
			newSystemAccountSecret(transCtx, shardingSpec, accountName)
		Expect(err).NotTo(HaveOccurred())
		Expect(secret.Data[constant.AccountPasswdForSecret]).To(HaveLen(12))
	})

	It("creates a passwordless account when neither configuration is supplied", func() {
		transCtx, shardingSpec := newContext(appsv1.SystemAccount{Name: accountName})

		secret, err := (&clusterShardingAccountTransformer{}).
			newSystemAccountSecret(transCtx, shardingSpec, accountName)
		Expect(err).NotTo(HaveOccurred())
		Expect(secret.Data).To(HaveKey(constant.AccountPasswdForSecret))
		Expect(secret.Data[constant.AccountPasswdForSecret]).To(BeEmpty())
	})

	It("preserves an empty password from a same-namespace secretRef", func() {
		referenced := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: "passwordless"},
			Data:       map[string][]byte{constant.AccountPasswdForSecret: {}},
		}
		transCtx, shardingSpec := newContext(appsv1.SystemAccount{Name: accountName}, referenced)
		shardingSpec.Template.SystemAccounts = []appsv1.ComponentSystemAccount{{
			Name: accountName,
			SecretRef: &appsv1.ProvisionSecretRef{
				Name: referenced.Name,
			},
		}}

		secret, err := (&clusterShardingAccountTransformer{}).
			newSystemAccountSecret(transCtx, shardingSpec, accountName)
		Expect(err).NotTo(HaveOccurred())
		Expect(secret.Data[constant.AccountPasswdForSecret]).To(BeEmpty())
	})

	It("rejects a cross-namespace secretRef", func() {
		transCtx, shardingSpec := newContext(appsv1.SystemAccount{Name: accountName})
		shardingSpec.Template.SystemAccounts = []appsv1.ComponentSystemAccount{{
			Name: accountName,
			SecretRef: &appsv1.ProvisionSecretRef{
				Name:      "password",
				Namespace: "other",
			},
		}}

		_, err := (&clusterShardingAccountTransformer{}).
			newSystemAccountSecret(transCtx, shardingSpec, accountName)
		Expect(err).To(MatchError("cross-namespace secretRef is not supported for shared sharding system accounts"))
	})
})
