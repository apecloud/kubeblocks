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
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	appsutil "github.com/apecloud/kubeblocks/controllers/apps/util"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/graph"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
)

var _ = Describe("cluster sharding account transformer", func() {
	It("uses sharding template secretRef password and copies provisioned annotation", func() {
		const (
			namespace    = "default"
			clusterName  = "cluster"
			shardingName = "shard"
			shardingDef  = "shardingdef"
			compDefName  = "compdef"
			accountName  = "root"
			sourceName   = "source-account"
			passwordKey  = "root-password"
		)

		sourceSecret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: namespace,
				Name:      sourceName,
				Annotations: map[string]string{
					constant.SystemAccountProvisionedAnnotationKey: "true",
				},
			},
			Data: map[string][]byte{
				passwordKey: []byte("source-password"),
			},
		}
		cluster := &appsv1.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: namespace,
				Name:      clusterName,
			},
		}
		sharding := &appsv1.ClusterSharding{
			Name:        shardingName,
			ShardingDef: shardingDef,
			Template: appsv1.ClusterComponentSpec{
				ComponentDef: compDefName,
				SystemAccounts: []appsv1.ComponentSystemAccount{
					{
						Name: accountName,
						SecretRef: &appsv1.ProvisionSecretRef{
							Name:      sourceName,
							Namespace: namespace,
							Password:  passwordKey,
						},
					},
				},
			},
		}
		graphCli := model.NewGraphClient(&appsutil.MockReader{
			Objects: []client.Object{sourceSecret},
		})
		transCtx := &clusterTransformContext{
			Context:     context.Background(),
			Client:      graphCli,
			Cluster:     cluster,
			OrigCluster: cluster.DeepCopy(),
			shardings:   []*appsv1.ClusterSharding{sharding},
			shardingDefs: map[string]*appsv1.ShardingDefinition{
				shardingDef: {
					ObjectMeta: metav1.ObjectMeta{Name: shardingDef},
					Spec: appsv1.ShardingDefinitionSpec{
						SystemAccounts: []appsv1.ShardingSystemAccount{
							{
								Name:   accountName,
								Shared: ptr.To(true),
							},
						},
					},
				},
			},
			componentDefs: map[string]*appsv1.ComponentDefinition{
				compDefName: {
					ObjectMeta: metav1.ObjectMeta{Name: compDefName},
					Spec: appsv1.ComponentDefinitionSpec{
						SystemAccounts: []appsv1.SystemAccount{
							{
								Name: accountName,
							},
						},
					},
				},
			},
			shardingComps: map[string][]*appsv1.ClusterComponentSpec{
				shardingName: {
					{
						Name:         "shard-0000",
						ComponentDef: compDefName,
					},
				},
			},
		}
		dag := graph.NewDAG()
		graphCli.Root(dag, cluster, cluster, model.ActionStatusPtr())

		err := (&clusterShardingAccountTransformer{}).Transform(transCtx, dag)

		Expect(err).ShouldNot(HaveOccurred())
		objs := graphCli.FindAll(dag, &corev1.Secret{})
		Expect(objs).Should(HaveLen(1))
		secret := objs[0].(*corev1.Secret)
		Expect(secret.Name).Should(Equal(shardingAccountSecretName(clusterName, shardingName, accountName)))
		Expect(secret.Data).Should(HaveKeyWithValue(constant.AccountNameForSecret, []byte(accountName)))
		Expect(secret.Data).Should(HaveKeyWithValue(constant.AccountPasswdForSecret, []byte("source-password")))
		Expect(secret.Annotations).Should(HaveKeyWithValue(constant.SystemAccountProvisionedAnnotationKey, "true"))

		rewrittenAccount := transCtx.shardings[0].Template.SystemAccounts[0]
		Expect(rewrittenAccount.SecretRef.Name).Should(Equal(secret.Name))
		Expect(rewrittenAccount.SecretRef.Namespace).Should(Equal(namespace))
		Expect(rewrittenAccount.SecretRef.Password).Should(BeEmpty())

		shardComponentAccount := transCtx.shardingComps[shardingName][0].SystemAccounts[0]
		Expect(shardComponentAccount.SecretRef.Name).Should(Equal(secret.Name))
		Expect(shardComponentAccount.SecretRef.Namespace).Should(Equal(namespace))
		Expect(shardComponentAccount.SecretRef.Password).Should(BeEmpty())
	})
})
