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
		return graph.ErrFastReturn
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

	// find all vertices of StatefulSet and Deployment, and connect them to ConfigMap,
	// this is to ensure that ConfigMap is deleted after StatefulSet and Deployment Workloads are deleted.
	vertices := findAll[*appsv1.StatefulSet](dag)
	deployVertices := findAll[*appsv1.Deployment](dag)
	vertices = append(vertices, deployVertices...)
	cmVertices := findAll[*corev1.ConfigMap](dag)
	if len(vertices) > 0 && len(cmVertices) > 0 {
		for _, vertex := range vertices {
			dag.RemoveEdge(graph.RealEdge(root, vertex))
			for _, cmVertex := range cmVertices {
				dag.Connect(cmVertex, vertex)
			}
		}
	}

	root.action = actionPtr(DELETE)

	// fast return, that is stopping the plan.Build() stage and jump to plan.Execute() directly
	return graph.ErrFastReturn
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

var _ graph.Transformer = &ClusterDeletionTransformer{}
