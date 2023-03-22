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
	// Action The action running time should be short. if it fails, it will be reconciled by the OpsRequest controller.
	// Do not patch OpsRequest status in this function with k8s client, just modify the status variable of ops.
	// The opsRequest controller will unify the patch it to the k8s apiServer.
	Action(reqCtx intctrlutil.RequestCtx, cli client.Client, opsResource *OpsResource) error
	// ReconcileAction loops till the operation is completed.
	// return OpsRequest.status.phase and requeueAfter time.
	ReconcileAction(reqCtx intctrlutil.RequestCtx, cli client.Client, opsResource *OpsResource) (appsv1alpha1.OpsPhase, time.Duration, error)
	// ActionStartedCondition append to OpsRequest.status.conditions when start performing Action function
	ActionStartedCondition(opsRequest *appsv1alpha1.OpsRequest) *metav1.Condition

	// SaveLastConfiguration saves last configuration to the OpsRequest.status.lastConfiguration,
	// and this method will be executed together when opsRequest to running.
	SaveLastConfiguration(reqCtx intctrlutil.RequestCtx, cli client.Client, opsResource *OpsResource) error

	// GetRealAffectedComponentMap returns a changed configuration componentName map by
	// compared current configuration with the last configuration.
	// we only changed the component status of cluster.status to the ToClusterPhase
	// of OpsBehaviour, which component name is in the returned componentName map.
	// Note: if the operation will not modify the Spec struct of the component workload,
	// GetRealAffectedComponentMap function should return nil unless phase management of cluster and components
	// is implemented at ReconcileAction function.
	GetRealAffectedComponentMap(opsRequest *appsv1alpha1.OpsRequest) realAffectedComponentMap
}

type realAffectedComponentMap map[string]struct{}

type OpsBehaviour struct {
	FromClusterPhases []appsv1alpha1.Phase

	// ToClusterPhase indicates that the cluster will enter this phase during the operation.
	ToClusterPhase appsv1alpha1.Phase

	// MaintainClusterPhaseBySelf indicates whether the operation will maintain cluster/component phase by itself.
	// Generally, the cluster/component phase will be maintained by cluster controller, but if your operation will not update
	// StatefulSet/Deployment by Cluster controller and make pod to rebuilt, you need to maintain the cluster/component phase yourself.
	MaintainClusterPhaseBySelf bool

	OpsHandler OpsHandler
}

type OpsResource struct {
	OpsRequest     *appsv1alpha1.OpsRequest
	Cluster        *appsv1alpha1.Cluster
	Recorder       record.EventRecorder
	ToClusterPhase appsv1alpha1.Phase
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
