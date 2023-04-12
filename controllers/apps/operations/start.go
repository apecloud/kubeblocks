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
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/constant"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
)

type StartOpsHandler struct{}

var _ OpsHandler = StartOpsHandler{}

func init() {
	stopBehaviour := OpsBehaviour{
		FromClusterPhases:                  []appsv1alpha1.ClusterPhase{appsv1alpha1.StoppedClusterPhase},
		ToClusterPhase:                     appsv1alpha1.SpecReconcilingClusterPhase, // appsv1alpha1.StartingPhase,
		OpsHandler:                         StartOpsHandler{},
		ProcessingReasonInClusterCondition: ProcessingReasonStarting,
	}

	opsMgr := GetOpsManager()
	opsMgr.RegisterOps(appsv1alpha1.StartType, stopBehaviour)
}

// ActionStartedCondition the started condition when handling the start request.
func (start StartOpsHandler) ActionStartedCondition(opsRequest *appsv1alpha1.OpsRequest) *metav1.Condition {
	return appsv1alpha1.NewStartCondition(opsRequest)
}

// Action modifies Cluster.spec.components[*].replicas from the opsRequest
func (start StartOpsHandler) Action(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource) error {
	cluster := opsRes.Cluster
	componentReplicasMap, err := start.getComponentReplicasSnapshot(cluster.Annotations)
	if err != nil {
		return err
	}
	for i, v := range cluster.Spec.ComponentSpecs {
		replicasOfSnapshot := componentReplicasMap[v.Name]
		if replicasOfSnapshot == 0 {
			continue
		}
		// only reset the component which replicas number is 0
		if v.Replicas == 0 {
			cluster.Spec.ComponentSpecs[i].Replicas = replicasOfSnapshot
		}
	}
	// delete the replicas snapshot of components from the cluster.
	delete(cluster.Annotations, constant.SnapShotForStartAnnotationKey)
	return cli.Update(reqCtx.Ctx, cluster)
}

// ReconcileAction will be performed when action is done and loops till OpsRequest.status.phase is Succeed/Failed.
// the Reconcile function for start opsRequest.
func (start StartOpsHandler) ReconcileAction(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource) (appsv1alpha1.OpsPhase, time.Duration, error) {
	getExpectReplicas := func(opsRequest *appsv1alpha1.OpsRequest, componentName string) *int32 {
		componentReplicasMap, _ := start.getComponentReplicasSnapshot(opsRequest.Annotations)
		replicas, ok := componentReplicasMap[componentName]
		if !ok {
			return nil
		}
		return &replicas
	}

	handleComponentProgress := func(reqCtx intctrlutil.RequestCtx,
		cli client.Client,
		opsRes *OpsResource,
		pgRes progressResource,
		compStatus *appsv1alpha1.OpsRequestComponentStatus) (int32, int32, error) {
		return handleComponentProgressForScalingReplicas(reqCtx, cli, opsRes, pgRes, compStatus, getExpectReplicas)
	}
	return ReconcileActionWithComponentOps(reqCtx, cli, opsRes, "", handleComponentProgress)
}

// SaveLastConfiguration records last configuration to the OpsRequest.status.lastConfiguration
func (start StartOpsHandler) SaveLastConfiguration(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource) error {
	opsRequest := opsRes.OpsRequest
	lastComponentInfo := map[string]appsv1alpha1.LastComponentConfiguration{}
	componentReplicasMap, err := start.getComponentReplicasSnapshot(opsRes.Cluster.Annotations)
	if err != nil {
		return err
	}
	if err = start.setOpsAnnotation(reqCtx, cli, opsRes, componentReplicasMap); err != nil {
		return err
	}
	for _, v := range opsRes.Cluster.Spec.ComponentSpecs {
		replicasOfSnapshot := componentReplicasMap[v.Name]
		if replicasOfSnapshot == 0 {
			continue
		}
		if v.Replicas == 0 {
			copyReplicas := v.Replicas
			lastComponentInfo[v.Name] = appsv1alpha1.LastComponentConfiguration{
				Replicas: &copyReplicas,
			}
		}
	}
	opsRequest.Status.LastConfiguration.Components = lastComponentInfo
	return nil
}

// GetRealAffectedComponentMap gets the real affected component map for the operation
func (start StartOpsHandler) GetRealAffectedComponentMap(opsRequest *appsv1alpha1.OpsRequest) realAffectedComponentMap {
	return getCompMapFromLastConfiguration(opsRequest)
}

// setOpsAnnotation sets the replicas snapshot of components before stopping the cluster to the annotations of this opsRequest.
func (start StartOpsHandler) setOpsAnnotation(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource, componentReplicasMap map[string]int32) error {
	annotations := opsRes.OpsRequest.Annotations
	if annotations == nil {
		annotations = map[string]string{}
	}
	componentReplicasSnapshot, err := json.Marshal(componentReplicasMap)
	if err != nil {
		return err
	}
	if _, ok := opsRes.OpsRequest.Annotations[constant.SnapShotForStartAnnotationKey]; !ok {
		patch := client.MergeFrom(opsRes.OpsRequest.DeepCopy())
		annotations[constant.SnapShotForStartAnnotationKey] = string(componentReplicasSnapshot)
		opsRes.OpsRequest.Annotations = annotations
		return cli.Patch(reqCtx.Ctx, opsRes.OpsRequest, patch)
	}
	return nil
}

// getComponentReplicasSnapshot gets the replicas snapshot of components from annotations.
func (start StartOpsHandler) getComponentReplicasSnapshot(annotations map[string]string) (map[string]int32, error) {
	componentReplicasMap := map[string]int32{}
	snapshotForStart := annotations[constant.SnapShotForStartAnnotationKey]
	if len(snapshotForStart) != 0 {
		if err := json.Unmarshal([]byte(snapshotForStart), &componentReplicasMap); err != nil {
			return componentReplicasMap, err
		}
	}
	return componentReplicasMap, nil
}
