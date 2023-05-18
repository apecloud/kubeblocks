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

package apps

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"golang.org/x/exp/slices"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/controllers/apps/components"
	"github.com/apecloud/kubeblocks/controllers/apps/components/util"
	opsutil "github.com/apecloud/kubeblocks/controllers/apps/operations/util"
	"github.com/apecloud/kubeblocks/controllers/k8score"
	"github.com/apecloud/kubeblocks/internal/constant"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
)

// EventTimeOut timeout of the event
const EventTimeOut = 30 * time.Second

// isTargetKindForEvent checks the event involve object is the target resources
func isTargetKindForEvent(event *corev1.Event) bool {
	return slices.Index([]string{constant.PodKind, constant.DeploymentKind, constant.StatefulSetKind}, event.InvolvedObject.Kind) != -1
}

// getFinalEventMessageForRecorder gets final event message by event involved object kind for recorded it
func getFinalEventMessageForRecorder(event *corev1.Event) string {
	if event.InvolvedObject.Kind == constant.PodKind {
		return fmt.Sprintf("Pod %s: %s", event.InvolvedObject.Name, event.Message)
	}
	return event.Message
}

// isExistsEventMsg checks whether the event is exists
func isExistsEventMsg(compStatusMessage map[string]string, event *corev1.Event) bool {
	if compStatusMessage == nil {
		return false
	}
	messageKey := util.GetComponentStatusMessageKey(event.InvolvedObject.Kind, event.InvolvedObject.Name)
	if message, ok := compStatusMessage[messageKey]; !ok {
		return false
	} else {
		return strings.Contains(message, event.Message)
	}

}

// updateComponentStatusMessage updates component status message map
func updateComponentStatusMessage(cluster *appsv1alpha1.Cluster,
	compName string,
	compStatus *appsv1alpha1.ClusterComponentStatus,
	event *corev1.Event) {
	var (
		kind = event.InvolvedObject.Kind
		name = event.InvolvedObject.Name
	)
	message := compStatus.GetObjectMessage(kind, name)
	// if the event message is not exists in message map, merge them.
	if !strings.Contains(message, event.Message) {
		message += event.Message + ";"
	}
	compStatus.SetObjectMessage(kind, name, message)
	cluster.Status.SetComponentStatus(compName, *compStatus)
}

// needSyncComponentStatusForEvent checks whether the component status needs to be synchronized the cluster status by event
func needSyncComponentStatusForEvent(cluster *appsv1alpha1.Cluster, componentName string, phase appsv1alpha1.ClusterComponentPhase, event *corev1.Event) bool {
	if phase == "" {
		return false
	}
	compStatus, ok := cluster.Status.Components[componentName]
	if !ok {
		compStatus = appsv1alpha1.ClusterComponentStatus{Phase: phase}
		updateComponentStatusMessage(cluster, componentName, &compStatus, event)
		return true
	}
	if compStatus.Phase != phase {
		compStatus.Phase = phase
		updateComponentStatusMessage(cluster, componentName, &compStatus, event)
		return true
	}
	// check whether it is a new warning event and the component phase is running
	if !isExistsEventMsg(compStatus.Message, event) && phase != appsv1alpha1.RunningClusterCompPhase {
		updateComponentStatusMessage(cluster, componentName, &compStatus, event)
		return true
	}
	return false
}

// getEventInvolvedObject gets event involved object for StatefulSet/Deployment/Pod workload
func getEventInvolvedObject(ctx context.Context, cli client.Client, event *corev1.Event) (client.Object, error) {
	objectKey := client.ObjectKey{
		Name:      event.InvolvedObject.Name,
		Namespace: event.InvolvedObject.Namespace,
	}
	var err error
	// If client.object interface object is used as a parameter, it will not return an error when the object is not found.
	// so we should specify the object type to get the object.
	switch event.InvolvedObject.Kind {
	case constant.PodKind:
		pod := &corev1.Pod{}
		err = cli.Get(ctx, objectKey, pod)
		return pod, err
	case constant.StatefulSetKind:
		sts := &appsv1.StatefulSet{}
		err = cli.Get(ctx, objectKey, sts)
		return sts, err
	case constant.DeploymentKind:
		deployment := &appsv1.Deployment{}
		err = cli.Get(ctx, objectKey, deployment)
		return deployment, err
	}
	return nil, err
}

