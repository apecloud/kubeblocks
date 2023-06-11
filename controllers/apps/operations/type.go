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

package operations

import (
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
)

type OpsHandler interface {
	// Action The action duration should be short. if it fails, it will be reconciled by the OpsRequest controller.
	// Do not patch OpsRequest status in this function with k8s client, just modify the status of ops.
	// The opsRequest controller will patch it to the k8s apiServer.
	Action(reqCtx intctrlutil.RequestCtx, cli client.Client, opsResource *OpsResource) error
	// ReconcileAction loops till the operation is completed.
	// return OpsRequest.status.phase and requeueAfter time.
	ReconcileAction(reqCtx intctrlutil.RequestCtx, cli client.Client, opsResource *OpsResource) (appsv1alpha1.OpsPhase, time.Duration, error)
	// ActionStartedCondition appends to OpsRequest.status.conditions when start performing Action function
	ActionStartedCondition(opsRequest *appsv1alpha1.OpsRequest) *metav1.Condition

	// SaveLastConfiguration saves last configuration to the OpsRequest.status.lastConfiguration,
	// and this method will be executed together when opsRequest in running.
	SaveLastConfiguration(reqCtx intctrlutil.RequestCtx, cli client.Client, opsResource *OpsResource) error

	// GetRealAffectedComponentMap returns a component map that is actually affected by the opsRequest.
	// when MaintainClusterPhaseBySelf of the opsBehaviour is true,
	// will change the phase of the component to Updating after Action is done which exists in this map.
	// Deprecated: will be removed soon.
	GetRealAffectedComponentMap(opsRequest *appsv1alpha1.OpsRequest) realAffectedComponentMap
}

type realAffectedComponentMap map[string]struct{}

type OpsBehaviour struct {
	FromClusterPhases []appsv1alpha1.ClusterPhase

	// ToClusterPhase indicates that the cluster will enter this phase during the operation.
	ToClusterPhase appsv1alpha1.ClusterPhase

	// MaintainClusterPhaseBySelf indicates whether the operation will maintain cluster/component phase by itself.
	// Generally, the cluster/component phase will be maintained by cluster controller, but if the operation will not update
	// StatefulSet/Deployment by Cluster controller and make pod rebuilt, maintain the cluster/component phase by self.
	MaintainClusterPhaseBySelf bool

	// ProcessingReasonInClusterCondition indicates the reason of the condition that type is "OpsRequestProcessed" in Cluster.Status.Conditions and
	// is only valid when ToClusterPhase is not empty. it will indicate what operation the cluster is doing and
	// will be displayed of "kbcli cluster list".
	ProcessingReasonInClusterCondition string

	// CancelFunc this function defines the cancel action and does not patch/update the opsRequest by client-go in here.
	// only update the opsRequest object, then opsRequest controller will update uniformly.
	CancelFunc func(reqCtx intctrlutil.RequestCtx, cli client.Client, opsResource *OpsResource) error

	OpsHandler OpsHandler
}

type OpsResource struct {
	OpsRequest     *appsv1alpha1.OpsRequest
	Cluster        *appsv1alpha1.Cluster
	Recorder       record.EventRecorder
	ToClusterPhase appsv1alpha1.ClusterPhase
}

type OpsManager struct {
	OpsMap map[appsv1alpha1.OpsType]OpsBehaviour
}

type progressResource struct {
	// opsMessageKey progress message key of specified OpsType, it is a verb and will form the message of progressDetail
	// such as "vertical scale" of verticalScaling OpsRequest.
	opsMessageKey       string
	clusterComponent    *appsv1alpha1.ClusterComponentSpec
	clusterComponentDef *appsv1alpha1.ClusterComponentDefinition
	opsIsCompleted      bool
}

const (
	// ProcessingReasonHorizontalScaling is the reason of the "OpsRequestProcessed" condition for the horizontal scaling opsRequest processing in cluster.
	ProcessingReasonHorizontalScaling = "HorizontalScaling"
	// ProcessingReasonVerticalScaling is the reason of the "OpsRequestProcessed" condition for the vertical scaling opsRequest processing in cluster.
	ProcessingReasonVerticalScaling = "VerticalScaling"
	// ProcessingReasonStarting is the reason of the "OpsRequestProcessed" condition for the start opsRequest processing in cluster.
	ProcessingReasonStarting = "Starting"
	// ProcessingReasonStopping is the reason of the "OpsRequestProcessed" condition for the stop opsRequest processing in cluster.
	ProcessingReasonStopping = "Stopping"
	// ProcessingReasonRestarting is the reason of the "OpsRequestProcessed" condition for the restart opsRequest processing in cluster.
	ProcessingReasonRestarting = "Restarting"
	// ProcessingReasonReconfiguring is the reason of the "OpsRequestProcessed" condition for the reconfiguration opsRequest processing in cluster.
	ProcessingReasonReconfiguring = "Reconfiguring"
	// ProcessingReasonVersionUpgrading is the reason of the "OpsRequestProcessed" condition for the version upgrade opsRequest processing in cluster.
	ProcessingReasonVersionUpgrading = "VersionUpgrading"
)
