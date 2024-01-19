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
	"errors"
	"fmt"

	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/graph"
)

// clusterLoadRefResourcesTransformer loads and validates referenced resources (cd & cv).
type clusterLoadRefResourcesTransformer struct{}

var _ graph.Transformer = &clusterLoadRefResourcesTransformer{}

func (t *clusterLoadRefResourcesTransformer) Transform(ctx graph.TransformContext, dag *graph.DAG) error {
	transCtx, _ := ctx.(*clusterTransformContext)
	cluster := transCtx.Cluster

	var err error
	defer func() {
		setProvisioningStartedCondition(&cluster.Status.Conditions, cluster.Name, cluster.Generation, err)
	}()

	allCompSpecs := t.allCompSpecs(cluster)
	allCompDefRefs := t.allCompDefRefs(cluster)
	if len(cluster.Spec.ClusterDefRef) == 0 && len(allCompDefRefs) != len(allCompSpecs) {
		return newRequeueError(requeueDuration, "either cluster definition or component definition should be provided")
	}
	if len(allCompDefRefs) != 0 && len(allCompDefRefs) != len(allCompSpecs) {
		return newRequeueError(requeueDuration, "two kinds of definitions cannot be used together")
	}

	validateExistence := func(key client.ObjectKey, object client.Object) error {
		err = transCtx.Client.Get(transCtx.Context, key, object)
		if err != nil {
			return newRequeueError(requeueDuration, err.Error())
		}
		return nil
	}

	// validate cd & cv's existence
	// if we can't get the referenced cd & cv, set provisioning condition failed, and jump to plan.Execute()
	var (
		cd *appsv1alpha1.ClusterDefinition
		cv *appsv1alpha1.ClusterVersion
	)
	if len(cluster.Spec.ClusterDefRef) > 0 {
		cd = &appsv1alpha1.ClusterDefinition{}
		if err = validateExistence(types.NamespacedName{Name: cluster.Spec.ClusterDefRef}, cd); err != nil {
			return err
		}
	}
	if len(cluster.Spec.ClusterVersionRef) > 0 {
		cv = &appsv1alpha1.ClusterVersion{}
		if err = validateExistence(types.NamespacedName{Name: cluster.Spec.ClusterVersionRef}, cv); err != nil {
			return err
		}
	}

	// validate cd & cv's availability
	// if wrong phase, set provisioning condition failed, and jump to plan.Execute()
	if (cd != nil && cd.Status.Phase != appsv1alpha1.AvailablePhase) || (cv != nil && cv.Status.Phase != appsv1alpha1.AvailablePhase) {
		message := fmt.Sprintf("ref resource is unavailable, this problem needs to be solved first. cd: %s", cd.Name)
		if cv != nil {
			message = fmt.Sprintf("%s, cv: %s", message, cv.Name)
		}
		err = errors.New(message)
		return newRequeueError(requeueDuration, message)
	}

	// inject cd & cv into the shared ctx
	transCtx.ClusterDef = cd
	if cd == nil {
		transCtx.ClusterDef = &appsv1alpha1.ClusterDefinition{}
	}
	transCtx.ClusterVer = cv
	if cv == nil {
		transCtx.ClusterVer = &appsv1alpha1.ClusterVersion{}
	}

	if err = t.loadAndCheckComponentDefinitions(transCtx, cluster); err != nil {
		return newRequeueError(requeueDuration, err.Error())
	}

	return nil
}

func (t *clusterLoadRefResourcesTransformer) allCompDefRefs(cluster *appsv1alpha1.Cluster) []string {
	refs := make([]string, 0)
	for _, compSpec := range cluster.Spec.ComponentSpecs {
		if len(compSpec.ComponentDef) == 0 {
			continue
		}
		refs = append(refs, compSpec.ComponentDef)
	}
	for _, shardingSpec := range cluster.Spec.ShardingSpecs {
		if len(shardingSpec.Template.ComponentDef) == 0 {
			continue
		}
		for i := 0; i < int(shardingSpec.Shards); i++ {
			refs = append(refs, shardingSpec.Template.ComponentDef)
		}
	}
	return refs
}

func (t *clusterLoadRefResourcesTransformer) allCompSpecs(cluster *appsv1alpha1.Cluster) []string {
	specs := make([]string, 0)
	for _, compSpec := range cluster.Spec.ComponentSpecs {
		specs = append(specs, compSpec.Name)
	}
	for _, shardingSpec := range cluster.Spec.ShardingSpecs {
		for i := 0; i < int(shardingSpec.Shards); i++ {
			shardName := constant.GenerateShardName(shardingSpec.Name, i)
			specs = append(specs, shardName)
		}
	}
	return specs
}

func (t *clusterLoadRefResourcesTransformer) loadAndCheckComponentDefinitions(
	ctx *clusterTransformContext, cluster *appsv1alpha1.Cluster) error {

	if ctx.ComponentDefs == nil {
		ctx.ComponentDefs = make(map[string]*appsv1alpha1.ComponentDefinition)
	}

	loadAndCheck := func(compDefName string) error {
		if len(compDefName) == 0 {
			return nil
		}
		if _, ok := ctx.ComponentDefs[compDefName]; ok {
			return nil
		}
		compDef := &appsv1alpha1.ComponentDefinition{}
		if err := ctx.Client.Get(ctx.Context, types.NamespacedName{Name: compDefName}, compDef); err != nil {
			return err
		}
		if compDef.Status.Phase != appsv1alpha1.AvailablePhase {
			return fmt.Errorf("the component definition referenced is unavailable: %s", compDefName)
		}
		ctx.ComponentDefs[compDefName] = compDef
		return nil
	}

	for _, compSpec := range cluster.Spec.ComponentSpecs {
		if err := loadAndCheck(compSpec.ComponentDef); err != nil {
			return err
		}
	}

	for _, shardingSpec := range cluster.Spec.ShardingSpecs {
		if err := loadAndCheck(shardingSpec.Template.ComponentDef); err != nil {
			return err
		}
	}

	return nil
}
