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
	"encoding/json"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/constant"
)

type StopOpsHandler struct{}

var _ OpsHandler = StopOpsHandler{}

func init() {
	stopBehaviour := OpsBehaviour{
		FromClusterPhases: []appsv1alpha1.Phase{appsv1alpha1.RunningPhase, appsv1alpha1.FailedPhase, appsv1alpha1.AbnormalPhase},
		ToClusterPhase:    appsv1alpha1.StoppingPhase,
		OpsHandler:        StopOpsHandler{},
	}

	opsMgr := GetOpsManager()
	opsMgr.RegisterOps(appsv1alpha1.StopType, stopBehaviour)
}

// ActionStartedCondition the started condition when handling the stop request.
func (stop StopOpsHandler) ActionStartedCondition(opsRequest *appsv1alpha1.OpsRequest) *metav1.Condition {
	return appsv1alpha1.NewStopCondition(opsRequest)
}

// Action modifies Cluster.spec.components[*].replicas from the opsRequest
func (stop StopOpsHandler) Action(opsRes *OpsResource) error {
	var (
		expectReplicas       = int32(0)
		componentReplicasMap = map[string]int32{}
		cluster              = opsRes.Cluster
	)
	for i, v := range cluster.Spec.ComponentSpecs {
		componentReplicasMap[v.Name] = v.Replicas
		cluster.Spec.ComponentSpecs[i].Replicas = expectReplicas
	}
	componentReplicasSnapshot, err := json.Marshal(componentReplicasMap)
	if err != nil {
		return err
	}
	if cluster.Annotations == nil {
		cluster.Annotations = map[string]string{}
	}
	// record the replicas snapshot of components to the annotations of cluster before stopping the cluster.
	cluster.Annotations[constant.SnapShotForStartAnnotationKey] = string(componentReplicasSnapshot)
	return opsRes.Client.Update(opsRes.Ctx, cluster)
}

// ReconcileAction will be performed when action is done and loops till OpsRequest.status.phase is Succeed/Failed.
// the Reconcile function for stop opsRequest.
func (stop StopOpsHandler) ReconcileAction(opsRes *OpsResource) (appsv1alpha1.Phase, time.Duration, error) {
	getExpectReplicas := func(opsRequest *appsv1alpha1.OpsRequest, componentName string) *int32 {
		expectReplicas := int32(0)
		return &expectReplicas
	}
	handleComponentProgress := func(opsRes *OpsResource,
		pgRes progressResource,
		compStatus *appsv1alpha1.OpsRequestComponentStatus) (int32, int32, error) {
		return handleComponentProgressForScalingReplicas(opsRes, pgRes, compStatus, getExpectReplicas)
	}
	return ReconcileActionWithComponentOps(opsRes, "", handleComponentProgress)
}

// SaveLastConfiguration records last configuration to the OpsRequest.status.lastConfiguration
func (stop StopOpsHandler) SaveLastConfiguration(opsRes *OpsResource) error {
	opsRequest := opsRes.OpsRequest
	lastComponentInfo := map[string]appsv1alpha1.LastComponentConfiguration{}
	for _, v := range opsRes.Cluster.Spec.ComponentSpecs {
		if v.Replicas != 0 {
			copyReplicas := v.Replicas
			lastComponentInfo[v.Name] = appsv1alpha1.LastComponentConfiguration{
				Replicas: &copyReplicas,
			}
		}
	}
	opsRequest.Status.LastConfiguration = appsv1alpha1.LastConfiguration{
		Components: lastComponentInfo,
	}
	return nil
}

// GetRealAffectedComponentMap gets the real affected component map for the operation
func (stop StopOpsHandler) GetRealAffectedComponentMap(opsRequest *appsv1alpha1.OpsRequest) realAffectedComponentMap {
	return getCompMapFromLastConfiguration(opsRequest)
}

// getCompMapFromLastConfiguration gets the component name map from status.lastConfiguration
func getCompMapFromLastConfiguration(opsRequest *appsv1alpha1.OpsRequest) realAffectedComponentMap {
	realChangedMap := realAffectedComponentMap{}
	for k := range opsRequest.Status.LastConfiguration.Components {
		realChangedMap[k] = struct{}{}
	}
	return realChangedMap
}
