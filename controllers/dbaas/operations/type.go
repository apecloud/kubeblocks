/*
Copyright ApeCloud Inc.

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
	"context"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"

	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
)

type OpsHandler interface {
	// Action The action running time should be short. if it fails, it will be reconciled by the OpsRequest controller.
	// if you do not want to be reconciled when the operation fails,
	// you need to call PatchOpsStatus function in ops_util.go and set OpsRequest.status.phase to Failed
	Action(opsResource *OpsResource) error
	// ReconcileAction loops till the operation is completed.
	// return OpsRequest.status.phase and requeueAfter time.
	ReconcileAction(opsResource *OpsResource) (dbaasv1alpha1.Phase, time.Duration, error)
	// ActionStartedCondition append to OpsRequest.status.conditions when start performing Action function
	ActionStartedCondition(opsRequest *dbaasv1alpha1.OpsRequest) *metav1.Condition

	// SaveLastConfiguration saves last configuration to the OpsRequest.status.lastConfiguration,
	// and this method will be executed together when opsRequest to running.
	SaveLastConfiguration(opsResource *OpsResource) error

	// GetRealAffectedComponentMap returns a changed configuration componentName map by
	// compared current configuration with the last configuration.
	// we only changed the component status of cluster.status to the ToClusterPhase
	// of OpsBehaviour, which component name is in the returned componentName map.
	// Note: if the operation will not modify the Spec struct of the component workload,
	// GetRealAffectedComponentMap function should return nil unless phase management of cluster and components
	// is implemented at ReconcileAction function.
	GetRealAffectedComponentMap(opsRequest *dbaasv1alpha1.OpsRequest) realAffectedComponentMap
}

type realAffectedComponentMap map[string]struct{}

type OpsBehaviour struct {
	FromClusterPhases []dbaasv1alpha1.Phase
	ToClusterPhase    dbaasv1alpha1.Phase
	OpsHandler        OpsHandler
}

type OpsResource struct {
	Ctx        context.Context
	Client     client.Client
	OpsRequest *dbaasv1alpha1.OpsRequest
	Cluster    *dbaasv1alpha1.Cluster
	Recorder   record.EventRecorder
}

type OpsManager struct {
	OpsMap map[dbaasv1alpha1.OpsType]OpsBehaviour
}

type progressResource struct {
	// opsMessageKey progress message key of specified OpsType, it is a verb and will form the message of progressDetail
	// such as "vertical scale" of verticalScaling OpsRequest.
	opsMessageKey       string
	clusterComponent    *dbaasv1alpha1.ClusterComponent
	clusterComponentDef *dbaasv1alpha1.ClusterDefinitionComponent
}
