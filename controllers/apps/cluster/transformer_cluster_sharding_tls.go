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
	"fmt"
	"reflect"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/utils/ptr"
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

const (
	shardingTLSCAKey   = "ca.crt"
	shardingTLSCertKey = "tls.crt"
	shardingTLSKeyKey  = "tls.key"
)

func (t *clusterShardingTLSTransformer) Transform(ctx graph.TransformContext, dag *graph.DAG) error {
	transCtx, _ := ctx.(*clusterTransformContext)
	if transCtx.OrigCluster.IsDeleting() {
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
		sharedTemplates := t.sharedShardTemplates(transCtx, sharding)
		if sharedTemplates.Len() == 0 {
			continue
		}
		if err := t.reconcileShardingTLS(transCtx, graphCli, dag, sharding, sharedTemplates); err != nil {
			return err
		}
	}
	return nil
}

func (t *clusterShardingTLSTransformer) sharedShardTemplates(
	transCtx *clusterTransformContext, sharding *appsv1.ClusterSharding) sets.Set[string] {
	shared := func(shardingDefName string) bool {
		shardingDef, ok := transCtx.shardingDefs[shardingDefName]
		if !ok || shardingDef.Spec.TLS == nil {
			return false
		}
		return ptr.Deref(shardingDef.Spec.TLS.Shared, false)
	}

	templates := sets.New[string]()
	activeTemplates := transCtx.shardingCompsWithTpl[sharding.Name]
	if _, active := activeTemplates[""]; active && shared(sharding.ShardingDef) {
		templates.Insert("") // the default shard template
	}
	for _, shardTemplate := range sharding.ShardTemplates {
		_, active := activeTemplates[shardTemplate.Name]
		if active && shared(ptr.Deref(shardTemplate.ShardingDef, sharding.ShardingDef)) {
			templates.Insert(shardTemplate.Name)
		}
	}
	return templates
}

func (t *clusterShardingTLSTransformer) reconcileShardingTLS(transCtx *clusterTransformContext,
	graphCli model.GraphClient, dag *graph.DAG, sharding *appsv1.ClusterSharding,
	sharedTemplates sets.Set[string]) error {
	if !sharding.Template.TLS {
		return nil
	}
	if sharding.Template.Issuer == nil {
		return fmt.Errorf("issuer shouldn't be nil when tls enabled")
	}
	if sharding.Template.Issuer.Name == appsv1.IssuerUserProvided {
		return nil // all components will share the same secret
	}
	if sharding.Template.Issuer.Name != appsv1.IssuerKubeBlocks {
		return fmt.Errorf("unsupported TLS issuer %q", sharding.Template.Issuer.Name)
	}

	if err := t.validateComponentDefinitions(transCtx, sharding, sharedTemplates); err != nil {
		return err
	}

	secret, err := t.checkTLSSecret(transCtx, sharding)
	if err != nil {
		return err
	}

	if secret == nil {
		obj, err1 := t.buildTLSSecret(transCtx, sharding)
		if err1 != nil {
			return err1
		}
		graphCli.Create(dag, obj)
	} else {
		proto := t.newTLSSecret(transCtx, sharding)
		secretCopy := secret.DeepCopy()
		secretCopy.Labels = proto.Labels
		secretCopy.Annotations = proto.Annotations
		if !reflect.DeepEqual(secret, secretCopy) {
			graphCli.Update(dag, secret, secretCopy)
		}
	}

	t.rewriteTLSConfig(transCtx, sharding, sharedTemplates)

	return nil
}

func (t *clusterShardingTLSTransformer) validateComponentDefinitions(transCtx *clusterTransformContext,
	sharding *appsv1.ClusterSharding, sharedTemplates sets.Set[string]) error {
	componentDefName := func(templateName string) string {
		if templateName == "" {
			return sharding.Template.ComponentDef
		}
		for _, shardTemplate := range sharding.ShardTemplates {
			if shardTemplate.Name == templateName {
				return ptr.Deref(shardTemplate.CompDef, sharding.Template.ComponentDef)
			}
		}
		return ""
	}

	for templateName := range sharedTemplates {
		name := componentDefName(templateName)
		compDef, ok := transCtx.componentDefs[name]
		if !ok || compDef == nil {
			return fmt.Errorf("component definition %q not found for shard template %q of sharding %q",
				name, templateName, sharding.Name)
		}
		if compDef.Spec.TLS == nil {
			return fmt.Errorf("TLS is enabled but component definition %q doesn't support it", compDef.Name)
		}
	}
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
	if apierrors.IsNotFound(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	managedLabels := constant.GetClusterLabels(cluster.Name, map[string]string{
		constant.KBAppShardingNameLabelKey: sharding.Name,
	})
	for key, value := range managedLabels {
		if secret.Labels[key] != value {
			return nil, fmt.Errorf("secret %s/%s already exists but is not managed by sharding %s",
				secret.Namespace, secret.Name, sharding.Name)
		}
	}
	return secret, nil
}

func (t *clusterShardingTLSTransformer) buildTLSSecret(transCtx *clusterTransformContext,
	sharding *appsv1.ClusterSharding) (*corev1.Secret, error) {
	synthesizedComp := component.SynthesizedComponent{
		Namespace:   transCtx.Cluster.Namespace,
		ClusterName: transCtx.Cluster.Name,
		Name:        sharding.Name,
	}
	secret := t.newTLSSecret(transCtx, sharding)
	caFile, certFile, keyFile := shardingTLSCAKey, shardingTLSCertKey, shardingTLSKeyKey
	keys := plan.TLSSecretKeys{CA: &caFile, Cert: &certFile, Key: &keyFile}
	return plan.ComposeTLSCertsWithSecret(keys, synthesizedComp, secret)
}

func (t *clusterShardingTLSTransformer) newTLSSecret(transCtx *clusterTransformContext,
	sharding *appsv1.ClusterSharding) *corev1.Secret {
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
		AddLabelsInMap(sharding.Template.Labels).
		AddLabelsInMap(constant.GetClusterLabels(clusterName, shardingLabels)).
		AddAnnotationsInMap(sharding.Template.Annotations).
		SetData(map[string][]byte{}).
		GetObject()
}

func (t *clusterShardingTLSTransformer) rewriteTLSConfig(
	transCtx *clusterTransformContext, sharding *appsv1.ClusterSharding, sharedTemplates sets.Set[string]) {
	newIssuer := func() *appsv1.Issuer {
		return &appsv1.Issuer{
			Name: appsv1.IssuerUserProvided,
			SecretRef: &appsv1.TLSSecretRef{
				Namespace: transCtx.Cluster.Namespace,
				Name:      shardingTLSSecretName(transCtx.Cluster.Name, sharding.Name),
				CA:        shardingTLSCAKey,
				Cert:      shardingTLSCertKey,
				Key:       shardingTLSKeyKey,
			},
		}
	}

	// Normalization expands a sharding into component specs before this transformer
	// runs. Rewrite only the expanded specs whose effective ShardingDefinition
	// enables sharing. shardingCompsWithTpl and shardingComps contain the same
	// component pointers, so updating the grouped view also updates the flat view.
	// Keep the Cluster spec unchanged so template-specific opt-outs continue to
	// inherit the user's original KubeBlocks issuer.
	for templateName, comps := range transCtx.shardingCompsWithTpl[sharding.Name] {
		if !sharedTemplates.Has(templateName) {
			continue
		}
		for _, comp := range comps {
			comp.Issuer = newIssuer()
		}
	}
}

func shardingTLSSecretName(cluster, sharding string) string {
	return fmt.Sprintf("%s-%s-tls", cluster, sharding)
}
