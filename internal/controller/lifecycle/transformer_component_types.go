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
	"context"
	"fmt"
	ictrltypes "github.com/apecloud/kubeblocks/internal/controller/types"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/controller/component"
	"github.com/apecloud/kubeblocks/internal/controller/graph"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
)

// TODO(refactor):
//  1. status management
//  2. component workload

type Component interface {
	GetName() string
	GetNamespace() string
	GetClusterName() string
	GetWorkloadType() appsv1alpha1.WorkloadType

	GetDefinition() *appsv1alpha1.ClusterDefinition
	GetVersion() *appsv1alpha1.ClusterVersion
	GetCluster() *appsv1alpha1.Cluster
	GetSynthesizedComponent() *component.SynthesizedComponent

	GetMatchingLabels() client.MatchingLabels

	// GetPhase() appsv1alpha1.ClusterComponentPhase
	// GetStatus() appsv1alpha1.ClusterComponentStatus

	// Exist checks whether the component exists in cluster, we say that a component exists iff the main workloads
	// exist in cluster, such as stateful set for consensus/replication/stateful and deployment for stateless.
	Exist(reqCtx intctrlutil.RequestCtx, cli client.Client) (bool, error)

	Create(reqCtx intctrlutil.RequestCtx, cli client.Client) error
	Delete(reqCtx intctrlutil.RequestCtx, cli client.Client) error
	Update(reqCtx intctrlutil.RequestCtx, cli client.Client) error
	Status(reqCtx intctrlutil.RequestCtx, cli client.Client) error

	ExpandVolume(reqCtx intctrlutil.RequestCtx, cli client.Client) error

	HorizontalScale(reqCtx intctrlutil.RequestCtx, cli client.Client) error

	Restart(reqCtx intctrlutil.RequestCtx, cli client.Client) error

	Snapshot(reqCtx intctrlutil.RequestCtx, cli client.Client) error

	// TODO(refactor): impl-related, will replace it with component workload
	addResource(obj client.Object, action *ictrltypes.LifecycleAction, parent *ictrltypes.LifecycleVertex) *ictrltypes.LifecycleVertex
	addWorkload(obj client.Object, action *ictrltypes.LifecycleAction, parent *ictrltypes.LifecycleVertex)
}

// /// TODO(refactor): copied from controllers/apps/components/types/Component, refine it later.
type ComponentSet interface {
	// IsRunning when relevant k8s workloads changes, it checks whether the component is running.
	// you can also reconcile the pods of component till the component is Running here.
	IsRunning(ctx context.Context, obj client.Object) (bool, error)

	// PodsReady checks whether all pods of the component are ready.
	// it means the pods are available in StatefulSet or Deployment.
	PodsReady(ctx context.Context, obj client.Object) (bool, error)

	// PodIsAvailable checks whether a pod of the component is available.
	// if the component is Stateless/StatefulSet, the available conditions follows as:
	// 1. the pod is ready.
	// 2. readyTime reached minReadySeconds.
	// if the component is ConsensusSet,it will be available when the pod is ready and labeled with its role.
	PodIsAvailable(pod *corev1.Pod, minReadySeconds int32) bool

	//// HandleProbeTimeoutWhenPodsReady if the component has no role probe, return false directly. otherwise,
	//// we should handle the component phase when the role probe timeout and return a bool.
	//// if return true, means probe is not timing out and need to requeue after an interval time to handle probe timeout again.
	//// else return false, means probe has timed out and needs to update the component phase to Failed or Abnormal.
	//HandleProbeTimeoutWhenPodsReady(ctx context.Context, recorder record.EventRecorder) (bool, error)
	HandleProbeTimeoutWhenPodsReady(status *appsv1alpha1.ClusterComponentStatus, pods []*corev1.Pod)

	// GetPhaseWhenPodsNotReady when the pods of component are not ready, calculate the component phase is Failed or Abnormal.
	// if return an empty phase, means the pods of component are ready and skips it.
	GetPhaseWhenPodsNotReady(ctx context.Context, componentName string) (appsv1alpha1.ClusterComponentPhase, error)

	HandleRestart(ctx context.Context, obj client.Object) ([]graph.Vertex, error)

	HandleRoleChange(ctx context.Context, obj client.Object) ([]graph.Vertex, error)
}

func NewComponent(cli client.Client,
	definition *appsv1alpha1.ClusterDefinition,
	version *appsv1alpha1.ClusterVersion,
	cluster *appsv1alpha1.Cluster,
	compName string,
	dag *graph.DAG) (Component, error) {
	var compDef *appsv1alpha1.ClusterComponentDefinition
	var compVer *appsv1alpha1.ClusterComponentVersion
	compSpec := cluster.GetComponentByName(compName)
	if compSpec != nil {
		compDef = definition.GetComponentDefByName(compSpec.ComponentDefRef)
		if compDef == nil {
			return nil, fmt.Errorf("referenced component definition is not exist, cluster: %s, component: %s, component definition ref:%s",
				cluster.Name, compSpec.Name, compSpec.ComponentDefRef)
		}
		if version != nil {
			compVer = version.GetDefNameMappingComponents()[compSpec.ComponentDefRef]
		}
	}

	if compSpec == nil || compDef == nil {
		// TODO(refactor): fix me
		return nil, fmt.Errorf("NotSupported")
	}

	switch compDef.WorkloadType {
	case appsv1alpha1.Replication:
		return newReplicationComponent(cli, definition, cluster, compDef, compVer, compSpec, dag), nil
	case appsv1alpha1.Consensus:
		return newConsensusComponent(cli, definition, cluster, compDef, compVer, compSpec, dag), nil
	case appsv1alpha1.Stateful:
		return newStatefulComponent(cli, definition, cluster, compDef, compVer, compSpec, dag), nil
	case appsv1alpha1.Stateless:
		return newStatelessComponent(cli, definition, cluster, compDef, compVer, compSpec, dag), nil
	}
	return nil, fmt.Errorf("unknown workload type: %s, cluster: %s, component: %s, component definition ref: %s",
		compDef.WorkloadType, cluster.Name, compSpec.Name, compSpec.ComponentDefRef)
}
