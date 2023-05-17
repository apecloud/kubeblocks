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

package lifecycle

import (
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/sets"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/controller/graph"
)

// ObjectActionTransformer reads all Vertex.Obj in cache and compute the diff DAG.
type ObjectActionTransformer struct{}

func (t *ObjectActionTransformer) Transform(ctx graph.TransformContext, dag *graph.DAG) error {
	transCtx, _ := ctx.(*ClusterTransformContext)
	origCluster := transCtx.OrigCluster

	// get the old objects snapshot
	ml := getAppInstanceAndManagedByML(*origCluster)
	oldSnapshot, err := getClusterOwningObjects(transCtx, *origCluster, ml, ownKinds()...)
	if err != nil {
		return err
	}

	rootVertex, err := findRootVertex(dag)
	if err != nil {
		return err
	}

	// we have the target objects snapshot in dag
	newNameVertices := make(map[gvkNObjKey]graph.Vertex)
	for _, vertex := range dag.Vertices() {
		v, _ := vertex.(*lifecycleVertex)
		if v == rootVertex {
			// ignore root vertex, i.e, cluster object.
			continue
		}
		name, err := getGVKName(v.obj, scheme)
		if err != nil {
			return err
		}
		newNameVertices[*name] = vertex
	}

	// now compute the diff between old and target snapshot and generate the plan
	oldNameSet := sets.KeySet(oldSnapshot)
	newNameSet := sets.KeySet(newNameVertices)

	createSet := newNameSet.Difference(oldNameSet)
	updateSet := newNameSet.Intersection(oldNameSet)
	deleteSet := oldNameSet.Difference(newNameSet)

	createNewVertices := func() {
		for name := range createSet {
			v, _ := newNameVertices[name].(*lifecycleVertex)
			v.action = actionPtr(CREATE)
		}
	}
	updateVertices := func() {
		for name := range updateSet {
			v, _ := newNameVertices[name].(*lifecycleVertex)
			v.oriObj = oldSnapshot[name]
			v.action = actionPtr(UPDATE)
		}
	}

	deleteOrphanVertices := func() {
		for name := range deleteSet {
			v := &lifecycleVertex{
				obj:      oldSnapshot[name],
				oriObj:   oldSnapshot[name],
				isOrphan: true,
				action:   actionPtr(DELETE),
			}
			dag.AddVertex(v)
			dag.Connect(rootVertex, v)
		}
	}

	filterSecretsCreatedBySystemAccountController := func() {
		defaultAccounts := []appsv1alpha1.AccountName{
			appsv1alpha1.AdminAccount,
			appsv1alpha1.DataprotectionAccount,
			appsv1alpha1.ProbeAccount,
			appsv1alpha1.MonitorAccount,
			appsv1alpha1.ReplicatorAccount,
		}
		secretVertices := findAll[*corev1.Secret](dag)
		for _, vertex := range secretVertices {
			v, _ := vertex.(*lifecycleVertex)
			secret, _ := v.obj.(*corev1.Secret)
			for _, account := range defaultAccounts {
				if strings.Contains(secret.Name, string(account)) {
					dag.RemoveVertex(vertex)
					break
				}
			}
		}
	}

	// generate the plan
	switch {
	case isClusterDeleting(*origCluster):
		for _, vertex := range dag.Vertices() {
			v, _ := vertex.(*lifecycleVertex)
			v.action = actionPtr(DELETE)
		}
		deleteOrphanVertices()
	case isClusterStatusUpdating(*origCluster):
		defer func() {
			vertices := findAllNot[*appsv1alpha1.Cluster](dag)
			for _, vertex := range vertices {
				v, _ := vertex.(*lifecycleVertex)
				v.immutable = true
			}
		}()
		fallthrough
	case isClusterUpdating(*origCluster):
		// vertices to be created
		createNewVertices()
		// vertices to be updated
		updateVertices()
		// vertices to be deleted
		deleteOrphanVertices()
		// filter secrets created by system account controller
		filterSecretsCreatedBySystemAccountController()
	}

	return nil
}

var _ graph.Transformer = &ObjectActionTransformer{}
