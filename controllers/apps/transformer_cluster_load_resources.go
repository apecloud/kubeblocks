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
	"k8s.io/apimachinery/pkg/types"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	"github.com/apecloud/kubeblocks/pkg/controller/graph"
	"github.com/apecloud/kubeblocks/pkg/generics"
)

// clusterValidationTransformer validates the cluster spec.
type clusterValidationTransformer struct{}

var _ graph.Transformer = &clusterValidationTransformer{}

func (t *clusterValidationTransformer) Transform(ctx graph.TransformContext, dag *graph.DAG) error {
	transCtx, _ := ctx.(*clusterTransformContext)
	cluster := transCtx.Cluster

	var err error
	defer func() {
		if err != nil {
			setProvisioningStartedCondition(&cluster.Status.Conditions, cluster.Name, cluster.Generation, err)
		}
	}()

	if err = t.apiValidation(cluster); err != nil {
		return newRequeueError(requeueDuration, err.Error())
	}

	if err = loadNCheckClusterDefinition(transCtx, cluster); err != nil {
		return newRequeueError(requeueDuration, err.Error())
	}

	if err = t.checkAllCompDefinition(cluster); err != nil {
		return newRequeueError(requeueDuration, err.Error())
	}

	if withClusterTopology(cluster) {
		// check again with cluster definition loaded,
		// and update topology to cluster spec in case the default topology changed.
		if err = t.checkNUpdateClusterTopology(transCtx, cluster); err != nil {
			return newRequeueError(requeueDuration, err.Error())
		}
	}
	return nil
}

func (t *clusterValidationTransformer) apiValidation(cluster *appsv1.Cluster) error {
	if withClusterTopology(cluster) || withClusterUserDefined(cluster) {
		return nil
	}
	return fmt.Errorf("cluster API validate error, clusterDef: %s, topology: %s, comps: %d",
		cluster.Spec.ClusterDef, cluster.Spec.Topology, clusterCompCnt(cluster))
}

func (t *clusterValidationTransformer) checkAllCompDefinition(cluster *appsv1.Cluster) error {
	validate := func(spec appsv1.ClusterComponentSpec) error {
		if len(spec.ComponentDef) > 0 {
			if err := component.ValidateCompDefRegexp(spec.ComponentDef); err != nil {
				return errors.Wrapf(err, "invalid reference component definition name pattern: %s", spec.ComponentDef)
			}
		}
		return nil
	}
	for _, compSpec := range cluster.Spec.ComponentSpecs {
		if err := validate(compSpec); err != nil {
			return err
		}
	}
	for _, shardingSpec := range cluster.Spec.ShardingSpecs {
		if err := validate(shardingSpec.Template); err != nil {
			return err
		}
	}
	return nil
}

func (t *clusterValidationTransformer) checkNUpdateClusterTopology(transCtx *clusterTransformContext, cluster *appsv1.Cluster) error {
	clusterTopology := referredClusterTopology(transCtx.ClusterDef, cluster.Spec.Topology)
	if clusterTopology == nil {
		return fmt.Errorf("specified cluster topology not found: %s", cluster.Spec.Topology)
	}

	comps := make(map[string]bool, 0)
	for _, comp := range clusterTopology.Components {
		comps[comp.Name] = true
	}
	for _, comp := range cluster.Spec.ComponentSpecs {
		if !comps[comp.Name] {
			return fmt.Errorf("component %s not defined in topology %s", comp.Name, clusterTopology.Name)
		}
	}

	cluster.Spec.Topology = clusterTopology.Name

	return nil
}

func loadNCheckClusterDefinition(transCtx *clusterTransformContext, cluster *appsv1.Cluster) error {
	var cd *appsv1.ClusterDefinition
	if len(cluster.Spec.ClusterDef) > 0 {
		cd = &appsv1.ClusterDefinition{}
		key := types.NamespacedName{Name: cluster.Spec.ClusterDef}
		if err := transCtx.Client.Get(transCtx.Context, key, cd); err != nil {
			return err
		}
	}

	if cd != nil {
		if cd.Generation != cd.Status.ObservedGeneration {
			return fmt.Errorf("the referenced ClusterDefinition is not up to date: %s", cd.Name)
		}
		if cd.Status.Phase != appsv1.AvailablePhase {
			return fmt.Errorf("the referenced ClusterDefinition is unavailable: %s", cd.Name)
		}
	}

	if cd == nil {
		cd = &appsv1.ClusterDefinition{}
	}
	transCtx.ClusterDef = cd
	return nil
}

func withClusterTopology(cluster *appsv1.Cluster) bool {
	return len(cluster.Spec.ClusterDef) > 0
}

func withClusterUserDefined(cluster *appsv1.Cluster) bool {
	hasCompDefSet := func(spec appsv1.ClusterComponentSpec) bool {
		return len(spec.ComponentDef) > 0
	}
	return len(cluster.Spec.ClusterDef) == 0 && len(cluster.Spec.Topology) == 0 &&
		clusterCompCnt(cluster) == clusterCompCntWithFunc(cluster, hasCompDefSet)
}

func clusterCompCnt(cluster *appsv1.Cluster) int {
	return clusterCompCntWithFunc(cluster, func(spec appsv1.ClusterComponentSpec) bool { return true })
}

func clusterCompCntWithFunc(cluster *appsv1.Cluster, match func(spec appsv1.ClusterComponentSpec) bool) int {
	cnt := generics.CountFunc(cluster.Spec.ComponentSpecs, match)
	for _, sharding := range cluster.Spec.ShardingSpecs {
		if match(sharding.Template) {
			cnt += int(sharding.Shards)
		}
	}
	return cnt
}
