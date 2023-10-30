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
	"context"
	"fmt"
	"strings"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	"github.com/apecloud/kubeblocks/pkg/controller/graph"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
	ictrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

// ClusterComponentTransformer transforms all cluster.Spec.ComponentSpecs to mapping Component objects
type ClusterComponentTransformer struct {
	client.Client
}

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
	graphCli, _ := transCtx.Client.(model.GraphClient)

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

	createCompObjects := func() error {
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

	updateCompObjects := func() error {
		for compName := range updateCompSet {
			runningComp, err1 := getCacheSnapshotComp(transCtx.Context, t.Client, cluster, compName)
			if err1 != nil && !apierrors.IsNotFound(err1) {
				return err1
			}
			comp, err2 := component.BuildProtoComponent(cluster, protoCompSpecMap[compName])
			if err2 != nil {
				return err2
			}
			if err1 != nil { // non-exist
				// to be backwards compatible with old API versions, for components that are already running but don't have a component CR, component CR needs to be generated.
				graphCli.Create(dag, comp)
			} else {
				graphCli.Update(dag, runningComp, copyAndMergeComponent(runningComp, comp, cluster))
			}
		}
		return nil
	}

	// component objects to be created
	if err := createCompObjects(); err != nil {
		return err
	}

	// component objects to be updated
	if err := updateCompObjects(); err != nil {
		return err
	}

	return nil
}

func (t *ClusterComponentTransformer) reconcileComponentsStatus(transCtx *clusterTransformContext, dag *graph.DAG) error {
	for compName := range transCtx.Cluster.Status.Components {
		comp, err := getCacheSnapshotComp(transCtx.Context, t.Client, transCtx.Cluster, compName)
		if err != nil {
			return err
		}
		status := t.buildClusterCompStatus(comp)
		if len(status.Phase) == 0 {
			continue
		}
		transCtx.Cluster.Status.Components[compName] = status
		fmt.Printf("status - comp %s status %s\n", compName, status.Phase)
	}
	return nil
}

func (t *ClusterComponentTransformer) initClusterCompStatus(cluster *appsv1alpha1.Cluster, compName string) {
	status := cluster.Status.Components
	if status == nil {
		status = make(map[string]appsv1alpha1.ClusterComponentStatus)
	}
	status[compName] = appsv1alpha1.ClusterComponentStatus{
		Phase: appsv1alpha1.CreatingClusterCompPhase,
	}
	fmt.Printf("init - comp %s status %s\n", compName, appsv1alpha1.CreatingClusterCompPhase)
	cluster.Status.Components = status
}

func (t *ClusterComponentTransformer) removeClusterCompStatus(cluster *appsv1alpha1.Cluster, compName string) {
	if cluster.Status.Components != nil {
		delete(cluster.Status.Components, compName)
	}
}

func (t *ClusterComponentTransformer) buildClusterCompStatus(comp *appsv1alpha1.Component) appsv1alpha1.ClusterComponentStatus {
	// TODO(component): conditions & roles(?)
	return appsv1alpha1.ClusterComponentStatus{
		Phase:   comp.Status.Phase,
		Message: comp.Status.Message,
	}
}

// copyAndMergeComponent merges two component objects for updating:
// 1. new a component object targetCompObj by copying from oldCompObj
// 2. merge all fields can be updated from newCompObj into targetCompObj
func copyAndMergeComponent(oldCompObj, newCompObj *appsv1alpha1.Component, cluster *appsv1alpha1.Cluster) *appsv1alpha1.Component {
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

	return compObjCopy
}

// getCacheSnapshotComp gets the component object from cache snapshot
func getCacheSnapshotComp(ctx context.Context, cli client.Client, cluster *appsv1alpha1.Cluster, compName string) (*appsv1alpha1.Component, error) {
	compKey := types.NamespacedName{
		Name:      component.FullName(cluster.Name, compName),
		Namespace: cluster.Namespace,
	}
	comp := &appsv1alpha1.Component{}
	if err := ictrlutil.ValidateExistence(ctx, cli, compKey, comp, false); err != nil {
		return nil, err
	}
	return comp, nil
}
