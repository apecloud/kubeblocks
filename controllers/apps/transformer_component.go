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
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	"github.com/apecloud/kubeblocks/pkg/controller/graph"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
	ictrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

// ComponentTransformer transforms all cluster.Spec.ComponentSpecs to mapping Component objects
type ComponentTransformer struct {
	client.Client
}

var _ graph.Transformer = &ComponentTransformer{}

func (c *ComponentTransformer) Transform(ctx graph.TransformContext, dag *graph.DAG) error {
	transCtx, _ := ctx.(*clusterTransformContext)
	cluster := transCtx.Cluster
	origCluster := transCtx.OrigCluster
	graphCli, _ := transCtx.Client.(model.GraphClient)

	reqCtx := ictrlutil.RequestCtx{
		Ctx:      transCtx.Context,
		Log:      transCtx.Logger,
		Recorder: transCtx.EventRecorder,
	}

	if model.IsObjectDeleting(origCluster) {
		return nil
	}

	if cluster.Spec.ComponentSpecs == nil {
		return nil
	}
	protoCompSpecMap := make(map[string]*appsv1alpha1.ClusterComponentSpec)
	for _, spec := range cluster.Spec.ComponentSpecs {
		protoCompSpecMap[spec.Name] = &spec
	}

	protoCompSet := sets.KeySet(protoCompSpecMap)
	// TODO(refactor): should review that whether it is reasonable to use component status
	clusterStatusCompSet := sets.KeySet(cluster.Status.Components)

	createCompSet := protoCompSet.Difference(clusterStatusCompSet)
	updateCompSet := protoCompSet.Intersection(clusterStatusCompSet)
	deleteCompSet := clusterStatusCompSet.Difference(protoCompSet)

	createCompObjects := func() error {
		for compName := range createCompSet {
			protoComp, err := component.BuildProtoComponent(reqCtx, c.Client, cluster, protoCompSpecMap[compName])
			if err != nil {
				return err
			}
			graphCli.Create(dag, protoComp)
		}
		return nil
	}

	updateCompObjects := func() error {
		for compName := range updateCompSet {
			runningComp, err := getCacheSnapshotComp(reqCtx, c.Client, compName, cluster.Namespace)
			if err != nil {
				return err
			}
			protoComp, err := component.BuildProtoComponent(reqCtx, c.Client, cluster, protoCompSpecMap[compName])
			if err != nil {
				return err
			}
			newObj := copyAndMergeComponent(runningComp, protoComp, cluster)
			graphCli.Update(dag, runningComp, newObj)
		}
		return nil
	}

	deleteCompObjects := func() error {
		for compName := range deleteCompSet {
			runningComp, err := getCacheSnapshotComp(reqCtx, c.Client, compName, cluster.Namespace)
			if err != nil {
				return err
			}
			graphCli.Delete(dag, runningComp)
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

	// component objects to be deleted
	if err := deleteCompObjects(); err != nil {
		return err
	}

	return nil
}

// copyAndMergeComponent merges two component objects for updating:
// 1. new a component object targetCompObj by copying from oldCompObj
// 2. merge all fields can be updated from newCompObj into targetCompObj
func copyAndMergeComponent(oldCompObj, newCompObj *appsv1alpha1.Component, cluster *appsv1alpha1.Cluster) *appsv1alpha1.Component {
	compObjCopy := oldCompObj.DeepCopy()
	compProto := newCompObj

	// mergeAnnotations keeps the original annotations.
	mergeMetadataMap := func(originalMap map[string]string, targetMap *map[string]string) {
		if targetMap == nil || originalMap == nil {
			return
		}
		if *targetMap == nil {
			*targetMap = map[string]string{}
		}
		for k, v := range originalMap {
			// if the annotation not exist in targetAnnotations, copy it from original.
			if _, ok := (*targetMap)[k]; !ok {
				(*targetMap)[k] = v
			}
		}
	}

	// merge labels and annotations
	mergeMetadataMap(compObjCopy.Annotations, &compProto.Annotations)
	mergeMetadataMap(compObjCopy.Labels, &compProto.Labels)
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
	compObjCopy.Spec.TLS = compProto.Spec.TLS
	compObjCopy.Spec.Issuer = compProto.Spec.Issuer

	return compObjCopy
}

// getCacheSnapshotComp gets the component object from cache snapshot
func getCacheSnapshotComp(reqCtx ictrlutil.RequestCtx, cli client.Client, compName, namespace string) (*appsv1alpha1.Component, error) {
	runningComp := &appsv1alpha1.Component{}
	if err := ictrlutil.ValidateExistence(reqCtx.Ctx, cli, types.NamespacedName{Name: compName, Namespace: namespace}, runningComp); err != nil {
		return nil, err
	}
	return runningComp, nil
}
