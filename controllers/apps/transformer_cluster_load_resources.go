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
	"fmt"

	"k8s.io/apimachinery/pkg/types"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/controller/apiconversion"
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

	if t.apiValidation(cluster); err != nil {
		return newRequeueError(requeueDuration, err.Error())
	}

	if err = t.loadNCheckClusterDefinition(transCtx, cluster); err != nil {
		return newRequeueError(requeueDuration, err.Error())
	}

	if err = t.loadNCheckClusterVersion(transCtx, cluster); err != nil {
		return newRequeueError(requeueDuration, err.Error())
	}

	if withClusterTopology(cluster) {
		// check again with cluster definition loaded
		if validateClusterTopology(transCtx.ClusterDef, cluster); err != nil {
			return newRequeueError(requeueDuration, err.Error())
		}
	}
	return nil
}

func (t *clusterLoadRefResourcesTransformer) apiValidation(cluster *appsv1alpha1.Cluster) error {
	if withClusterTopology(cluster) {
		return nil
	}
	if legacyClusterDef(cluster) {
		return nil
	}
	if apiconversion.HasSimplifiedClusterAPI(cluster) {
		return nil
	}
	return fmt.Errorf("cluster API validate error, clusterDef: %s, topology: %s, comps: %d, legacy comps: %d, new comps: %d, simplified API: %v",
		cluster.Spec.ClusterDefRef, cluster.Spec.Topology, compCnt(cluster), legacyCompCnt(cluster), newCompCnt(cluster), apiconversion.HasSimplifiedClusterAPI(cluster))
}

func (t *clusterLoadRefResourcesTransformer) loadNCheckClusterDefinition(transCtx *clusterTransformContext, cluster *appsv1alpha1.Cluster) error {
	var cd *appsv1alpha1.ClusterDefinition
	if len(cluster.Spec.ClusterDefRef) > 0 {
		cd = &appsv1alpha1.ClusterDefinition{}
		key := types.NamespacedName{Name: cluster.Spec.ClusterDefRef}
		if err := transCtx.Client.Get(transCtx.Context, key, cd); err != nil {
			return err
		}
	}

	if cd != nil && cd.Status.Phase != appsv1alpha1.AvailablePhase {
		return fmt.Errorf("referred ClusterDefinition is unavailable: %s", cd.Name)
	}

	if cd == nil {
		cd = &appsv1alpha1.ClusterDefinition{}
	}
	transCtx.ClusterDef = cd
	return nil
}

func (t *clusterLoadRefResourcesTransformer) loadNCheckClusterVersion(transCtx *clusterTransformContext, cluster *appsv1alpha1.Cluster) error {
	var cv *appsv1alpha1.ClusterVersion
	if len(cluster.Spec.ClusterVersionRef) > 0 {
		cv = &appsv1alpha1.ClusterVersion{}
		key := types.NamespacedName{Name: cluster.Spec.ClusterVersionRef}
		if err := transCtx.Client.Get(transCtx.Context, key, cv); err != nil {
			return err
		}
	}

	if cv != nil && cv.Status.Phase != appsv1alpha1.AvailablePhase {
		return fmt.Errorf("referred ClusterVersion is unavailable: %s", cv.Name)
	}

	if cv == nil {
		cv = &appsv1alpha1.ClusterVersion{}
	}
	transCtx.ClusterVer = cv
	return nil
}

func validateClusterTopology(clusterDef *appsv1alpha1.ClusterDefinition, cluster *appsv1alpha1.Cluster) error {
	clusterTopology := referredClusterTopology(clusterDef, cluster.Spec.Topology)
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
	return nil
}

func withClusterTopology(cluster *appsv1alpha1.Cluster) bool {
	return len(cluster.Spec.ClusterDefRef) > 0 && compCnt(cluster) == newCompCnt(cluster)
}

func legacyClusterDef(cluster *appsv1alpha1.Cluster) bool {
	return len(cluster.Spec.ClusterDefRef) > 0 && len(cluster.Spec.Topology) == 0 && compCnt(cluster) == legacyCompCnt(cluster)
}

func compCnt(cluster *appsv1alpha1.Cluster) int {
	return len(cluster.Spec.ComponentSpecs)
}

func legacyCompCnt(cluster *appsv1alpha1.Cluster) int {
	cnt := 0
	for _, comp := range cluster.Spec.ComponentSpecs {
		if len(comp.ComponentDef) == 0 {
			cnt += 1
		}
	}
	return cnt
}

func newCompCnt(cluster *appsv1alpha1.Cluster) int {
	cnt := 0
	for _, comp := range cluster.Spec.ComponentSpecs {
		if len(comp.ComponentDef) != 0 {
			cnt += 1
		}
	}
	return cnt
}
