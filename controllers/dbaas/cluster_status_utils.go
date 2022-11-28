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

package dbaas

import (
	"context"
	"fmt"
	"strings"
	"time"

	"golang.org/x/exp/slices"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"

	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
	"github.com/apecloud/kubeblocks/controllers/dbaas/component"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
)

// isTargetKindForEvent check the event involve object is the target resources
func isTargetKindForEvent(event *corev1.Event) bool {
	return slices.Index([]string{PodKind, DeploymentKind, StatefulSetKind}, event.InvolvedObject.Kind) != -1
}

// isOvertimeAndOccursTimesForEvent check whether the duration and number of events reach the threshold
func isOvertimeAndOccursTimesForEvent(event *corev1.Event) bool {
	var (
		intervalTime           = 30 * time.Second
		occursEventTimes int32 = 3
	)
	return event.LastTimestamp.After(event.FirstTimestamp.Add(intervalTime)) &&
		event.Count >= occursEventTimes
}

// isOperationsPhaseForCluster determine whether operations are in progress according to the cluster status
func isOperationsPhaseForCluster(phase dbaasv1alpha1.Phase) bool {
	return slices.Index([]dbaasv1alpha1.Phase{dbaasv1alpha1.CreatingPhase, dbaasv1alpha1.UpdatingPhase}, phase) != -1
}

func calculateComponentPhaseForEvent(isFailed, isWarning bool) dbaasv1alpha1.Phase {
	var componentPhase dbaasv1alpha1.Phase
	// if leader is ready, set component phase to Warning
	if isFailed {
		componentPhase = dbaasv1alpha1.FailedPhase
	} else if isWarning {
		componentPhase = dbaasv1alpha1.AbnormalPhase
	}
	return componentPhase
}

// checkRelatedPodIsTerminating check related pods is terminating for Stateless/Stateful
func checkRelatedPodIsTerminating(ctx context.Context, cli client.Client, cluster *dbaasv1alpha1.Cluster, componentName string) (bool, error) {
	podList := &corev1.PodList{}
	if err := cli.List(ctx, podList, client.InNamespace(cluster.Namespace),
		getComponentMatchLabels(cluster.Name, componentName)); err != nil {
		return false, err
	}
	for _, v := range podList.Items {
		// if the pod is terminating, ignore the warning event
		if v.DeletionTimestamp != nil {
			return true, nil
		}
	}
	return false, nil
}

// getStatefulPhaseForEvent get the component phase for stateful type
func getStatefulPhaseForEvent(ctx context.Context, cli client.Client, cluster *dbaasv1alpha1.Cluster, componentName string) (dbaasv1alpha1.Phase, error) {
	var (
		isFailed          = true
		isWarning         bool
		stsList           = &appsv1.StatefulSetList{}
		podsIsTerminating bool
		err               error
	)
	if podsIsTerminating, err = checkRelatedPodIsTerminating(ctx, cli, cluster, componentName); err != nil || podsIsTerminating {
		return "", err
	}
	if err = getObjectListByComponentName(ctx, cli, cluster, stsList, componentName); err != nil {
		return "", err
	}
	for _, v := range stsList.Items {
		if v.Status.AvailableReplicas < 1 {
			continue
		}
		isFailed = false
		if v.Status.AvailableReplicas < *v.Spec.Replicas {
			isWarning = true
		}
	}
	return calculateComponentPhaseForEvent(isFailed, isWarning), nil
}

// getStatelessPhaseForEvent get the component phase for stateless type
func getStatelessPhaseForEvent(ctx context.Context, cli client.Client, cluster *dbaasv1alpha1.Cluster, componentName string) (dbaasv1alpha1.Phase, error) {
	var (
		isFailed          = true
		isWarning         bool
		deployList        = &appsv1.DeploymentList{}
		podsIsTerminating bool
		err               error
	)
	if podsIsTerminating, err = checkRelatedPodIsTerminating(ctx, cli, cluster, componentName); err != nil || podsIsTerminating {
		return "", err
	}
	if err = getObjectListByComponentName(ctx, cli, cluster, deployList, componentName); err != nil {
		return "", err
	}
	for _, v := range deployList.Items {
		if v.Status.AvailableReplicas < 1 {
			continue
		}
		isFailed = false
		if v.Status.AvailableReplicas < *v.Spec.Replicas {
			isWarning = true
		}
	}
	return calculateComponentPhaseForEvent(isFailed, isWarning), nil
}

