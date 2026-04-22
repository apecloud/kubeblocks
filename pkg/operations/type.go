/*
Copyright (C) 2022-2025 ApeCloud Co., Ltd

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

package operations

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	opsv1alpha1 "github.com/apecloud/kubeblocks/apis/operations/v1alpha1"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

type OpsHandler interface {
	// Action The action duration should be short. if it fails, it will be reconciled by the OpsRequest controller.
	// Do not patch OpsRequest status in this function with k8s client, just modify the status of ops.
	// The opsRequest controller will patch it to the k8s apiServer.
	Action(reqCtx intctrlutil.RequestCtx, cli client.Client, opsResource *OpsResource) error

	// ReconcileAction loops till the operation is completed.
	// return OpsRequest.status.phase and requeueAfter time.
	ReconcileAction(reqCtx intctrlutil.RequestCtx, cli client.Client, opsResource *OpsResource) (opsv1alpha1.OpsPhase, time.Duration, error)

	// ActionStartedCondition appends to OpsRequest.status.conditions when start performing Action function
	ActionStartedCondition(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource) (*metav1.Condition, error)

	// SaveLastConfiguration saves last configuration to the OpsRequest.status.lastConfiguration,
	// and this method will be executed together when opsRequest in running.
	SaveLastConfiguration(reqCtx intctrlutil.RequestCtx, cli client.Client, opsResource *OpsResource) error
}

type OpsBehaviour struct {
	FromClusterPhases []appsv1.ClusterPhase

	// ToClusterPhase indicates that the cluster will enter this phase during the operation.
	// All opsRequest with ToClusterPhase are mutually exclusive.
	ToClusterPhase appsv1.ClusterPhase

	// CancelFunc this function defines the cancel action and does not patch/update the opsRequest by client-go in here.
	// only update the opsRequest object, then opsRequest controller will update uniformly.
	CancelFunc func(reqCtx intctrlutil.RequestCtx, cli client.Client, opsResource *OpsResource) error

	// IsClusterCreation indicates whether the opsRequest will create a new cluster.
	IsClusterCreation bool

	// QueueByCluster indicates that the operation is queued for execution within the cluster-wide scope.
	QueueByCluster bool

	// QueueWithSelf indicates that the operation is queued for execution within opsType scope.
	QueueBySelf bool

	OpsHandler OpsHandler
}

type OpsResource struct {
	OpsDef         *opsv1alpha1.OpsDefinition
	OpsRequest     *opsv1alpha1.OpsRequest
	Cluster        *appsv1.Cluster
	Recorder       record.EventRecorder
	ToClusterPhase appsv1.ClusterPhase
	Runtimes       map[string]OpsRuntime
}

type OpsManager struct {
	OpsMap map[opsv1alpha1.OpsType]OpsBehaviour
}

type progressResource struct {
	// opsMessageKey progress message key of specified OpsType, it is a verb and will form the message of progressDetail
	// such as "vertical scale" of verticalScaling OpsRequest.
	opsMessageKey string
	// cluster component name. By default, it is the componentSpec.name.
	// but if it is a sharding component, the componentName is generated randomly.
	fullComponentName string
	// specifies the number of shards. if nil, it is not a sharding component.
	shards           *int32
	clusterComponent *appsv1.ClusterComponentSpec
	clusterDef       *appsv1.ClusterDefinition
	componentDef     *appsv1.ComponentDefinition
	// record which pods need to updated during this operation.
	// key is podName, value is instance template name.
	updatedPodSet map[string]string
	createdPodSet map[string]string
	deletedPodSet map[string]string
	compOps       ComponentOpsInterface
	// checks if it needs to wait the component to complete.
	// if only updates a part of pods, set it to false.
	noWaitComponentCompleted bool
}

// OpsRuntime abstracts the standard ops paths that only need workload/member views
// plus a small set of runtime-owned actions.
//
// Explicitly out of scope for this abstraction:
// - RebuildInstance, which still depends on direct Pod/PVC/PV/InstanceSet actions
// - Custom, which still depends on direct Pod/Job/ConfigMap/Secret based execution
type OpsRuntime interface {
	GetWorkload(namespace, clusterName, compName string) (Workload, error)
	GetInstance(namespace, clusterName, compName, instanceName string) (Instance, error)
	ListInstances(namespace, clusterName, compName string) ([]Instance, error)
	GenerateInstanceNameSet(clusterName, compName string, compReplicas int32, instances []appsv1.InstanceTemplate, offlineInstances []string) (map[string]string, error)
	GenerateTemplateInstanceNames(clusterName, compName, templateName string, replicas int32, offlineInstances []string, ordinals appsv1.Ordinals) ([]string, error)
	Switchover(ctx context.Context, namespace, clusterName, compName, instanceName, candidateName string) error
}

type Workload interface {
	GetMinReadySeconds() int32
	GetInstanceNameSet() sets.Set[string]
	GetCurrentRevisionMap() map[string]string
	GetNotReadyInstanceNameSet() sets.Set[string]
	GetNotAvailableInstanceNameSet() sets.Set[string]
	GetFailedInstanceNameSet() sets.Set[string]
}

type Instance interface {
	GetName() string
	GetComponentName() string
	GetCreationTimestamp() metav1.Time
	IsDeleting() bool
	GetRole() string
	IsAvailable(minReadySeconds int32, roleAware bool) bool
	IsFailedAndTimedOut() bool
	GetImage(containerName string) string
	GetStatusImage(containerName string) string
	GetResources(containerName string) corev1.ResourceRequirements
	GetNodeName() string
	GetTolerations() []corev1.Toleration
	GetAffinity() *corev1.Affinity
	GetTopologySpreadConstraints() []corev1.TopologySpreadConstraint
	GetPodVolumes() []corev1.Volume
	GetVolumeMounts(containerName string) []corev1.VolumeMount
	GetVolume(name string) (InstanceVolume, bool)
}

type InstanceVolume interface {
	GetClaimName() string
	GetRequestedStorage() resource.Quantity
	GetCapacity() resource.Quantity
	IsBound() bool
	IsExpanding() bool
}

func (r *OpsResource) GetRuntime(name string) (OpsRuntime, error) {
	if r == nil {
		return nil, fmt.Errorf("ops resource is nil")
	}
	if runtime, ok := r.Runtimes[name]; ok {
		return runtime, nil
	}
	return nil, fmt.Errorf("ops runtime not found for component/sharding %q", name)
}
