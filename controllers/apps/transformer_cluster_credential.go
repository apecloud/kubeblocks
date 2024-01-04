/*
Copyright (C) 2022-2023 ApeCloud Co., Ltd

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
	"github.com/apecloud/kubeblocks/pkg/common"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/apecloud/kubeblocks/pkg/controller/component"
	"github.com/apecloud/kubeblocks/pkg/controller/factory"
	"github.com/apecloud/kubeblocks/pkg/controller/graph"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
)

// clusterCredentialTransformer creates the default cluster connection credential secret
type clusterConnCredentialTransformer struct{}

var _ graph.Transformer = &clusterConnCredentialTransformer{}

func (t *clusterConnCredentialTransformer) Transform(ctx graph.TransformContext, dag *graph.DAG) error {
	transCtx, _ := ctx.(*clusterTransformContext)
	if model.IsObjectDeleting(transCtx.OrigCluster) {
		return nil
	}
	if common.IsCompactMode(transCtx.OrigCluster.Annotations) {
		transCtx.V(1).Info("Cluster is in compact mode, no need to create accounts related secrets", "cluster", client.ObjectKeyFromObject(transCtx.OrigCluster))
		return nil
	}

	if !t.isLegacyCluster(transCtx) {
		return nil
	}
	return t.buildClusterConnCredential(transCtx, dag)
}

func (t *clusterConnCredentialTransformer) isLegacyCluster(transCtx *clusterTransformContext) bool {
	for _, comp := range transCtx.ComponentSpecs {
		compDef, ok := transCtx.ComponentDefs[comp.ComponentDef]
		if ok && (len(compDef.UID) > 0 || !compDef.CreationTimestamp.IsZero()) {
			return false
		}
	}
	return true
}

func (t *clusterConnCredentialTransformer) buildClusterConnCredential(transCtx *clusterTransformContext, dag *graph.DAG) error {
	graphCli, _ := transCtx.Client.(model.GraphClient)
	synthesizedComponent := t.buildSynthesizedComponent(transCtx)
	if synthesizedComponent == nil {
		return nil
	}
	secret := factory.BuildConnCredential(transCtx.ClusterDef, transCtx.Cluster, synthesizedComponent)
	if secret == nil {
		return nil
	}
	err := transCtx.Client.Get(transCtx.Context, client.ObjectKeyFromObject(secret), &corev1.Secret{})
	if err != nil && !apierrors.IsNotFound(err) {
		return err
	}
	if apierrors.IsNotFound(err) {
		graphCli.Create(dag, secret)
	}
	return nil
}

func (t *clusterConnCredentialTransformer) buildSynthesizedComponent(transCtx *clusterTransformContext) *component.SynthesizedComponent {
	for _, compDef := range transCtx.ClusterDef.Spec.ComponentDefs {
		if compDef.Service == nil {
			continue
		}
		for _, compSpec := range transCtx.ComponentSpecs {
			if compDef.Name != compSpec.ComponentDefRef {
				continue
			}
			return &component.SynthesizedComponent{
				Name:     compSpec.Name,
				Services: []corev1.Service{{Spec: compDef.Service.ToSVCSpec()}},
			}
		}
	}
	return nil
}
