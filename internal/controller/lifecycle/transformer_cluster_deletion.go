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

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	dataprotectionv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/constant"
	"github.com/apecloud/kubeblocks/internal/controller/graph"
)

// ClusterDeletionTransformer handles cluster deletion
type ClusterDeletionTransformer struct{}

func (t *ClusterDeletionTransformer) Transform(ctx graph.TransformContext, dag *graph.DAG) error {
	transCtx, _ := ctx.(*ClusterTransformContext)
	cluster := transCtx.OrigCluster
	if !isClusterDeleting(*cluster) {
		return nil
	}

	// list all kinds to be deleted based on v1alpha1.TerminationPolicyType
	kinds := make([]client.ObjectList, 0)
	switch cluster.Spec.TerminationPolicy {
	case v1alpha1.DoNotTerminate:
		transCtx.EventRecorder.Eventf(cluster, corev1.EventTypeWarning, "DoNotTerminate", "spec.terminationPolicy %s is preventing deletion.", cluster.Spec.TerminationPolicy)
		return graph.ErrNoops
	case v1alpha1.Halt:
		kinds = kindsForHalt()
	case v1alpha1.Delete:
		kinds = kindsForDelete()
	case v1alpha1.WipeOut:
		kinds = kindsForWipeOut()
	}

	transCtx.EventRecorder.Eventf(cluster, corev1.EventTypeNormal, constant.ReasonDeletingCR, "Deleting %s: %s",
		strings.ToLower(cluster.GetObjectKind().GroupVersionKind().Kind), cluster.GetName())

	// list all objects owned by this cluster in cache, and delete them all
	// there is chance that objects leak occurs because of cache stale
	// ignore the problem currently
	// TODO: GC the leaked objects
	ml := getAppInstanceML(*cluster)
	snapshot, err := readCacheSnapshot(transCtx, *cluster, ml, kinds...)
	if err != nil {
		return err
	}
	root, err := findRootVertex(dag)
	if err != nil {
		return err
	}
	for _, object := range snapshot {
		vertex := &lifecycleVertex{obj: object, action: actionPtr(DELETE)}
		dag.AddVertex(vertex)
		dag.Connect(root, vertex)
	}

	// adjust the dependency resource deletion order
	adjustDependencyResourceDeletionOrder(root, dag)

	root.action = actionPtr(DELETE)

	// fast return, that is stopping the plan.Build() stage and jump to plan.Execute() directly
	return graph.ErrNoops
}

func kindsForDoNotTerminate() []client.ObjectList {
	return []client.ObjectList{}
}

func kindsForHalt() []client.ObjectList {
	kinds := kindsForDoNotTerminate()
	kindsPlus := []client.ObjectList{
		&appsv1.StatefulSetList{},
		&appsv1.DeploymentList{},
		&corev1.ServiceList{},
		&corev1.SecretList{},
		&corev1.ConfigMapList{},
		&policyv1.PodDisruptionBudgetList{},
	}
	return append(kinds, kindsPlus...)
}

func kindsForDelete() []client.ObjectList {
	kinds := kindsForHalt()
	kindsPlus := []client.ObjectList{
		&corev1.PersistentVolumeClaimList{},
		&dataprotectionv1alpha1.BackupPolicyList{},
		&batchv1.JobList{},
	}
	return append(kinds, kindsPlus...)
}

func kindsForWipeOut() []client.ObjectList {
	kinds := kindsForDelete()
	kindsPlus := []client.ObjectList{
		&dataprotectionv1alpha1.BackupList{},
	}
	return append(kinds, kindsPlus...)
}

// adjustDependencyResourceDeletionOrder adjusts the deletion order of resources by adjusting DAG topology.
// find all vertices of StatefulSets and connect them to ConfigMap,
// this is to ensure that ConfigMap is deleted after StatefulSet Workloads are deleted.
func adjustDependencyResourceDeletionOrder(root *lifecycleVertex, dag *graph.DAG) {
	vertices := findAll[*appsv1.StatefulSet](dag)
	cmVertices := findAll[*corev1.ConfigMap](dag)
	if len(vertices) > 0 && len(cmVertices) > 0 {
		for _, vertex := range vertices {
			dag.RemoveEdge(graph.RealEdge(root, vertex))
			for _, cmVertex := range cmVertices {
				dag.Connect(cmVertex, vertex)
			}
		}
	}
}

var _ graph.Transformer = &ClusterDeletionTransformer{}
