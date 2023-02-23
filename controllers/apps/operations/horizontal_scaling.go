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

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
)

type horizontalScalingOpsHandler struct{}

var _ OpsHandler = horizontalScalingOpsHandler{}

func init() {
	horizontalScalingBehaviour := OpsBehaviour{
		FromClusterPhases: []appsv1alpha1.Phase{appsv1alpha1.RunningPhase, appsv1alpha1.FailedPhase, appsv1alpha1.AbnormalPhase},
		ToClusterPhase:    appsv1alpha1.HorizontalScalingPhase,
		OpsHandler:        horizontalScalingOpsHandler{},
	}

	opsMgr := GetOpsManager()
	opsMgr.RegisterOps(appsv1alpha1.HorizontalScalingType, horizontalScalingBehaviour)
}

// ActionStartedCondition the started condition when handling the horizontal scaling request.
func (hs horizontalScalingOpsHandler) ActionStartedCondition(opsRequest *appsv1alpha1.OpsRequest) *metav1.Condition {
	return appsv1alpha1.NewHorizontalScalingCondition(opsRequest)
}

// Action modifies Cluster.spec.components[*].replicas from the opsRequest
func (hs horizontalScalingOpsHandler) Action(opsRes *OpsResource) error {
	var (
		horizontalScalingMap = opsRes.OpsRequest.ConvertHorizontalScalingListToMap()
		horizontalScaling    appsv1alpha1.HorizontalScaling
		ok                   bool
	)

	for index, component := range opsRes.Cluster.Spec.ComponentSpecs {
		if horizontalScaling, ok = horizontalScalingMap[component.Name]; !ok {
			continue
		}
		if horizontalScaling.Replicas != 0 {
			r := horizontalScaling.Replicas
			opsRes.Cluster.Spec.ComponentSpecs[index].Replicas = r
		}
	}
	return opsRes.Client.Update(opsRes.Ctx, opsRes.Cluster)
}

// ReconcileAction will be performed when action is done and loops till OpsRequest.status.phase is Succeed/Failed.
// the Reconcile function for horizontal scaling opsRequest.
func (hs horizontalScalingOpsHandler) ReconcileAction(opsRes *OpsResource) (appsv1alpha1.Phase, time.Duration, error) {
	handleComponentProgress := func(opsRes *OpsResource,
		pgRes progressResource,
		compStatus *appsv1alpha1.OpsRequestComponentStatus) (int32, int32, error) {
		return handleComponentProgressForScalingReplicas(opsRes, pgRes, compStatus, hs.getExpectReplicas)
	}
	return ReconcileActionWithComponentOps(opsRes, "", handleComponentProgress)
}

// GetRealAffectedComponentMap gets the real affected component map for the operation
func (hs horizontalScalingOpsHandler) GetRealAffectedComponentMap(opsRequest *appsv1alpha1.OpsRequest) realAffectedComponentMap {
	realChangedMap := realAffectedComponentMap{}
	hsMap := opsRequest.ConvertHorizontalScalingListToMap()
	for k, v := range opsRequest.Status.LastConfiguration.Components {
		currHs, ok := hsMap[k]
		if !ok {
			continue
		}
		if v.Replicas == nil || *v.Replicas != currHs.Replicas {
			realChangedMap[k] = struct{}{}
		}
	}
	return realChangedMap
}

// SaveLastConfiguration records last configuration to the OpsRequest.status.lastConfiguration
func (hs horizontalScalingOpsHandler) SaveLastConfiguration(opsRes *OpsResource) error {
	opsRequest := opsRes.OpsRequest
	lastComponentInfo := map[string]appsv1alpha1.LastComponentConfiguration{}
	componentNameMap := opsRequest.GetComponentNameMap()
	for _, v := range opsRes.Cluster.Spec.ComponentSpecs {
		if _, ok := componentNameMap[v.Name]; !ok {
			continue
		}
		copyReplicas := v.Replicas
		lastComponentInfo[v.Name] = appsv1alpha1.LastComponentConfiguration{
			Replicas: &copyReplicas,
		}
	}
	opsRequest.Status.LastConfiguration = appsv1alpha1.LastConfiguration{
		Components: lastComponentInfo,
	}
	return nil
}

func (hs horizontalScalingOpsHandler) getExpectReplicas(opsRequest *appsv1alpha1.OpsRequest, componentName string) *int32 {
	for _, v := range opsRequest.Spec.HorizontalScalingList {
		if v.ComponentName == componentName {
			return &v.Replicas
		}
	}
	return nil
}
