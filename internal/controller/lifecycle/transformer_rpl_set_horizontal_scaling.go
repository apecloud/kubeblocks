/*
Copyright ApeCloud, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package lifecycle

import (
	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/constant"
	"github.com/apecloud/kubeblocks/internal/controller/component"
	appsv1 "k8s.io/api/apps/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/apecloud/kubeblocks/controllers/apps/components/replicationset"
	"github.com/apecloud/kubeblocks/internal/controller/graph"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
)

type rplSetHorizontalScalingTransformer struct {
	cc  compoundCluster
	cli client.Client
	ctx intctrlutil.RequestCtx
}

func (r *rplSetHorizontalScalingTransformer) Transform(dag *graph.DAG) error {
	hasScaling, err := r.hasReplicationSetHScaling()
	if err != nil {
		return err
	}
	if !hasScaling {
		return nil
	}
	vertices, err := findAll[*appsv1.StatefulSet](dag)
	if err != nil {
		return err
	}
	// stsList is used to handle statefulSets horizontal scaling when workloadType is replication
	var stsList []*appsv1.StatefulSet
	for _, vertex := range vertices {
		v, _ := vertex.(*lifecycleVertex)
		stsList = append(stsList, v.obj.(*appsv1.StatefulSet))
	}
	if err := replicationset.HandleReplicationSet(r.ctx.Ctx, r.cli, r.cc.cluster, stsList); err != nil {
		return err
	}

	return nil
}

// TODO: fix stale cache problem
// TODO: if sts created in last reconcile-loop not present in cache, hasReplicationSetHScaling return false positive
func (r *rplSetHorizontalScalingTransformer) hasReplicationSetHScaling() (bool, error) {
	stsList, err := r.listAllStsOwnedByCluster()
	if err != nil {
		return false, err
	}
	if len(stsList) == 0 {
		return false, err
	}

	clusterCompSpecMap := r.cc.cluster.GetDefNameMappingComponents()
	clusterCompVerMap := r.cc.cv.GetDefNameMappingComponents()
	for _, compDef := range r.cc.cd.Spec.ComponentDefs {
		if compDef.WorkloadType != appsv1alpha1.Replication {
			continue
		}
		compDefName := compDef.Name
		compVer := clusterCompVerMap[compDefName]
		compSpecs := clusterCompSpecMap[compDefName]
		for _, compSpec := range compSpecs {
			comp := component.BuildComponent(r.ctx, *r.cc.cluster, r.cc.cd, compDef, compSpec, compVer)
			compSts := filterStsOwnedByComp(stsList, comp.Name)
			if len(compSts) != int(comp.Replicas) {
				return true, nil
			}
		}
	}

	return false, nil
}

func filterStsOwnedByComp(list []appsv1.StatefulSet, compName string) []appsv1.StatefulSet {
	sts := make([]appsv1.StatefulSet, 0)
	for _, s := range list {
		if s.Labels[constant.KBAppComponentLabelKey] == compName {
			sts = append(sts, s)
		}
	}
	return sts
}

func (r *rplSetHorizontalScalingTransformer) listAllStsOwnedByCluster() ([]appsv1.StatefulSet, error) {
	stsList := &appsv1.StatefulSetList{}
	if err := r.cli.List(r.ctx.Ctx, stsList,
		client.MatchingLabels{constant.AppInstanceLabelKey: r.cc.cluster.Name},
		client.InNamespace(r.cc.cluster.Namespace)); err != nil {
		return nil, err
	}
	allSts := make([]appsv1.StatefulSet, 0)
	for _, item := range stsList.Items {
		allSts = append(allSts, item)
	}
	return allSts, nil
}
