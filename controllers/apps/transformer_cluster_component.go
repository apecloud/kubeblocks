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
	"strings"

	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	"github.com/apecloud/kubeblocks/pkg/controller/graph"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
	ictrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

// clusterComponentTransformer transforms all cluster.Spec.ComponentSpecs to mapping Component objects
type clusterComponentTransformer struct{}

var _ graph.Transformer = &clusterComponentTransformer{}

func (t *clusterComponentTransformer) Transform(ctx graph.TransformContext, dag *graph.DAG) error {
	transCtx, _ := ctx.(*clusterTransformContext)
	if model.IsObjectDeleting(transCtx.OrigCluster) {
		return nil
	}

	if len(transCtx.ComponentSpecs) == 0 {
		return nil
	}

	allCompsReady, err := checkAllCompsReady(transCtx, transCtx.Cluster)
	if err != nil {
		return err
	}

	// if all component objects ready and cluster is not updating, skip reconciling components
	if !transCtx.OrigCluster.IsUpdating() && allCompsReady {
		return nil
	}

	return t.reconcileComponents(transCtx, dag)
}

func (t *clusterComponentTransformer) reconcileComponents(transCtx *clusterTransformContext, dag *graph.DAG) error {
	cluster := transCtx.Cluster

	protoCompSpecMap := make(map[string]*appsv1alpha1.ClusterComponentSpec)
	for _, compSpec := range transCtx.ComponentSpecs {
		protoCompSpecMap[compSpec.Name] = compSpec
	}

	protoCompSet := sets.KeySet(protoCompSpecMap)
	runningCompSet, err := component.GetClusterComponentShortNameSet(transCtx.Context, transCtx.Client, cluster)
	if err != nil {
		return err
	}

	createCompSet := protoCompSet.Difference(runningCompSet)
	updateCompSet := protoCompSet.Intersection(runningCompSet)
	deleteCompSet := runningCompSet.Difference(protoCompSet)
	if len(deleteCompSet) > 0 {
		return fmt.Errorf("cluster components cannot be removed at runtime: %s",
			strings.Join(deleteCompSet.UnsortedList(), ","))
	}

	// component objects to be created
	if err := t.handleCompsCreate(transCtx, dag, protoCompSpecMap, createCompSet, transCtx.Labels, transCtx.Annotations); err != nil {
		return err
	}

	// component objects to be updated
	if err := t.handleCompsUpdate(transCtx, dag, protoCompSpecMap, updateCompSet, transCtx.Labels, transCtx.Annotations); err != nil {
		return err
	}

	return nil
}

func (t *clusterComponentTransformer) handleCompsCreate(transCtx *clusterTransformContext, dag *graph.DAG,
	protoCompSpecMap map[string]*appsv1alpha1.ClusterComponentSpec, createCompSet sets.Set[string],
	protoCompLabelsMap, protoCompAnnotationsMap map[string]map[string]string) error {
	cluster := transCtx.Cluster
	graphCli, _ := transCtx.Client.(model.GraphClient)
	for compName := range createCompSet {
		comp, err := component.BuildComponent(cluster, protoCompSpecMap[compName], protoCompLabelsMap[compName], protoCompAnnotationsMap[compName])
		if err != nil {
			return err
		}
		graphCli.Create(dag, comp)
		t.initClusterCompStatus(cluster, compName)
	}
	return nil
}

func (t *clusterComponentTransformer) initClusterCompStatus(cluster *appsv1alpha1.Cluster, compName string) {
	if cluster.Status.Components == nil {
		cluster.Status.Components = make(map[string]appsv1alpha1.ClusterComponentStatus)
	}
	cluster.Status.Components[compName] = appsv1alpha1.ClusterComponentStatus{}
}

