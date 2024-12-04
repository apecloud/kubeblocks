/*
Copyright (C) 2022-2024 ApeCloud Co., Ltd

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

package apps

import (
	"fmt"
	"reflect"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	"github.com/apecloud/kubeblocks/pkg/common"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/builder"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	"github.com/apecloud/kubeblocks/pkg/controller/graph"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
	"github.com/apecloud/kubeblocks/pkg/controller/plan"
)

// clusterShardingTLSTransformer handles shared TLS for sharding.
type clusterShardingTLSTransformer struct{}

var _ graph.Transformer = &clusterShardingTLSTransformer{}

func (t *clusterShardingTLSTransformer) Transform(ctx graph.TransformContext, dag *graph.DAG) error {
	transCtx, _ := ctx.(*clusterTransformContext)
	if model.IsObjectDeleting(transCtx.Cluster) {
		return nil
	}

	if common.IsCompactMode(transCtx.Cluster.Annotations) {
		transCtx.V(1).Info("Cluster is in compact mode, no need to create tls related objects", "cluster", client.ObjectKeyFromObject(transCtx.Cluster))
		return nil
	}

	graphCli, _ := transCtx.Client.(model.GraphClient)
	return t.reconcileShardingTLSs(transCtx, graphCli, dag)
}

func (t *clusterShardingTLSTransformer) reconcileShardingTLSs(
	transCtx *clusterTransformContext, graphCli model.GraphClient, dag *graph.DAG) error {
	for _, sharding := range transCtx.shardings {
		shardDef, ok := transCtx.shardingDefs[sharding.ShardingDef]
		if ok {
			tls := shardDef.Spec.TLS
			if tls != nil && tls.Shared != nil && *tls.Shared {
				if err := t.reconcileShardingTLS(transCtx, graphCli, dag, sharding); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func (t *clusterShardingTLSTransformer) reconcileShardingTLS(transCtx *clusterTransformContext,
	graphCli model.GraphClient, dag *graph.DAG, sharding *appsv1.ClusterSharding) error {
	if !sharding.Template.TLS {
		return nil
	}
	if sharding.Template.Issuer == nil {
		return fmt.Errorf("issuer shouldn't be nil when tls enabled")
	}
	if sharding.Template.Issuer.Name == appsv1.IssuerUserProvided {
		return nil // all components will share the same secret
	}

	secret, err := t.checkTLSSecret(transCtx, sharding)
	if err != nil {
		return err
	}

	compDef := transCtx.componentDefs[sharding.Template.ComponentDef]
	if secret == nil {
		obj, err1 := t.buildTLSSecret(transCtx, sharding, compDef)
		if err1 != nil {
			return err1
		}
		graphCli.Create(dag, obj)
	} else {
		proto := t.newTLSSecret(transCtx, sharding, compDef)
		secretCopy := secret.DeepCopy()
		secretCopy.Labels = proto.Labels
		secretCopy.Annotations = proto.Annotations
		if !reflect.DeepEqual(secret, secretCopy) {
			graphCli.Update(dag, secret, secretCopy)
		}
	}

	t.rewriteTLSConfig(transCtx, sharding, compDef)

	return nil
}

func (t *clusterShardingTLSTransformer) checkTLSSecret(
	transCtx *clusterTransformContext, sharding *appsv1.ClusterSharding) (*corev1.Secret, error) {
	var (
		cluster = transCtx.Cluster
	)
	secretKey := types.NamespacedName{
		Namespace: cluster.Namespace,
		Name:      shardingTLSSecretName(cluster.Name, sharding.Name),
	}
	secret := &corev1.Secret{}
	err := transCtx.GetClient().Get(transCtx.GetContext(), secretKey, secret)
	if err != nil {
		return nil, client.IgnoreNotFound(err)
	}
	return secret, nil
}

func (t *clusterShardingTLSTransformer) buildTLSSecret(transCtx *clusterTransformContext,
	sharding *appsv1.ClusterSharding, compDef *appsv1.ComponentDefinition) (*corev1.Secret, error) {
	synthesizedComp := component.SynthesizedComponent{
		Namespace:   transCtx.Cluster.Namespace,
		ClusterName: transCtx.Cluster.Name,
		Name:        sharding.Name,
	}
	secret := t.newTLSSecret(transCtx, sharding, compDef)
	return plan.ComposeTLSSecret(compDef, synthesizedComp, secret)
}

func (t *clusterShardingTLSTransformer) newTLSSecret(transCtx *clusterTransformContext,
	sharding *appsv1.ClusterSharding, compDef *appsv1.ComponentDefinition) *corev1.Secret {
	var (
		cluster      = transCtx.Cluster
		namespace    = cluster.Namespace
		clusterName  = cluster.Name
		shardingName = sharding.Name
	)
	shardingLabels := map[string]string{
		constant.KBAppShardingNameLabelKey: shardingName,
	}
	return builder.NewSecretBuilder(namespace, shardingTLSSecretName(clusterName, shardingName)).
		AddLabelsInMap(constant.GetClusterLabels(clusterName, shardingLabels)).
		AddLabelsInMap(sharding.Template.Labels).
		AddLabelsInMap(compDef.Spec.Labels).
		AddAnnotationsInMap(sharding.Template.Annotations).
		AddAnnotationsInMap(compDef.Spec.Annotations).
		SetStringData(map[string]string{}).
		GetObject()
}

func (t *clusterShardingTLSTransformer) rewriteTLSConfig(
	transCtx *clusterTransformContext, sharding *appsv1.ClusterSharding, compDef *appsv1.ComponentDefinition) {
	sharding.Template.Issuer = &appsv1.Issuer{
		Name: appsv1.IssuerUserProvided,
		SecretRef: &appsv1.TLSSecretRef{
			Name: shardingTLSSecretName(transCtx.Cluster.Name, sharding.Name),
		},
	}
	tls := compDef.Spec.TLS
	if tls.CAFile != nil {
		sharding.Template.Issuer.SecretRef.CA = *tls.CAFile
	}
	if tls.CertFile != nil {
		sharding.Template.Issuer.SecretRef.Cert = *tls.CertFile
	}
	if tls.KeyFile != nil {
		sharding.Template.Issuer.SecretRef.Key = *tls.KeyFile
	}
}

func shardingTLSSecretName(cluster, sharding string) string {
	return fmt.Sprintf("%s-%s-tls", cluster, sharding)
}