// handleClusterPhaseWhenCompsNotReady handles the Cluster.status.phase when some components are Abnormal or Failed.
// TODO: Clear definitions need to be added to determine whether components will affect cluster availability in ClusterDefinition.
func handleClusterPhaseWhenCompsNotReady(cluster *appsv1alpha1.Cluster,
	componentMap map[string]string,
	clusterAvailabilityEffectMap map[string]bool) {
	var (
		clusterIsFailed   bool
		failedCompCount   int
		isVolumeExpanding bool
	)
	opsRecords, _ := opsutil.GetOpsRequestSliceFromCluster(cluster)
	if len(opsRecords) != 0 && opsRecords[0].Type == appsv1alpha1.VolumeExpansionType {
		isVolumeExpanding = true
	}
	for k, v := range cluster.Status.Components {
		// determine whether other components are still doing operation, i.e., create/restart/scaling.
		// waiting for operation to complete except for volumeExpansion operation.
		// because this operation will not affect cluster availability.
		if !slices.Contains(appsv1alpha1.GetComponentTerminalPhases(), v.Phase) && !isVolumeExpanding {
			return
		}
		if v.Phase == appsv1alpha1.FailedClusterCompPhase {
			failedCompCount += 1
			componentDefName := componentMap[k]
			// if the component can affect cluster availability, set Cluster.status.phase to Failed
			if clusterAvailabilityEffectMap[componentDefName] {
				clusterIsFailed = true
				break
			}
		}
	}
	// If all components fail or there are failed components that affect the availability of the cluster, set phase to Failed
	if failedCompCount == len(cluster.Status.Components) || clusterIsFailed {
		cluster.Status.Phase = appsv1alpha1.FailedClusterPhase
	} else {
		cluster.Status.Phase = appsv1alpha1.AbnormalClusterPhase
	}
}

// getComponentRelatedInfo gets componentMap, clusterAvailabilityMap and component definition information
func getComponentRelatedInfo(cluster *appsv1alpha1.Cluster, clusterDef *appsv1alpha1.ClusterDefinition,
	componentName string) (map[string]string, map[string]bool, *appsv1alpha1.ClusterComponentDefinition, error) {
	var (
		compDefName  string
		componentMap = map[string]string{}
		componentDef *appsv1alpha1.ClusterComponentDefinition
	)
	for _, v := range cluster.Spec.ComponentSpecs {
		componentMap[v.Name] = v.ComponentDefRef
		if compDefName == "" && v.Name == componentName {
			compDefName = v.ComponentDefRef
		}
	}
	if compDefName == "" {
		return nil, nil, nil, fmt.Errorf("expected %s component not found", componentName)
	}
	clusterAvailabilityEffectMap := map[string]bool{}
	for i, v := range clusterDef.Spec.ComponentDefs {
		clusterAvailabilityEffectMap[v.Name] = componentDef.GetMaxUnavailable() != nil
		if componentDef == nil && v.Name == compDefName {
			componentDef = &clusterDef.Spec.ComponentDefs[i]
		}
	}
	if componentDef == nil {
		return nil, nil, nil, fmt.Errorf("expected %s componentDef not found", compDefName)
	}
	return componentMap, clusterAvailabilityEffectMap, componentDef, nil
}

// handleClusterStatusByEvent handles the cluster status when warning event happened
func handleClusterStatusByEvent(ctx context.Context, cli client.Client, recorder record.EventRecorder, event *corev1.Event) error {
	var (
		cluster    = &appsv1alpha1.Cluster{}
		clusterDef = &appsv1alpha1.ClusterDefinition{}
		phase      appsv1alpha1.ClusterComponentPhase
		err        error
	)
	object, err := getEventInvolvedObject(ctx, cli, event)
	if err != nil {
		return err
	}
	if object == nil || !intctrlutil.WorkloadFilterPredicate(object) {
		return nil
	}
	labels := object.GetLabels()
	if err = cli.Get(ctx, client.ObjectKey{Name: labels[constant.AppInstanceLabelKey], Namespace: object.GetNamespace()}, cluster); err != nil {
		return err
	}
	if err = cli.Get(ctx, client.ObjectKey{Name: cluster.Spec.ClusterDefRef}, clusterDef); err != nil {
		return err
	}
	componentName := labels[constant.KBAppComponentLabelKey]
	// get the component phase by component name and sync to Cluster.status.components
	patch := client.MergeFrom(cluster.DeepCopy())
	clusterComponent := cluster.Spec.GetComponentByName(componentName)
	if clusterComponent == nil {
		return nil
	}
	componentMap, clusterAvailabilityEffectMap, componentDef, err := getComponentRelatedInfo(cluster, clusterDef, componentName)
	if err != nil {
		return err
	}
	// get the component status by event and check whether the component status needs to be synchronized to the cluster
	component, err := components.NewComponentByType(cli, cluster, clusterComponent, *componentDef)
	if err != nil {
		return err
	}
	phase, err = component.GetPhaseWhenPodsNotReady(ctx, componentName)
	if err != nil {
		return err
	}
	if !needSyncComponentStatusForEvent(cluster, componentName, phase, event) {
		return nil
	}
	// handle Cluster.status.phase when some components are not ready.
	handleClusterPhaseWhenCompsNotReady(cluster, componentMap, clusterAvailabilityEffectMap)
	if err = cli.Status().Patch(ctx, cluster, patch); err != nil {
		return err
	}
	recorder.Eventf(cluster, corev1.EventTypeWarning, event.Reason, getFinalEventMessageForRecorder(event))
	return opsutil.MarkRunningOpsRequestAnnotation(ctx, cli, cluster)
}