// getConsensusPhaseForEvent get the component phase for consensus type
func getConsensusPhaseForEvent(ctx context.Context, cli client.Client, cluster *dbaasv1alpha1.Cluster, componentDef *dbaasv1alpha1.ClusterDefinitionComponent, componentName string) (dbaasv1alpha1.Phase, error) {
	var (
		isFailed      = true
		isWarning     bool
		podList       = &corev1.PodList{}
		allPodIsReady = true
	)
	if err := cli.List(ctx, podList, client.InNamespace(cluster.Namespace),
		getComponentMatchLabels(cluster.Name, componentName)); err != nil {
		return "", err
	}
	if len(podList.Items) == 0 {
		return dbaasv1alpha1.FailedPhase, nil
	}
	for _, v := range podList.Items {
		// if the pod is terminating, ignore the warning event
		if v.DeletionTimestamp != nil {
			return "", nil
		}
		labelValue := v.Labels[intctrlutil.ConsensusSetRoleLabelKey]
		if labelValue == componentDef.ConsensusSpec.Leader.Name {
			isFailed = false
		}
		// if no role label, the pod is not ready
		if labelValue == "" {
			isWarning = true
		}
		if !intctrlutil.PodIsReady(&v) {
			allPodIsReady = false
		}
	}
	// if all pod is ready, ignore the warning event
	if allPodIsReady {
		return "", nil
	}
	return calculateComponentPhaseForEvent(isFailed, isWarning), nil
}

// getFinalEventMessageForRecorder get final event message by event involved object kind for recorded it
func getFinalEventMessageForRecorder(event *corev1.Event) string {
	if event.InvolvedObject.Kind == PodKind {
		return fmt.Sprintf("Pod %s: %s", event.InvolvedObject.Name, event.Message)
	}
	return event.Message
}

// getStatusComponentMessage get component status message
func getStatusComponentMessage(statusComponentMessage string, event *corev1.Event) string {
	message := event.Message
	if event.InvolvedObject.Kind == PodKind {
		message = mergePodEventMessage(statusComponentMessage, event)
	}
	return message
}

// mergePodEventMessage merge pod event message to component message
func mergePodEventMessage(statusComponentMessage string, event *corev1.Event) string {
	var (
		startSign = "Pods"
		errorSign = "error occurred"
		podName   = event.InvolvedObject.Name
		message   string
	)
	msgList := strings.Split(statusComponentMessage, ":")
	// check whether the component message is merged
	if len(msgList) > 2 && msgList[0] == startSign && strings.Contains(msgList[1], errorSign) {
		podsInfo := strings.Replace(msgList[1], errorSign, "", 1)
		podList := strings.Split(strings.TrimSpace(podsInfo), ",")
		if !slices.Contains(podList, event.InvolvedObject.Name) {
			podList = append(podList, event.InvolvedObject.Name)
		}
		msg := msgList[2]
		if !strings.Contains(msg, event.Message) {
			msg += ";" + event.Message
		}
		message = fmt.Sprintf("%s: %s %s: %s", startSign, strings.Join(podList, ","), errorSign, msg)
	} else {
		message = fmt.Sprintf("%s: %s %s: %s", startSign, podName, errorSign, event.Message)
	}
	return message
}

// isExistsEventMsg check whether the event is exists
func isExistsEventMsg(statusComponentMessage string, event *corev1.Event) bool {
	isExists := strings.Contains(statusComponentMessage, event.Message)
	// if involved object kind is Pod, we should check whether the pod name has merged into the component status message
	if event.InvolvedObject.Kind == PodKind {
		return isExists && strings.Contains(statusComponentMessage, event.InvolvedObject.Name)
	}
	return isExists
}

