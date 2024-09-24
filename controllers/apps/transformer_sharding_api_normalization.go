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

	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	"github.com/apecloud/kubeblocks/pkg/controller/graph"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
	"github.com/apecloud/kubeblocks/pkg/controllerutil"
)

// shardingAPINormalizationTransformer handles cluster with sharding topology and component API conversion.
type shardingAPINormalizationTransformer struct{}

var _ graph.Transformer = &shardingAPINormalizationTransformer{}

func (t *shardingAPINormalizationTransformer) Transform(ctx graph.TransformContext, dag *graph.DAG) error {
	transCtx, _ := ctx.(*shardingTransformContext)

	cluster := transCtx.Cluster
	if model.IsObjectDeleting(transCtx.OrigCluster) || len(cluster.Spec.ShardingSpecs) == 0 {
		return nil
	}

	var err error
	defer func() {
		setProvisioningStartedCondition(&cluster.Status.Conditions, cluster.Name, cluster.Generation, err)
	}()

	if err = t.validateSpec(cluster); err != nil {
		return err
	}

	if err = t.validateComponentDef(cluster); err != nil {
		return err
	}

	// build all component specs generated from shardingSpecs
	if err = t.buildShardingCompSpecs(transCtx, cluster); err != nil {
		return err
	}

	// resolve all component definitions referenced
	if err = t.resolveShardingCompDefinitions(transCtx); err != nil {
		return err
	}

	// update the resolved component definitions and service versions to sharding template.
	t.updateShardingTemplate(transCtx)

	return nil
}

func (t *shardingAPINormalizationTransformer) validateSpec(cluster *appsv1.Cluster) error {
	shardCompNameMap := map[string]sets.Empty{}
	for _, v := range cluster.Spec.ShardingSpecs {
		shardCompNameMap[v.Name] = sets.Empty{}
	}
	for _, v := range cluster.Spec.ComponentSpecs {
		if _, ok := shardCompNameMap[v.Name]; ok {
			return fmt.Errorf(`duplicate component name "%s" in spec.shardingSpec`, v.Name)
		}
	}
	return nil
}

func (t *shardingAPINormalizationTransformer) validateComponentDef(cluster *appsv1.Cluster) error {
	for _, shardingSpec := range cluster.Spec.ShardingSpecs {
		if len(shardingSpec.Template.ComponentDef) == 0 {
			continue
		}
		if err := component.ValidateCompDefRegexp(shardingSpec.Template.ComponentDef); err != nil {
			return errors.Wrapf(err, "invalid reference component definition name pattern: %s", shardingSpec.Template.ComponentDef)
		}
	}
	return nil
}

func (t *shardingAPINormalizationTransformer) buildShardingCompSpecs(transCtx *shardingTransformContext,
	cluster *appsv1.Cluster) error {
	for i, sharding := range cluster.Spec.ShardingSpecs {
		shardingComps, err := controllerutil.GenShardingCompSpecList(transCtx.Context, transCtx.Client, cluster, &cluster.Spec.ShardingSpecs[i])
		if err != nil {
			return err
		}
		transCtx.ShardingToComponentSpecs[sharding.Name] = shardingComps
	}
	// TODO: cluster definition topology supports sharding
	return nil
}

func (t *shardingAPINormalizationTransformer) resolveShardingCompDefinitions(transCtx *shardingTransformContext) error {
	if transCtx.ComponentDefs == nil {
		transCtx.ComponentDefs = make(map[string]*appsv1.ComponentDefinition)
	}

	for k, compSpecs := range transCtx.ShardingToComponentSpecs {
		if len(compSpecs) == 0 {
			continue
		}
		// all the components in the same sharding should have the same component definition and service version
		compDef, serviceVersion, err := compSpecResolveCompDefinitionNServiceVersion(transCtx.Context, transCtx.Client, transCtx.Cluster, compSpecs[0])
		if err != nil {
			return err
		}
		// set the componentDef and serviceVersion as resolved
		for i := range compSpecs {
			transCtx.ComponentDefs[compDef.Name] = compDef
			transCtx.ShardingToComponentSpecs[k][i].ComponentDef = compDef.Name
			transCtx.ShardingToComponentSpecs[k][i].ServiceVersion = serviceVersion
		}
	}

	return nil
}

func (t *shardingAPINormalizationTransformer) updateShardingTemplate(transCtx *shardingTransformContext) {
	var (
		cluster = transCtx.Cluster
	)
	for i, sharding := range cluster.Spec.ShardingSpecs {
		for k, v := range transCtx.ShardingToComponentSpecs {
			if sharding.Name == k {
				if len(v) == 0 {
					continue
				}
				cluster.Spec.ShardingSpecs[i].Template.ComponentDef = v[0].ComponentDef
				cluster.Spec.ShardingSpecs[i].Template.ServiceVersion = v[0].ServiceVersion
				break
			}
		}
	}
}

func withShardingDefined(obj client.Object) bool {
	labels := obj.GetLabels()
	return labels != nil && labels[constant.KBAppShardingNameLabelKey] != ""
}

func withoutShardingDefined(obj client.Object) bool {
	return !withShardingDefined(obj)
}
