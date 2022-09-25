/*
Copyright 2022.

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
	"fmt"
	"time"

	"golang.org/x/exp/slices"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
)

// ReconcileActionWithCluster it will be performed when action is done and loop util OpsRequest.status.phase is Succeed.
// if OpsRequest.spec.clusterOps is not null, you can use it to OpsBehaviour.ReconcileAction.
func ReconcileActionWithCluster(opsRes *OpsResource) error {
	var (
		opsRequest = opsRes.OpsRequest
		isChanged  bool
	)
	patch := client.MergeFrom(opsRequest.DeepCopy())
	if opsRequest.Status.Components == nil {
		opsRequest.Status.Components = map[string]dbaasv1alpha1.OpsRequestStatusComponent{}
	}
	for k, v := range opsRes.Cluster.Status.Components {
		// the operation occurs in the cluster, such as upgrade.
		// However, it is also possible that only the corresponding components in the cluster have changed,
		// and the phase is updating So we need to monitor these components and send the corresponding event
		if statusComponent, ok := opsRequest.Status.Components[k]; (!ok && v.Phase == dbaasv1alpha1.UpdatingPhase) || statusComponent.Phase != v.Phase {
			isChanged = true
			opsRequest.Status.Components[k] = dbaasv1alpha1.OpsRequestStatusComponent{Phase: v.Phase}
			sendEventWhenComponentPhaseChanged(opsRes, k, v.Phase)
		}
	}
	if isChanged {
		if err := opsRes.Client.Status().Patch(opsRes.Ctx, opsRequest, patch); err != nil {
			return err
		}
	}
	if opsRes.Cluster.Status.Phase != dbaasv1alpha1.RunningPhase {
		return fmt.Errorf("opsRequest is not completed")
	}
	return nil
}

// ReconcileActionWithComponentOps it will be performed when action is done and loop util OpsRequest.status.phase is Succeed.
// if OpsRequest.spec.componentOps is not null, you can use it to OpsBehaviour.ReconcileAction.
func ReconcileActionWithComponentOps(opsRes *OpsResource) error {
	var (
		opsRequest       = opsRes.OpsRequest
		componentNameMap = getComponentsNameMap(opsRequest)
		isOk             = true
		isChanged        bool
	)
	patch := client.MergeFrom(opsRequest.DeepCopy())
	if opsRequest.Status.Components == nil {
		opsRequest.Status.Components = map[string]dbaasv1alpha1.OpsRequestStatusComponent{}
	}
	for k, v := range opsRes.Cluster.Status.Components {
		if _, ok := componentNameMap[k]; !ok {
			continue
		}
		if v.Phase != dbaasv1alpha1.RunningPhase {
			isOk = false
		}
		if statusComponent, ok := opsRequest.Status.Components[k]; !ok || statusComponent.Phase != v.Phase {
			isChanged = true
			opsRequest.Status.Components[k] = dbaasv1alpha1.OpsRequestStatusComponent{Phase: v.Phase}
			sendEventWhenComponentPhaseChanged(opsRes, k, v.Phase)
		}

	}
	if isChanged {
		if err := opsRes.Client.Status().Patch(opsRes.Ctx, opsRequest, patch); err != nil {
			return err
		}
	}
	if !isOk {
		return fmt.Errorf("opsRequest is not completed")
	}
	return nil
}

// sendEventWhenComponentStatusChanged send an event when OpsRequest.status.components[*].phase is changed
func sendEventWhenComponentPhaseChanged(opsRes *OpsResource, componentName string, phase dbaasv1alpha1.Phase) {
	var (
		tip    string
		reason string
	)
	if phase == dbaasv1alpha1.RunningPhase {
		tip = "Successfully"
		reason = dbaasv1alpha1.ReasonSuccessful
	} else {
		reason = dbaasv1alpha1.ReasonStarting
	}
	message := fmt.Sprintf("%s %s component: %s in Cluster: %s",
		tip, opsRes.OpsRequest.Spec.Type, componentName, opsRes.OpsRequest.Spec.ClusterRef)
	opsRes.Recorder.Event(opsRes.OpsRequest, corev1.EventTypeNormal, reason, message)
}

// PatchOpsStatus patch OpsRequest.status
func PatchOpsStatus(opsRes *OpsResource,
	phase dbaasv1alpha1.Phase,
	condition ...*metav1.Condition) error {

	opsRequest := opsRes.OpsRequest
	patch := client.MergeFrom(opsRequest.DeepCopy())
	for _, v := range condition {
		if v == nil {
			continue
		}
		opsRequest.SetStatusCondition(*v)
		// provide an event
		eventType := corev1.EventTypeNormal
		if phase == dbaasv1alpha1.FailedPhase {
			eventType = corev1.EventTypeWarning
		}
		opsRes.Recorder.Event(opsRequest, eventType, v.Reason, v.Message)
	}
	if slices.Index([]dbaasv1alpha1.Phase{dbaasv1alpha1.SucceedPhase, dbaasv1alpha1.FailedPhase}, phase) != -1 {
		opsRequest.Status.CompletionTimestamp = &metav1.Time{Time: time.Now()}
		// when OpsRequest is completed, do it
		if err := deleteOpsRequestAnnotationInCluster(opsRes); err != nil {
			return err
		}
	}
	if phase == dbaasv1alpha1.RunningPhase && opsRequest.Status.Phase != phase {
		opsRequest.Status.StartTimestamp = &metav1.Time{Time: time.Now()}
	}
	opsRequest.Status.Phase = phase
	return opsRes.Client.Status().Patch(opsRes.Ctx, opsRequest, patch)
}

func PatchClusterNotFound(opsRes *OpsResource) error {
	message := fmt.Sprintf("spec.clusterRef %s is not Found", opsRes.OpsRequest.Spec.ClusterRef)
	condition := dbaasv1alpha1.NewValidateFailedCondition(dbaasv1alpha1.ReasonClusterNotFound, message)
	return PatchOpsStatus(opsRes, dbaasv1alpha1.FailedPhase, condition)
}

func PatchOpsDefinitionUnavailable(opsRes *OpsResource, opsDefName string) error {
	message := fmt.Sprintf("OpsDefinition: %s is unavailable for sepc.type: %s", opsDefName, opsRes.OpsRequest.Spec.Type)
	condition := dbaasv1alpha1.NewValidateFailedCondition(dbaasv1alpha1.ReasonOpsDefinitionUnavailable, message)
	return PatchOpsStatus(opsRes, dbaasv1alpha1.FailedPhase, condition)
}

func PatchOpsDefinitionNotFound(opsRes *OpsResource) error {
	message := fmt.Sprintf("spec.type %s is not supported for spec.clusterRef %s", opsRes.OpsRequest.Spec.Type, opsRes.Cluster.Name)
	condition := dbaasv1alpha1.NewValidateFailedCondition(dbaasv1alpha1.ReasonOpsTypeNotSupported, message)
	return PatchOpsStatus(opsRes, dbaasv1alpha1.FailedPhase, condition)
}

func patchOpsBehaviourNotFound(opsRes *OpsResource) error {
	message := fmt.Sprintf("spec.type %s is not supported", opsRes.OpsRequest.Spec.Type)
	condition := dbaasv1alpha1.NewValidateFailedCondition(dbaasv1alpha1.ReasonOpsTypeNotSupported, message)
	return PatchOpsStatus(opsRes, dbaasv1alpha1.FailedPhase, condition)
}

func patchClusterPhaseMisMatch(opsRes *OpsResource) error {
	message := fmt.Sprintf("can not run the OpsRequest when Cluster.status.phase is %s in spec.clusterRef: %s",
		opsRes.Cluster.Status.Phase, opsRes.Cluster.Name)
	condition := dbaasv1alpha1.NewValidateFailedCondition(dbaasv1alpha1.ReasonClusterPhaseMisMatch, message)
	return PatchOpsStatus(opsRes, dbaasv1alpha1.FailedPhase, condition)
}

func patchClusterExistOtherOperation(opsRes *OpsResource, opsRequestName string) error {
	message := fmt.Sprintf("spec.clusterRef: %s is running the OpsRequest: %s",
		opsRes.Cluster.Name, opsRequestName)
	condition := dbaasv1alpha1.NewValidateFailedCondition(dbaasv1alpha1.ReasonClusterExistOtherOperation, message)
	return PatchOpsStatus(opsRes, dbaasv1alpha1.FailedPhase, condition)
}

func getOpsRequestAnnotation(cluster *dbaasv1alpha1.Cluster) *string {
	if cluster.Annotations == nil {
		return nil
	}
	if val, ok := cluster.Annotations[OpsRequestAnnotationKey]; ok {
		return &val
	}
	return nil
}

func patchOpsRequestToRunning(opsRes *OpsResource, opsBehaviour *OpsBehaviour) error {
	var condition *metav1.Condition
	validatePassCondition := dbaasv1alpha1.NewValidatePassedCondition(opsRes.OpsRequest.Name)
	if opsBehaviour.ActionStartedCondition != nil {
		condition = opsBehaviour.ActionStartedCondition(opsRes.OpsRequest)
	}
	return PatchOpsStatus(opsRes, dbaasv1alpha1.RunningPhase, validatePassCondition, condition)
}

// getComponentsNameMap covert spec.componentOps.componentNames list to map
func getComponentsNameMap(opsRequest *dbaasv1alpha1.OpsRequest) map[string]struct{} {
	componentOps := opsRequest.Spec.ComponentOps
	if componentOps == nil || componentOps.ComponentNames == nil {
		return map[string]struct{}{}
	}
	componentNameMap := make(map[string]struct{})
	for _, componentName := range componentOps.ComponentNames {
		componentNameMap[componentName] = struct{}{}
	}
	return componentNameMap
}

// patchClusterStatus update Cluster.status to record cluster and components information
func patchClusterStatus(opsRes *OpsResource, toClusterState dbaasv1alpha1.Phase) error {
	if toClusterState == "" {
		return nil
	}
	componentNameMap := getComponentsNameMap(opsRes.OpsRequest)
	patch := client.MergeFrom(opsRes.Cluster.DeepCopy())
	opsRes.Cluster.Status.Phase = toClusterState
	if componentNameMap != nil && opsRes.Cluster.Status.Components != nil {
		for k, v := range opsRes.Cluster.Status.Components {
			if _, ok := componentNameMap[k]; ok {
				v.Phase = toClusterState
			}
		}
	}
	if err := opsRes.Client.Status().Patch(opsRes.Ctx, opsRes.Cluster, patch); err != nil {
		return err
	}
	opsRes.Recorder.Eventf(opsRes.Cluster, corev1.EventTypeNormal, string(opsRes.OpsRequest.Spec.Type),
		"Start %s in Cluster: %s", opsRes.OpsRequest.Spec.Type, opsRes.Cluster.Name)
	return nil
}

// deleteOpsRequestAnnotationInCluster when OpsRequest.status.phase is Succeed or Failed
// we should delete the OpsRequest Annotation in cluster, unlock cluster
func deleteOpsRequestAnnotationInCluster(opsRes *OpsResource) error {
	if opsRes.Cluster.Annotations == nil {
		return nil
	}
	if val, ok := opsRes.Cluster.Annotations[OpsRequestAnnotationKey]; !ok || val != opsRes.OpsRequest.Name {
		return nil
	}
	patch := client.MergeFrom(opsRes.Cluster.DeepCopy())
	delete(opsRes.Cluster.Annotations, OpsRequestAnnotationKey)
	return opsRes.Client.Patch(opsRes.Ctx, opsRes.Cluster, patch)
}

// addOpsRequestAnnotationToCluster when OpsRequest.phase is Running, we should add the OpsRequest Annotation to Cluster.metadata.Annotations
func addOpsRequestAnnotationToCluster(opsRes *OpsResource) error {
	patch := client.MergeFrom(opsRes.Cluster.DeepCopy())
	if opsRes.Cluster.Annotations == nil {
		opsRes.Cluster.Annotations = map[string]string{}
	}
	opsRes.Cluster.Annotations[OpsRequestAnnotationKey] = opsRes.OpsRequest.Name
	return opsRes.Client.Patch(opsRes.Ctx, opsRes.Cluster, patch)
}