// needSyncComponentStatusForEvent check whether the component status needs to be synchronized the cluster status by event
func needSyncComponentStatusForEvent(cluster *dbaasv1alpha1.Cluster, componentName string, phase dbaasv1alpha1.Phase, event *corev1.Event) bool {
	var (
		statusComponent dbaasv1alpha1.ClusterStatusComponent
		ok              bool
	)
	if phase == "" {
		return false
	}
	if cluster.Status.Components == nil {
		cluster.Status.Components = map[string]dbaasv1alpha1.ClusterStatusComponent{}
	}
	if statusComponent, ok = cluster.Status.Components[componentName]; !ok {
		cluster.Status.Components[componentName] = dbaasv1alpha1.ClusterStatusComponent{Phase: phase, Message: event.Message}
		return true
	}
	if statusComponent.Phase != phase {
		statusComponent.Phase = phase
		statusComponent.Message = getStatusComponentMessage(statusComponent.Message, event)
		return true
	}
	// check whether it is a new warning event and the component phase is running
	if !isExistsEventMsg(statusComponent.Message, event) && phase != dbaasv1alpha1.RunningPhase {
		statusComponent.Message = getStatusComponentMessage(statusComponent.Message, event)
		return true
	}
	return false
}

// getEventInvolvedObject get event involved object for StatefulSet/Deployment/Pod workload
func getEventInvolvedObject(ctx context.Context, cli client.Client, event *corev1.Event) (client.Object, error) {
	objectKey := client.ObjectKey{
		Name:      event.InvolvedObject.Name,
		Namespace: event.InvolvedObject.Namespace,
	}
	var err error
	switch event.InvolvedObject.Kind {
	case PodKind:
		pod := &corev1.Pod{}
		err = cli.Get(ctx, objectKey, pod)
		return pod, err
	case StatefulSetKind:
		sts := &appsv1.StatefulSet{}
		err = cli.Get(ctx, objectKey, sts)
		return sts, err
	case DeploymentKind:
		deployment := &appsv1.Deployment{}
		err = cli.Get(ctx, objectKey, deployment)
		return deployment, err
	}
	return nil, err
}

// handleClusterStatusPhaseByEvent handle the Cluster.status.phase when warning event happened.
func handleClusterStatusPhaseByEvent(cluster *dbaasv1alpha1.Cluster, componentMap map[string]string, clusterAvailabilityMap map[string]bool) {
	var (
		isFailed             bool
		needSyncClusterPhase = true
	)
	for k, v := range cluster.Status.Components {
		componentType := componentMap[k]
		clusterAvailabilityEffect := clusterAvailabilityMap[componentType]
		// if the component is in Failed phase and the component can affect cluster availability, set Cluster.status.phase to Failed
		if clusterAvailabilityEffect && v.Phase == dbaasv1alpha1.FailedPhase {
			isFailed = true
		}
		// determine whether other components are still operation, i.e., create/restart/scaling
		if isOperationsPhaseForCluster(v.Phase) {
			needSyncClusterPhase = false
		}
	}
	if !needSyncClusterPhase {
		return
	}
	// if the cluster is not in Failed phase, set Cluster.status.phase to Warning
	if isFailed {
		cluster.Status.Phase = dbaasv1alpha1.FailedPhase
	} else if cluster.Status.Phase != dbaasv1alpha1.FailedPhase {
		cluster.Status.Phase = dbaasv1alpha1.AbnormalPhase
	}
}

// getClusterAvailabilityEffect whether the component will affect the cluster availability.
// if the component can affect and be Failed, the cluster will be Failed too.
func getClusterAvailabilityEffect(componentDef *dbaasv1alpha1.ClusterDefinitionComponent) bool {
	switch componentDef.ComponentType {
	case dbaasv1alpha1.Consensus:
		return true
	default:
		// other types of components need to judge whether there has PodDisruptionBudget
		return existsPDBSpec(componentDef.PDBSpec)
	}
}

