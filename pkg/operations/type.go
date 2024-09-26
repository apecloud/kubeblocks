/*
Copyright (C) 2022-2024 ApeCloud Co., Ltd

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
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

type reconfigureParams struct {
	resource *OpsResource
	reqCtx   intctrlutil.RequestCtx
	cli      client.Client

	clusterName         string
	componentName       string
	opsRequest          *opsv1alpha1.OpsRequest
	configurationItem   opsv1alpha1.ConfigurationItem
	configurationStatus *opsv1alpha1.ReconfiguringStatus
}

type OpsResource struct {
	OpsDef         *opsv1alpha1.OpsDefinition
	OpsRequest     *opsv1alpha1.OpsRequest
	Cluster        *appsv1.Cluster
	Recorder       record.EventRecorder
	ToClusterPhase appsv1.ClusterPhase
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
	// checks if the component is a sharding component
	isShardingComponent bool
	clusterComponent    *appsv1.ClusterComponentSpec
	clusterDef          *appsv1.ClusterDefinition
	componentDef        *appsv1.ComponentDefinition
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
