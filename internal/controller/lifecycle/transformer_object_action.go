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
	oldSnapshot, err := readCacheSnapshot(transCtx, *origCluster, ownKinds()...)
	if err != nil {
		return err
	}

	rootVertex, err := findRootVertex(dag)
	if err != nil {
		return err
	}

	// we have the target objects snapshot in dag
	newNameVertices := make(map[gvkName]graph.Vertex)
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
