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

	"k8s.io/apimachinery/pkg/types"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/controller/apiconversion"
	"github.com/apecloud/kubeblocks/pkg/controller/graph"
	"github.com/apecloud/kubeblocks/pkg/generics"
)

// clusterLoadRefResourcesTransformer loads and validates referenced resources (cd & cv).
type clusterLoadRefResourcesTransformer struct{}

var _ graph.Transformer = &clusterLoadRefResourcesTransformer{}

func (t *clusterLoadRefResourcesTransformer) Transform(ctx graph.TransformContext, dag *graph.DAG) error {
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

	if err = loadNCheckClusterVersion(transCtx, cluster); err != nil {
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

func (t *clusterLoadRefResourcesTransformer) apiValidation(cluster *appsv1alpha1.Cluster) error {
	if withClusterTopology(cluster) ||
		withClusterUserDefined(cluster) ||
		withClusterLegacyDefinition(cluster) ||
		withClusterSimplifiedAPI(cluster) {
		return nil
	}
	return fmt.Errorf("cluster API validate error, clusterDef: %s, topology: %s, comps: %d, legacy comps: %d, simplified API: %v",
		cluster.Spec.ClusterDefRef, cluster.Spec.Topology, clusterCompCnt(cluster), legacyClusterCompCnt(cluster), withClusterSimplifiedAPI(cluster))
}

func (t *clusterLoadRefResourcesTransformer) checkNUpdateClusterTopology(transCtx *clusterTransformContext, cluster *appsv1alpha1.Cluster) error {
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

func loadNCheckClusterDefinition(transCtx *clusterTransformContext, cluster *appsv1alpha1.Cluster) error {
	var cd *appsv1alpha1.ClusterDefinition
	if len(cluster.Spec.ClusterDefRef) > 0 {
		cd = &appsv1alpha1.ClusterDefinition{}
		key := types.NamespacedName{Name: cluster.Spec.ClusterDefRef}
		if err := transCtx.Client.Get(transCtx.Context, key, cd); err != nil {
			return err
		}
	}

	if cd != nil {
		if cd.Generation != cd.Status.ObservedGeneration {
			return fmt.Errorf("the referenced ClusterDefinition is not up to date: %s", cd.Name)
		}
		if cd.Status.Phase != appsv1alpha1.AvailablePhase {
			return fmt.Errorf("the referenced ClusterDefinition is unavailable: %s", cd.Name)
		}
	}

	if cd == nil {
		cd = &appsv1alpha1.ClusterDefinition{}
	}
	transCtx.ClusterDef = cd
	return nil
}

func loadNCheckClusterVersion(transCtx *clusterTransformContext, cluster *appsv1alpha1.Cluster) error {
	var cv *appsv1alpha1.ClusterVersion
	if len(cluster.Spec.ClusterVersionRef) > 0 {
		cv = &appsv1alpha1.ClusterVersion{}
		key := types.NamespacedName{Name: cluster.Spec.ClusterVersionRef}
		if err := transCtx.Client.Get(transCtx.Context, key, cv); err != nil {
			return err
		}
	}

	if cv != nil {
		if cv.Generation != cv.Status.ObservedGeneration {
			return fmt.Errorf("the referenced ClusterVersion is not up to date: %s", cv.Name)
		}
		if cv.Status.Phase != appsv1alpha1.AvailablePhase {
			return fmt.Errorf("the referenced ClusterVersion is unavailable: %s", cv.Name)
		}
	}

	if cv == nil {
		cv = &appsv1alpha1.ClusterVersion{}
	}
	transCtx.ClusterVer = cv
	return nil
}

func withClusterTopology(cluster *appsv1alpha1.Cluster) bool {
	return len(cluster.Spec.ClusterDefRef) > 0 && legacyClusterCompCnt(cluster) == 0 && !compatibleUserDefinedInNewAPI(cluster)
}

func withClusterUserDefined(cluster *appsv1alpha1.Cluster) bool {
	return (len(cluster.Spec.ClusterDefRef) == 0 && len(cluster.Spec.Topology) == 0 && legacyClusterCompCnt(cluster) == 0) ||
		compatibleUserDefinedInNewAPI(cluster)
}

func withClusterLegacyDefinition(cluster *appsv1alpha1.Cluster) bool {
	return len(cluster.Spec.ClusterDefRef) > 0 && len(cluster.Spec.Topology) == 0 && clusterCompCnt(cluster) == legacyClusterCompCnt(cluster)
}

func withClusterSimplifiedAPI(cluster *appsv1alpha1.Cluster) bool {
	return apiconversion.HasSimplifiedClusterAPI(cluster)
}

func clusterCompCnt(cluster *appsv1alpha1.Cluster) int {
	return clusterCompCntWithFunc(cluster, func(spec appsv1alpha1.ClusterComponentSpec) bool { return true })
}

func legacyClusterCompCnt(cluster *appsv1alpha1.Cluster) int {
	isLegacyComp := func(spec appsv1alpha1.ClusterComponentSpec) bool {
		return len(spec.ComponentDefRef) != 0 && len(spec.ComponentDef) == 0
	}
	return clusterCompCntWithFunc(cluster, isLegacyComp)
}

func hasLegacyClusterCompSet(cluster *appsv1alpha1.Cluster) bool {
	hasLegacyCompSet := func(spec appsv1alpha1.ClusterComponentSpec) bool {
		return len(spec.ComponentDefRef) != 0
	}
	return clusterCompCntWithFunc(cluster, hasLegacyCompSet) > 0
}

func clusterCompCntWithFunc(cluster *appsv1alpha1.Cluster, match func(spec appsv1alpha1.ClusterComponentSpec) bool) int {
	cnt := generics.CountFunc(cluster.Spec.ComponentSpecs, match)
	for _, sharding := range cluster.Spec.ShardingSpecs {
		if match(sharding.Template) {
			cnt += int(sharding.Shards)
		}
	}
	return cnt
}

func compatibleUserDefinedInNewAPI(cluster *appsv1alpha1.Cluster) bool {
	// clusterDefinitionRef = xxxxx, componentDefRef = abc, componentDef = xyz
	return len(cluster.Spec.ClusterDefRef) > 0 && len(cluster.Spec.Topology) == 0 && legacyClusterCompCnt(cluster) == 0 && hasLegacyClusterCompSet(cluster)
}
