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

package apps

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"golang.org/x/exp/slices"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/controllers/apps/components"
	"github.com/apecloud/kubeblocks/controllers/apps/components/util"
	opsutil "github.com/apecloud/kubeblocks/controllers/apps/operations/util"
	"github.com/apecloud/kubeblocks/controllers/k8score"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
)

// postHandler defines the handler after patching cluster status.
type postHandler func(cluster *appsv1alpha1.Cluster) error

// clusterStatusHandler a cluster status handler which changes of Cluster.status will be patched uniformly by doChainClusterStatusHandler.
type clusterStatusHandler func(cluster *appsv1alpha1.Cluster) (bool, postHandler)

const (
	// EventTimeOut timeout of the event
	EventTimeOut = 30 * time.Second
)

// doChainClusterStatusHandler chain processing clusterStatusHandler.
func doChainClusterStatusHandler(ctx context.Context,
	cli client.Client,
	cluster *appsv1alpha1.Cluster,
	handlers ...clusterStatusHandler) error {
	patch := client.MergeFrom(cluster.DeepCopy())
	var (
		needPatchStatus bool
		postHandlers    = make([]func(cluster *appsv1alpha1.Cluster) error, 0, len(handlers))
	)
	for _, statusHandler := range handlers {
		needPatch, postFunc := statusHandler(cluster)
		if needPatch {
			needPatchStatus = true
		}
		if postFunc != nil {
			postHandlers = append(postHandlers, postFunc)
		}
	}
	if !needPatchStatus {
		return nil
	}
	if err := cli.Status().Patch(ctx, cluster, patch); err != nil {
		return err
	}
	// perform the handlers after patched the cluster status.
	for _, postFunc := range postHandlers {
		if err := postFunc(cluster); err != nil {
			return err
		}
	}
	return nil
}

// isTargetKindForEvent checks the event involve object is the target resources
func isTargetKindForEvent(event *corev1.Event) bool {
	return slices.Index([]string{intctrlutil.PodKind, intctrlutil.DeploymentKind, intctrlutil.StatefulSetKind}, event.InvolvedObject.Kind) != -1
}

// getFinalEventMessageForRecorder gets final event message by event involved object kind for recorded it
func getFinalEventMessageForRecorder(event *corev1.Event) string {
	if event.InvolvedObject.Kind == intctrlutil.PodKind {
		return fmt.Sprintf("Pod %s: %s", event.InvolvedObject.Name, event.Message)
	}
	return event.Message
}

// isExistsEventMsg checks whether the event is exists
func isExistsEventMsg(statusComponentMessage map[string]string, event *corev1.Event) bool {
	if statusComponentMessage == nil {
		return false
	}
	messageKey := util.GetStatusComponentMessageKey(event.InvolvedObject.Kind, event.InvolvedObject.Name)
	if message, ok := statusComponentMessage[messageKey]; !ok {
		return false
	} else {
		return strings.Contains(message, event.Message)
	}

}

// updateStatusComponentMessage updates status component message map
func updateStatusComponentMessage(statusComponent *appsv1alpha1.ClusterStatusComponent, event *corev1.Event) {
	var (
		kind = event.InvolvedObject.Kind
		name = event.InvolvedObject.Name
	)
	if statusComponent.Message == nil {
		statusComponent.Message = appsv1alpha1.ComponentMessageMap{}
		statusComponent.Message.SetObjectMessage(kind, name, event.Message)
		return
	}
	message := statusComponent.Message.GetObjectMessage(kind, name)
	// if the event message is not exists in message map, merge them.
	if !strings.Contains(message, event.Message) {
		message += event.Message + ";"
	}
	statusComponent.Message.SetObjectMessage(kind, name, message)
}