// getComponentRelatedInfo get componentMap, clusterAvailabilityMap and component definition information
func getComponentRelatedInfo(cluster *dbaasv1alpha1.Cluster, clusterDef *dbaasv1alpha1.ClusterDefinition, componentName string) (map[string]string, map[string]bool, dbaasv1alpha1.ClusterDefinitionComponent) {
	var (
		typeName     string
		componentMap = map[string]string{}
		componentDef dbaasv1alpha1.ClusterDefinitionComponent
	)
	for _, v := range cluster.Spec.Components {
		if v.Name == componentName {
			typeName = v.Type
		}
		componentMap[v.Name] = v.Type
	}
	clusterAvailabilityEffectMap := map[string]bool{}
	for _, v := range clusterDef.Spec.Components {
		clusterAvailabilityEffectMap[v.TypeName] = getClusterAvailabilityEffect(&v)
		if v.TypeName == typeName {
			componentDef = v
		}
	}
	return componentMap, clusterAvailabilityEffectMap, componentDef
}

// handleClusterStatusByEvent handle the cluster status when warning event happened
func handleClusterStatusByEvent(ctx context.Context, cli client.Client, recorder record.EventRecorder, object client.Object, event *corev1.Event) error {
	var (
		cluster    = &dbaasv1alpha1.Cluster{}
		labels     = object.GetLabels()
		clusterDef = &dbaasv1alpha1.ClusterDefinition{}
		phase      dbaasv1alpha1.Phase
		err        error
	)
	if err = cli.Get(ctx, client.ObjectKey{Name: labels[intctrlutil.AppInstanceLabelKey], Namespace: object.GetNamespace()}, cluster); err != nil {
		return err
	}
	if err = cli.Get(ctx, client.ObjectKey{Name: cluster.Spec.ClusterDefRef}, clusterDef); err != nil {
		return err
	}
	componentName := labels[intctrlutil.AppComponentLabelKey]
	// get the component phase by component type and sync to Cluster.status.components
	patch := client.MergeFrom(cluster.DeepCopy())
	componentMap, clusterAvailabilityEffectMap, componentDef := getComponentRelatedInfo(cluster, clusterDef, componentName)
	// get the component status by event and check whether the component status needs to be synchronized to the cluster
	switch componentDef.ComponentType {
	case dbaasv1alpha1.Consensus:
		phase, err = getConsensusPhaseForEvent(ctx, cli, cluster, &componentDef, componentName)
	case dbaasv1alpha1.Stateful:
		phase, err = getStatefulPhaseForEvent(ctx, cli, cluster, componentName)
	case dbaasv1alpha1.Stateless:
		phase, err = getStatelessPhaseForEvent(ctx, cli, cluster, componentName)
	}
	if err != nil {
		return err
	}
	if !needSyncComponentStatusForEvent(cluster, componentName, phase, event) {
		return nil
	}
	// handle Cluster.status.phase
	handleClusterStatusPhaseByEvent(cluster, componentMap, clusterAvailabilityEffectMap)
	if err = cli.Status().Patch(ctx, cluster, patch); err != nil {
		return err
	}
	recorder.Eventf(cluster, corev1.EventTypeWarning, event.Reason, getFinalEventMessageForRecorder(event))
	return component.MarkRunningOpsRequestAnnotation(ctx, cli, cluster)
}

// handleEventForClusterStatus handle event for cluster Warning and Failed phase
func handleEventForClusterStatus(ctx context.Context, cli client.Client, recorder record.EventRecorder, event *corev1.Event) error {
	var (
		err    error
		object client.Object
	)
	if event.Type != corev1.EventTypeWarning || !isTargetKindForEvent(event) {
		return nil
	}
	if !isOvertimeAndOccursTimesForEvent(event) {
		return nil
	}
	if object, err = getEventInvolvedObject(ctx, cli, event); err != nil {
		return err
	}
	if object == nil || !component.WorkloadFilterPredicate(object) {
		return nil
	}
	return handleClusterStatusByEvent(ctx, cli, recorder, object, event)
}