// TODO: Unified cluster event processing
// handleEventForClusterStatus handles event for cluster Warning and Failed phase
func handleEventForClusterStatus(ctx context.Context, cli client.Client, recorder record.EventRecorder, event *corev1.Event) error {
	type predicateProcessor struct {
		pred      func() bool
		processor func() error
	}
	nilReturnHandler := func() error { return nil }
	pps := []predicateProcessor{
		{
			// handle cronjob complete or fail event
			pred: func() bool {
				return event.InvolvedObject.Kind == constant.CronJobKind &&
					event.Reason == "SawCompletedJob"
			},
			processor: func() error {
				return handleDeletePVCCronJobEvent(ctx, cli, recorder, event)
			},
		},
		{
			pred: func() bool {
				return event.Type != corev1.EventTypeWarning ||
					!isTargetKindForEvent(event)
			},
			processor: nilReturnHandler,
		},
		{
			pred: func() bool {
				// the error repeated several times, so we can sure it's a real error to the cluster.
				return !k8score.IsOvertimeEvent(event, EventTimeOut)
			},
			processor: nilReturnHandler,
		},
		{
			// handle cluster workload error events such as pod/statefulset/deployment errors
			// must be the last one
			pred: func() bool {
				return true
			},
			processor: func() error {
				return handleClusterStatusByEvent(ctx, cli, recorder, event)
			},
		},
	}
	for _, pp := range pps {
		if pp.pred() {
			return pp.processor()
		}
	}
	return nil
}

func handleDeletePVCCronJobEvent(ctx context.Context,
	cli client.Client,
	recorder record.EventRecorder,
	event *corev1.Event) error {
	re := regexp.MustCompile("status: Failed")
	var (
		err    error
		object client.Object
	)
	matches := re.FindStringSubmatch(event.Message)
	if len(matches) == 0 {
		// delete pvc success, then delete cronjob
		return checkedDeleteDeletePVCCronJob(ctx, cli, event.InvolvedObject.Name, event.InvolvedObject.Namespace)
	}
	// cronjob failed
	if object, err = getEventInvolvedObject(ctx, cli, event); err != nil {
		return err
	}
	if object == nil {
		return nil
	}
	labels := object.GetLabels()
	cluster := appsv1alpha1.Cluster{}
	if err = cli.Get(ctx, client.ObjectKey{Name: labels[constant.AppInstanceLabelKey],
		Namespace: object.GetNamespace()}, &cluster); err != nil {
		return err
	}
	componentName := labels[constant.KBAppComponentLabelKey]
	// update component phase to abnormal
	if err = updateComponentStatusPhase(cli,
		ctx,
		&cluster,
		componentName,
		appsv1alpha1.AbnormalClusterCompPhase,
		event.Message,
		object); err != nil {
		return err
	}
	recorder.Eventf(&cluster, corev1.EventTypeWarning, event.Reason, event.Message)
	return nil
}

func checkedDeleteDeletePVCCronJob(ctx context.Context, cli client.Client, name string, namespace string) error {
	// label check
	cronJob := batchv1.CronJob{}
	if err := cli.Get(ctx, types.NamespacedName{
		Namespace: namespace,
		Name:      name,
	}, &cronJob); err != nil {
		return client.IgnoreNotFound(err)
	}
	if cronJob.ObjectMeta.Labels[constant.AppManagedByLabelKey] != constant.AppName {
		return nil
	}
	// check the delete-pvc-cronjob annotation.
	// the reason for this is that the backup policy also creates cronjobs,
	// which need to be distinguished by the annotation.
	if cronJob.ObjectMeta.Annotations[lifecycleAnnotationKey] != lifecycleDeletePVCAnnotation {
		return nil
	}
	// if managed by kubeblocks, then it must be the cronjob used to delete pvc, delete it since it's completed
	if err := cli.Delete(ctx, &cronJob); err != nil {
		return client.IgnoreNotFound(err)
	}
	return nil
}

func updateComponentStatusPhase(cli client.Client,
	ctx context.Context,
	cluster *appsv1alpha1.Cluster,
	componentName string,
	phase appsv1alpha1.ClusterComponentPhase,
	message string,
	object client.Object) error {
	c, ok := cluster.Status.Components[componentName]
	if ok && c.Phase == phase {
		return nil
	}
	c.SetObjectMessage(object.GetObjectKind().GroupVersionKind().Kind, object.GetName(), message)
	patch := client.MergeFrom(cluster.DeepCopy())
	cluster.Status.SetComponentStatus(componentName, c)
	return cli.Status().Patch(ctx, cluster, patch)
}

// existsOperations checks if the cluster is doing operations
func existsOperations(cluster *appsv1alpha1.Cluster) bool {
	opsRequestMap, _ := opsutil.GetOpsRequestSliceFromCluster(cluster)
	_, isRestoring := cluster.Annotations[constant.RestoreFromBackUpAnnotationKey]
	return len(opsRequestMap) > 0 || isRestoring
}
