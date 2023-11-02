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
	"reflect"
	"strings"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	"github.com/apecloud/kubeblocks/pkg/controller/graph"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
	ictrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

// ClusterComponentTransformer transforms all cluster.Spec.ComponentSpecs to mapping Component objects
type ClusterComponentTransformer struct{}

var _ graph.Transformer = &ClusterComponentTransformer{}

func (t *ClusterComponentTransformer) Transform(ctx graph.TransformContext, dag *graph.DAG) error {
	transCtx, _ := ctx.(*clusterTransformContext)
	if model.IsObjectDeleting(transCtx.OrigCluster) {
		return nil
	}

	// has no components defined
	if len(transCtx.ComponentSpecs) == 0 {
		return nil
	}
	if transCtx.OrigCluster.IsUpdating() {
		return t.reconcileComponents(transCtx, dag)
	}
	return t.reconcileComponentsStatus(transCtx, dag)
}

func (t *ClusterComponentTransformer) reconcileComponents(transCtx *clusterTransformContext, dag *graph.DAG) error {
	cluster := transCtx.Cluster

	protoCompSpecMap := make(map[string]*appsv1alpha1.ClusterComponentSpec)
	for _, spec := range transCtx.ComponentSpecs {
		protoCompSpecMap[spec.Name] = spec
	}

	protoCompSet := sets.KeySet(protoCompSpecMap)
	// TODO(refactor): should review that whether it is reasonable to use component status
	clusterStatusCompSet := sets.KeySet(cluster.Status.Components)

	createCompSet := protoCompSet.Difference(clusterStatusCompSet)
	updateCompSet := protoCompSet.Intersection(clusterStatusCompSet)
	deleteCompSet := clusterStatusCompSet.Difference(protoCompSet)
	if len(deleteCompSet) > 0 {
		return fmt.Errorf("cluster components cannot be removed at runtime: %s",
			strings.Join(deleteCompSet.UnsortedList(), ","))
	}

	// component objects to be created
	if err := t.handleCompsCreate(transCtx, dag, protoCompSpecMap, createCompSet); err != nil {
		return err
	}

	// component objects to be updated
	if err := t.handleCompsUpdate(transCtx, dag, protoCompSpecMap, updateCompSet); err != nil {
		return err
	}

	return nil
}

func (t *ClusterComponentTransformer) handleCompsCreate(transCtx *clusterTransformContext, dag *graph.DAG,
	protoCompSpecMap map[string]*appsv1alpha1.ClusterComponentSpec, createCompSet sets.Set[string]) error {
	cluster := transCtx.Cluster
	graphCli, _ := transCtx.Client.(model.GraphClient)
	for compName := range createCompSet {
		comp, err := component.BuildProtoComponent(cluster, protoCompSpecMap[compName])
		if err != nil {
			return err
		}
		graphCli.Create(dag, comp)
		t.initClusterCompStatus(cluster, compName)
	}
	return nil
}

func (t *ClusterComponentTransformer) initClusterCompStatus(cluster *appsv1alpha1.Cluster, compName string) {
	if cluster.Status.Components == nil {
		cluster.Status.Components = make(map[string]appsv1alpha1.ClusterComponentStatus)
	}
	cluster.Status.Components[compName] = appsv1alpha1.ClusterComponentStatus{}
}

func (t *ClusterComponentTransformer) handleCompsUpdate(transCtx *clusterTransformContext, dag *graph.DAG,
	protoCompSpecMap map[string]*appsv1alpha1.ClusterComponentSpec, updateCompSet sets.Set[string]) error {
	cluster := transCtx.Cluster
	graphCli, _ := transCtx.Client.(model.GraphClient)
	for compName := range updateCompSet {
		runningComp, getErr := getRunningCompObject(transCtx, cluster, compName)
		if getErr != nil && !apierrors.IsNotFound(getErr) {
			return getErr
		}
		comp, buildErr := component.BuildProtoComponent(cluster, protoCompSpecMap[compName])
		if buildErr != nil {
			return buildErr
		}
		if getErr != nil { // non-exist
			// to be backwards compatible with old API versions, for components that are already running but don't have a component CR, component CR needs to be generated.
			graphCli.Create(dag, comp)
		} else {
			if newCompObj := copyAndMergeComponent(runningComp, comp); newCompObj != nil {
				graphCli.Update(dag, runningComp, newCompObj)
			}
		}
	}
	return nil
}

func (t *ClusterComponentTransformer) reconcileComponentsStatus(transCtx *clusterTransformContext, dag *graph.DAG) error {
	cluster := transCtx.Cluster
	if cluster.Status.Components == nil {
		cluster.Status.Components = make(map[string]appsv1alpha1.ClusterComponentStatus)
	}
	// We cannot use cluster.status.components because of simplified API generated component is not in it.
	for _, compSpec := range transCtx.ComponentSpecs {
		compKey := types.NamespacedName{
			Namespace: cluster.Namespace,
			Name:      component.FullName(cluster.Name, compSpec.Name),
		}
		comp := &appsv1alpha1.Component{}
		if err := transCtx.Client.Get(transCtx.Context, compKey, comp); err != nil {
			return err
		}
		status := t.buildClusterCompStatus(comp)
		if len(status.Phase) == 0 {
			continue
		}
		cluster.Status.Components[compSpec.Name] = status
	}
	return nil
}

// buildClusterCompStatus builds cluster component status from specified component object.
func (t *ClusterComponentTransformer) buildClusterCompStatus(comp *appsv1alpha1.Component) appsv1alpha1.ClusterComponentStatus {
	// TODO(component): conditions & roles(?)
	return appsv1alpha1.ClusterComponentStatus{
		Phase:   comp.Status.Phase,
		Message: comp.Status.Message,
	}
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
	ictrlutil.MergeMetadataMap(compObjCopy.Annotations, &compProto.Annotations)
	ictrlutil.MergeMetadataMap(compObjCopy.Labels, &compProto.Labels)
	compObjCopy.Annotations = compProto.Annotations
	compObjCopy.Labels = compProto.Labels

	// merge spec
	compObjCopy.Spec.Monitor = compProto.Spec.Monitor
	compObjCopy.Spec.Resources = compProto.Spec.Resources
	compObjCopy.Spec.ServiceRefs = compProto.Spec.ServiceRefs
	compObjCopy.Spec.Replicas = compProto.Spec.Replicas
	compObjCopy.Spec.Configs = compProto.Spec.Configs
	compObjCopy.Spec.EnabledLogs = compProto.Spec.EnabledLogs
	compObjCopy.Spec.VolumeClaimTemplates = compProto.Spec.VolumeClaimTemplates
	compObjCopy.Spec.UpdateStrategy = compProto.Spec.UpdateStrategy
	compObjCopy.Spec.ServiceAccountName = compProto.Spec.ServiceAccountName
	compObjCopy.Spec.Affinity = compProto.Spec.Affinity
	compObjCopy.Spec.Tolerations = compProto.Spec.Tolerations
	compObjCopy.Spec.TLSConfig = compProto.Spec.TLSConfig

	if reflect.DeepEqual(oldCompObj.Annotations, compObjCopy.Annotations) &&
		reflect.DeepEqual(oldCompObj.Labels, compObjCopy.Labels) &&
		reflect.DeepEqual(oldCompObj.Spec, compObjCopy.Spec) {
		return nil
	}
	return compObjCopy
}
