/*
Copyright (C) 2022 ApeCloud Co., Ltd

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

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	dataprotectionv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	client2 "github.com/apecloud/kubeblocks/internal/controller/client"
	"github.com/apecloud/kubeblocks/internal/controller/graph"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
)

// objectActionTransformer reads all Vertex.Obj in cache and compute the diff DAG.
type objectActionTransformer struct {
	cli client2.ReadonlyClient
	ctx intctrlutil.RequestCtx
}

func ownKinds() []client.ObjectList {
	return []client.ObjectList{
		&appsv1.StatefulSetList{},
		&appsv1.DeploymentList{},
		&corev1.ServiceList{},
		&corev1.SecretList{},
		&corev1.ConfigMapList{},
		&policyv1.PodDisruptionBudgetList{},
		&dataprotectionv1alpha1.BackupPolicyList{},
	}
}

func (c *objectActionTransformer) Transform(dag *graph.DAG) error {
	rootVertex, err := findRootVertex(dag)
	if err != nil {
		return err
	}
	origCluster, _ := rootVertex.oriObj.(*appsv1alpha1.Cluster)

	// get the old objects snapshot
	oldSnapshot, err := readCacheSnapshot(c.ctx.Ctx, c.cli, *origCluster, ownKinds()...)
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