func (t *clusterComponentTransformer) handleCompsUpdate(transCtx *clusterTransformContext, dag *graph.DAG,
	protoCompSpecMap map[string]*appsv1alpha1.ClusterComponentSpec, updateCompSet sets.Set[string],
	protoCompLabelsMap, protoCompAnnotationsMap map[string]map[string]string) error {
	cluster := transCtx.Cluster
	graphCli, _ := transCtx.Client.(model.GraphClient)
	for compName := range updateCompSet {
		runningComp, getErr := getRunningCompObject(transCtx, cluster, compName)
		if getErr != nil {
			return getErr
		}
		comp, buildErr := component.BuildComponent(cluster, protoCompSpecMap[compName], protoCompLabelsMap[compName], protoCompAnnotationsMap[compName])
		if buildErr != nil {
			return buildErr
		}
		if newCompObj := copyAndMergeComponent(runningComp, comp); newCompObj != nil {
			graphCli.Update(dag, runningComp, newCompObj)
		}
	}
	return nil
}

func checkAllCompsReady(transCtx *clusterTransformContext, cluster *appsv1alpha1.Cluster) (bool, error) {
	compList := &appsv1alpha1.ComponentList{}
	labels := constant.GetClusterWellKnownLabels(cluster.Name)
	if err := transCtx.Client.List(transCtx.Context, compList, client.InNamespace(cluster.Namespace), client.MatchingLabels(labels)); err != nil {
		return false, err
	}
	if len(compList.Items) != len(transCtx.ComponentSpecs) {
		return false, nil
	}
	return true, nil
}

// getRunningCompObject gets the component object from cache snapshot
func getRunningCompObject(transCtx *clusterTransformContext, cluster *appsv1alpha1.Cluster, compName string) (*appsv1alpha1.Component, error) {
	compKey := types.NamespacedName{
		Namespace: cluster.Namespace,
		Name:      component.FullName(cluster.Name, compName),
	}
	comp := &appsv1alpha1.Component{}
	if err := transCtx.Client.Get(transCtx.Context, compKey, comp); err != nil {
		return nil, err
	}
	return comp, nil
}

// copyAndMergeComponent merges two component objects for updating:
// 1. new a component object targetCompObj by copying from oldCompObj
// 2. merge all fields can be updated from newCompObj into targetCompObj
func copyAndMergeComponent(oldCompObj, newCompObj *appsv1alpha1.Component) *appsv1alpha1.Component {
	compObjCopy := oldCompObj.DeepCopy()
	compProto := newCompObj

	// merge labels and annotations
	ictrlutil.MergeMetadataMapInplace(compObjCopy.Annotations, &compProto.Annotations)
	ictrlutil.MergeMetadataMapInplace(compObjCopy.Labels, &compProto.Labels)
	compObjCopy.Annotations = compProto.Annotations
	compObjCopy.Labels = compProto.Labels

	// merge spec
	compObjCopy.Spec.Monitor = compProto.Spec.Monitor
	compObjCopy.Spec.ClassDefRef = compProto.Spec.ClassDefRef
	compObjCopy.Spec.Resources = compProto.Spec.Resources
	compObjCopy.Spec.ServiceRefs = compProto.Spec.ServiceRefs
	compObjCopy.Spec.Replicas = compProto.Spec.Replicas
	compObjCopy.Spec.Configs = compProto.Spec.Configs
	compObjCopy.Spec.EnabledLogs = compProto.Spec.EnabledLogs
	compObjCopy.Spec.VolumeClaimTemplates = compProto.Spec.VolumeClaimTemplates
	compObjCopy.Spec.ServiceAccountName = compProto.Spec.ServiceAccountName
	compObjCopy.Spec.Affinity = compProto.Spec.Affinity
	compObjCopy.Spec.Tolerations = compProto.Spec.Tolerations
	compObjCopy.Spec.TLSConfig = compProto.Spec.TLSConfig
	compObjCopy.Spec.Nodes = compProto.Spec.Nodes
	compObjCopy.Spec.Instances = compProto.Spec.Instances

	if reflect.DeepEqual(oldCompObj.Annotations, compObjCopy.Annotations) &&
		reflect.DeepEqual(oldCompObj.Labels, compObjCopy.Labels) &&
		reflect.DeepEqual(oldCompObj.Spec, compObjCopy.Spec) {
		return nil
	}
	return compObjCopy
}