// needSyncComponentStatusForEvent checks whether the component status needs to be synchronized the cluster status by event
func needSyncComponentStatusForEvent(cluster *appsv1alpha1.Cluster, componentName string, phase appsv1alpha1.Phase, event *corev1.Event) bool {
	var (
		status          = &cluster.Status
		statusComponent appsv1alpha1.ClusterStatusComponent
		ok              bool
	)
	if phase == "" {
		return false
	}
	if cluster.Status.Components == nil {
		status.Components = map[string]appsv1alpha1.ClusterStatusComponent{}
	}
	if statusComponent, ok = cluster.Status.Components[componentName]; !ok {
		statusComponent = appsv1alpha1.ClusterStatusComponent{Phase: phase}
		updateStatusComponentMessage(&statusComponent, event)
		status.Components[componentName] = statusComponent
		return true
	}
	if statusComponent.Phase != phase {
		statusComponent.Phase = phase
		updateStatusComponentMessage(&statusComponent, event)
		status.Components[componentName] = statusComponent
		return true
	}
	// check whether it is a new warning event and the component phase is running
	if !isExistsEventMsg(statusComponent.Message, event) && phase != appsv1alpha1.RunningPhase {
		updateStatusComponentMessage(&statusComponent, event)
		status.Components[componentName] = statusComponent
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
	case intctrlutil.PodKind:
		pod := &corev1.Pod{}
		err = cli.Get(ctx, objectKey, pod)
		return pod, err
	case intctrlutil.StatefulSetKind:
		sts := &appsv1.StatefulSet{}
		err = cli.Get(ctx, objectKey, sts)
		return sts, err
	case intctrlutil.DeploymentKind:
		deployment := &appsv1.Deployment{}
		err = cli.Get(ctx, objectKey, deployment)
		return deployment, err
	}
	return nil, err
}

// handleClusterAbnormalOrFailedPhase handles the Cluster.status.phase when components phase of cluster are Abnormal or Failed.
func handleClusterAbnormalOrFailedPhase(cluster *appsv1alpha1.Cluster, componentMap map[string]string, clusterAvailabilityMap map[string]bool) {
	var (
		isFailed                       bool
		needSyncClusterPhase           = true
		replicasNotReadyComponentNames = map[string]struct{}{}
		notReadyComponentNames         = map[string]struct{}{}
	)
	for k, v := range cluster.Status.Components {
		componentType := componentMap[k]
		clusterAvailabilityEffect := clusterAvailabilityMap[componentType]
		// if the component is in Failed phase and the component can affect cluster availability, set Cluster.status.phase to Failed
		if clusterAvailabilityEffect && v.Phase == appsv1alpha1.FailedPhase {
			isFailed = true
		}
		// determine whether other components are still doing operation, i.e., create/restart/scaling.
		// if exists operations, it will be handled by cluster controller to sync Cluster.status.phase.
		// but volumeExpansion operation is ignored, because this operation will not affect cluster availability.
		if !util.IsCompleted(v.Phase) && v.Phase != appsv1alpha1.VolumeExpandingPhase {
			needSyncClusterPhase = false
		}
		if v.PodsReady == nil || !*v.PodsReady {
			replicasNotReadyComponentNames[k] = struct{}{}
			notReadyComponentNames[k] = struct{}{}
		}
		if util.IsFailedOrAbnormal(v.Phase) {
			notReadyComponentNames[k] = struct{}{}
		}
	}
	// record the not ready conditions in cluster
	if len(replicasNotReadyComponentNames) > 0 {
		cluster.SetStatusCondition(newReplicasNotReadyCondition(replicasNotReadyComponentNames))
	}
	// record the not ready conditions in cluster
	if len(notReadyComponentNames) > 0 {
		cluster.SetStatusCondition(newComponentsNotReadyCondition(notReadyComponentNames))
	}
	if !needSyncClusterPhase {
		return
	}
	// if the cluster is not in Failed phase, set Cluster.status.phase to Abnormal
	if isFailed {
		cluster.Status.Phase = appsv1alpha1.FailedPhase
	} else if cluster.Status.Phase != appsv1alpha1.FailedPhase {
		cluster.Status.Phase = appsv1alpha1.AbnormalPhase
	}
}

// getClusterAvailabilityEffect whether the component will affect the cluster availability.
// if the component can affect and be Failed, the cluster will be Failed too.
func getClusterAvailabilityEffect(componentDef *appsv1alpha1.ClusterDefinitionComponent) bool {
	switch componentDef.WorkloadType {
	case appsv1alpha1.Consensus:
		return true
	case appsv1alpha1.Replication:
		return true
	default:
		// other types of components need to judge whether there has PodDisruptionBudget
		return intctrlutil.ExistsPDBSpec(componentDef.PDBSpec)
	}
}

// getComponentRelatedInfo gets componentMap, clusterAvailabilityMap and component definition information
func getComponentRelatedInfo(cluster *appsv1alpha1.Cluster, clusterDef *appsv1alpha1.ClusterDefinition, componentName string) (map[string]string, map[string]bool, appsv1alpha1.ClusterDefinitionComponent) {
	var (
		typeName     string
		componentMap = map[string]string{}
		componentDef appsv1alpha1.ClusterDefinitionComponent
	)
	for _, v := range cluster.Spec.ComponentSpecs {
		if v.Name == componentName {
			typeName = v.ComponentDefRef
		}
		componentMap[v.Name] = v.ComponentDefRef
	}
	clusterAvailabilityEffectMap := map[string]bool{}
	for _, v := range clusterDef.Spec.ComponentDefs {
		clusterAvailabilityEffectMap[v.Name] = getClusterAvailabilityEffect(&v)
		if v.Name == typeName {
			componentDef = v
		}
	}
	return componentMap, clusterAvailabilityEffectMap, componentDef
}

