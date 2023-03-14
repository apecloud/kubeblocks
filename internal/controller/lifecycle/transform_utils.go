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
	"fmt"
	types2 "github.com/apecloud/kubeblocks/internal/controller/client"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	dataprotectionv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	opsutil "github.com/apecloud/kubeblocks/controllers/apps/operations/util"
	"github.com/apecloud/kubeblocks/internal/controller/graph"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
)

func findAll[T interface{}](dag *graph.DAG) []graph.Vertex {
	vertices := make([]graph.Vertex, 0)
	for _, vertex := range dag.Vertices() {
		v, _ := vertex.(*lifecycleVertex)
		if _, ok := v.obj.(T); ok {
			vertices = append(vertices, vertex)
		}
	}
	return vertices
}

func findAllNot[T interface{}](dag *graph.DAG) []graph.Vertex {
	vertices := make([]graph.Vertex, 0)
	for _, vertex := range dag.Vertices() {
		v, _ := vertex.(*lifecycleVertex)
		if _, ok := v.obj.(T); !ok {
			vertices = append(vertices, vertex)
		}
	}
	return vertices
}

func findRootVertex(dag *graph.DAG) (*lifecycleVertex, error) {
	root := dag.Root()
	if root == nil {
		return nil, fmt.Errorf("root vertex not found: %v", dag)
	}
	rootVertex, _ := root.(*lifecycleVertex)
	return rootVertex, nil
}

func getGVKName(object client.Object, scheme *runtime.Scheme) (*gvkName, error) {
	gvk, err := apiutil.GVKForObject(object, scheme)
	if err != nil {
		return nil, err
	}
	return &gvkName{
		gvk:  gvk,
		ns:   object.GetNamespace(),
		name: object.GetName(),
	}, nil
}

func isOwnerOf(owner, obj client.Object, scheme *runtime.Scheme) bool {
	ro, ok := owner.(runtime.Object)
	if !ok {
		return false
	}
	gvk, err := apiutil.GVKForObject(ro, scheme)
	if err != nil {
		return false
	}
	ref := metav1.OwnerReference{
		APIVersion: gvk.GroupVersion().String(),
		Kind:       gvk.Kind,
		UID:        owner.GetUID(),
		Name:       owner.GetName(),
	}
	owners := obj.GetOwnerReferences()
	referSameObject := func(a, b metav1.OwnerReference) bool {
		aGV, err := schema.ParseGroupVersion(a.APIVersion)
		if err != nil {
			return false
		}

		bGV, err := schema.ParseGroupVersion(b.APIVersion)
		if err != nil {
			return false
		}

		return aGV.Group == bGV.Group && a.Kind == b.Kind && a.Name == b.Name
	}
	for _, ownerRef := range owners {
		if referSameObject(ownerRef, ref) {
			return true
		}
	}
	return false
}

func actionPtr(action Action) *Action {
	return &action
}

func objectScheme() (*runtime.Scheme, error) {
	s := scheme.Scheme
	if err := scheme.AddToScheme(s); err != nil {
		return nil, err
	}
	if err := appsv1alpha1.AddToScheme(s); err != nil {
		return nil, err
	}
	return s, nil
}

func newRequeueError(after time.Duration, reason string) error {
	return &realRequeueError{
		reason:       reason,
		requeueAfter: after,
	}
}

func isClusterDeleting(cluster appsv1alpha1.Cluster) bool {
	return !cluster.GetDeletionTimestamp().IsZero()
}

func isClusterUpdating(cluster appsv1alpha1.Cluster) bool {
	return cluster.Status.ObservedGeneration != cluster.Generation
}

func isClusterStatusUpdating(cluster appsv1alpha1.Cluster) bool {
	return !isClusterDeleting(cluster) && !isClusterUpdating(cluster)
}

// updateClusterPhaseWhenConditionsError when cluster status is ConditionsError and the cluster applies resources successful,
// we should update the cluster to the correct state
func updateClusterPhaseWhenConditionsError(cluster *appsv1alpha1.Cluster) {
	if cluster.Status.Phase != appsv1alpha1.ConditionsErrorPhase {
		return
	}
	if cluster.Status.ObservedGeneration == 0 {
		cluster.Status.Phase = appsv1alpha1.CreatingPhase
		return
	}
	opsRequestSlice, _ := opsutil.GetOpsRequestSliceFromCluster(cluster)
	// if no operations in cluster, means user update the cluster.spec directly
	if len(opsRequestSlice) == 0 {
		cluster.Status.Phase = appsv1alpha1.SpecUpdatingPhase
		return
	}
	// if exits opsRequests are running, set the cluster phase to the early target phase with the OpsRequest
	cluster.Status.Phase = opsRequestSlice[0].ToClusterPhase
}

// checkReferencingCRStatus checks if cluster referenced CR is available
func checkReferencedCRStatus(referencedCRPhase appsv1alpha1.Phase) error {
	if referencedCRPhase == appsv1alpha1.AvailablePhase {
		return nil
	}
	return newRequeueError(ControllerErrorRequeueTime, "cluster definition not available")
}

func getBackupObjects(reqCtx intctrlutil.RequestCtx,
	cli types2.ReadonlyClient,
	namespace string,
	backupName string) (*dataprotectionv1alpha1.Backup, *dataprotectionv1alpha1.BackupTool, error) {
	// get backup
	backup := &dataprotectionv1alpha1.Backup{}
	if err := cli.Get(reqCtx.Ctx, types.NamespacedName{Name: backupName, Namespace: namespace}, backup); err != nil {
		return nil, nil, err
	}

	// get backup tool
	backupTool := &dataprotectionv1alpha1.BackupTool{}
	if err := cli.Get(reqCtx.Ctx, types.NamespacedName{Name: backup.Status.BackupToolName}, backupTool); err != nil {
		return nil, nil, err
	}
	return backup, backupTool, nil
}
