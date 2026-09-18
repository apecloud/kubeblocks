/*
Copyright (C) 2022-2026 ApeCloud Co., Ltd

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

	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	opsv1alpha1 "github.com/apecloud/kubeblocks/apis/operations/v1alpha1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
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
	compOps          ComponentOpsInterface
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
	GenerateInstanceNameSet(clusterName, compName string, compReplicas int32, instances []appsv1.InstanceTemplate, offlineInstances []string) (map[string]string, error)
	Switchover(ctx context.Context, synthesizedComp *component.SynthesizedComponent, instanceName, candidateName string) error
}

type Workload interface {
	GetInstanceStatuses() []workloads.InstanceStatus
}

type Instance interface {
	GetName() string
	HasPod() bool
	GetRole() string
	IsFailedAndTimedOut() bool
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