// handleClusterStatusByEvent handles the cluster status when warning event happened
func handleClusterStatusByEvent(ctx context.Context, cli client.Client, recorder record.EventRecorder, event *corev1.Event) error {
	var (
		cluster    = &appsv1alpha1.Cluster{}
		clusterDef = &appsv1alpha1.ClusterDefinition{}
		phase      appsv1alpha1.Phase
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
	clusterComponent := cluster.GetComponentByName(componentName)
	// get the component status by event and check whether the component status needs to be synchronized to the cluster
	component := components.NewComponentByType(ctx, cli, cluster, &componentDef, clusterComponent)
	if component == nil {
		return nil
	}
	phase, err = component.GetPhaseWhenPodsNotReady(componentName)
	if err != nil {
		return err
	}
	if !needSyncComponentStatusForEvent(cluster, componentName, phase, event) {
		return nil
	}
	// handle Cluster.status.phase
	handleClusterAbnormalOrFailedPhase(cluster, componentMap, clusterAvailabilityEffectMap)
	if err = cli.Status().Patch(ctx, cluster, patch); err != nil {
		return err
	}
	recorder.Eventf(cluster, corev1.EventTypeWarning, event.Reason, getFinalEventMessageForRecorder(event))
	return opsutil.MarkRunningOpsRequestAnnotation(ctx, cli, cluster)
}

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
				return event.InvolvedObject.Kind == intctrlutil.CronJob &&
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
	if err = cli.Get(ctx, client.ObjectKey{Name: labels[intctrlutil.AppInstanceLabelKey],
		Namespace: object.GetNamespace()}, &cluster); err != nil {
		return err
	}
	componentName := labels[intctrlutil.AppComponentLabelKey]
	// update component phase to abnormal
	if err = updateComponentStatusPhase(cli,
		ctx,
		&cluster,
		componentName,
		appsv1alpha1.AbnormalPhase,
		event.Message,
		object); err != nil {
		return err
	}
	recorder.Eventf(&cluster, corev1.EventTypeWarning, event.Reason, event.Message)
	return nil
}

func checkedDeleteDeletePVCCronJob(ctx context.Context, cli client.Client, name string, namespace string) error {
	// label check
	cronJob := v1.CronJob{}
	if err := cli.Get(ctx, types.NamespacedName{
		Namespace: namespace,
		Name:      name,
	}, &cronJob); err != nil {
		return client.IgnoreNotFound(err)
	}
	if cronJob.ObjectMeta.Labels[intctrlutil.AppManagedByLabelKey] != intctrlutil.AppName {
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
	phase appsv1alpha1.Phase,
	message string,
	object client.Object) error {
	c, ok := cluster.Status.Components[componentName]
	if ok && c.Phase == phase {
		return nil
	}
	// c.Message can be nil
	if c.Message == nil {
		c.Message = appsv1alpha1.ComponentMessageMap{}
	}
	c.Message.SetObjectMessage(object.GetObjectKind().GroupVersionKind().Kind, object.GetName(), message)
	patch := client.MergeFrom(cluster.DeepCopy())
	cluster.Status.Components[componentName] = c
	return cli.Status().Patch(ctx, cluster, patch)
}

// syncComponentPhaseWhenSpecUpdating when workload of the component changed
// and component phase is not the phase of operations, sync component phase to 'SpecUpdating'.
func syncComponentPhaseWhenSpecUpdating(cluster *appsv1alpha1.Cluster,
	componentName string) {
	if len(componentName) == 0 {
		return
	}
	if cluster.Status.Components == nil {
		cluster.Status.Components = map[string]appsv1alpha1.ClusterStatusComponent{
			componentName: {
				Phase: appsv1alpha1.SpecUpdatingPhase,
			},
		}
		return
	}
	statusComponent := cluster.Status.Components[componentName]
	// if component phase is not the phase of operations, sync component phase to 'SpecUpdating'
	if util.IsCompleted(statusComponent.Phase) {
		statusComponent.Phase = appsv1alpha1.SpecUpdatingPhase
		cluster.Status.Components[componentName] = statusComponent
	}
}
